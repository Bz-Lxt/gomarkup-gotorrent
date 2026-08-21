package ratelimit

import (
	"testing"
	"time"
)

func TestUnlimited(t *testing.T) {
	l := New(0)
	if !l.Allow(1 << 20) {
		t.Error("未限速应立即允许")
	}
	start := time.Now()
	l.Wait(1 << 20)
	if time.Since(start) > 50*time.Millisecond {
		t.Error("未限速不应阻塞")
	}
}

func TestAllowAndWait(t *testing.T) {
	l := New(2000)
	if !l.Allow(100) {
		t.Fatal("首次应允许")
	}
	l.mu.Lock()
	l.tokens = 0
	l.last = time.Now()
	l.mu.Unlock()
	start := time.Now()
	l.Wait(500) // 2000 B/s 下约 250ms
	if time.Since(start) < 80*time.Millisecond {
		t.Error("限速 Wait 应有可见延迟")
	}
}

func TestSetRate(t *testing.T) {
	l := New(100)
	l.SetRate(0)
	if l.Rate() != 0 {
		t.Error("SetRate(0) 后 Rate 应为 0")
	}
}
