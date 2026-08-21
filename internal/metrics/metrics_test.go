package metrics

import (
	"testing"
	"time"
)

func TestEWMA(t *testing.T) {
	e := NewEWMA(0.5)
	if e.Add(10) != 10 {
		t.Fatal("首个样本应为自身")
	}
	v := e.Add(0)
	if v < 4 || v > 6 {
		t.Errorf("EWMA(10,0) alpha=0.5 应得 5, 得到 %v", v)
	}
	e.Reset()
	if e.Value() != 0 {
		t.Error("Reset 后应为 0")
	}
}

func TestCountersAndSampler(t *testing.T) {
	c := &Counters{}
	c.AddDown(100)
	c.AddUp(20)
	d, u := c.Snapshot()
	if d != 100 || u != 20 {
		t.Fatalf("Snapshot = %d,%d", d, u)
	}
	s := NewSampler()
	s.lastAt = time.Now().Add(-time.Second)
	s.Tick(1000, 500)
	down, up := s.Rates()
	if down <= 0 || up <= 0 {
		t.Errorf("采样后速率应为正: down=%d up=%d", down, up)
	}
}

func TestRatio(t *testing.T) {
	if Ratio(50, 100) != 0.5 {
		t.Errorf("ratio = %v", Ratio(50, 100))
	}
	if Ratio(10, 0) != 999 {
		t.Error("只上传应返回 999")
	}
	if Ratio(0, 0) != 0 {
		t.Error("全零应为 0")
	}
}
