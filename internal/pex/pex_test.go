package pex

import "testing"

func TestEncodeDecode(t *testing.T) {
	msg := Message{
		Added:   []Peer{{IP: "1.2.3.4", Port: 6881}, {IP: "5.6.7.8", Port: 80}},
		Dropped: []Peer{{IP: "9.9.9.9", Port: 1}},
	}
	raw := Encode(msg)
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Added) != 2 || got.Added[0].Addr() != "1.2.3.4:6881" {
		t.Errorf("Added = %+v", got.Added)
	}
	if len(got.Dropped) != 1 || got.Dropped[0].IP != "9.9.9.9" {
		t.Errorf("Dropped = %+v", got.Dropped)
	}
}

func TestDedup(t *testing.T) {
	in := []Peer{{IP: "1.1.1.1", Port: 1}, {IP: "1.1.1.1", Port: 1}, {IP: "2.2.2.2", Port: 2}}
	out := Dedup(in)
	if len(out) != 2 {
		t.Errorf("Dedup 后长度 = %d", len(out))
	}
}

func TestDecodeTooShort(t *testing.T) {
	if _, err := Decode([]byte{0, 1}); err == nil {
		t.Error("过短应失败")
	}
}
