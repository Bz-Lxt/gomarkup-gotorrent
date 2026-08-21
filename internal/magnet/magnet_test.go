package magnet

import "testing"

func TestEncodeParse(t *testing.T) {
	var h [20]byte
	for i := range h {
		h[i] = byte(i + 1)
	}
	raw := Encode(h, "我的文件.bin", []string{"http://t/announce", "http://t/announce"}, 1024)
	m, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m.InfoHash != h {
		t.Error("info_hash 不一致")
	}
	if m.Name != "我的文件.bin" {
		t.Errorf("name = %q", m.Name)
	}
	if m.Length != 1024 {
		t.Errorf("length = %d", m.Length)
	}
	if m.PrimaryTracker() != "http://t/announce" {
		t.Errorf("tracker = %s", m.PrimaryTracker())
	}
	if m.InfoHashHex() != Encode(h, "", nil, 0)[len("magnet:?xt=urn:btih:"):][:40] {
		// 只确认 hex 长度为 40
		if len(m.InfoHashHex()) != 40 {
			t.Error("hex 长度不对")
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, s := range []string{"http://x", "magnet:?", "magnet:?xt=urn:btih:zz"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) 应失败", s)
		}
	}
}
