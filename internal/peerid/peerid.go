// Package peerid 负责 Peer ID 的生成与解析。
// 格式遵循 Azureus 风格：-<客户端2字符><版本4字符>-<12 字节随机>，共 20 字节。
// GoTorrent 使用前缀 "-GT0001-"。
package peerid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// Size Peer ID 固定长度。
	Size = 20
	// Prefix GoTorrent 客户端标识与版本号。
	Prefix = "-GT0001-"
	// Alphabet 随机部分使用的字符集（可打印、便于日志）。
	Alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// ID 是 20 字节的节点标识。
type ID [Size]byte

// Generate 生成一个新的 GoTorrent Peer ID。
func Generate() (ID, error) {
	var id ID
	copy(id[:], Prefix)
	var rnd [12]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return ID{}, fmt.Errorf("生成 PeerID 随机部分失败: %w", err)
	}
	for i := 0; i < 12; i++ {
		id[8+i] = Alphabet[int(rnd[i])%len(Alphabet)]
	}
	return id, nil
}

// FromBytes 从恰好 20 字节的切片构造 ID。
func FromBytes(b []byte) (ID, error) {
	if len(b) != Size {
		return ID{}, fmt.Errorf("PeerID 长度必须为 %d，得到 %d", Size, len(b))
	}
	var id ID
	copy(id[:], b)
	return id, nil
}

// Parse 从字符串解析 Peer ID（恰好 20 个字节的 ASCII，或 40 位十六进制）。
func Parse(s string) (ID, error) {
	if len(s) == Size {
		var id ID
		copy(id[:], s)
		return id, nil
	}
	if len(s) == Size*2 {
		raw, err := hex.DecodeString(s)
		if err != nil {
			return ID{}, fmt.Errorf("PeerID 十六进制非法: %w", err)
		}
		return FromBytes(raw)
	}
	return ID{}, fmt.Errorf("PeerID 长度非法: %d", len(s))
}

// String 返回可打印形式（按字节原样转 string）。
func (id ID) String() string { return string(id[:]) }

// Hex 返回 40 位小写十六进制。
func (id ID) Hex() string { return hex.EncodeToString(id[:]) }

// Bytes 返回底层切片副本。
func (id ID) Bytes() []byte {
	out := make([]byte, Size)
	copy(out, id[:])
	return out
}

// Array 返回底层数组。
func (id ID) Array() [Size]byte { return id }

// IsGoTorrent 判断是否为本客户端生成的 ID。
func (id ID) IsGoTorrent() bool {
	return strings.HasPrefix(id.String(), Prefix)
}

// ClientName 根据 Azureus 风格前缀识别常见客户端名称。
func (id ID) ClientName() string {
	s := id.String()
	if len(s) < 8 || s[0] != '-' || s[7] != '-' {
		return "unknown"
	}
	code := s[1:3]
	switch code {
	case "GT":
		return "GoTorrent"
	case "UT":
		return "µTorrent"
	case "TR":
		return "Transmission"
	case "qB":
		return "qBittorrent"
	case "DE":
		return "Deluge"
	case "LT":
		return "libtorrent"
	default:
		return code
	}
}

// Version 返回版本字段（第 3–6 字符），无法识别时返回空串。
func (id ID) Version() string {
	s := id.String()
	if len(s) < 8 || s[0] != '-' || s[7] != '-' {
		return ""
	}
	return s[3:7]
}
