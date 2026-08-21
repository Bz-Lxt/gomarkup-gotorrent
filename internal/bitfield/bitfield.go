// Package bitfield 实现 BitTorrent 的位图结构，用于记录分片下载进度。
// 每个分片占 1 bit，最高位对应分片 0。
package bitfield

// Bitfield 表示分片位图。
type Bitfield []byte

// New 创建可容纳 n 个分片的位图。
func New(n int) Bitfield {
	return make(Bitfield, (n+7)/8)
}

// HasPiece 判断分片 i 是否已下载。
func (b Bitfield) HasPiece(i int) bool {
	if i < 0 || i/8 >= len(b) {
		return false
	}
	return b[i/8]&(0x80>>uint(i%8)) != 0
}

// SetPiece 将分片 i 标记为已下载。
func (b Bitfield) SetPiece(i int) {
	if i < 0 || i/8 >= len(b) {
		return
	}
	b[i/8] |= 0x80 >> uint(i%8)
}

// ClearPiece 清除分片 i 的标记（校验失败时回退）。
func (b Bitfield) ClearPiece(i int) {
	if i < 0 || i/8 >= len(b) {
		return
	}
	b[i/8] &^= 0x80 >> uint(i%8)
}

// Count 返回已置位的分片数。
func (b Bitfield) Count() int {
	n := 0
	for _, by := range b {
		for by != 0 {
			n += int(by & 1)
			by >>= 1
		}
	}
	return n
}

// Copy 返回位图副本。
func (b Bitfield) Copy() Bitfield {
	out := make(Bitfield, len(b))
	copy(out, b)
	return out
}
