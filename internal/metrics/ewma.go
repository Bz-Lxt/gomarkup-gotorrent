package metrics

import "sync"

// EWMA 指数加权移动平均，用于平滑速率估计。
type EWMA struct {
	mu    sync.Mutex
	alpha float64
	value float64
	inited bool
}

// NewEWMA 创建平滑系数 alpha 的估计器，alpha 越大越贴近最新样本。
func NewEWMA(alpha float64) *EWMA {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.3
	}
	return &EWMA{alpha: alpha}
}

// Add 喂入一个新样本，返回更新后的估计值。
func (e *EWMA) Add(sample float64) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.inited {
		e.value = sample
		e.inited = true
		return e.value
	}
	e.value = e.alpha*sample + (1-e.alpha)*e.value
	return e.value
}

// Value 返回当前估计。
func (e *EWMA) Value() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.value
}

// Reset 清空状态。
func (e *EWMA) Reset() {
	e.mu.Lock()
	e.value = 0
	e.inited = false
	e.mu.Unlock()
}
