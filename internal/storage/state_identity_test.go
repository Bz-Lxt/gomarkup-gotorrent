package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"gotorrent/internal/metainfo"
	"gotorrent/internal/storage"
)

func TestResumeStateIsolatedForTorrentsSharingFirstPiece(t *testing.T) {
	const pieceLength = 4
	firstData := []byte("sameAAAA")
	secondData := []byte("sameBBBB")

	createTorrent := func(path string, data []byte) *metainfo.TorrentFile {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		tf, err := metainfo.Create(path, "http://tracker/announce", "", pieceLength)
		if err != nil {
			t.Fatal(err)
		}
		return tf
	}

	root := t.TempDir()
	firstTorrent := createTorrent(filepath.Join(root, "sources", "first.bin"), firstData)
	secondTorrent := createTorrent(filepath.Join(root, "sources", "second.bin"), secondData)
	if firstTorrent.InfoHash == secondTorrent.InfoHash {
		t.Fatal("test torrents must have distinct content identities")
	}
	if firstTorrent.PieceHashes[0] != secondTorrent.PieceHashes[0] {
		t.Fatal("test torrents must share their first piece")
	}

	downloadDir := filepath.Join(root, "downloads")
	firstStore, err := storage.Open(downloadDir, firstTorrent)
	if err != nil {
		t.Fatal(err)
	}
	if err := firstStore.WritePiece(0, firstData[:pieceLength]); err != nil {
		firstStore.Close()
		t.Fatal(err)
	}
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}

	secondStore, err := storage.Open(downloadDir, secondTorrent)
	if err != nil {
		t.Fatal(err)
	}
	defer secondStore.Close()

	if secondStore.HasPiece(0) {
		piece, err := secondStore.ReadPiece(0)
		if err != nil {
			t.Fatalf("read inherited piece: %v", err)
		}
		if secondTorrent.VerifyPiece(0, piece) {
			t.Fatal("test setup unexpectedly produced a valid piece for the second torrent")
		}
		t.Fatal("a different torrent inherited completed-piece state from the previous download")
	}
}
