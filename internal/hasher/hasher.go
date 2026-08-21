// Package hasher 封装分片 SHA-1 计算，供种子生成与落盘校验复用。
package hasher

import (
	"crypto/sha1"
	"fmt"
	"io"
)

// Size SHA-1 摘要长度。
const Size = 20

// Hash 计算一段数据的 SHA-1。
func Hash(data []byte) [Size]byte {
	return sha1.Sum(data)
}

// Equal 比较两个摘要。
func Equal(a, b [Size]byte) bool {
	return a == b
}

// Concat 将分片哈希拼接为种子文件 pieces 字段。
func Concat(hashes [][Size]byte) []byte {
	out := make([]byte, 0, len(hashes)*Size)
	for _, h := range hashes {
		out = append(out, h[:]...)
	}
	return out
}

// Split 将 pieces 字段拆分为 20 字节哈希列表。
func Split(pieces []byte) ([][Size]byte, error) {
	if len(pieces) == 0 || len(pieces)%Size != 0 {
		return nil, fmt.Errorf("pieces 长度 %d 不是 %d 的正倍数", len(pieces), Size)
	}
	n := len(pieces) / Size
	out := make([][Size]byte, n)
	for i := 0; i < n; i++ {
		copy(out[i][:], pieces[i*Size:(i+1)*Size])
	}
	return out, nil
}

// HashReader 按 pieceLength 切分 r 并逐块计算 SHA-1。
// 空输入会返回一个对空数据的哈希，保证种子至少包含一个分片。
func HashReader(r io.Reader, pieceLength int) ([][Size]byte, error) {
	if pieceLength <= 0 {
		return nil, fmt.Errorf("pieceLength 必须为正: %d", pieceLength)
	}
	var hashes [][Size]byte
	buf := make([]byte, pieceLength)
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			hashes = append(hashes, Hash(buf[:n]))
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取数据失败: %w", err)
		}
	}
	if len(hashes) == 0 {
		hashes = append(hashes, Hash(nil))
	}
	return hashes, nil
}

// Verify 判断 data 的 SHA-1 是否等于 expect。
func Verify(data []byte, expect [Size]byte) bool {
	return Hash(data) == expect
}
