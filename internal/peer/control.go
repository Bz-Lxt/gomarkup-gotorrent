package peer

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"gotorrent/internal/magnet"
	"gotorrent/internal/metainfo"
)

// ControlServer 是节点的 HTTP 控制台（操作界面 + API）。
type ControlServer struct {
	peer           *Peer
	defaultTracker string
}

// NewControlServer 创建控制台。
func NewControlServer(p *Peer, defaultTracker string) *ControlServer {
	return &ControlServer{peer: p, defaultTracker: defaultTracker}
}

// Handler 返回路由。
func (cs *ControlServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", cs.handleConsole)
	mux.HandleFunc("/api/info", cs.handleInfo)
	mux.HandleFunc("/api/torrents", cs.handleTorrents)
	mux.HandleFunc("/api/seed", cs.handleSeed)
	mux.HandleFunc("/api/download", cs.handleDownload)
	mux.HandleFunc("/api/remove", cs.handleRemove)
	mux.HandleFunc("/api/magnet", cs.handleMagnet)
	mux.HandleFunc("/api/events", cs.handleEvents)
	return mux
}

func (cs *ControlServer) handleConsole(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(consoleHTML))
}

func (cs *ControlServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"peer_id": cs.peer.ID(),
		"port":    cs.peer.Port(),
		"dir":     cs.peer.Dir(),
		"tracker": cs.defaultTracker,
	})
}

func (cs *ControlServer) handleTorrents(w http.ResponseWriter, r *http.Request) {
	sessions := cs.peer.Sessions()
	for _, s := range sessions {
		if bf, err := cs.peer.SessionBitfield(s["info_hash"].(string)); err == nil {
			s["bitfield"] = base64.StdEncoding.EncodeToString(bf)
		}
	}
	writeJSON(w, map[string]any{"torrents": sessions})
}

// handleSeed 做种接口：{"path": "/abs/path/file.bin", "tracker": "http://..."}
func (cs *ControlServer) handleSeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path    string `json:"path"`
		Tracker string `json:"tracker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "请求格式错误: "+err.Error())
		return
	}
	if req.Path == "" {
		writeErr(w, "缺少文件路径")
		return
	}
	if req.Tracker == "" {
		req.Tracker = cs.defaultTracker
	}
	s, err := cs.peer.AddSeed(req.Path, req.Tracker)
	if err != nil {
		writeErr(w, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "info_hash": s.InfoHashHex(), "name": s.tf.Name})
}

// handleDownload 下载接口：支持 multipart 上传 .torrent 文件，或 JSON {"path": "..."}。
func (cs *ControlServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data []byte
	ct := r.Header.Get("Content-Type")
	if ct == "application/json" {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
			writeErr(w, "缺少种子文件路径")
			return
		}
		tf, err := metainfo.Load(req.Path)
		if err != nil {
			writeErr(w, "种子加载失败: "+err.Error())
			return
		}
		cs.startDownload(w, tf)
		return
	}

	// multipart 上传
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, "请上传 .torrent 文件")
		return
	}
	defer file.Close()
	data, err = io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil {
		writeErr(w, "读取上传文件失败")
		return
	}
	tf, err := metainfo.Parse(data)
	if err != nil {
		writeErr(w, "种子解析失败: "+err.Error())
		return
	}
	cs.startDownload(w, tf)
}

func (cs *ControlServer) startDownload(w http.ResponseWriter, tf *metainfo.TorrentFile) {
	s, err := cs.peer.AddDownload(tf)
	if err != nil {
		writeErr(w, err.Error())
		return
	}
	log.Printf("[control] 添加下载 %s (info_hash=%s)", tf.Name, s.InfoHashHex())
	writeJSON(w, map[string]any{"ok": true, "info_hash": s.InfoHashHex(), "name": tf.Name})
}

// handleRemove 移除任务：{"info_hash": "..."}
func (cs *ControlServer) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		InfoHash string `json:"info_hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "请求格式错误")
		return
	}
	if err := cs.peer.RemoveSession(req.InfoHash); err != nil {
		writeErr(w, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleMagnet 通过 magnet 链接添加下载：{"magnet":"magnet:?xt=..."}。
// 需要数据目录中已有对应 .torrent（Magnet 本身不含分片哈希）。
func (cs *ControlServer) handleMagnet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Magnet string `json:"magnet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Magnet == "" {
		writeErr(w, "缺少 magnet 链接")
		return
	}
	uri, err := magnet.Parse(req.Magnet)
	if err != nil {
		writeErr(w, err.Error())
		return
	}
	tf, err := cs.peer.FindTorrentByHash(uri.InfoHash)
	if err != nil {
		writeErr(w, "无法仅凭 magnet 开始下载（缺少分片哈希）。请先获取 .torrent 文件，或从做种节点复制种子到数据目录。info_hash="+uri.InfoHashHex())
		return
	}
	if uri.PrimaryTracker() != "" && tf.Announce == "" {
		tf.Announce = uri.PrimaryTracker()
	}
	cs.startDownload(w, tf)
}

func (cs *ControlServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"events": cs.peer.Events(80)})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
}
