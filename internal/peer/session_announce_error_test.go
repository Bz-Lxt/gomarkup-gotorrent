package peer_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gotorrent/internal/announce"
	"gotorrent/internal/metainfo"
	"gotorrent/internal/peer"
)

func TestRejectedAnnounceDoesNotDialReturnedPeers(t *testing.T) {
	trap, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	trapAddr := trap.Addr().(*net.TCPAddr)

	served := make(chan struct{}, 1)
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := announce.Response{Interval: 15}
		if r.URL.Query().Get("event") != string(announce.EventStopped) {
			response.Failure = "swarm temporarily unavailable"
			response.Peers = []announce.Peer{{
				PeerID: "stale-peer",
				IP:     trapAddr.IP.String(),
				Port:   trapAddr.Port,
			}}
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode tracker response: %v", err)
			return
		}
		if r.URL.Query().Get("event") != string(announce.EventStopped) {
			select {
			case served <- struct{}{}:
			default:
			}
		}
	}))
	t.Cleanup(func() {
		tracker.Close()
		_ = trap.Close()
	})

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "rejected-announce.bin")
	contents := []byte("piece contents that are not preallocated zeros")
	if err := os.WriteFile(sourcePath, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	tf, err := metainfo.Create(sourcePath, tracker.URL, "", len(contents))
	if err != nil {
		t.Fatal(err)
	}

	node, err := peer.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := node.AddDownload(tf)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = node.RemoveSession(session.InfoHashHex())
	})

	select {
	case <-served:
	case <-time.After(2 * time.Second):
		t.Fatal("peer did not announce to tracker")
	}

	if err := trap.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := trap.AcceptTCP()
	if err == nil {
		_ = conn.Close()
		t.Fatal("peer dialed an address from a rejected announce response")
	}
	if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("waiting for peer connection: %v", err)
	}
}
