package fileset

import "testing"

func TestLayoutMap(t *testing.T) {
	s := &Set{
		Name: "album",
		Files: []File{
			{Path: []string{"a.mp3"}, Length: 100},
			{Path: []string{"dir", "b.mp3"}, Length: 50},
			{Path: []string{"c.mp3"}, Length: 20},
		},
	}
	if s.TotalLength() != 170 {
		t.Fatalf("TotalLength = %d", s.TotalLength())
	}
	lay, err := NewLayout(s)
	if err != nil {
		t.Fatal(err)
	}
	spans, err := lay.Map(80, 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 2 {
		t.Fatalf("跨文件数 = %d, 期望 2", len(spans))
	}
	if spans[0].FileIndex != 0 || spans[0].FileOffset != 80 || spans[0].Length != 20 {
		t.Errorf("span0 = %+v", spans[0])
	}
	if spans[1].FileIndex != 1 || spans[1].FileOffset != 0 || spans[1].Length != 20 {
		t.Errorf("span1 = %+v", spans[1])
	}
}

func TestRejectTraversal(t *testing.T) {
	s := &Set{Name: "x", Files: []File{{Path: []string{"..", "etc"}, Length: 1}}}
	if err := s.Validate(); err == nil {
		t.Error("路径穿越应被拒绝")
	}
}

func TestSingle(t *testing.T) {
	s := Single("f.bin", 10)
	if s.TotalLength() != 10 || s.Files[0].RelPath() != "f.bin" {
		t.Errorf("Single 异常: %+v", s)
	}
}

func TestPieceSpans(t *testing.T) {
	s := Single("f.bin", 300)
	lay, err := NewLayout(s)
	if err != nil {
		t.Fatal(err)
	}
	spans, err := lay.PieceSpans(1, 100, 300)
	if err != nil || len(spans) != 1 || spans[0].FileOffset != 100 {
		t.Errorf("PieceSpans = %+v err=%v", spans, err)
	}
}

func TestMapOutOfRange(t *testing.T) {
	s := Single("f.bin", 10)
	lay, _ := NewLayout(s)
	if _, err := lay.Map(0, 11); err == nil {
		t.Error("越界应失败")
	}
}
