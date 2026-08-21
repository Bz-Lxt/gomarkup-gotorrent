package announce

// Event Tracker 事件。
type Event string

const (
	EventNone      Event = ""
	EventStarted   Event = "started"
	EventCompleted Event = "completed"
	EventStopped   Event = "stopped"
)

// Request 一次 announce 请求。
type Request struct {
	AnnounceURL string
	InfoHash    string
	PeerID      string
	Port        int
	Uploaded    int64
	Downloaded  int64
	Left        int64
	Name        string
	Length      int64
	Event       Event
	Compact     bool
	NumWant     int
	IP          string
}

// Peer 是 Tracker 返回的一个邻居节点。
type Peer struct {
	PeerID string `json:"peer_id"`
	IP     string `json:"ip"`
	Port   int    `json:"port"`
}

// Response Tracker 响应。
type Response struct {
	Interval   int    `json:"interval"`
	Complete   int    `json:"complete"`
	Incomplete int    `json:"incomplete"`
	Peers      []Peer `json:"peers"`
	Failure    string `json:"failure reason,omitempty"`
}
