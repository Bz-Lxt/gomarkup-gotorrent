package tracker

import (
	"testing"
	"time"

	"gotorrent/internal/announce"
)

func TestAnnounceAndLeave(t *testing.T) {
	tr := New(time.Minute)
	var hash [20]byte
	hash[0] = 1
	p1 := PeerInfo{PeerID: "peer-a", IP: "127.0.0.1", Port: 6881, Left: 100}
	others := tr.Announce(hash, p1, "f.bin", 1000, 10)
	if len(others) != 0 {
		t.Fatalf("首个节点不应看到邻居: %d", len(others))
	}
	p2 := PeerInfo{PeerID: "peer-b", IP: "10.0.0.2", Port: 6882, Left: 0}
	others = tr.Announce(hash, p2, "f.bin", 1000, 10)
	if len(others) != 1 || others[0].PeerID != "peer-a" {
		t.Fatalf("应返回 peer-a: %+v", others)
	}
	swarms := tr.Swarms()
	if len(swarms) != 1 {
		t.Fatalf("swarm 数 = %d", len(swarms))
	}
	seeders, leechers := swarms[0].Stats()
	if seeders != 1 || leechers != 1 {
		t.Errorf("seeders=%d leechers=%d", seeders, leechers)
	}
	tr.Leave(hash, "peer-a")
	swarms = tr.Swarms()
	if len(swarms[0].Peers) != 1 {
		t.Errorf("Leave 后应剩 1 个节点")
	}
}

func TestCompactAndCounters(t *testing.T) {
	peers := []*PeerInfo{{PeerID: "x", IP: "1.2.3.4", Port: 80}}
	raw := CompactPeers(peers)
	got, err := announce.DecodeCompact(raw)
	if err != nil || len(got) != 1 || got[0].Port != 80 {
		t.Fatalf("CompactPeers 失败: %v %+v", err, got)
	}
	list := ToAnnouncePeers(peers)
	if len(list) != 1 || list[0].IP != "1.2.3.4" {
		t.Errorf("ToAnnouncePeers = %+v", list)
	}
	c := NewCounters()
	c.IncrAnnounce("started", 10)
	c.IncrAnnounce("completed", 0)
	c.IncrRejected()
	snap := c.Snapshot()
	if snap["started"].(int64) != 1 || snap["rejected"].(int64) != 1 {
		t.Errorf("计数器: %+v", snap)
	}
}
