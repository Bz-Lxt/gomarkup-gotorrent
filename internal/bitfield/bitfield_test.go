package bitfield

import "testing"

func TestSetAndHas(t *testing.T) {
	b := New(20)
	if b.HasPiece(0) || b.HasPiece(19) {
		t.Fatal("新位图不应有任何置位")
	}
	for _, i := range []int{0, 7, 8, 19} {
		b.SetPiece(i)
		if !b.HasPiece(i) {
			t.Errorf("分片 %d 应已置位", i)
		}
	}
	if b.HasPiece(1) || b.HasPiece(9) {
		t.Error("未置位的分片返回了 true")
	}
	if got := b.Count(); got != 4 {
		t.Errorf("Count = %d, 期望 4", got)
	}
}

func TestClear(t *testing.T) {
	b := New(10)
	b.SetPiece(3)
	b.ClearPiece(3)
	if b.HasPiece(3) {
		t.Error("清除后仍置位")
	}
}

func TestOutOfRange(t *testing.T) {
	b := New(4)
	b.SetPiece(100) // 不应 panic
	if b.HasPiece(100) {
		t.Error("越界分片应为 false")
	}
}

func TestCopy(t *testing.T) {
	b := New(8)
	b.SetPiece(2)
	c := b.Copy()
	c.SetPiece(3)
	if b.HasPiece(3) {
		t.Error("Copy 与原位图共享了底层数组")
	}
}
