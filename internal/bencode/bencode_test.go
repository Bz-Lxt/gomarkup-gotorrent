package bencode

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []any{
		"hello",
		int64(-42),
		[]any{"spam", int64(42)},
		map[string]any{
			"announce": "http://tracker/announce",
			"info": map[string]any{
				"name":         "file.bin",
				"length":       int64(1024),
				"piece length": int64(262144),
				"pieces":       []byte("\x01\x02\x03"),
			},
		},
	}
	for _, c := range cases {
		data, err := Encode(c)
		if err != nil {
			t.Fatalf("Encode(%v) 失败: %v", c, err)
		}
		got, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(%q) 失败: %v", data, err)
		}
		if !reflect.DeepEqual(normalize(c), got) {
			t.Errorf("往返不一致:\n编码前: %#v\n解码后: %#v", normalize(c), got)
		}
	}
}

// normalize 将 string 统一转为 []byte 便于比较。
func normalize(v any) any {
	switch val := v.(type) {
	case string:
		return []byte(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = normalize(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[k] = normalize(item)
		}
		return out
	default:
		return v
	}
}

func TestEncodeDictKeySorted(t *testing.T) {
	data, err := Encode(map[string]any{"b": int64(1), "a": int64(2)})
	if err != nil {
		t.Fatal(err)
	}
	want := "d1:ai2e1:bi1ee"
	if string(data) != want {
		t.Errorf("字典 key 未排序: got %q want %q", data, want)
	}
}

func TestDecodeTrailingData(t *testing.T) {
	if _, err := Decode([]byte("i1eX")); err == nil {
		t.Error("期望对多余数据报错")
	}
}

func TestDecodeInvalid(t *testing.T) {
	for _, s := range []string{"", "x", "i1", "3:ab", "d1:a"} {
		if _, err := Decode([]byte(s)); err == nil {
			t.Errorf("Decode(%q) 期望报错", s)
		}
	}
}
