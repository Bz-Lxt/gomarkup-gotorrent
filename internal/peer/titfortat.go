package peer

import (
	"math/rand"
	"sort"
	"time"
)

const (
	chokeInterval    = 10 * time.Second // Tit-for-Tat 评估周期
	unchokeCount     = 3                // 每周期固定放行数（另有 1 个乐观放行）
	optimisticPeriod = 3                // 每 N 轮做一次乐观放行
)

// chokeLoop 每 10 秒评估一次：放行对我们上传最快的 3 个节点，
// 并周期性乐观放行 1 个随机节点（给新节点机会）；其余全部 Choke。
func (s *Session) chokeLoop() {
	ticker := time.NewTicker(chokeInterval)
	defer ticker.Stop()
	round := 0
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.chokeRound(round)
			round++
		}
	}
}

// maybeFastUnchoke 当放行名额未满时立即放行新表示感兴趣的节点，
// 避免新节点空等一个 Tit-for-Tat 周期。
func (s *Session) maybeFastUnchoke(c *peerConn) {
	s.mu.Lock()
	count := 0
	for other := range s.conns {
		if other != c && other.interestedIn() && !other.isChoking() {
			count++
		}
	}
	slots := s.unchokeSlots()
	s.mu.Unlock()
	if count < slots {
		c.setChoke(false)
	}
}

func (s *Session) unchokeSlots() int {
	if s.peer != nil && s.peer.cfg != nil && s.peer.cfg.UnchokeSlots > 0 {
		return s.peer.cfg.UnchokeSlots
	}
	return unchokeCount
}

func (s *Session) chokeRound(round int) {
	s.mu.Lock()
	conns := make([]*peerConn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	slots := s.unchokeSlots()
	s.mu.Unlock()

	type cand struct {
		c    *peerConn
		down int64
	}
	var interested []cand
	for _, c := range conns {
		if c.interestedIn() {
			down, _ := c.snapshotRates()
			interested = append(interested, cand{c, down})
		}
	}

	// 按对我们的贡献（下载速率）降序 —— Tit-for-Tat：你对我好，我才对你好
	sort.Slice(interested, func(i, j int) bool { return interested[i].down > interested[j].down })

	unchoked := make(map[*peerConn]bool)
	for i := 0; i < len(interested) && i < slots; i++ {
		unchoked[interested[i].c] = true
	}

	// 乐观放行：每 3 轮随机选一个被 Choke 的感兴趣节点
	if round%optimisticPeriod == 0 {
		var chokedList []cand
		for _, cd := range interested {
			if !unchoked[cd.c] {
				chokedList = append(chokedList, cd)
			}
		}
		if len(chokedList) > 0 {
			unchoked[chokedList[rand.Intn(len(chokedList))].c] = true
		}
	}

	for _, cd := range interested {
		cd.c.setChoke(!unchoked[cd.c])
	}
}
