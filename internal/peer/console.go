package peer

// consoleHTML 节点控制台页面：做种、添加下载、查看进度（含分片位图可视化）。
const consoleHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GoTorrent 节点控制台</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
         background: #0f1420; color: #dbe2f0; min-height: 100vh; }
  header { background: linear-gradient(135deg, #1a2340, #16213e); padding: 20px 32px;
           border-bottom: 1px solid #2a3a5f; display: flex; align-items: center; gap: 14px; flex-wrap: wrap; }
  header .logo { font-size: 24px; font-weight: 700; color: #5aa9ff; }
  header .meta { color: #7f8db0; font-size: 12px; font-family: monospace; }
  main { padding: 24px 32px; max-width: 1100px; margin: 0 auto; }
  .cards { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 24px; }
  @media (max-width: 800px) { .cards { grid-template-columns: 1fr; } }
  .card { background: #1a2340; border: 1px solid #2a3a5f; border-radius: 10px; padding: 18px 20px; }
  .card h3 { font-size: 15px; margin-bottom: 12px; color: #9db4dd; }
  .card input[type=text] { width: 100%; background: #0f1420; border: 1px solid #2a3a5f; color: #dbe2f0;
           border-radius: 6px; padding: 9px 12px; font-size: 13px; margin-bottom: 10px; outline: none; }
  .card input[type=text]:focus { border-color: #5aa9ff; }
  .btn { background: #2f6fed; color: #fff; border: none; border-radius: 6px; padding: 9px 20px;
         font-size: 13px; cursor: pointer; transition: background .15s; }
  .btn:hover { background: #4a84f5; }
  .btn.gray { background: #37415c; } .btn.gray:hover { background: #46527a; }
  .btn.red { background: #59303a; color: #f28b9d; font-size: 12px; padding: 5px 12px; }
  .btn.red:hover { background: #6d3a46; }
  .file-btn { display: inline-block; background: #0f1420; border: 1px dashed #2a3a5f; border-radius: 6px;
              padding: 9px 14px; font-size: 13px; color: #7f8db0; cursor: pointer; margin-bottom: 10px; }
  .file-btn:hover { border-color: #5aa9ff; color: #9db4dd; }
  .task { background: #1a2340; border: 1px solid #2a3a5f; border-radius: 10px; padding: 16px 20px; margin-bottom: 14px; }
  .task-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 10px; }
  .task-head .name { font-weight: 600; font-size: 15px; }
  .badge { font-size: 11px; padding: 3px 10px; border-radius: 20px; }
  .badge.dl { background: #3d2a14; color: #fbbf24; }
  .badge.sd { background: #143d2b; color: #4ade80; }
  .task-head .hash { font-family: monospace; font-size: 11px; color: #5a6a90; }
  .task-head .spacer { flex: 1; }
  .pbar { background: #0f1420; border-radius: 5px; height: 10px; overflow: hidden; margin-bottom: 10px; }
  .pbar > div { height: 100%; background: linear-gradient(90deg, #5aa9ff, #4ade80); transition: width .5s; }
  .task-info { display: flex; gap: 18px; font-size: 12px; color: #7f8db0; flex-wrap: wrap; margin-bottom: 10px; }
  .task-info b { color: #dbe2f0; font-weight: 600; }
  .pieces { display: flex; flex-wrap: wrap; gap: 2px; }
  .pieces i { width: 10px; height: 10px; border-radius: 2px; background: #232f4d; }
  .pieces i.done { background: #4ade80; }
  .empty { text-align: center; color: #7f8db0; padding: 40px 0; font-size: 14px; }
  #toast { position: fixed; top: 20px; right: 20px; padding: 12px 20px; border-radius: 8px;
           font-size: 13px; display: none; z-index: 99; }
  #toast.ok { background: #143d2b; color: #4ade80; display: block; }
  #toast.err { background: #59303a; color: #f28b9d; display: block; }
</style>
</head>
<body>
<header>
  <div class="logo">GoTorrent 节点</div>
  <div class="meta" id="node-meta">加载中...</div>
</header>
<main>
  <div class="cards">
    <div class="card">
      <h3>开始做种（发布文件）</h3>
      <input type="text" id="seed-path" placeholder="本机文件绝对路径，如 /tmp/bigfile.bin">
      <input type="text" id="seed-tracker" placeholder="Tracker 地址（留空用默认）">
      <button class="btn" onclick="doSeed()">创建种子并做种</button>
    </div>
    <div class="card">
      <h3>添加下载</h3>
      <label class="file-btn">选择 .torrent 文件<input type="file" id="torrent-file" accept=".torrent" style="display:none" onchange="fileChosen(this)"></label>
      <span id="file-name" style="font-size:12px;color:#7f8db0"></span>
      <div style="margin-bottom:10px"></div>
      <button class="btn" onclick="doUpload()">上传并开始下载</button>
      <div style="color:#5a6a90;font-size:12px;margin-top:10px">或输入本机种子路径：</div>
      <div style="display:flex;gap:8px;margin-top:8px">
        <input type="text" id="dl-path" placeholder="/path/to/file.torrent" style="flex:1;margin-bottom:0">
        <button class="btn gray" onclick="doDownloadByPath()">添加</button>
      </div>
      <div style="color:#5a6a90;font-size:12px;margin-top:10px">或粘贴 Magnet 链接（需本地已有对应 .torrent）：</div>
      <div style="display:flex;gap:8px;margin-top:8px">
        <input type="text" id="magnet" placeholder="magnet:?xt=urn:btih:..." style="flex:1;margin-bottom:0">
        <button class="btn gray" onclick="doMagnet()">添加</button>
      </div>
    </div>
  </div>
  <h3 style="font-size:15px;color:#9db4dd;margin-bottom:12px">任务列表</h3>
  <div id="tasks"></div>
</main>
<div id="toast"></div>
<script>
function fmtBytes(n) {
  if (!n) return '0 B';
  const u = ['B','KB','MB','GB','TB']; const i = Math.floor(Math.log2(n)/10);
  return (n/Math.pow(1024,i)).toFixed(i?1:0) + ' ' + u[i];
}
function fmtRate(n) { return fmtBytes(n) + '/s'; }
function toast(msg, ok) {
  const t = document.getElementById('toast');
  t.textContent = msg; t.className = ok ? 'ok' : 'err';
  setTimeout(() => t.className = '', 3000);
}
async function api(url, opts) {
  const res = await fetch(url, opts);
  const data = await res.json();
  if (data.ok === false) throw new Error(data.error || '操作失败');
  return data;
}
async function loadInfo() {
  const d = await api('/api/info');
  document.getElementById('node-meta').textContent =
    'PeerID: ' + d.peer_id + ' | P2P 端口: ' + d.port + ' | 数据目录: ' + d.dir;
  if (d.tracker) document.getElementById('seed-tracker').placeholder = 'Tracker 地址（默认 ' + d.tracker + '）';
}
async function doSeed() {
  const path = document.getElementById('seed-path').value.trim();
  if (!path) return toast('请输入文件路径', false);
  try {
    const d = await api('/api/seed', {method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({path, tracker: document.getElementById('seed-tracker').value.trim()})});
    toast('开始做种: ' + d.name, true);
    refresh();
  } catch (e) { toast(e.message, false); }
}
function fileChosen(input) {
  document.getElementById('file-name').textContent = input.files[0] ? input.files[0].name : '';
}
async function doUpload() {
  const f = document.getElementById('torrent-file').files[0];
  if (!f) return toast('请选择 .torrent 文件', false);
  const fd = new FormData(); fd.append('file', f);
  try {
    const d = await api('/api/download', {method:'POST', body: fd});
    toast('已添加下载: ' + d.name, true);
    refresh();
  } catch (e) { toast(e.message, false); }
}
async function doDownloadByPath() {
  const path = document.getElementById('dl-path').value.trim();
  if (!path) return toast('请输入种子路径', false);
  try {
    const d = await api('/api/download', {method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({path})});
    toast('已添加下载: ' + d.name, true);
    refresh();
  } catch (e) { toast(e.message, false); }
}
async function doMagnet() {
  const magnet = document.getElementById('magnet').value.trim();
  if (!magnet) return toast('请输入 magnet 链接', false);
  try {
    const d = await api('/api/magnet', {method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({magnet})});
    toast('已添加下载: ' + d.name, true);
    refresh();
  } catch (e) { toast(e.message, false); }
}
async function doRemove(hash) {
  if (!confirm('确定移除该任务？（不会删除已下载文件）')) return;
  try {
    await api('/api/remove', {method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({info_hash: hash})});
    refresh();
  } catch (e) { toast(e.message, false); }
}
function pieceGrid(t) {
  if (!t.bitfield) return '';
  const bytes = Uint8Array.from(atob(t.bitfield), c => c.charCodeAt(0));
  let html = '<div class="pieces">';
  for (let i = 0; i < t.num_pieces; i++) {
    const has = (bytes[i >> 3] & (0x80 >> (i & 7))) !== 0;
    html += '<i class="' + (has ? 'done' : '') + '" title="分片 ' + i + (has ? ' 已完成' : '') + '"></i>';
  }
  return html + '</div>';
}
async function refresh() {
  try {
    const d = await api('/api/torrents');
    const list = d.torrents || [];
    if (!list.length) {
      document.getElementById('tasks').innerHTML = '<div class="empty">暂无任务，先做种或添加一个下载吧</div>';
      return;
    }
    document.getElementById('tasks').innerHTML = list.map(t => {
      const pct = t.num_pieces ? (100 * t.completed_pieces / t.num_pieces) : 0;
      return '<div class="task"><div class="task-head">' +
        '<span class="name">' + t.name + '</span>' +
        '<span class="badge ' + (t.state === 'seeding' ? 'sd' : 'dl') + '">' +
          (t.state === 'seeding' ? '做种中' : '下载中') + '</span>' +
        '<span class="hash">' + t.info_hash.slice(0, 16) + '…</span>' +
        '<span class="spacer"></span>' +
        '<button class="btn red" onclick="doRemove(\'' + t.info_hash + '\')">移除</button></div>' +
        '<div class="pbar"><div style="width:' + pct.toFixed(1) + '%"></div></div>' +
        '<div class="task-info">' +
        '<span>进度 <b>' + t.completed_pieces + '/' + t.num_pieces + '</b> 分片 (' + pct.toFixed(1) + '%)</span>' +
        '<span>大小 <b>' + fmtBytes(t.length) + '</b></span>' +
        '<span>下载 <b>' + fmtRate(t.down_rate) + '</b></span>' +
        '<span>上传 <b>' + fmtRate(t.up_rate) + '</b></span>' +
        '<span>累计 下/传 <b>' + fmtBytes(t.downloaded) + ' / ' + fmtBytes(t.uploaded) + '</b></span>' +
        '<span>连接节点 <b>' + t.peers + '</b></span>' +
        (t.magnet ? '<span style="font-family:monospace;font-size:11px;word-break:break-all">magnet: ' + t.magnet.slice(0, 48) + '…</span>' : '') +
        '</div>' + pieceGrid(t) + '</div>';
    }).join('');
  } catch (e) { console.error(e); }
}
loadInfo();
refresh();
setInterval(refresh, 2000);
</script>
</body>
</html>
`
