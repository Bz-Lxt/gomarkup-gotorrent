package metainfo

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateParseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// 造一个 600KB 的随机文件 -> 3 个分片（256K*2 + 剩余）
	data := make([]byte, 600*1024)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "test.bin")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}

	tf, err := Create(src, "http://127.0.0.1:8080/announce", "测试种子", DefaultPieceLength)
	if err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	if tf.NumPieces() != 3 {
		t.Fatalf("分片数 = %d, 期望 3", tf.NumPieces())
	}
	if got := tf.PieceSize(2); got != 600*1024-2*DefaultPieceLength {
		t.Errorf("最后分片大小 = %d", got)
	}

	encoded, err := tf.Encode()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(encoded)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if parsed.InfoHash != tf.InfoHash {
		t.Error("InfoHash 往返不一致")
	}
	if parsed.Name != "test.bin" || parsed.Length != int64(len(data)) {
		t.Errorf("元信息不一致: %+v", parsed)
	}
	if len(parsed.PieceHashes) != 3 {
		t.Fatalf("解析后分片数 = %d", len(parsed.PieceHashes))
	}

	// 校验分片
	if !parsed.VerifyPiece(0, data[:DefaultPieceLength]) {
		t.Error("分片 0 校验失败")
	}
	if parsed.VerifyPiece(0, data[1:DefaultPieceLength+1]) {
		t.Error("错误数据不应通过校验")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tf, err := Create(src, "http://tracker/announce", "", 4)
	if err != nil {
		t.Fatal(err)
	}
	torrentPath := filepath.Join(dir, "a.txt.torrent")
	if err := tf.Save(torrentPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(torrentPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.InfoHash != tf.InfoHash {
		t.Error("Save/Load 后 InfoHash 不一致")
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse([]byte("not bencode")); err == nil {
		t.Error("期望解析失败")
	}
	if _, err := Parse([]byte("d3:foo3:bare")); err == nil {
		t.Error("缺少 info 应失败")
	}
}
