package fileset

import "fmt"

// Span 描述一段逻辑字节区间落在哪个文件的哪个偏移。
type Span struct {
	FileIndex  int
	FileOffset int64
	Length     int64
}

// Layout 将连续逻辑偏移映射到各个文件。
type Layout struct {
	set     *Set
	offsets []int64 // 每个文件的起始逻辑偏移
}

// NewLayout 根据文件集合构建布局。
func NewLayout(s *Set) (*Layout, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	off := make([]int64, len(s.Files))
	var cur int64
	for i, f := range s.Files {
		off[i] = cur
		cur += f.Length
	}
	return &Layout{set: s, offsets: off}, nil
}

// Map 将 [offset, offset+length) 映射为一组 Span。
func (l *Layout) Map(offset, length int64) ([]Span, error) {
	if offset < 0 || length < 0 {
		return nil, fmt.Errorf("偏移/长度非法: offset=%d length=%d", offset, length)
	}
	total := l.set.TotalLength()
	if offset+length > total {
		return nil, fmt.Errorf("超出文件总长度: %d+%d > %d", offset, length, total)
	}
	if length == 0 {
		return nil, nil
	}
	var spans []Span
	remain := length
	cur := offset
	for i := 0; i < len(l.set.Files) && remain > 0; i++ {
		start := l.offsets[i]
		end := start + l.set.Files[i].Length
		if cur >= end || cur+remain <= start {
			continue
		}
		from := cur
		if from < start {
			from = start
		}
		to := cur + remain
		if to > end {
			to = end
		}
		n := to - from
		spans = append(spans, Span{
			FileIndex:  i,
			FileOffset: from - start,
			Length:     n,
		})
		remain -= n
		cur += n
	}
	return spans, nil
}

// PieceSpans 返回第 pieceIndex 个分片覆盖的文件区间。
func (l *Layout) PieceSpans(pieceIndex, pieceLength int, fileLength int64) ([]Span, error) {
	off := int64(pieceIndex) * int64(pieceLength)
	size := int64(pieceLength)
	if off+size > fileLength {
		size = fileLength - off
	}
	if size < 0 {
		return nil, fmt.Errorf("分片 %d 超出文件", pieceIndex)
	}
	return l.Map(off, size)
}
