// Peer 节点入口：P2P 监听 + HTTP 控制台。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"gotorrent/internal/config"
	"gotorrent/internal/logx"
	"gotorrent/internal/peer"
)

func main() {
	cfgPath := flag.String("config", "", "JSON 配置文件（可覆盖下列默认项）")
	listen := flag.String("listen", "", "P2P 协议监听地址")
	control := flag.String("control", "", "HTTP 控制台监听地址")
	dir := flag.String("dir", "", "数据目录（下载文件与种子存放处）")
	trackerURL := flag.String("tracker", "", "默认 Tracker 地址")
	downLimit := flag.Int64("down-limit", 0, "下载限速（字节/秒，0 不限）")
	upLimit := flag.Int64("up-limit", 0, "上传限速（字节/秒，0 不限）")
	flag.Parse()

	cfg, err := config.LoadPeer(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *control != "" {
		cfg.Control = *control
	}
	if *dir != "" {
		cfg.Dir = *dir
	}
	if *trackerURL != "" {
		cfg.Tracker = *trackerURL
	}
	if *downLimit > 0 {
		cfg.DownLimitBps = *downLimit
	}
	if *upLimit > 0 {
		cfg.UpLimitBps = *upLimit
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	logx.Peer.SetLevel(logx.ParseLevel(cfg.LogLevel))
	p, err := peer.NewWithConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := p.Listen(cfg.Listen); err != nil {
		log.Fatal(err)
	}

	cs := peer.NewControlServer(p, cfg.Tracker)
	fmt.Printf("GoTorrent 节点已启动\n")
	fmt.Printf("  控制台:   http://localhost%s/\n", cfg.Control)
	fmt.Printf("  P2P 端口: %s\n", cfg.Listen)
	fmt.Printf("  数据目录: %s\n", cfg.Dir)
	log.Fatal(http.ListenAndServe(cfg.Control, cs.Handler()))
}
