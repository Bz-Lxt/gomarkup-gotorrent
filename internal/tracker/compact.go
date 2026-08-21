package tracker

import "gotorrent/internal/announce"

// CompactPeers 将 Tracker 内部节点列表转为 compact 字节串（仅 IPv4）。
func CompactPeers(peers []*PeerInfo) []byte {
	list := make([]announce.Peer, 0, len(peers))
	for _, p := range peers {
		if p == nil {
			continue
		}
		list = append(list, announce.Peer{PeerID: p.PeerID, IP: p.IP, Port: p.Port})
	}
	return announce.EncodeCompact(list)
}

// ToAnnouncePeers 转为 announce 包的 Peer 切片。
func ToAnnouncePeers(peers []*PeerInfo) []announce.Peer {
	out := make([]announce.Peer, 0, len(peers))
	for _, p := range peers {
		if p == nil {
			continue
		}
		out = append(out, announce.Peer{PeerID: p.PeerID, IP: p.IP, Port: p.Port})
	}
	return out
}
