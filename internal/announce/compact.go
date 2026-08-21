package announce

import (
	"encoding/binary"
	"fmt"
	"net"
)

// EncodeCompact 将节点列表编码为 BitTorrent compact 格式（每节点 6 字节：4 字节 IPv4 + 2 字节端口）。
// 非 IPv4 节点会被跳过。
func EncodeCompact(peers []Peer) []byte {
	out := make([]byte, 0, len(peers)*6)
	for _, p := range peers {
		ip := net.ParseIP(p.IP)
		if ip == nil {
			continue
		}
		v4 := ip.To4()
		if v4 == nil {
			continue
		}
		if p.Port <= 0 || p.Port > 65535 {
			continue
		}
		var rec [6]byte
		copy(rec[:4], v4)
		binary.BigEndian.PutUint16(rec[4:], uint16(p.Port))
		out = append(out, rec[:]...)
	}
	return out
}

// DecodeCompact 解析 compact 节点列表。
func DecodeCompact(data []byte) ([]Peer, error) {
	if len(data)%6 != 0 {
		return nil, fmt.Errorf("compact peers 长度 %d 不是 6 的倍数", len(data))
	}
	n := len(data) / 6
	out := make([]Peer, 0, n)
	for i := 0; i < n; i++ {
		off := i * 6
		ip := net.IPv4(data[off], data[off+1], data[off+2], data[off+3]).String()
		port := int(binary.BigEndian.Uint16(data[off+4 : off+6]))
		out = append(out, Peer{IP: ip, Port: port})
	}
	return out, nil
}

// Addr 返回 host:port。
func (p Peer) Addr() string {
	return net.JoinHostPort(p.IP, itoa(p.Port))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [6]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
