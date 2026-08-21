package peer

import (
	"math/rand"
	"sync"

	"gotorrent/internal/bitfield"
)

// piecePicker 会话级分片选择器：Rarest-First（最稀有优先）策略。
// 优先下载 Swarm 中持有节点最少的分片，提升整体可用性。
type piecePicker struct {
	mu           sync.Mutex
	numPieces    int
	owned        bitfield.Bitfield // 我们已拥有的分片
	availability []int             // 每个分片在所有已连接 Peer 中的持有数
	inFlight     map[int]bool      // 正在下载中的分片（同一分片同一时刻只分配给一个连接）
}

func newPiecePicker(numPieces int, owned bitfield.Bitfield) *piecePicker {
	return &piecePicker{
		numPieces:    numPieces,
		owned:        owned.Copy(),
		availability: make([]int, numPieces),
		inFlight:     make(map[int]bool),
	}
}

// AddPeer 将一个 Peer 的位图纳入可用性统计。
func (p *piecePicker) AddPeer(bf bitfield.Bitfield) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < p.numPieces; i++ {
		if bf.HasPiece(i) {
			p.availability[i]++
		}
	}
}

// RemovePeer 连接断开时移除其位图统计。
func (p *piecePicker) RemovePeer(bf bitfield.Bitfield) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < p.numPieces; i++ {
		if bf.HasPiece(i) && p.availability[i] > 0 {
			p.availability[i]--
		}
	}
}

// AddHave 对方通过 Have 消息声明了新分片。
func (p *piecePicker) AddHave(i int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if i >= 0 && i < p.numPieces {
		p.availability[i]++
	}
}

// SetOwned 标记分片已下载完成。
func (p *piecePicker) SetOwned(i int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.owned.SetPiece(i)
	delete(p.inFlight, i)
}

// Release 释放下载中的分片（连接断开或校验失败时），使其可被重新分配。
func (p *piecePicker) Release(i int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inFlight, i)
}

// Pick 为该连接挑选一个最稀有的分片；没有可选分片时返回 -1。
func (p *piecePicker) Pick(peerBF bitfield.Bitfield) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	best, bestAvail := -1, int(^uint(0)>>1) // max int
	for i := 0; i < p.numPieces; i++ {
		if p.owned.HasPiece(i) || p.inFlight[i] || !peerBF.HasPiece(i) {
			continue
		}
		avail := p.availability[i]
		if avail < bestAvail || (avail == bestAvail && rand.Intn(2) == 0) {
			best, bestAvail = i, avail
		}
	}
	if best >= 0 {
		p.inFlight[best] = true
	}
	return best
}

// Interesting 判断对方是否拥有我们需要的分片。
func (p *piecePicker) Interesting(peerBF bitfield.Bitfield) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < p.numPieces; i++ {
		if !p.owned.HasPiece(i) && peerBF.HasPiece(i) {
			return true
		}
	}
	return false
}
