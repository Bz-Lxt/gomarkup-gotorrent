package storage

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"gotorrent/internal/metainfo"
)

func setup(t *testing.T) (string, *metainfo.TorrentFile, []byte) {
	t.Helper()
	dir := t.TempDir()
	data := make([]byte, 700*1024)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	tf, err := metainfo.Create(src, "http://t/announce", "", metainfo.DefaultPieceLength)
	if err != nil {
		t.Fatal(err)
	}
	return dir, tf, data
}

func TestWriteAndResume(t *testing.T) {
	dir, tf, data := setup(t)
	dlDir := filepath.Join(dir, "dl")

	s, err := Open(dlDir, tf)
	if err != nil {
		t.Fatal(err)
	}
	if s.CompletedPieces() != 0 {
		t.Fatal("新存储不应有完成分片")
	}

	// 写入分片 0，然后关闭模拟中断
	if err := s.WritePiece(0, data[:metainfo.DefaultPieceLength]); err != nil {
		t.Fatal(err)
	}
	s.Close()

	// 重新打开，应恢复位图
	s2, err := Open(dlDir, tf)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if !s2.HasPiece(0) {
		t.Error("断点续传：分片 0 的进度丢失")
	}
	if s2.HasPiece(1) {
		t.Error("分片 1 不应已完成")
	}

	// 写入剩余分片
	for i := 1; i < tf.NumPieces(); i++ {
		begin := int64(i) * int64(tf.PieceLength)
		end := begin + int64(tf.PieceSize(i))
		if err := s2.WritePiece(i, data[begin:end]); err != nil {
			t.Fatalf("写入分片 %d 失败: %v", i, err)
		}
	}
	if !s2.Complete() {
		t.Error("应已全部完成")
	}

	// 数据一致性
	got, err := os.ReadFile(filepath.Join(dlDir, "src.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Error("下载结果与源文件不一致")
	}

	// 清理状态后重新打开应为全新进度
	s2.ClearState()
}

func TestWritePieceBadHash(t *testing.T) {
	dir, tf, _ := setup(t)
	s, err := Open(filepath.Join(dir, "dl"), tf)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	bad := make([]byte, tf.PieceSize(0))
	if err := s.WritePiece(0, bad); err == nil {
		t.Error("错误数据应校验失败")
	}
	if s.HasPiece(0) {
		t.Error("校验失败的分片不应置位")
	}
}

func TestMarkSeeding(t *testing.T) {
	dir, tf, _ := setup(t)
	// 把源文件直接放进下载目录，MarkSeeding 应全部置位
	s, err := Open(dir, tf)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	bad, err := s.MarkSeeding()
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 0 {
		t.Errorf("存在校验失败分片: %v", bad)
	}
	if !s.Complete() {
		t.Error("做种标记后应完整")
	}
}
