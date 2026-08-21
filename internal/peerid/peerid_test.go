package peerid

import "testing"

func TestGenerate(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if !id.IsGoTorrent() {
		t.Errorf("应识别为 GoTorrent: %s", id)
	}
	if id.ClientName() != "GoTorrent" {
		t.Errorf("ClientName = %s", id.ClientName())
	}
	if id.Version() != "0001" {
		t.Errorf("Version = %s", id.Version())
	}
	if got, err := Parse(id.String()); err != nil || got != id {
		t.Errorf("Parse(String) 往返失败: %v %v", got, err)
	}
	if got, err := Parse(id.Hex()); err != nil || got != id {
		t.Errorf("Parse(Hex) 往返失败: %v %v", got, err)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse("short"); err == nil {
		t.Error("过短应失败")
	}
	if _, err := FromBytes([]byte("xx")); err == nil {
		t.Error("长度不对应失败")
	}
}

func TestClientNameUnknown(t *testing.T) {
	var id ID
	copy(id[:], "not-azureus-style!!!!")
	if id.ClientName() != "unknown" {
		t.Errorf("期望 unknown, 得到 %s", id.ClientName())
	}
}
