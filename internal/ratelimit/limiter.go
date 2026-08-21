// Package ratelimit 实现令牌桶限速器，用于限制上传/下载带宽。
package ratelimit

import (
	"sync"
	"time"
)

// Limiter 令牌桶。RateBps 为 0 表示不限速。
type Limiter struct {
	mu       sync.Mutex
	rateBps  int64
	burst    int64
	tokens   float64
	last     time.Time
}

// New 创建限速器。rateBps 为每秒允许的字节数，burst 为桶容量（默认等于 1 秒额度）。
func New(rateBps int64) *Limiter {
	burst := rateBps
	if burst < 16*1024 {
		burst = 16 * 1024
	}
	return &Limiter{
		rateBps: rateBps,
		burst:   burst,
		tokens:  float64(burst),
		last:    time.Now(),
	}
}

// SetRate 动态调整速率。
func (l *Limiter) SetRate(rateBps int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refillLocked(time.Now())
	l.rateBps = rateBps
	if rateBps > l.burst {
		l.burst = rateBps
	}
}

// Rate 返回当前速率上限。
func (l *Limiter) Rate() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rateBps
}

// Wait 阻塞直到可以消耗 n 字节的额度。n<=0 或未限速时立即返回。
func (l *Limiter) Wait(n int) {
	if n <= 0 {
		return
	}
	for {
		wait := l.reserve(n)
		if wait <= 0 {
			return
		}
		time.Sleep(wait)
	}
}

// Allow 尝试立即消耗 n 字节，成功返回 true。
func (l *Limiter) Allow(n int) bool {
	if n <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.refillLocked(now)
	if l.rateBps <= 0 {
		return true
	}
	need := float64(n)
	if l.tokens >= need {
		l.tokens -= need
		return true
	}
	return false
}

func (l *Limiter) reserve(n int) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.refillLocked(now)
	if l.rateBps <= 0 {
		return 0
	}
	need := float64(n)
	if l.tokens >= need {
		l.tokens -= need
		return 0
	}
	deficit := need - l.tokens
	l.tokens = 0
	sec := deficit / float64(l.rateBps)
	return time.Duration(sec * float64(time.Second))
}

func (l *Limiter) refillLocked(now time.Time) {
	if l.rateBps <= 0 {
		l.tokens = float64(l.burst)
		l.last = now
		return
	}
	elapsed := now.Sub(l.last).Seconds()
	if elapsed <= 0 {
		return
	}
	l.tokens += elapsed * float64(l.rateBps)
	if l.tokens > float64(l.burst) {
		l.tokens = float64(l.burst)
	}
	l.last = now
}
