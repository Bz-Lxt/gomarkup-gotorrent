// Package metrics 统计传输字节、速率与会话级计数器。
package metrics

import (
	"sync"
	"time"
)

// Counters 累计上传/下载字节。
type Counters struct {
	mu         sync.Mutex
	Downloaded int64
	Uploaded   int64
}

// AddDown 增加下载计数。
func (c *Counters) AddDown(n int64) {
	c.mu.Lock()
	c.Downloaded += n
	c.mu.Unlock()
}

// AddUp 增加上传计数。
func (c *Counters) AddUp(n int64) {
	c.mu.Lock()
	c.Uploaded += n
	c.mu.Unlock()
}

// Snapshot 返回当前累计值。
func (c *Counters) Snapshot() (down, up int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Downloaded, c.Uploaded
}

// Sampler 周期性采样累计值，输出瞬时速率（字节/秒）。
type Sampler struct {
	mu       sync.Mutex
	lastDown int64
	lastUp   int64
	lastAt   time.Time
	downEWMA *EWMA
	upEWMA   *EWMA
	DownRate int64
	UpRate   int64
}

// NewSampler 创建速率采样器。
func NewSampler() *Sampler {
	return &Sampler{
		lastAt:   time.Now(),
		downEWMA: NewEWMA(0.4),
		upEWMA:   NewEWMA(0.4),
	}
}

// Tick 用最新累计值更新速率。
func (s *Sampler) Tick(downloaded, uploaded int64) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	dt := now.Sub(s.lastAt).Seconds()
	if dt <= 0 {
		return
	}
	down := float64(downloaded-s.lastDown) / dt
	up := float64(uploaded-s.lastUp) / dt
	if down < 0 {
		down = 0
	}
	if up < 0 {
		up = 0
	}
	s.DownRate = int64(s.downEWMA.Add(down))
	s.UpRate = int64(s.upEWMA.Add(up))
	s.lastDown = downloaded
	s.lastUp = uploaded
	s.lastAt = now
}

// Rates 返回平滑后的下载/上传速率。
func (s *Sampler) Rates() (down, up int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DownRate, s.UpRate
}

// SessionStats 会话级聚合统计，供 API 使用。
type SessionStats struct {
	InfoHash       string `json:"info_hash"`
	Name           string `json:"name"`
	Length         int64  `json:"length"`
	Completed      int    `json:"completed_pieces"`
	TotalPieces    int    `json:"num_pieces"`
	Downloaded     int64  `json:"downloaded"`
	Uploaded       int64  `json:"uploaded"`
	DownRate       int64  `json:"down_rate"`
	UpRate         int64  `json:"up_rate"`
	Peers          int    `json:"peers"`
	State          string `json:"state"`
	Ratio          float64 `json:"ratio"`
}

// Ratio 计算分享率 uploaded/downloaded。
func Ratio(uploaded, downloaded int64) float64 {
	if downloaded <= 0 {
		if uploaded > 0 {
			return 999
		}
		return 0
	}
	return float64(uploaded) / float64(downloaded)
}
