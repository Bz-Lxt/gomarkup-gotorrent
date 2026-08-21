package peer

import (
	"fmt"
	"net"
	"sync"
	"time"

	"gotorrent/internal/announce"
	"gotorrent/internal/eventlog"
	"gotorrent/internal/logx"
	"gotorrent/internal/magnet"
	"gotorrent/internal/metainfo"
	"gotorrent/internal/metrics"
	"gotorrent/internal/ratelimit"
	"gotorrent/internal/storage"
	"gotorrent/internal/util"
	"gotorrent/internal/wire"
)

const (
	maxOutboundConns   = 30
	defaultAnnInterval = 15
)

// Session 表示一个种子的下载/做种会话。
type Session struct {
	peer   *Peer
	tf     *metainfo.TorrentFile
	store  *storage.Store
	picker *piecePicker
	ann    announcer
	dial   func(addr string)
	events *eventlog.Log
	sample *metrics.Sampler
	downLim *ratelimit.Limiter
	upLim   *ratelimit.Limiter
	log     *logx.Logger

	mu       sync.Mutex
	conns    map[*peerConn]struct{}
	seedMode bool // 创建时即为完整文件（做种）
	finished bool // 是否已发送过 completed 事件

	stopCh   chan struct{}
	stopOnce sync.Once
}

// announcer 是 announce 客户端接口；*announce.Client 天然满足，便于测试替换。
type announcer interface {
	Do(announce.Request) (*announce.Response, error)
}

func newSession(p *Peer, tf *metainfo.TorrentFile, store *storage.Store, seedMode bool) *Session {
	s := &Session{
		peer:     p,
		tf:       tf,
		store:    store,
		picker:   newPiecePicker(tf.NumPieces(), store.Bitfield()),
		ann:      announce.NewClient(),
		events:   p.events,
		sample:   metrics.NewSampler(),
		downLim:  ratelimit.New(downLimitOf(p)),
		upLim:    ratelimit.New(upLimitOf(p)),
		log:      logx.Session,
		conns:    make(map[*peerConn]struct{}),
		seedMode: seedMode,
		stopCh:   make(chan struct{}),
	}
	s.dial = func(addr string) { go p.dialPeer(s, addr) }
	return s
}

// newTestSession 构造一个不启动后台循环、ann/dial 可替换的会话，仅供测试。
func newTestSession(p *Peer, tf *metainfo.TorrentFile, store *storage.Store) *Session {
	return newSession(p, tf, store, false)
}

// InfoHashHex 返回 info_hash 的十六进制表示。
func (s *Session) InfoHashHex() string { return util.EncodeHex20(s.tf.InfoHash) }

// Magnet 返回该任务的 Magnet 链接。
func (s *Session) Magnet() string {
	return magnet.Encode(s.tf.InfoHash, s.tf.Name, []string{s.tf.Announce}, s.tf.Length)
}

// run 启动会话的后台循环：Tracker 汇报、Tit-for-Tat 阻塞算法、速率采样。
func (s *Session) run() {
	go s.announceLoop()
	go s.chokeLoop()
	go s.rateLoop()
}

// Stop 停止会话并断开所有连接。
func (s *Session) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		s.announce(announce.EventStopped)
		s.mu.Lock()
		for c := range s.conns {
			c.close()
		}
		s.mu.Unlock()
		s.store.Close()
		s.note(eventlog.KindState, "任务已停止")
	})
}

func (s *Session) note(kind eventlog.Kind, format string, args ...any) {
	if s.events == nil {
		return
	}
	s.events.Append(kind, s.tf.Name, fmt.Sprintf(format, args...))
}

// ---- 连接管理 ----

func (s *Session) addConn(c *peerConn) {
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
	s.note(eventlog.KindPeer, "连接 %s", c.addr)
}

func (s *Session) removeConn(c *peerConn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()

	c.mu.Lock()
	bf, work := c.peerBF, c.work
	c.mu.Unlock()
	if bf != nil {
		s.picker.RemovePeer(bf)
	}
	if work != nil {
		s.picker.Release(work.index)
	}
	s.note(eventlog.KindPeer, "断开 %s", c.addr)
}

// numConns 返回当前连接数。
func (s *Session) numConns() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

func (s *Session) maxConns() int {
	if s.peer != nil && s.peer.cfg != nil && s.peer.cfg.MaxPeers > 0 {
		return s.peer.cfg.MaxPeers
	}
	return maxOutboundConns
}

// finishPiece 在一个分片集齐后校验、落盘并广播 Have。
func (s *Session) finishPiece(work *pieceWork, from *peerConn) {
	if err := s.store.WritePiece(work.index, work.buf); err != nil {
		s.log.Warnf("分片 %d 校验失败，重新排队: %v", work.index, err)
		s.picker.Release(work.index)
		s.note(eventlog.KindError, "分片 %d 校验失败", work.index)
		return
	}
	s.picker.SetOwned(work.index)
	s.log.Infof("%s 分片 %d/%d 完成", s.tf.Name, s.store.CompletedPieces(), s.tf.NumPieces())
	s.note(eventlog.KindPiece, "分片 %d/%d 完成", s.store.CompletedPieces(), s.tf.NumPieces())

	s.mu.Lock()
	for c := range s.conns {
		c.send(wire.NewHave(work.index))
	}
	s.mu.Unlock()

	if s.store.Complete() {
		s.onDownloadComplete()
	}
}

// onDownloadComplete 下载完成：通知 Tracker、广播 NotInterested、清理续传状态。
func (s *Session) onDownloadComplete() {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	s.mu.Unlock()

	s.log.Infof("%s 下载完成，转为做种", s.tf.Name)
	s.note(eventlog.KindState, "下载完成，转为做种")
	s.store.ClearState()
	s.announce(announce.EventCompleted)

	s.mu.Lock()
	for c := range s.conns {
		c.mu.Lock()
		interested := c.amInterested
		c.amInterested = false
		c.mu.Unlock()
		if interested {
			c.send(&wire.Message{ID: wire.MsgNotInterested})
		}
	}
	s.mu.Unlock()
}

// ---- Tracker 汇报 ----

func (s *Session) announceLoop() {
	s.announce(announce.EventStarted)
	interval := time.Duration(defaultAnnInterval) * time.Second
	if s.peer != nil && s.peer.cfg != nil && s.peer.cfg.AnnounceSec > 0 {
		interval = time.Duration(s.peer.cfg.AnnounceSec) * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if iv := s.announce(announce.EventNone); iv > 0 {
				interval = time.Duration(iv) * time.Second
				ticker.Reset(interval)
			}
		}
	}
}

// announce 向 Tracker 汇报一次，返回建议的汇报间隔；并把返回的节点交给拨号器。
func (s *Session) announce(event announce.Event) int {
	resp, err := s.ann.Do(announce.Request{
		AnnounceURL: s.tf.Announce,
		InfoHash:    s.InfoHashHex(),
		PeerID:      s.peer.ID(),
		Port:        s.peer.Port(),
		Uploaded:    s.store.Uploaded,
		Downloaded:  s.store.Downloaded,
		Left:        s.store.BytesLeft(),
		Name:        s.tf.Name,
		Length:      s.tf.Length,
		Event:       event,
	})
	if err != nil {
		s.log.Warnf("announce 失败: %v", err)
		s.note(eventlog.KindAnnounce, "汇报失败: %v", err)
		// 维护窗口里边缘 Tracker 会复用上一轮响应外壳：HTTP 200 的 JSON 同时
		// 带 failure reason 和旧的 peers。此时绝不能继续拨号，否则旧 peer 的
		// 监听端仍会立刻收到新的 TCP 连接。拒绝必须真正隔离住出站连接。
		if resp == nil {
			return 0
		}
		return resp.Interval
	}
	if event != announce.EventNone {
		s.note(eventlog.KindAnnounce, "event=%s peers=%d", event, len(resp.Peers))
	}

	if !s.store.Complete() {
		for _, p := range resp.Peers {
			if s.numConns() >= s.maxConns() {
				break
			}
			addr := net.JoinHostPort(p.IP, fmt.Sprintf("%d", p.Port))
			s.dial(addr)
		}
	}
	return resp.Interval
}

// rateLoop 每 3 秒采样一次速率供界面展示。
func (s *Session) rateLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.sample.Tick(s.store.Downloaded, s.store.Uploaded)
		}
	}
}

// ---- 状态快照（供 API） ----

// Snapshot 返回会话的界面展示数据。
func (s *Session) Snapshot() map[string]any {
	downRate, upRate := s.sample.Rates()
	state := "downloading"
	if s.store.Complete() {
		state = "seeding"
	}
	return map[string]any{
		"info_hash":        s.InfoHashHex(),
		"name":             s.tf.Name,
		"length":           s.tf.Length,
		"piece_length":     s.tf.PieceLength,
		"num_pieces":       s.tf.NumPieces(),
		"completed_pieces": s.store.CompletedPieces(),
		"downloaded":       s.store.Downloaded,
		"uploaded":         s.store.Uploaded,
		"down_rate":        downRate,
		"up_rate":          upRate,
		"peers":            s.numConns(),
		"state":            state,
		"announce":         s.tf.Announce,
		"magnet":           s.Magnet(),
		"ratio":            metrics.Ratio(s.store.Uploaded, s.store.Downloaded),
	}
}

func downLimitOf(p *Peer) int64 {
	if p != nil && p.cfg != nil {
		return p.cfg.DownLimitBps
	}
	return 0
}

func upLimitOf(p *Peer) int64 {
	if p != nil && p.cfg != nil {
		return p.cfg.UpLimitBps
	}
	return 0
}
