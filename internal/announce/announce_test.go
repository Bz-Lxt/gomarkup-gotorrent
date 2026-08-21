package announce

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompactRoundTrip(t *testing.T) {
	peers := []Peer{
		{IP: "1.2.3.4", Port: 6881},
		{IP: "8.8.8.8", Port: 80},
		{IP: "::1", Port: 1}, // IPv6 应被跳过
	}
	raw := EncodeCompact(peers)
	if len(raw) != 12 {
		t.Fatalf("compact 长度 = %d, 期望 12", len(raw))
	}
	got, err := DecodeCompact(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].IP != "1.2.3.4" || got[0].Port != 6881 {
		t.Errorf("解码结果异常: %+v", got)
	}
}

func TestDecodeCompactBadLen(t *testing.T) {
	if _, err := DecodeCompact([]byte{1, 2, 3}); err == nil {
		t.Error("非法长度应失败")
	}
}

func TestClientDo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("info_hash") != "aa" {
			t.Errorf("info_hash = %s", r.URL.Query().Get("info_hash"))
		}
		json.NewEncoder(w).Encode(Response{
			Interval: 20,
			Peers:    []Peer{{IP: "10.0.0.1", Port: 6881}},
		})
	}))
	defer srv.Close()

	c := NewClient()
	resp, err := c.Do(Request{
		AnnounceURL: srv.URL,
		InfoHash:    "aa",
		PeerID:      "-GT0001-xxxxxxxxxxxx",
		Port:        6881,
		Event:       EventStarted,
		Name:        "f.bin",
		Length:      100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Interval != 20 || len(resp.Peers) != 1 {
		t.Errorf("响应异常: %+v", resp)
	}
	if resp.Peers[0].Addr() != "10.0.0.1:6881" {
		t.Errorf("Addr = %s", resp.Peers[0].Addr())
	}
}

func TestClientMissingURL(t *testing.T) {
	if _, err := NewClient().Do(Request{}); err == nil {
		t.Error("缺少 URL 应失败")
	}
}
