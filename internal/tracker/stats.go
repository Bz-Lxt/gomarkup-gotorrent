package tracker

import (
	"sync"
	"time"
)

// Counters Tracker 全局计数器。
type Counters struct {
	mu            sync.Mutex
	Announces     int64
	Started       int64
	Completed     int64
	Stopped       int64
	Rejected      int64
	BytesReported int64
	StartedAt     time.Time
}

// NewCounters 创建计数器。
func NewCounters() *Counters {
	return &Counters{StartedAt: time.Now()}
}

// IncrAnnounce 记录一次 announce。
func (c *Counters) IncrAnnounce(event string, left int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Announces++
	switch event {
	case "started":
		c.Started++
	case "completed":
		c.Completed++
	case "stopped":
		c.Stopped++
	}
	if left == 0 && event != "stopped" {
		// 做种汇报不额外计数
	}
}

// IncrRejected 记录一次非法请求。
func (c *Counters) IncrRejected() {
	c.mu.Lock()
	c.Rejected++
	c.mu.Unlock()
}

// Snapshot 返回可 JSON 序列化的快照。
func (c *Counters) Snapshot() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{
		"announces":  c.Announces,
		"started":    c.Started,
		"completed":  c.Completed,
		"stopped":    c.Stopped,
		"rejected":   c.Rejected,
		"uptime_sec": int64(time.Since(c.StartedAt).Seconds()),
	}
}
