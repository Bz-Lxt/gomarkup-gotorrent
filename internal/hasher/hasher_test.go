package hasher

import (
	"bytes"
	"testing"
)

func TestHashReaderAndSplit(t *testing.T) {
	data := bytes.Repeat([]byte("abcd"), 1000) // 4000 字节
	hashes, err := HashReader(bytes.NewReader(data), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(hashes) != 4 {
		t.Fatalf("分片数 = %d, 期望 4", len(hashes))
	}
	concat := Concat(hashes)
	got, err := Split(concat)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0] != hashes[0] {
		t.Error("Split/Concat 往返不一致")
	}
	if !Verify(data[:1024], hashes[0]) {
		t.Error("第一片校验失败")
	}
	if Verify(data[1:1025], hashes[0]) {
		t.Error("错误数据不应通过")
	}
}

func TestHashReaderEmpty(t *testing.T) {
	hashes, err := HashReader(bytes.NewReader(nil), 256)
	if err != nil {
		t.Fatal(err)
	}
	if len(hashes) != 1 {
		t.Fatalf("空输入应有 1 个哈希, 得到 %d", len(hashes))
	}
	if hashes[0] != Hash(nil) {
		t.Error("空输入哈希不正确")
	}
}

func TestSplitInvalid(t *testing.T) {
	if _, err := Split([]byte("abc")); err == nil {
		t.Error("非整倍数应失败")
	}
	if _, err := HashReader(bytes.NewReader(nil), 0); err == nil {
		t.Error("pieceLength=0 应失败")
	}
}
