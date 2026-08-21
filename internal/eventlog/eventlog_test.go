package eventlog

import "testing"

func TestRing(t *testing.T) {
	l := New(3)
	l.Append(KindInfo, "a", "1")
	l.Append(KindInfo, "a", "2")
	l.Append(KindInfo, "a", "3")
	l.Append(KindInfo, "a", "4")
	if l.Len() != 3 {
		t.Fatalf("Len = %d", l.Len())
	}
	all := l.Recent(0)
	if len(all) != 3 || all[0].Message != "2" || all[2].Message != "4" {
		t.Errorf("Recent 顺序错误: %+v", all)
	}
	last := l.Recent(1)
	if len(last) != 1 || last[0].Message != "4" {
		t.Errorf("Recent(1) = %+v", last)
	}
	l.Clear()
	if l.Len() != 0 {
		t.Error("Clear 后应为空")
	}
}
