package util

import (
	"encoding/hex"
	"fmt"
)

// DecodeHex20 将 40 位十六进制字符串解码为 20 字节。
func DecodeHex20(s string) ([20]byte, error) {
	var out [20]byte
	if len(s) != 40 {
		return out, fmt.Errorf("期望 40 位十六进制，得到长度 %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return out, fmt.Errorf("十六进制非法: %w", err)
	}
	copy(out[:], b)
	return out, nil
}

// EncodeHex20 将 20 字节编码为小写十六进制。
func EncodeHex20(b [20]byte) string {
	return hex.EncodeToString(b[:])
}

// ShortHash 返回 info_hash 的前 8 字节十六进制（便于日志）。
func ShortHash(b [20]byte) string {
	return hex.EncodeToString(b[:8])
}
