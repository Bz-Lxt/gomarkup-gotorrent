package logx

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevels(t *testing.T) {
	if ParseLevel("debug") != LevelDebug || ParseLevel("WARN") != LevelWarn {
		t.Fatal("ParseLevel 失败")
	}
	if LevelInfo.String() != "INFO" || Level(9).String() != "UNKNOWN" {
		t.Fatal("Level.String 失败")
	}
	var buf bytes.Buffer
	l := New("test")
	l.SetOutput(&buf)
	l.SetLevel(LevelWarn)
	l.Debugf("hidden")
	l.Infof("hidden2")
	l.Warnf("shown %d", 1)
	l.Errorf("err")
	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Error("低于阈值的日志不应输出")
	}
	if !strings.Contains(out, "shown 1") || !strings.Contains(out, "ERROR") {
		t.Errorf("输出异常: %s", out)
	}
}
