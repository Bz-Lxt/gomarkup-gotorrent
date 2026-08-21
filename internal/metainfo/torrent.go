// Package metainfo 负责 .torrent 种子文件的生成与解析。
// 遵循 BitTorrent 元信息结构（bencode 编码）：
//
//	{
//	  "announce": "http://tracker/announce",
//	  "comment": "...",
//	  "created by": "GoTorrent",
//	  "creation date": 1234567890,
//	  "info": {
//	    "name": "file.bin",
//	    "length": 12345,
//	    "piece length": 262144,
//	    "pieces": "<20字节SHA1拼接>"
//	  }
//	}
package metainfo

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gotorrent/internal/bencode"
	"gotorrent/internal/hasher"
)

// DefaultPieceLength 默认分片大小 256KB。
const DefaultPieceLength = 256 * 1024

// TorrentFile 是解析后的种子文件。
type TorrentFile struct {
	Announce    string   // Tracker 地址
	Comment     string   // 备注
	CreatedBy   string   // 创建工具
	Name        string   // 文件名
	Length      int64    // 文件总大小
	PieceLength int      // 分片大小
	PieceHashes [][20]byte // 每个分片的 SHA-1
	InfoHash    [20]byte // info 字典 bencode 后的 SHA-1，唯一标识一个种子
}

// NumPieces 返回分片总数。
func (t *TorrentFile) NumPieces() int {
	return len(t.PieceHashes)
}

// PieceSize 返回第 i 个分片的实际大小（最后一个分片可能不足 PieceLength）。
func (t *TorrentFile) PieceSize(i int) int {
	if i == len(t.PieceHashes)-1 {
		last := t.Length % int64(t.PieceLength)
		if last == 0 {
			return t.PieceLength
		}
		return int(last)
	}
	return t.PieceLength
}

// Create 从磁盘文件生成种子文件：按 pieceLength 切分并逐块计算 SHA-1。
func Create(filePath, announce, comment string, pieceLength int) (*TorrentFile, error) {
	if pieceLength <= 0 {
		pieceLength = DefaultPieceLength
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("暂不支持目录做种: %s", filePath)
	}

	tf := &TorrentFile{
		Announce:    announce,
		Comment:     comment,
		CreatedBy:   "GoTorrent/1.0",
		Name:        filepath.Base(filePath),
		Length:      st.Size(),
		PieceLength: pieceLength,
	}

	hashes, err := hasher.HashReader(f, pieceLength)
	if err != nil {
		return nil, fmt.Errorf("计算分片哈希失败: %w", err)
	}
	tf.PieceHashes = hashes
	if err := tf.computeInfoHash(); err != nil {
		return nil, err
	}
	return tf, nil
}

// Encode 序列化为 .torrent 文件内容（bencode）。
func (t *TorrentFile) Encode() ([]byte, error) {
	pieces := hasher.Concat(t.PieceHashes)
	return bencode.Encode(map[string]any{
		"announce":      t.Announce,
		"comment":       t.Comment,
		"created by":    t.CreatedBy,
		"creation date": time.Now().Unix(),
		"info": map[string]any{
			"name":         t.Name,
			"length":       t.Length,
			"piece length": int64(t.PieceLength),
			"pieces":       pieces,
		},
	})
}

// Save 将种子写入磁盘。
func (t *TorrentFile) Save(path string) error {
	data, err := t.Encode()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Parse 解析 .torrent 文件内容。
func Parse(data []byte) (*TorrentFile, error) {
	v, err := bencode.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("种子文件解析失败: %w", err)
	}
	root, err := bencode.AsDict(v)
	if err != nil {
		return nil, err
	}
	infoRaw, ok := root["info"]
	if !ok {
		return nil, fmt.Errorf("种子缺少 info 字段")
	}
	info, err := bencode.AsDict(infoRaw)
	if err != nil {
		return nil, err
	}

	tf := &TorrentFile{}
	if tf.Announce, err = bencode.AsString(root["announce"]); err != nil {
		return nil, fmt.Errorf("announce 字段非法: %w", err)
	}
	if s, err := bencode.AsString(root["comment"]); err == nil {
		tf.Comment = s
	}
	if s, err := bencode.AsString(root["created by"]); err == nil {
		tf.CreatedBy = s
	}
	if tf.Name, err = bencode.AsString(info["name"]); err != nil {
		return nil, fmt.Errorf("name 字段非法: %w", err)
	}
	if tf.Length, err = bencode.AsInt(info["length"]); err != nil {
		return nil, fmt.Errorf("length 字段非法: %w", err)
	}
	pl, err := bencode.AsInt(info["piece length"])
	if err != nil {
		return nil, fmt.Errorf("piece length 字段非法: %w", err)
	}
	tf.PieceLength = int(pl)

	pieces, err := bencode.AsBytes(info["pieces"])
	if err != nil {
		return nil, fmt.Errorf("pieces 字段非法: %w", err)
	}
	hashes, err := hasher.Split(pieces)
	if err != nil {
		return nil, err
	}
	tf.PieceHashes = hashes

	// info_hash 必须基于原始 info 字典的字节重新编码计算。
	// 由于我们的解码器保留全部字段且编码器按 key 排序，重编码结果与原文一致。
	if err := tf.computeInfoHash(); err != nil {
		return nil, err
	}
	return tf, nil
}

// Load 从磁盘读取并解析种子文件。
func Load(path string) (*TorrentFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// computeInfoHash 重新编码 info 字典并计算 SHA-1。
func (t *TorrentFile) computeInfoHash() error {
	pieces := hasher.Concat(t.PieceHashes)
	raw, err := bencode.Encode(map[string]any{
		"name":         t.Name,
		"length":       t.Length,
		"piece length": int64(t.PieceLength),
		"pieces":       pieces,
	})
	if err != nil {
		return err
	}
	t.InfoHash = hasher.Hash(raw)
	return nil
}

// VerifyPiece 校验分片数据的 SHA-1 是否与种子记录一致。
func (t *TorrentFile) VerifyPiece(index int, data []byte) bool {
	if index < 0 || index >= len(t.PieceHashes) {
		return false
	}
	return hasher.Verify(data, t.PieceHashes[index])
}
