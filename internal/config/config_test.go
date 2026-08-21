package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAndValidate(t *testing.T) {
	p := DefaultPeer()
	p.MaxPeers = 0
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if p.MaxPeers != 30 {
		t.Errorf("MaxPeers 默认修正失败: %d", p.MaxPeers)
	}
	tr := DefaultTracker()
	tr.Interval = 1
	if err := tr.Validate(); err != nil {
		t.Fatal(err)
	}
	if tr.Interval != 15 {
		t.Errorf("Interval 默认修正失败: %d", tr.Interval)
	}
	if tr.PeerTTLDuration().Seconds() != float64(tr.PeerTTL) {
		t.Error("PeerTTLDuration 不一致")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peer.json")
	c := DefaultPeer()
	c.Dir = "/tmp/x"
	c.DownLimitBps = 1024
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPeer(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dir != "/tmp/x" || got.DownLimitBps != 1024 {
		t.Errorf("Load 结果: %+v", got)
	}
	trPath := filepath.Join(dir, "tr.json")
	tr := DefaultTracker()
	tr.Addr = ":9999"
	if err := tr.Save(trPath); err != nil {
		t.Fatal(err)
	}
	gotTr, err := LoadTracker(trPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotTr.Addr != ":9999" {
		t.Errorf("Tracker Addr = %s", gotTr.Addr)
	}
}

func TestLoadMissing(t *testing.T) {
	c, err := LoadPeer(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || c.Listen == "" {
		t.Errorf("缺失文件应返回默认: %v %+v", err, c)
	}
	if _, err := LoadPeer(""); err != nil {
		t.Error(err)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(p, []byte("{"), 0o644)
	if _, err := LoadPeer(p); err == nil {
		t.Error("非法 JSON 应失败")
	}
}
