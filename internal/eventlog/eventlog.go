// Package eventlog 维护有上限的会话事件环形缓冲区，供控制台展示最近活动。
package eventlog

import (
	"sync"
	"time"
)

// Kind 事件类型。
type Kind string

const (
	KindInfo     Kind = "info"
	KindPeer     Kind = "peer"
	KindPiece    Kind = "piece"
	KindAnnounce Kind = "announce"
	KindError    Kind = "error"
	KindState    Kind = "state"
)

// Event 一条事件。
type Event struct {
	Time    time.Time `json:"time"`
	Kind    Kind      `json:"kind"`
	Torrent string    `json:"torrent,omitempty"`
	Message string    `json:"message"`
}

// Log 固定容量的环形日志。
type Log struct {
	mu   sync.Mutex
	cap  int
	buf  []Event
	next int
	size int
}

// New 创建容量为 cap 的日志；cap<=0 时使用 200。
func New(cap int) *Log {
	if cap <= 0 {
		cap = 200
	}
	return &Log{cap: cap, buf: make([]Event, cap)}
}

// Append 追加一条事件。
func (l *Log) Append(kind Kind, torrent, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf[l.next] = Event{Time: time.Now(), Kind: kind, Torrent: torrent, Message: message}
	l.next = (l.next + 1) % l.cap
	if l.size < l.cap {
		l.size++
	}
}

// Recent 返回最近 n 条（从旧到新）。n<=0 表示全部。
func (l *Log) Recent(n int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n <= 0 || n > l.size {
		n = l.size
	}
	out := make([]Event, 0, n)
	start := (l.next - l.size + l.cap) % l.cap
	// 跳过较旧的 size-n 条
	skip := l.size - n
	for i := 0; i < l.size; i++ {
		if i < skip {
			continue
		}
		idx := (start + i) % l.cap
		out = append(out, l.buf[idx])
	}
	return out
}

// Len 返回当前条数。
func (l *Log) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.size
}

// Clear 清空。
func (l *Log) Clear() {
	l.mu.Lock()
	l.next = 0
	l.size = 0
	l.mu.Unlock()
}
