// Package config 定义 Tracker 与 Peer 的运行时配置，支持默认值、JSON 文件与校验。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// PeerConfig 是节点进程的配置。
type PeerConfig struct {
	Listen         string `json:"listen"`          // P2P 监听地址
	Control        string `json:"control"`         // HTTP 控制台地址
	Dir            string `json:"dir"`             // 数据目录
	Tracker        string `json:"tracker"`         // 默认 Tracker
	MaxPeers       int    `json:"max_peers"`       // 单会话最大外连数
	UnchokeSlots   int    `json:"unchoke_slots"`   // Tit-for-Tat 固定放行数
	DownLimitBps   int64  `json:"down_limit_bps"`  // 下载限速，0 表示不限
	UpLimitBps     int64  `json:"up_limit_bps"`    // 上传限速，0 表示不限
	AnnounceSec    int    `json:"announce_sec"`    // 默认汇报间隔
	LogLevel       string `json:"log_level"`       // DEBUG/INFO/WARN/ERROR
}

// TrackerConfig 是 Tracker 进程的配置。
type TrackerConfig struct {
	Addr      string `json:"addr"`
	Interval  int    `json:"interval"`
	PeerTTL   int    `json:"peer_ttl_sec"`
	MaxPeers  int    `json:"max_peers"`
	LogLevel  string `json:"log_level"`
}

// DefaultPeer 返回节点默认配置。
func DefaultPeer() *PeerConfig {
	return &PeerConfig{
		Listen:       ":6881",
		Control:      ":9000",
		Dir:          "downloads",
		Tracker:      "http://localhost:8080/announce",
		MaxPeers:     30,
		UnchokeSlots: 3,
		AnnounceSec:  15,
		LogLevel:     "INFO",
	}
}

// DefaultTracker 返回 Tracker 默认配置。
func DefaultTracker() *TrackerConfig {
	return &TrackerConfig{
		Addr:     ":8080",
		Interval: 15,
		PeerTTL:  90,
		MaxPeers: 50,
		LogLevel: "INFO",
	}
}

// Validate 校验节点配置。
func (c *PeerConfig) Validate() error {
	if c.Listen == "" {
		return fmt.Errorf("listen 不能为空")
	}
	if c.Control == "" {
		return fmt.Errorf("control 不能为空")
	}
	if c.Dir == "" {
		return fmt.Errorf("dir 不能为空")
	}
	if c.MaxPeers < 1 {
		c.MaxPeers = 30
	}
	if c.UnchokeSlots < 1 {
		c.UnchokeSlots = 3
	}
	if c.AnnounceSec < 5 {
		c.AnnounceSec = 15
	}
	if c.DownLimitBps < 0 {
		c.DownLimitBps = 0
	}
	if c.UpLimitBps < 0 {
		c.UpLimitBps = 0
	}
	return nil
}

// Validate 校验 Tracker 配置。
func (c *TrackerConfig) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("addr 不能为空")
	}
	if c.Interval < 5 {
		c.Interval = 15
	}
	if c.PeerTTL < 15 {
		c.PeerTTL = 90
	}
	if c.MaxPeers < 1 {
		c.MaxPeers = 50
	}
	return nil
}

// PeerTTLDuration 返回节点超时时间。
func (c *TrackerConfig) PeerTTLDuration() time.Duration {
	return time.Duration(c.PeerTTL) * time.Second
}

// LoadPeer 从 JSON 文件加载节点配置；文件不存在时返回默认配置。
func LoadPeer(path string) (*PeerConfig, error) {
	c := DefaultPeer()
	if path == "" {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// LoadTracker 从 JSON 文件加载 Tracker 配置。
func LoadTracker(path string) (*TrackerConfig, error) {
	c := DefaultTracker()
	if path == "" {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Save 将节点配置写回 JSON 文件。
func (c *PeerConfig) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Save 将 Tracker 配置写回 JSON 文件。
func (c *TrackerConfig) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
