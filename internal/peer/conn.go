package peer

import (
	"log"
	"net"
	"sync"
	"time"

	"gotorrent/internal/bitfield"
	"gotorrent/internal/pex"
	"gotorrent/internal/wire"
)

const (
	blockSize      = wire.MaxBlockSize // 16KB
	pipelineDepth  = 16                // 每个连接最多同时挂起的块请求数
	requestTimeout = 45 * time.Second  // 块请求超时
)

// pieceWork 记录一个正在从某连接下载的分片的块级进度。
type pieceWork struct {
	index         int
	buf           []byte
	totalBlocks   int
	received      []bool
	requested     []bool
	receivedCount int
}

func newPieceWork(index, size int) *pieceWork {
	n := (size + blockSize - 1) / blockSize
	return &pieceWork{
		index: index, buf: make([]byte, size), totalBlocks: n,
		received: make([]bool, n), requested: make([]bool, n),
	}
}

// nextBlock 返回下一个待请求的块。
func (pw *pieceWork) nextBlock() (begin, length int, ok bool) {
	for i := 0; i < pw.totalBlocks; i++ {
		if !pw.requested[i] && !pw.received[i] {
			begin = i * blockSize
			length = blockSize
			if begin+length > len(pw.buf) {
				length = len(pw.buf) - begin
			}
			return begin, length, true
		}
	}
	return 0, 0, false
}

func (pw *pieceWork) markReceived(begin int, data []byte) {
	i := begin / blockSize
	if i < pw.totalBlocks && !pw.received[i] {
		copy(pw.buf[begin:], data)
		pw.received[i] = true
		pw.receivedCount++
	}
}

func (pw *pieceWork) complete() bool { return pw.receivedCount == pw.totalBlocks }

type blockKey struct{ index, begin int }

// peerConn 表示与一个远程 Peer 的连接，同时承载下载与上传。
type peerConn struct {
	session  *Session
	conn     net.Conn
	addr     string
	remoteID string

	mu             sync.Mutex
	peerBF         bitfield.Bitfield
	amChoking      bool // 我们是否拒绝对方请求（默认拒绝）
	amInterested   bool
	peerChoking    bool // 对方是否拒绝我们请求（默认拒绝）
	peerInterested bool
	work           *pieceWork
	outstanding    map[blockKey]time.Time
	downBytes      int64 // 本轮 Tit-for-Tat 周期内从对方下载的字节
	upBytes        int64 // 本轮内向对方上传的字节

	sendCh    chan *wire.Message
	done      chan struct{}
	closeOnce sync.Once
}

func newPeerConn(s *Session, c net.Conn, addr string) *peerConn {
	return &peerConn{
		session: s, conn: c, addr: addr,
		amChoking: true, peerChoking: true,
		outstanding: make(map[blockKey]time.Time),
		sendCh:      make(chan *wire.Message, 64),
		done:        make(chan struct{}),
	}
}

// run 启动读写循环，直到连接关闭。
func (pc *peerConn) run() {
	go pc.writeLoop()
	go pc.timeoutLoop()
	pc.readLoop() // 读循环退出即关闭连接
}

func (pc *peerConn) close() {
	pc.closeOnce.Do(func() {
		close(pc.done)
		pc.conn.Close()
	})
}

func (pc *peerConn) send(m *wire.Message) {
	select {
	case pc.sendCh <- m:
	case <-pc.done:
	default: // 发送缓冲满则丢弃（对端异常时避免阻塞）
	}
}

func (pc *peerConn) writeLoop() {
	for {
		select {
		case <-pc.done:
			return
		case m := <-pc.sendCh:
			pc.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := m.Write(pc.conn); err != nil {
				pc.close()
				return
			}
		}
	}
}

// timeoutLoop 检查挂起过久的块请求：超时即断开（简化处理，重连后重新分配）。
func (pc *peerConn) timeoutLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-pc.done:
			return
		case <-ticker.C:
			pc.mu.Lock()
			stale := false
			for _, t := range pc.outstanding {
				if time.Since(t) > requestTimeout {
					stale = true
					break
				}
			}
			pc.mu.Unlock()
			if stale {
				log.Printf("[peer] %s 请求超时，断开连接", pc.addr)
				pc.close()
				return
			}
		}
	}
}

func (pc *peerConn) readLoop() {
	defer pc.close()
	pc.conn.SetReadDeadline(time.Now().Add(3 * time.Minute))
	for {
		m, err := wire.Read(pc.conn)
		if err != nil {
			return
		}
		pc.conn.SetReadDeadline(time.Now().Add(3 * time.Minute))
		pc.handleMessage(m)
	}
}

func (pc *peerConn) handleMessage(m *wire.Message) {
	switch m.ID {
	case 255: // keep-alive
	case wire.MsgChoke:
		pc.mu.Lock()
		pc.peerChoking = true
		pc.mu.Unlock()
	case wire.MsgUnchoke:
		pc.mu.Lock()
		pc.peerChoking = false
		pc.mu.Unlock()
		pc.fillPipeline()
	case wire.MsgInterested:
		pc.mu.Lock()
		pc.peerInterested = true
		pc.mu.Unlock()
		pc.session.maybeFastUnchoke(pc)
	case wire.MsgNotInterested:
		pc.mu.Lock()
		pc.peerInterested = false
		pc.mu.Unlock()
	case wire.MsgHave:
		if idx, err := m.ParseHave(); err == nil {
			pc.mu.Lock()
			pc.peerBF.SetPiece(idx)
			pc.mu.Unlock()
			pc.session.picker.AddHave(idx)
			pc.maybeInterest()
		}
	case wire.MsgBitfield:
		bf := bitfield.Bitfield(m.Payload)
		pc.mu.Lock()
		pc.peerBF = bf.Copy()
		pc.mu.Unlock()
		pc.session.picker.AddPeer(bf)
		pc.maybeInterest()
	case wire.MsgRequest:
		pc.handleRequest(m)
	case wire.MsgPiece:
		pc.handlePiece(m)
	case wire.MsgCancel:
		// 简化实现：请求是同步处理的，忽略 Cancel
	case wire.MsgExtended:
		pc.handleExtended(m)
	}
}

func (pc *peerConn) handleExtended(m *wire.Message) {
	extID, payload, err := m.ParseExtended()
	if err != nil || extID != wire.ExtPexID {
		return
	}
	msg, err := pex.Decode(payload)
	if err != nil || pc.session.store.Complete() {
		return
	}
	for _, peer := range pex.Dedup(msg.Added) {
		if pc.session.numConns() >= pc.session.maxConns() {
			return
		}
		go pc.session.peer.dialPeer(pc.session, peer.Addr())
	}
}

// maybeInterest 在对方位图更新后判断是否发送 Interested。
func (pc *peerConn) maybeInterest() {
	pc.mu.Lock()
	bf, interested := pc.peerBF, pc.amInterested
	pc.mu.Unlock()
	if bf == nil {
		return
	}
	if !interested && pc.session.picker.Interesting(bf) {
		pc.mu.Lock()
		pc.amInterested = true
		pc.mu.Unlock()
		pc.send(&wire.Message{ID: wire.MsgInterested})
	}
}

// fillPipeline 在未被阻塞时保持流水线中有足够多的块请求。
func (pc *peerConn) fillPipeline() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.peerChoking || pc.session.store.Complete() {
		return
	}
	for len(pc.outstanding) < pipelineDepth {
		if pc.work == nil {
			idx := pc.session.picker.Pick(pc.peerBF)
			if idx < 0 {
				return // 对方没有我们需要的分片
			}
			pc.work = newPieceWork(idx, pc.session.tf.PieceSize(idx))
		}
		begin, length, ok := pc.work.nextBlock()
		if !ok {
			return // 当前分片的块都已请求，等待响应
		}
		pc.work.requested[begin/blockSize] = true
		pc.outstanding[blockKey{pc.work.index, begin}] = time.Now()
		pc.send(wire.NewRequest(pc.work.index, begin, length))
	}
}

// handlePiece 处理收到的块数据；分片集齐后校验落盘并广播 Have。
func (pc *peerConn) handlePiece(m *wire.Message) {
	index, begin, block, err := m.ParsePiece()
	if err != nil {
		return
	}
	if pc.session.downLim != nil {
		pc.session.downLim.Wait(len(block))
	}

	pc.mu.Lock()
	delete(pc.outstanding, blockKey{index, begin})
	pc.downBytes += int64(len(block))
	if pc.work == nil || pc.work.index != index {
		pc.mu.Unlock()
		return // 非当前分片（超时重发等场景），忽略
	}
	pc.work.markReceived(begin, block)
	done := pc.work.complete()
	var work *pieceWork
	if done {
		work = pc.work
		pc.work = nil
	}
	pc.mu.Unlock()

	if done {
		pc.session.finishPiece(work, pc)
	}
	pc.fillPipeline()
}

// handleRequest 处理对方的块请求：仅在未被我们 Choke 且我们拥有该分片时响应。
func (pc *peerConn) handleRequest(m *wire.Message) {
	index, begin, length, err := m.ParseRequest()
	if err != nil || length > blockSize*2 {
		return
	}
	pc.mu.Lock()
	choked := pc.amChoking
	pc.mu.Unlock()
	if choked || !pc.session.store.HasPiece(index) {
		return
	}
	data, err := pc.session.store.ReadBlock(index, begin, length)
	if err != nil {
		return
	}
	if pc.session.upLim != nil {
		pc.session.upLim.Wait(len(data))
	}
	pc.mu.Lock()
	pc.upBytes += int64(len(data))
	pc.mu.Unlock()
	pc.send(wire.NewPiece(index, begin, data))
}

// snapshotRates 读取并清零本轮速率计数（供 Tit-for-Tat 使用）。
func (pc *peerConn) snapshotRates() (down, up int64) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	down, up = pc.downBytes, pc.upBytes
	pc.downBytes, pc.upBytes = 0, 0
	return
}

// interestedIn 返回对方是否对我们感兴趣。
func (pc *peerConn) interestedIn() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.peerInterested
}

// isChoking 返回我们是否正在 Choke 对方。
func (pc *peerConn) isChoking() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.amChoking
}

// setChoke 更新 Choke 状态并通知对方。
func (pc *peerConn) setChoke(choking bool) {
	pc.mu.Lock()
	changed := pc.amChoking != choking
	pc.amChoking = choking
	pc.mu.Unlock()
	if changed {
		if choking {
			pc.send(&wire.Message{ID: wire.MsgChoke})
		} else {
			pc.send(&wire.Message{ID: wire.MsgUnchoke})
		}
	}
}
