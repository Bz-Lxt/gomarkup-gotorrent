package tracker

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Server 是 Tracker 的 HTTP 服务。
type Server struct {
	tr       *Tracker
	interval int // 建议的 announce 间隔（秒）
	maxPeers int // 每次最多返回的节点数
	stats    *Counters
}

// NewServer 创建 HTTP 服务。
func NewServer(tr *Tracker, interval, maxPeers int) *Server {
	return &Server{tr: tr, interval: interval, maxPeers: maxPeers, stats: NewCounters()}
}

// Handler 返回路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/announce", s.handleAnnounce)
	mux.HandleFunc("/api/swarms", s.handleSwarms)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/", s.handleDashboard)
	return mux
}

// announceResponse 返回给 Peer 的响应（JSON，简化自标准 bencode 格式）。
type announceResponse struct {
	Interval   int         `json:"interval"`
	Complete   int         `json:"complete"`   // 做种数
	Incomplete int         `json:"incomplete"` // 下载中节点数
	Peers      []*PeerInfo `json:"peers"`
}

func (s *Server) handleAnnounce(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	infoHashHex := q.Get("info_hash")
	hashBytes, err := hex.DecodeString(infoHashHex)
	if err != nil || len(hashBytes) != 20 {
		s.stats.IncrRejected()
		http.Error(w, `{"error":"info_hash 必须是 40 位十六进制"}`, http.StatusBadRequest)
		return
	}
	var infoHash [20]byte
	copy(infoHash[:], hashBytes)

	peerID := q.Get("peer_id")
	if peerID == "" {
		s.stats.IncrRejected()
		http.Error(w, `{"error":"缺少 peer_id"}`, http.StatusBadRequest)
		return
	}
	port, err := strconv.Atoi(q.Get("port"))
	if err != nil || port <= 0 || port > 65535 {
		http.Error(w, `{"error":"port 非法"}`, http.StatusBadRequest)
		return
	}

	// 优先使用显式 ip 参数，否则取来源 IP
	ip := q.Get("ip")
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}

	p := PeerInfo{
		PeerID:     peerID,
		IP:         ip,
		Port:       port,
		Uploaded:   parseInt64(q.Get("uploaded")),
		Downloaded: parseInt64(q.Get("downloaded")),
		Left:       parseInt64(q.Get("left")),
		Name:       q.Get("peer_name"),
	}
	event := q.Get("event")

	s.stats.IncrAnnounce(event, p.Left)

	if event == "stopped" {
		s.tr.Leave(infoHash, peerID)
		log.Printf("[tracker] %-8s peer=%s ip=%s", "stopped", peerID, ip)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(announceResponse{Interval: s.interval})
		return
	}

	peers := s.tr.Announce(infoHash, p, q.Get("name"), parseInt64(q.Get("length")), s.maxPeers)
	if event == "started" || event == "completed" {
		log.Printf("[tracker] %-9s peer=%s ip=%s:%d left=%d", event, peerID, ip, port, p.Left)
	}

	seeders, leechers := 0, 0
	for _, sw := range s.tr.Swarms() {
		if sw.InfoHash == infoHashHex {
			seeders, leechers = sw.Stats()
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(announceResponse{
		Interval:   s.interval,
		Complete:   seeders,
		Incomplete: leechers,
		Peers:      peers,
	})
}

// handleSwarms 返回所有 Swarm 状态（管理页面数据源）。
func (s *Server) handleSwarms(w http.ResponseWriter, r *http.Request) {
	type swarmView struct {
		InfoHash string      `json:"info_hash"`
		Name     string      `json:"name"`
		Length   int64       `json:"length"`
		Seeders  int         `json:"seeders"`
		Leechers int         `json:"leechers"`
		Peers    []*PeerInfo `json:"peers"`
	}
	swarms := s.tr.Swarms()
	views := make([]swarmView, 0, len(swarms))
	for _, sw := range swarms {
		seeders, leechers := sw.Stats()
		peers := make([]*PeerInfo, 0, len(sw.Peers))
		for _, p := range sw.Peers {
			peers = append(peers, p)
		}
		views = append(views, swarmView{
			InfoHash: sw.InfoHash, Name: sw.Name, Length: sw.Length,
			Seeders: seeders, Leechers: leechers, Peers: peers,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"time":   time.Now().Format(time.RFC3339),
		"swarms": views,
		"stats":  s.stats.Snapshot(),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.stats.Snapshot())
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
