package peer_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"gotorrent/internal/announce"
	"gotorrent/internal/bitfield"
	"gotorrent/internal/metainfo"
	"gotorrent/internal/peer"
	"gotorrent/internal/wire"
)

func TestDownloadContinuesRequestingAfterPieceCompletion(t *testing.T) {
	seedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer seedListener.Close()

	_, portText, err := net.SplitHostPort(seedListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	seedPort, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	trackerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := announce.Response{Interval: 60}
		if r.URL.Query().Get("event") != string(announce.EventStopped) {
			response.Peers = []announce.Peer{{
				PeerID: "fake-seeder",
				IP:     "127.0.0.1",
				Port:   seedPort,
			}}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode tracker response: %v", err)
		}
	}))
	defer trackerServer.Close()

	const pieceLength = 4 * 1024
	payload := make([]byte, 3*pieceLength)
	for i := range payload {
		payload[i] = byte((i*31 + 7) % 251)
	}
	sourcePath := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(sourcePath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	tf, err := metainfo.Create(sourcePath, trackerServer.URL, "", pieceLength)
	if err != nil {
		t.Fatal(err)
	}

	seedResult := make(chan error, 1)
	go func() {
		seedResult <- serveFirstPieceAndObserveNext(seedListener, tf, payload)
	}()

	downloader, err := peer.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := downloader.AddDownload(tf)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Stop()

	select {
	case err := <-seedResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("downloader did not complete the initial peer exchange")
	}

	sessions := downloader.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("session count = %d, want 1", len(sessions))
	}
	if got := sessions[0]["completed_pieces"]; got != 1 {
		t.Fatalf("completed pieces = %v, want 1 before the second piece response", got)
	}
}

func serveFirstPieceAndObserveNext(ln net.Listener, tf *metainfo.TorrentFile, payload []byte) error {
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept downloader: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}

	handshake, err := wire.ReadHandshake(conn)
	if err != nil {
		return fmt.Errorf("read downloader handshake: %w", err)
	}
	if handshake.InfoHash != tf.InfoHash {
		return fmt.Errorf("downloader announced unexpected info hash")
	}
	var seedID [20]byte
	copy(seedID[:], "-GT0001-fake-seeder")
	if err := (&wire.Handshake{InfoHash: tf.InfoHash, PeerID: seedID}).Write(conn); err != nil {
		return fmt.Errorf("write seeder handshake: %w", err)
	}

	available := bitfield.New(tf.NumPieces())
	for i := 0; i < tf.NumPieces(); i++ {
		available.SetPiece(i)
	}
	if err := wire.NewBitfield(available).Write(conn); err != nil {
		return fmt.Errorf("write seeder bitfield: %w", err)
	}
	if err := waitForMessage(conn, wire.MsgInterested); err != nil {
		return err
	}
	if err := (&wire.Message{ID: wire.MsgUnchoke}).Write(conn); err != nil {
		return fmt.Errorf("write unchoke: %w", err)
	}

	request, err := waitForRequest(conn)
	if err != nil {
		return err
	}
	firstIndex, begin, length, err := request.ParseRequest()
	if err != nil {
		return err
	}
	start := firstIndex*tf.PieceLength + begin
	end := start + length
	if firstIndex < 0 || firstIndex >= tf.NumPieces() || begin < 0 || start < 0 || end > len(payload) {
		return fmt.Errorf("invalid first request: piece=%d begin=%d length=%d", firstIndex, begin, length)
	}
	if err := wire.NewPiece(firstIndex, begin, payload[start:end]).Write(conn); err != nil {
		return fmt.Errorf("write first piece: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	next, err := waitForRequest(conn)
	if err != nil {
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			return fmt.Errorf("no request for another piece after completing piece %d", firstIndex)
		}
		return err
	}
	nextIndex, _, _, err := next.ParseRequest()
	if err != nil {
		return err
	}
	if nextIndex == firstIndex {
		return fmt.Errorf("piece %d was requested again instead of advancing", firstIndex)
	}
	return nil
}

func waitForMessage(conn net.Conn, id wire.MessageID) error {
	for {
		message, err := wire.Read(conn)
		if err != nil {
			return fmt.Errorf("wait for %s: %w", id, err)
		}
		if message.ID == id {
			return nil
		}
	}
}

func waitForRequest(conn net.Conn) (*wire.Message, error) {
	for {
		message, err := wire.Read(conn)
		if err != nil {
			return nil, err
		}
		if message.ID == wire.MsgRequest {
			return message, nil
		}
	}
}
