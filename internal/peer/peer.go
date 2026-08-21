// Package peer 实现 P2P 节点：监听/发起连接、管理下载与做种会话、
// 执行 Tit-for-Tat 激励策略，并提供 HTTP 控制台。
package peer

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gotorrent/internal/config"
	"gotorrent/internal/eventlog"
	"gotorrent/internal/logx"
	"gotorrent/internal/metainfo"
	"gotorrent/internal/peerid"
	"gotorrent/internal/storage"
	"gotorrent/internal/util"
	"gotorrent/internal/wire"
)

// Peer 是一个 P2P 节点。
type Peer struct {
	peerID peerid.ID
	dir    string
	cfg    *config.PeerConfig
	events *eventlog.Log
	log    *logx.Logger

	mu       sync.Mutex
	sessions map[[20]byte]*Session
	listener net.Listener
}

// New 创建节点。dir 为数据目录。
func New(dir string) (*Peer, error) {
	cfg := config.DefaultPeer()
	cfg.Dir = dir
	return NewWithConfig(cfg)
}

// NewWithConfig 使用完整配置创建节点。
func NewWithConfig(cfg *config.PeerConfig) (*Peer, error) {
	if cfg == nil {
		cfg = config.DefaultPeer()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, err
	}
	id, err := peerid.Generate()
	if err != nil {
		return nil, err
	}
	lg := logx.New("peer")
	lg.SetLevel(logx.ParseLevel(cfg.LogLevel))
	return &Peer{
		peerID:   id,
		dir:      cfg.Dir,
		cfg:      cfg,
		events:   eventlog.New(300),
		log:      lg,
		sessions: make(map[[20]byte]*Session),
	}, nil
}

// ID 返回节点 ID 字符串。
func (p *Peer) ID() string { return p.peerID.String() }

// Events 返回最近的节点事件。
func (p *Peer) Events(n int) []eventlog.Event {
	if p.events == nil {
		return nil
	}
	return p.events.Recent(n)
}

// Port 返回 P2P 协议监听端口。
func (p *Peer) Port() int {
	if p.listener == nil {
		return 0
	}
	_, portStr, _ := net.SplitHostPort(p.listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return port
}

// Dir 返回数据目录。
func (p *Peer) Dir() string { return p.dir }

// Listen 启动 P2P 协议监听。
func (p *Peer) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("P2P 监听失败: %w", err)
	}
	p.listener = ln
	p.log.Infof("P2P 监听于 %s, PeerID=%s", ln.Addr(), p.ID())
	go p.acceptLoop()
	return nil
}

func (p *Peer) acceptLoop() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return
		}
		go p.handleInbound(conn)
	}
}

// handleInbound 处理入站连接：读取握手 -> 找到对应会话 -> 回应握手 -> 交换位图。
func (p *Peer) handleInbound(conn net.Conn) {
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	hs, err := wire.ReadHandshake(conn)
	if err != nil {
		conn.Close()
		return
	}

	p.mu.Lock()
	s, ok := p.sessions[hs.InfoHash]
	p.mu.Unlock()
	if !ok {
		p.log.Warnf("入站连接请求未知种子，关闭: %s", conn.RemoteAddr())
		conn.Close()
		return
	}

	ourHS := &wire.Handshake{InfoHash: hs.InfoHash, PeerID: p.peerID.Array()}
	if err := ourHS.Write(conn); err != nil {
		conn.Close()
		return
	}
	conn.SetDeadline(time.Time{})

	pc := newPeerConn(s, conn, conn.RemoteAddr().String())
	pc.remoteID = string(hs.PeerID[:])
	pc.send(wire.NewBitfield(s.store.Bitfield()))
	s.addConn(pc)
	p.log.Infof("接受入站连接 %s (会话 %s)", conn.RemoteAddr(), s.tf.Name)
	pc.run()
	s.removeConn(pc)
}

// dialPeer 主动连接一个节点（下载时由 announce 结果驱动）。
func (p *Peer) dialPeer(s *Session, addr string) {
	select {
	case <-s.stopCh:
		return
	default:
	}

	conn, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		return
	}
	conn.SetDeadline(time.Now().Add(15 * time.Second))

	hs := &wire.Handshake{InfoHash: s.tf.InfoHash, PeerID: p.peerID.Array()}
	if err := hs.Write(conn); err != nil {
		conn.Close()
		return
	}
	resp, err := wire.ReadHandshake(conn)
	if err != nil || resp.InfoHash != s.tf.InfoHash {
		conn.Close()
		return
	}
	conn.SetDeadline(time.Time{})

	pc := newPeerConn(s, conn, addr)
	pc.remoteID = string(resp.PeerID[:])
	pc.send(wire.NewBitfield(s.store.Bitfield()))
	s.addConn(pc)
	p.log.Infof("连接节点 %s (会话 %s)", addr, s.tf.Name)
	pc.run()
	s.removeConn(pc)
}

// ---- 会话管理 ----

// AddDownload 添加一个下载任务。
func (p *Peer) AddDownload(tf *metainfo.TorrentFile) (*Session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.sessions[tf.InfoHash]; ok {
		return nil, fmt.Errorf("该种子已在任务列表中")
	}
	store, err := storage.Open(p.dir, tf)
	if err != nil {
		return nil, err
	}
	s := newSession(p, tf, store, false)
	p.sessions[tf.InfoHash] = s
	s.run()
	resumed := store.CompletedPieces()
	if resumed > 0 {
		p.log.Infof("断点续传 %s: 已有 %d/%d 分片", tf.Name, resumed, tf.NumPieces())
		p.events.Append(eventlog.KindState, tf.Name, fmt.Sprintf("断点续传 %d/%d 分片", resumed, tf.NumPieces()))
	} else {
		p.events.Append(eventlog.KindState, tf.Name, "开始下载")
	}
	return s, nil
}

// AddSeed 从本地文件创建种子并开始做种。
// 文件会被复制到数据目录（若尚不在其中），.torrent 保存到数据目录。
func (p *Peer) AddSeed(filePath, trackerURL string) (*Session, error) {
	st, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}
	if st.IsDir() {
		return nil, fmt.Errorf("暂不支持目录做种")
	}

	tf, err := metainfo.Create(filePath, trackerURL, "created by GoTorrent", metainfo.DefaultPieceLength)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	if _, ok := p.sessions[tf.InfoHash]; ok {
		p.mu.Unlock()
		return nil, fmt.Errorf("相同内容的种子已在做种")
	}
	p.mu.Unlock()

	// 把文件复制到数据目录
	dst := filepath.Join(p.dir, filepath.Base(filePath))
	if abs1, _ := filepath.Abs(filePath); abs1 != mustAbs(dst) {
		if err := copyFile(filePath, dst); err != nil {
			return nil, fmt.Errorf("复制文件到数据目录失败: %w", err)
		}
	}
	// 保存 .torrent 文件，方便分发给其他节点
	if err := tf.Save(dst + ".torrent"); err != nil {
		return nil, err
	}

	store, err := storage.Open(p.dir, tf)
	if err != nil {
		return nil, err
	}
	bad, err := store.MarkSeeding()
	if err != nil {
		store.Close()
		return nil, err
	}
	if len(bad) > 0 {
		store.Close()
		return nil, fmt.Errorf("文件校验失败的分片: %v", bad)
	}

	s := newSession(p, tf, store, true)
	p.mu.Lock()
	p.sessions[tf.InfoHash] = s
	p.mu.Unlock()
	s.run()
	p.log.Infof("开始做种 %s (info_hash=%s)", tf.Name, s.InfoHashHex())
	p.events.Append(eventlog.KindState, tf.Name, "开始做种")
	return s, nil
}

// RemoveSession 停止并移除一个会话。
func (p *Peer) RemoveSession(infoHashHex string) error {
	var key [20]byte
	b, err := decodeHex20(infoHashHex)
	if err != nil {
		return err
	}
	copy(key[:], b)

	p.mu.Lock()
	s, ok := p.sessions[key]
	if ok {
		delete(p.sessions, key)
	}
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("会话不存在")
	}
	s.Stop()
	return nil
}

// Sessions 返回所有会话快照。
func (p *Peer) Sessions() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]any, 0, len(p.sessions))
	for _, s := range p.sessions {
		out = append(out, s.Snapshot())
	}
	return out
}

// SessionBitfield 返回指定会话的位图（base64），供前端绘制分片图。
func (p *Peer) SessionBitfield(infoHashHex string) ([]byte, error) {
	b, err := decodeHex20(infoHashHex)
	if err != nil {
		return nil, err
	}
	var key [20]byte
	copy(key[:], b)
	p.mu.Lock()
	s, ok := p.sessions[key]
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("会话不存在")
	}
	return s.store.Bitfield(), nil
}

func decodeHex20(s string) ([]byte, error) {
	arr, err := util.DecodeHex20(s)
	if err != nil {
		return nil, err
	}
	return arr[:], nil
}

// FindTorrentByHash 在数据目录中查找匹配 info_hash 的 .torrent 文件。
func (p *Peer) FindTorrentByHash(hash [20]byte) (*metainfo.TorrentFile, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".torrent" {
			continue
		}
		tf, err := metainfo.Load(filepath.Join(p.dir, e.Name()))
		if err != nil {
			continue
		}
		if tf.InfoHash == hash {
			return tf, nil
		}
	}
	return nil, fmt.Errorf("数据目录中没有匹配的种子文件")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func mustAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
