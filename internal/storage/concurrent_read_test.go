package storage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"gotorrent/internal/metainfo"
	"gotorrent/internal/storage"
)

func TestConcurrentReadBlocksKeepRequestedOffsets(t *testing.T) {
	const (
		pieceCount = 64
		pieceSize  = 64 * 1024
		rounds     = 5
	)

	dir := t.TempDir()
	data := make([]byte, pieceCount*pieceSize)
	for piece := 0; piece < pieceCount; piece++ {
		block := data[piece*pieceSize : (piece+1)*pieceSize]
		for i := range block {
			block[i] = byte(piece + 1)
		}
	}

	path := filepath.Join(dir, "seed.bin")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	tf, err := metainfo.Create(path, "http://tracker.example/announce", "", pieceSize)
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(dir, tf)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	oldProcs := runtime.GOMAXPROCS(8)
	defer runtime.GOMAXPROCS(oldProcs)

	type result struct {
		index int
		data  []byte
		err   error
	}
	for round := 0; round < rounds; round++ {
		start := make(chan struct{})
		results := make(chan result, pieceCount)
		var wg sync.WaitGroup
		for index := 0; index < pieceCount; index++ {
			index := index
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				got, err := store.ReadBlock(index, 0, pieceSize)
				results <- result{index: index, data: got, err: err}
			}()
		}
		close(start)
		wg.Wait()
		close(results)

		for got := range results {
			if got.err != nil {
				t.Fatalf("round %d: ReadBlock(%d) returned error: %v", round, got.index, got.err)
			}
			want := data[got.index*pieceSize : (got.index+1)*pieceSize]
			if !bytes.Equal(got.data, want) {
				t.Fatalf("round %d: concurrent ReadBlock(%d) returned data from another offset", round, got.index)
			}
		}
	}
}
