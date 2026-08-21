// Package bencode 实现 BitTorrent 使用的 bencode 编码格式。
// 支持四种类型：字符串（[]byte）、整数（int64）、列表、字典。
package bencode

import (
	"bytes"
	"fmt"
	"sort"
)

// Encode 将 value 编码为 bencode 格式。
// 支持的 Go 类型：string、[]byte、int、int64、[]any、map[string]any。
func Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeValue(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeValue(buf *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case string:
		fmt.Fprintf(buf, "%d:%s", len(val), val)
	case []byte:
		fmt.Fprintf(buf, "%d:", len(val))
		buf.Write(val)
	case int:
		fmt.Fprintf(buf, "i%de", val)
	case int64:
		fmt.Fprintf(buf, "i%de", val)
	case []any:
		buf.WriteByte('l')
		for _, item := range val {
			if err := encodeValue(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	case map[string]any:
		// bencode 字典要求 key 按字典序排列
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('d')
		for _, k := range keys {
			fmt.Fprintf(buf, "%d:%s", len(k), k)
			if err := encodeValue(buf, val[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	default:
		return fmt.Errorf("bencode: 不支持的类型 %T", v)
	}
	return nil
}
