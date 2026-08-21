// Package tracker 实现轻量级 Tracker：记录每个 Swarm（info_hash）下
// 有哪些 Peer、各 Peer 的进度（做种/下载中），并向 Peer 返回邻居列表。
package tracker

import (
	"encoding/hex"
	"log"
	"sync"
	"time"
)

// PeerInfo 表示 Swarm 中的一个节点。
type PeerInfo struct {
	PeerID     string    `json:"peer_id"`
	IP         string    `json:"ip"`
	Port       int       `json:"port"`
	Uploaded   int64     `json:"uploaded"`
	Downloaded int64     `json:"downloaded"`
	Left       int64     `json:"left"` // 剩余字节，0 表示做种者
	Name       string    `json:"name"` // 节点备注名
	LastSeen   time.Time `json:"last_seen"`
}

// Seeder 判断该节点是否为做种者。
func (p *PeerInfo) Seeder() bool { return p.Left == 0 }

// Swarm 表示一个文件的分发群（同一 info_hash 的所有节点）。
type Swarm struct {
	InfoHash string             `json:"info_hash"`
	Name     string             `json:"name"`
	Length   int64              `json:"length"`
	Peers    map[string]*PeerInfo `json:"peers"` // key: peer_id
}

// Stats 返回做种数与下载数。
func (s *Swarm) Stats() (seeders, leechers int) {
	for _, p := range s.Peers {
		if p.Seeder() {
			seeders++
		} else {
			leechers++
		}
	}
	return
}

// Tracker 是核心状态：info_hash -> Swarm。
type Tracker struct {
	mu      sync.RWMutex
	swarms  map[string]*Swarm
	peerTTL time.Duration // 超过该时间未 announce 的节点被剔除
}

// New 创建 Tracker。peerTTL 为节点过期时间。
func New(peerTTL time.Duration) *Tracker {
	t := &Tracker{
		swarms:  make(map[string]*Swarm),
		peerTTL: peerTTL,
	}
	go t.reaper()
	return t
}

// Announce 处理一次节点汇报，返回该 Swarm 中的其他节点（最多 maxPeers 个）。
func (t *Tracker) Announce(infoHash [20]byte, p PeerInfo, name string, length int64, maxPeers int) []*PeerInfo {
	key := hex.EncodeToString(infoHash[:])
	t.mu.Lock()
	defer t.mu.Unlock()

	sw, ok := t.swarms[key]
	if !ok {
		sw = &Swarm{InfoHash: key, Name: name, Length: length, Peers: make(map[string]*PeerInfo)}
		t.swarms[key] = sw
	}
	if sw.Name == "" {
		sw.Name = name
	}
	if sw.Length == 0 {
		sw.Length = length
	}
	p.LastSeen = time.Now()
	selfID := p.PeerID
	sw.Peers[selfID] = &p

	out := make([]*PeerInfo, 0, len(sw.Peers))
	for id, peer := range sw.Peers {
		if id == selfID {
			continue
		}
		out = append(out, peer)
		if len(out) >= maxPeers {
			break
		}
	}
	return out
}

// Leave 处理 stopped 事件，将节点从 Swarm 移除。
func (t *Tracker) Leave(infoHash [20]byte, peerID string) {
	key := hex.EncodeToString(infoHash[:])
	t.mu.Lock()
	defer t.mu.Unlock()
	if sw, ok := t.swarms[key]; ok {
		delete(sw.Peers, peerID)
		if len(sw.Peers) == 0 {
			delete(t.swarms, key)
		}
	}
}

// Swarms 返回当前所有 Swarm 的快照（供管理页面使用）。
func (t *Tracker) Swarms() []*Swarm {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*Swarm, 0, len(t.swarms))
	for _, sw := range t.swarms {
		cp := &Swarm{
			InfoHash: sw.InfoHash,
			Name:     sw.Name,
			Length:   sw.Length,
			Peers:    make(map[string]*PeerInfo, len(sw.Peers)),
		}
		for id, p := range sw.Peers {
			pp := *p
			cp.Peers[id] = &pp
		}
		out = append(out, cp)
	}
	return out
}

// reaper 定期清理超时未汇报的节点。
func (t *Tracker) reaper() {
	ticker := time.NewTicker(t.peerTTL / 2)
	defer ticker.Stop()
	for range ticker.C {
		deadline := time.Now().Add(-t.peerTTL)
		t.mu.Lock()
		for key, sw := range t.swarms {
			for id, p := range sw.Peers {
				if p.LastSeen.Before(deadline) {
					log.Printf("[tracker] 节点 %s (%s) 超时剔除", p.PeerID, p.IP)
					delete(sw.Peers, id)
				}
			}
			if len(sw.Peers) == 0 {
				delete(t.swarms, key)
			}
		}
		t.mu.Unlock()
	}
}
