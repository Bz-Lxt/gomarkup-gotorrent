// Package logx 提供带模块前缀与级别的轻量日志封装，底层仍使用标准库 log。
package logx

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// Level 日志级别。
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel 解析级别字符串，无法识别时返回 LevelInfo。
func ParseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return LevelDebug
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// Logger 带模块名的日志器。
type Logger struct {
	mu     sync.Mutex
	module string
	level  Level
	inner  *log.Logger
}

// New 创建日志器。module 会出现在每条日志前缀中。
func New(module string) *Logger {
	return &Logger{
		module: module,
		level:  LevelInfo,
		inner:  log.New(os.Stderr, "", log.LstdFlags),
	}
}

// SetLevel 设置最低输出级别。
func (l *Logger) SetLevel(lv Level) {
	l.mu.Lock()
	l.level = lv
	l.mu.Unlock()
}

// SetOutput 重定向输出。
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	l.inner.SetOutput(w)
	l.mu.Unlock()
}

func (l *Logger) enabled(lv Level) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return lv >= l.level
}

func (l *Logger) logf(lv Level, format string, args ...any) {
	if !l.enabled(lv) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.inner.Printf("[%s] %-5s %s", l.module, lv.String(), msg)
}

// Debugf 输出调试日志。
func (l *Logger) Debugf(format string, args ...any) { l.logf(LevelDebug, format, args...) }

// Infof 输出信息日志。
func (l *Logger) Infof(format string, args ...any) { l.logf(LevelInfo, format, args...) }

// Warnf 输出警告日志。
func (l *Logger) Warnf(format string, args ...any) { l.logf(LevelWarn, format, args...) }

// Errorf 输出错误日志。
func (l *Logger) Errorf(format string, args ...any) { l.logf(LevelError, format, args...) }

// 默认模块日志器，供尚未注入 Logger 的旧代码过渡使用。
var (
	Peer    = New("peer")
	Tracker = New("tracker")
	Session = New("session")
)
