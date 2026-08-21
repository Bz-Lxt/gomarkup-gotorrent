package metainfo_test

import (
	"os"
	"path/filepath"
	"testing"

	"gotorrent/internal/metainfo"
)

func TestCreateEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.bin")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tf, err := metainfo.Create(path, "http://tracker.invalid/announce", "", 1024)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if tf == nil {
		t.Fatal("Create() returned a nil torrent")
	}
	if tf.Length != 0 {
		t.Fatalf("Length = %d, want 0", tf.Length)
	}
	if tf.NumPieces() != 1 {
		t.Fatalf("NumPieces() = %d, want 1", tf.NumPieces())
	}
	if !tf.VerifyPiece(0, nil) {
		t.Fatal("the only piece does not verify as empty content")
	}

	encoded, err := tf.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	parsed, err := metainfo.Parse(encoded)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.NumPieces() != 1 || !parsed.VerifyPiece(0, nil) {
		t.Fatal("empty-file piece was not preserved by torrent encoding")
	}
}
