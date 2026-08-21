// Package pex 实现简化版 Peer Exchange（BEP-11 子集）：在已连接节点之间交换邻居地址。
// 编码采用 compact IPv4 列表（每节点 6 字节）。
package pex

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// Peer 一个可交换的邻居。
type Peer struct {
	IP   string
	Port int
}

// Message PEX 消息：新增与删除的节点。
type Message struct {
	Added   []Peer
	Dropped []Peer
}

// Encode 将消息编码为 payload：<nAdded:2><added...><nDropped:2><dropped...>。
func Encode(m Message) []byte {
	added := compact(m.Added)
	dropped := compact(m.Dropped)
	out := make([]byte, 4+len(added)+len(dropped))
	binary.BigEndian.PutUint16(out[0:2], uint16(len(m.Added)))
	copy(out[2:], added)
	off := 2 + len(added)
	binary.BigEndian.PutUint16(out[off:off+2], uint16(len(m.Dropped)))
	copy(out[off+2:], dropped)
	return out
}

// Decode 解析 PEX payload。
func Decode(data []byte) (Message, error) {
	var msg Message
	if len(data) < 4 {
		return msg, fmt.Errorf("PEX 消息过短: %d", len(data))
	}
	nAdded := int(binary.BigEndian.Uint16(data[0:2]))
	need := 2 + nAdded*6
	if len(data) < need+2 {
		return msg, fmt.Errorf("PEX added 段截断")
	}
	var err error
	msg.Added, err = expand(data[2:need], nAdded)
	if err != nil {
		return msg, err
	}
	nDropped := int(binary.BigEndian.Uint16(data[need : need+2]))
	rest := data[need+2:]
	if len(rest) < nDropped*6 {
		return msg, fmt.Errorf("PEX dropped 段截断")
	}
	msg.Dropped, err = expand(rest[:nDropped*6], nDropped)
	return msg, err
}

func compact(peers []Peer) []byte {
	out := make([]byte, 0, len(peers)*6)
	for _, p := range peers {
		ip := net.ParseIP(p.IP).To4()
		if ip == nil || p.Port <= 0 || p.Port > 65535 {
			continue
		}
		var rec [6]byte
		copy(rec[:4], ip)
		binary.BigEndian.PutUint16(rec[4:], uint16(p.Port))
		out = append(out, rec[:]...)
	}
	return out
}

func expand(data []byte, n int) ([]Peer, error) {
	if len(data) != n*6 {
		return nil, fmt.Errorf("compact 长度不匹配")
	}
	out := make([]Peer, 0, n)
	for i := 0; i < n; i++ {
		off := i * 6
		out = append(out, Peer{
			IP:   net.IPv4(data[off], data[off+1], data[off+2], data[off+3]).String(),
			Port: int(binary.BigEndian.Uint16(data[off+4 : off+6])),
		})
	}
	return out, nil
}

// Addr 返回 host:port。
func (p Peer) Addr() string {
	return net.JoinHostPort(p.IP, fmt.Sprintf("%d", p.Port))
}

// Dedup 按 IP:Port 去重。
func Dedup(peers []Peer) []Peer {
	seen := make(map[string]bool, len(peers))
	out := make([]Peer, 0, len(peers))
	for _, p := range peers {
		k := p.Addr()
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

// Interval 建议的 PEX 交换周期。
const Interval = 60 * time.Second
