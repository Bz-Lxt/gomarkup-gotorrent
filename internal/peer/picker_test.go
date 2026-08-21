package peer

import (
	"testing"

	"gotorrent/internal/bitfield"
)

func TestPickerRarestFirst(t *testing.T) {
	owned := bitfield.New(4)
	p := newPiecePicker(4, owned)

	a := bitfield.New(4)
	a.SetPiece(0)
	a.SetPiece(1)
	b := bitfield.New(4)
	b.SetPiece(1)
	p.AddPeer(a)
	p.AddPeer(b)
	// 分片 0 只有 a 有（稀有），分片 1 两人有
	have := bitfield.New(4)
	have.SetPiece(0)
	have.SetPiece(1)
	got := p.Pick(have)
	if got != 0 {
		t.Fatalf("应优先挑选最稀有的分片 0, 得到 %d", got)
	}
	if p.Pick(have) != 1 {
		t.Fatal("下一片应为 1")
	}
	if p.Pick(have) != -1 {
		t.Fatal("没有更多可选分片")
	}
	p.Release(0)
	p.SetOwned(1)
	if !p.Interesting(have) {
		t.Error("释放 0 后应对 0 感兴趣")
	}
	p.RemovePeer(a)
}
