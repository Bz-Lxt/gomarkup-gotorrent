// Package announce 实现 Tracker HTTP 客户端（JSON 响应）与 compact 节点列表编解码。
package announce

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client 向 Tracker 发起 announce。
type Client struct {
	HTTP    *http.Client
	Timeout time.Duration
}

// NewClient 创建默认超时 8 秒的客户端。
func NewClient() *Client {
	return &Client{
		Timeout: 8 * time.Second,
		HTTP:    &http.Client{Timeout: 8 * time.Second},
	}
}

// Do 发送一次 announce，返回解析后的响应。
func (c *Client) Do(req Request) (*Response, error) {
	if req.AnnounceURL == "" {
		return nil, fmt.Errorf("缺少 announce URL")
	}
	u, err := url.Parse(req.AnnounceURL)
	if err != nil {
		return nil, fmt.Errorf("announce URL 非法: %w", err)
	}
	q := u.Query()
	q.Set("info_hash", req.InfoHash)
	q.Set("peer_id", req.PeerID)
	q.Set("port", strconv.Itoa(req.Port))
	q.Set("uploaded", strconv.FormatInt(req.Uploaded, 10))
	q.Set("downloaded", strconv.FormatInt(req.Downloaded, 10))
	q.Set("left", strconv.FormatInt(req.Left, 10))
	if req.Name != "" {
		q.Set("name", req.Name)
	}
	if req.Length > 0 {
		q.Set("length", strconv.FormatInt(req.Length, 10))
	}
	if req.Event != EventNone {
		q.Set("event", string(req.Event))
	}
	if req.Compact {
		q.Set("compact", "1")
	}
	if req.NumWant > 0 {
		q.Set("numwant", strconv.Itoa(req.NumWant))
	}
	if req.IP != "" {
		q.Set("ip", req.IP)
	}
	u.RawQuery = q.Encode()

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: c.Timeout}
	}
	resp, err := httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("announce 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("announce HTTP %d: %s", resp.StatusCode, body)
	}
	var ar Response
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("announce 响应解析失败: %w", err)
	}
	if ar.Failure != "" {
		return &ar, fmt.Errorf("tracker 拒绝: %s", ar.Failure)
	}
	return &ar, nil
}
