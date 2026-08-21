// Package util 提供跨模块复用的格式化与校验辅助函数。
package util

import (
	"fmt"
	"math"
)

// FormatBytes 将字节数格式化为人类可读形式（B/KB/MB/GB/TB）。
func FormatBytes(n int64) string {
	if n < 0 {
		return "-" + FormatBytes(-n)
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	if f >= 100 {
		return fmt.Sprintf("%.0f %s", f, units[i])
	}
	if f >= 10 {
		return fmt.Sprintf("%.1f %s", f, units[i])
	}
	return fmt.Sprintf("%.2f %s", f, units[i])
}

// FormatRate 将字节/秒格式化为速率字符串。
func FormatRate(n int64) string {
	return FormatBytes(n) + "/s"
}

// Percent 返回已完成比例（0–100）。分母为 0 时返回 100。
func Percent(done, total int64) float64 {
	if total <= 0 {
		return 100
	}
	p := float64(done) * 100 / float64(total)
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return math.Round(p*10) / 10
}

// ClampInt 将 v 限制在 [lo, hi] 区间。
func ClampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ClampInt64 将 v 限制在 [lo, hi] 区间。
func ClampInt64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
