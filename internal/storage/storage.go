// Package storage 负责下载数据的落盘与断点续传状态的持久化。
//
// 目录布局：
//
//	<dir>/<name>                  数据文件（创建时预分配全长）
//	<dir>/.state/<infohash>.json  进度状态（位图 + 统计），下载完成后删除
package storage

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gotorrent/internal/bitfield"
	"gotorrent/internal/metainfo"
)

// State 是持久化到磁盘的下载进度。
type State struct {
	InfoHash   string `json:"info_hash"`
	Name       string `json:"name"`
	Length     int64  `json:"length"`
	Bitfield   string `json:"bitfield"` // base64 编码的位图
	Downloaded int64  `json:"downloaded"`
	Uploaded   int64  `json:"uploaded"`
}

// Store 管理一个种子的数据文件与进度状态。
type Store struct {
	mu   sync.RWMutex
	dir  string
	tf   *metainfo.TorrentFile
	file *os.File
	bf   bitfield.Bitfield

	Downloaded int64 // 累计下载字节（含续传前）
	Uploaded   int64 // 累计上传字节
}

// Open 打开（或创建）一个种子的存储。
// 若存在状态文件则恢复位图，实现断点续传。
func Open(dir string, tf *metainfo.TorrentFile) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(stateDir(dir), 0o755); err != nil {
		return nil, err
	}

	dataPath := filepath.Join(dir, sanitize(tf.Name))
	_, statErr := os.Stat(dataPath)
	existed := statErr == nil

	f, err := os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开数据文件失败: %w", err)
	}
	// 预分配全长，保证任意偏移写入安全
	if st, _ := f.Stat(); st.Size() != tf.Length {
		if err := f.Truncate(tf.Length); err != nil {
			f.Close()
			return nil, fmt.Errorf("预分配文件失败: %w", err)
		}
	}

	s := &Store{dir: dir, tf: tf, file: f, bf: bitfield.New(tf.NumPieces())}

	// 优先从状态文件恢复断点续传进度
	st, err := s.loadState()
	if err == nil && st != nil {
		raw, err := base64.StdEncoding.DecodeString(st.Bitfield)
		if err == nil && len(raw) == len(s.bf) {
			s.bf = bitfield.Bitfield(raw)
			s.Downloaded = st.Downloaded
			s.Uploaded = st.Uploaded
			return s, nil
		}
	}

	// 状态文件缺失但数据文件已存在：逐块校验 SHA-1 重建位图
	// （例如重新添加已下载完成的种子，或状态文件意外丢失）
	if existed && tf.Length > 0 {
		for i := 0; i < tf.NumPieces(); i++ {
			buf := make([]byte, tf.PieceSize(i))
			if _, err := f.ReadAt(buf, int64(i)*int64(tf.PieceLength)); err != nil {
				break
			}
			if tf.VerifyPiece(i, buf) {
				s.bf.SetPiece(i)
			}
		}
	}
	return s, nil
}

// Bitfield 返回当前位图副本。
func (s *Store) Bitfield() bitfield.Bitfield {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bf.Copy()
}

// HasPiece 判断分片是否已完成。
func (s *Store) HasPiece(i int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bf.HasPiece(i)
}

// CompletedPieces 返回已完成分片数。
func (s *Store) CompletedPieces() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bf.Count()
}

// Complete 判断是否全部分片均已完成。
func (s *Store) Complete() bool {
	return s.CompletedPieces() == s.tf.NumPieces()
}

// ReadPiece 读取分片数据（供上传给其他 Peer）。
func (s *Store) ReadPiece(index int) ([]byte, error) {
	size := s.tf.PieceSize(index)
	buf := make([]byte, size)
	_, err := s.file.ReadAt(buf, int64(index)*int64(s.tf.PieceLength))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.Uploaded += int64(size)
	s.mu.Unlock()
	return buf, nil
}

// ReadBlock 读取分片内的一个块。
func (s *Store) ReadBlock(index, begin, length int) ([]byte, error) {
	if begin < 0 || length <= 0 || begin+length > s.tf.PieceSize(index) {
		return nil, fmt.Errorf("块范围越界: piece=%d begin=%d len=%d", index, begin, length)
	}
	buf := make([]byte, length)
	_, err := s.file.ReadAt(buf, int64(index)*int64(s.tf.PieceLength)+int64(begin))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.Uploaded += int64(length)
	s.mu.Unlock()
	return buf, nil
}

// WritePiece 校验并写入一个完整分片；校验失败返回错误且不更新位图。
func (s *Store) WritePiece(index int, data []byte) error {
	if !s.tf.VerifyPiece(index, data) {
		return fmt.Errorf("分片 %d SHA-1 校验失败", index)
	}
	if _, err := s.file.WriteAt(data, int64(index)*int64(s.tf.PieceLength)); err != nil {
		return err
	}
	s.mu.Lock()
	s.bf.SetPiece(index)
	s.Downloaded += int64(len(data))
	s.mu.Unlock()
	return s.SaveState()
}

// MarkSeeding 将全部位图置位（做种场景，文件已在本地完整存在）。
// 会逐块校验 SHA-1，返回校验失败的分片列表。
func (s *Store) MarkSeeding() ([]int, error) {
	var bad []int
	for i := 0; i < s.tf.NumPieces(); i++ {
		buf := make([]byte, s.tf.PieceSize(i))
		if _, err := s.file.ReadAt(buf, int64(i)*int64(s.tf.PieceLength)); err != nil {
			return nil, err
		}
		if s.tf.VerifyPiece(i, buf) {
			s.mu.Lock()
			s.bf.SetPiece(i)
			s.mu.Unlock()
		} else {
			bad = append(bad, i)
		}
	}
	return bad, s.SaveState()
}

// BytesLeft 返回剩余待下载字节数。
func (s *Store) BytesLeft() int64 {
	done := int64(0)
	s.mu.RLock()
	for i := 0; i < s.tf.NumPieces(); i++ {
		if s.bf.HasPiece(i) {
			done += int64(s.tf.PieceSize(i))
		}
	}
	s.mu.RUnlock()
	return s.tf.Length - done
}

// SaveState 将当前进度写入状态文件。
func (s *Store) SaveState() error {
	s.mu.RLock()
	st := State{
		InfoHash:   hex.EncodeToString(s.tf.InfoHash[:]),
		Name:       s.tf.Name,
		Length:     s.tf.Length,
		Bitfield:   base64.StdEncoding.EncodeToString(s.bf),
		Downloaded: s.Downloaded,
		Uploaded:   s.Uploaded,
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.statePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.statePath())
}

// ClearState 下载完成后删除状态文件。
func (s *Store) ClearState() {
	os.Remove(s.statePath())
}

// Close 关闭数据文件。
func (s *Store) Close() error {
	return s.file.Close()
}

func (s *Store) loadState() (*State, error) {
	data, err := os.ReadFile(s.statePath())
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Store) statePath() string {
	return filepath.Join(stateDir(s.dir), hex.EncodeToString(s.tf.InfoHash[:])+".json")
}

func stateDir(dir string) string { return filepath.Join(dir, ".state") }

// sanitize 去掉文件名中的路径分隔符，防止目录穿越。
func sanitize(name string) string {
	return filepath.Base(filepath.Clean(name))
}
