// Package magnet 实现 BitTorrent Magnet URI 的生成与解析（BEP-9 子集）。
//
//	magnet:?xt=urn:btih:<infohash>&dn=<name>&tr=<tracker>&xl=<length>
package magnet

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// URI 解析后的 Magnet 链接。
type URI struct {
	InfoHash [20]byte
	Name     string
	Trackers []string
	Length   int64
}

// Parse 解析 magnet:?... 字符串。
func Parse(raw string) (*URI, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(raw), "magnet:?") {
		return nil, fmt.Errorf("不是 magnet 链接")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("magnet 解析失败: %w", err)
	}
	q := u.Query()
	xt := q.Get("xt")
	hash, err := parseXT(xt)
	if err != nil {
		return nil, err
	}
	m := &URI{InfoHash: hash, Name: q.Get("dn")}
	if xl := q.Get("xl"); xl != "" {
		m.Length, _ = strconv.ParseInt(xl, 10, 64)
	}
	if tr := q.Get("tr"); tr != "" {
		m.Trackers = append(m.Trackers, tr)
	}
	// url.Query 对重复 key 只保留部分实现差异，再手工扫一遍
	for _, pair := range strings.Split(u.RawQuery, "&") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if k == "tr" {
			dec, err := url.QueryUnescape(v)
			if err == nil && dec != "" && !contains(m.Trackers, dec) {
				m.Trackers = append(m.Trackers, dec)
			}
		}
	}
	return m, nil
}

func parseXT(xt string) ([20]byte, error) {
	var out [20]byte
	xt = strings.TrimSpace(xt)
	const prefix = "urn:btih:"
	if !strings.HasPrefix(strings.ToLower(xt), prefix) {
		return out, fmt.Errorf("缺少 xt=urn:btih:<hash>")
	}
	h := xt[len(prefix):]
	switch len(h) {
	case 40:
		b, err := hex.DecodeString(h)
		if err != nil {
			return out, fmt.Errorf("info_hash 十六进制非法: %w", err)
		}
		copy(out[:], b)
		return out, nil
	case 32:
		// Base32（部分客户端使用），这里要求十六进制以保持实现简单
		return out, fmt.Errorf("暂不支持 Base32 info_hash，请使用 40 位十六进制")
	default:
		return out, fmt.Errorf("info_hash 长度非法: %d", len(h))
	}
}

// Encode 生成 Magnet URI。
func Encode(infoHash [20]byte, name string, trackers []string, length int64) string {
	var b strings.Builder
	b.WriteString("magnet:?xt=urn:btih:")
	b.WriteString(hex.EncodeToString(infoHash[:]))
	if name != "" {
		b.WriteString("&dn=")
		b.WriteString(url.QueryEscape(name))
	}
	if length > 0 {
		b.WriteString("&xl=")
		b.WriteString(strconv.FormatInt(length, 10))
	}
	seen := map[string]bool{}
	for _, tr := range trackers {
		if tr == "" || seen[tr] {
			continue
		}
		seen[tr] = true
		b.WriteString("&tr=")
		b.WriteString(url.QueryEscape(tr))
	}
	return b.String()
}

// InfoHashHex 返回 40 位十六进制。
func (m *URI) InfoHashHex() string {
	return hex.EncodeToString(m.InfoHash[:])
}

// PrimaryTracker 返回第一个 Tracker，没有则为空。
func (m *URI) PrimaryTracker() string {
	if len(m.Trackers) == 0 {
		return ""
	}
	return m.Trackers[0]
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
