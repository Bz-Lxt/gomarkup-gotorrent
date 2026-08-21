# GoTorrent — 简易 P2P 去中心化文件分发系统

用 Go 实现的简化版 BitTorrent 文件分发系统，纯标准库、零第三方依赖，包含 Tracker 服务器、P2P 节点（下载/做种）与 Web 操作管理页面。

## 功能特性

| 功能 | 实现 |
|------|------|
| **Tracker 服务器** | 轻量级 HTTP Tracker，协调各 Peer 并记录分片持有情况；自带 Swarm 管理面板 |
| **文件分片与校验** | 默认 256KB 分片，逐块 SHA-1 校验，生成标准 bencode 结构的 `.torrent` 种子文件 |
| **P2P 并发下载** | 从多个 Peer 并发下载不同分片（Rarest-First 选片 + 16KB 块流水线），同时向其他 Peer 上传已下载分片 |
| **Tit-for-Tat 激励** | 每 10s 评估：放行上传贡献最高的 3 个节点 + 周期性乐观放行 1 个随机节点，其余 Choke |
| **断点续传** | Bitfield 位图实时持久化到 `.state/` 目录，重启后自动恢复进度；状态丢失时按 SHA-1 扫描重建位图 |

## 项目结构

```
├── cmd/
│   ├── tracker/main.go      # Tracker 服务器入口
│   ├── peer/main.go         # Peer 节点入口
│   └── mktorrent/main.go    # 命令行生成 .torrent / magnet
├── internal/
│   ├── bencode/             # bencode 编解码（种子文件格式）
│   ├── metainfo/            # .torrent 生成/解析、info_hash、分片校验
│   ├── bitfield/            # 分片位图
│   ├── wire/                # P2P 有线协议（握手 + 消息编解码）
│   ├── tracker/             # Tracker 核心状态 + HTTP 服务 + 管理页面
│   ├── storage/             # 分片落盘 + 断点续传状态持久化
│   ├── peer/                # 节点：连接管理、并发下载、上传、Tit-for-Tat、控制台
│   ├── announce/            # Tracker HTTP 客户端 + compact 节点列表
│   ├── magnet/              # Magnet URI 生成/解析
│   ├── pex/                 # Peer Exchange 邻居交换
│   ├── ratelimit/           # 令牌桶上下行限速
│   ├── fileset/             # 多文件种子的偏移映射
│   ├── hasher/ metrics/ config/ logx/ eventlog/ peerid/ util/
└── README.md
```

## 快速开始

### 1. 构建

```bash
go build -o bin/tracker ./cmd/tracker
go build -o bin/peer    ./cmd/peer
```

### 2. 启动 Tracker

```bash
./bin/tracker -addr :8080
```

管理页面：<http://localhost:8080/>（查看所有 Swarm、做种/下载节点、进度与流量统计）

### 3. 启动两个 Peer 节点

```bash
# 节点 A（做种方）
./bin/peer -listen :6881 -control :9000 -dir ./peerA

# 节点 B（下载方）
./bin/peer -listen :6882 -control :9001 -dir ./peerB
```

每个节点自带控制台：`http://localhost:9000/`、`http://localhost:9001/`

### 4. 做种与下载

1. 打开节点 A 控制台，在「开始做种」中输入本机文件绝对路径（如 `/tmp/bigfile.bin`），点击创建。
   系统会生成 `.torrent` 文件（保存在 A 的数据目录），并开始做种。
2. 打开节点 B 控制台，在「添加下载」中上传该 `.torrent` 文件（或输入其路径）。
3. B 会自动向 Tracker 汇报、发现 A、并发下载分片；完成后自动转为做种。
4. 下载中途可直接 `Ctrl+C` 杀掉 B，重启后重新添加同一种子即可断点续传。

## 协议设计

### 有线协议（TCP，简化版 BitTorrent）

```
握手:  <pstrlen:1><"GoTorrent protocol"><reserved:8><info_hash:20><peer_id:20>
消息:  <length:4 大端><id:1><payload>     length=0 为 keep-alive
```

| ID | 消息 | 说明 |
|----|------|------|
| 0 | Choke | 拒绝对方的上传请求 |
| 1 | Unchoke | 允许对方请求数据 |
| 2/3 | Interested / NotInterested | 表达对对方分片的兴趣 |
| 4 | Have | 声明新完成的分片 |
| 5 | Bitfield | 握手后交换完整位图 |
| 6 | Request | 请求块 `<index><begin><length>`，块最大 16KB |
| 7 | Piece | 返回块数据 `<index><begin><block>` |
| 8 | Cancel | 取消请求 |

### Tracker Announce（HTTP/JSON）

```
GET /announce?info_hash=<hex40>&peer_id=<id>&port=<p>&uploaded=<n>&downloaded=<n>&left=<n>&event=started|completed|stopped&name=<文件名>&length=<字节数>
→ {"interval":15, "complete":1, "incomplete":2, "peers":[{"peer_id":"...","ip":"...","port":6881}]}
```

## HTTP API（节点控制台）

| 接口 | 说明 |
|------|------|
| `GET  /api/info` | 节点信息（PeerID、端口、数据目录） |
| `GET  /api/torrents` | 任务列表（进度、速率、位图 base64） |
| `POST /api/seed` | 做种：`{"path":"/abs/file","tracker":"http://..."}` |
| `POST /api/download` | 下载：multipart 上传 `.torrent` 或 `{"path":"..."}` |
| `POST /api/remove` | 移除任务：`{"info_hash":"..."}` |
| `POST /api/magnet` | 通过 Magnet 添加（需本地已有对应 .torrent） |
| `GET  /api/events` | 最近会话事件（连接、分片、announce） |

命令行生成种子：

```bash
go run ./cmd/mktorrent -announce http://localhost:8080/announce /path/to/file.bin
```

## 运行测试

```bash
go test ./...
```

覆盖 bencode 编解码、位图、种子生成/解析、协议消息、存储与断点续传。

## 已知简化（教学项目）

- 单文件种子（不支持多文件/目录）
-  announce 使用 JSON 而非 bencode 响应
- 无 DHT / PEX / UDP Tracker，节点发现完全依赖 Tracker
- 请求超时直接断开连接，依赖重连重新分配分片
