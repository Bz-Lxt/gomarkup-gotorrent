// Tracker 服务器入口。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"gotorrent/internal/config"
	"gotorrent/internal/logx"
	"gotorrent/internal/tracker"
)

func main() {
	cfgPath := flag.String("config", "", "JSON 配置文件")
	addr := flag.String("addr", "", "HTTP 监听地址")
	interval := flag.Int("interval", 0, "建议 Peer 汇报间隔（秒）")
	peerTTL := flag.Int("peer-ttl", 0, "节点超时剔除时间（秒）")
	maxPeers := flag.Int("max-peers", 0, "每次 announce 最多返回的节点数")
	flag.Parse()

	cfg, err := config.LoadTracker(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	if *addr != "" {
		cfg.Addr = *addr
	}
	if *interval > 0 {
		cfg.Interval = *interval
	}
	if *peerTTL > 0 {
		cfg.PeerTTL = *peerTTL
	}
	if *maxPeers > 0 {
		cfg.MaxPeers = *maxPeers
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	logx.Tracker.SetLevel(logx.ParseLevel(cfg.LogLevel))
	tr := tracker.New(cfg.PeerTTLDuration())
	srv := tracker.NewServer(tr, cfg.Interval, cfg.MaxPeers)

	fmt.Printf("GoTorrent Tracker 已启动\n")
	fmt.Printf("  管理页面:  http://localhost%s/\n", cfg.Addr)
	fmt.Printf("  Announce:  http://localhost%s/announce\n", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, srv.Handler()))
}
