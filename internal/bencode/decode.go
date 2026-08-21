package bencode

import (
	"bytes"
	"fmt"
	"io"
)

// Decode 解析 bencode 数据，返回 string（[]byte 形式）、int64、[]any 或 map[string]any。
// 字符串统一返回 []byte，调用方按需转换为 string。
func Decode(data []byte) (any, error) {
	d := &decoder{buf: bytes.NewReader(data)}
	v, err := d.parse()
	if err != nil {
		return nil, err
	}
	if d.buf.Len() != 0 {
		return nil, fmt.Errorf("bencode: 末尾存在 %d 字节多余数据", d.buf.Len())
	}
	return v, nil
}

type decoder struct {
	buf *bytes.Reader
}

func (d *decoder) parse() (any, error) {
	c, err := d.buf.ReadByte()
	if err != nil {
		return nil, err
	}
	switch {
	case c == 'i':
		return d.parseInt()
	case c == 'l':
		return d.parseList()
	case c == 'd':
		return d.parseDict()
	case c >= '0' && c <= '9':
		d.buf.UnreadByte()
		return d.parseString()
	default:
		return nil, fmt.Errorf("bencode: 非法起始字符 %q", c)
	}
}

func (d *decoder) parseInt() (int64, error) {
	s, err := d.readUntil('e')
	if err != nil {
		return 0, err
	}
	var n int64
	if _, err := fmt.Sscanf(string(s), "%d", &n); err != nil {
		return 0, fmt.Errorf("bencode: 非法整数 %q", s)
	}
	return n, nil
}

func (d *decoder) parseString() ([]byte, error) {
	lenStr, err := d.readUntil(':')
	if err != nil {
		return nil, err
	}
	var n int
	if _, err := fmt.Sscanf(string(lenStr), "%d", &n); err != nil || n < 0 {
		return nil, fmt.Errorf("bencode: 非法字符串长度 %q", lenStr)
	}
	out := make([]byte, n)
	if _, err := io.ReadFull(d.buf, out); err != nil {
		return nil, fmt.Errorf("bencode: 字符串数据不足: %w", err)
	}
	return out, nil
}

func (d *decoder) parseList() ([]any, error) {
	var list []any
	for {
		c, err := d.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		if c == 'e' {
			return list, nil
		}
		d.buf.UnreadByte()
		v, err := d.parse()
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
}

func (d *decoder) parseDict() (map[string]any, error) {
	dict := make(map[string]any)
	for {
		c, err := d.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		if c == 'e' {
			return dict, nil
		}
		d.buf.UnreadByte()
		key, err := d.parseString()
		if err != nil {
			return nil, err
		}
		v, err := d.parse()
		if err != nil {
			return nil, err
		}
		dict[string(key)] = v
	}
}

func (d *decoder) readUntil(delim byte) ([]byte, error) {
	var out []byte
	for {
		c, err := d.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		if c == delim {
			return out, nil
		}
		out = append(out, c)
	}
}

// ---- 类型断言辅助函数 ----

// AsString 将解码值转为 string。
func AsString(v any) (string, error) {
	b, ok := v.([]byte)
	if !ok {
		return "", fmt.Errorf("bencode: 期望字符串，得到 %T", v)
	}
	return string(b), nil
}

// AsBytes 将解码值转为 []byte。
func AsBytes(v any) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("bencode: 期望字节串，得到 %T", v)
	}
	return b, nil
}

// AsInt 将解码值转为 int64。
func AsInt(v any) (int64, error) {
	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("bencode: 期望整数，得到 %T", v)
	}
	return n, nil
}

// AsDict 将解码值转为字典。
func AsDict(v any) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("bencode: 期望字典，得到 %T", v)
	}
	return m, nil
}
