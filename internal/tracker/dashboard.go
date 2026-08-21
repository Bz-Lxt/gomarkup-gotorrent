package tracker

// dashboardHTML Tracker 管理页面：展示所有 Swarm、做种/下载节点与进度。
const dashboardHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GoTorrent Tracker</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
         background: #0f1420; color: #dbe2f0; min-height: 100vh; }
  header { background: linear-gradient(135deg, #1a2340, #16213e); padding: 22px 32px;
           border-bottom: 1px solid #2a3a5f; display: flex; align-items: center; gap: 14px; }
  header .logo { font-size: 26px; font-weight: 700; color: #5aa9ff; }
  header .sub { color: #7f8db0; font-size: 13px; }
  header .clock { margin-left: auto; color: #7f8db0; font-size: 13px; }
  main { padding: 24px 32px; max-width: 1200px; margin: 0 auto; }
  .stats { display: flex; gap: 16px; margin-bottom: 24px; flex-wrap: wrap; }
  .stat-card { background: #1a2340; border: 1px solid #2a3a5f; border-radius: 10px;
               padding: 16px 24px; min-width: 150px; }
  .stat-card .num { font-size: 28px; font-weight: 700; color: #5aa9ff; }
  .stat-card .label { font-size: 12px; color: #7f8db0; margin-top: 4px; }
  .swarm { background: #1a2340; border: 1px solid #2a3a5f; border-radius: 10px;
           margin-bottom: 18px; overflow: hidden; }
  .swarm-head { padding: 14px 20px; display: flex; align-items: center; gap: 12px;
                background: #1e2a4a; flex-wrap: wrap; }
  .swarm-head .name { font-weight: 600; font-size: 15px; }
  .swarm-head .hash { font-family: monospace; font-size: 11px; color: #7f8db0; }
  .badge { font-size: 11px; padding: 3px 10px; border-radius: 20px; }
  .badge.seed { background: #143d2b; color: #4ade80; }
  .badge.leech { background: #3d2a14; color: #fbbf24; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th, td { padding: 10px 20px; text-align: left; }
  th { color: #7f8db0; font-weight: 500; font-size: 12px; border-bottom: 1px solid #2a3a5f; }
  td { border-bottom: 1px solid #232f4d; }
  tr:last-child td { border-bottom: none; }
  .mono { font-family: monospace; font-size: 12px; }
  .role-seed { color: #4ade80; } .role-leech { color: #fbbf24; }
  .pbar { background: #0f1420; border-radius: 4px; height: 8px; width: 140px; overflow: hidden; }
  .pbar > div { height: 100%; background: linear-gradient(90deg, #5aa9ff, #4ade80); }
  .empty { text-align: center; color: #7f8db0; padding: 60px 0; font-size: 14px; }
</style>
</head>
<body>
<header>
  <div class="logo">GoTorrent Tracker</div>
  <div class="sub">P2P 分发协调服务</div>
  <div class="clock" id="clock"></div>
</header>
<main>
  <div class="stats" id="stats"></div>
  <div id="swarms"></div>
</main>
<script>
function fmtBytes(n) {
  if (n === 0) return '0 B';
  const u = ['B','KB','MB','GB','TB']; const i = Math.floor(Math.log2(n)/10);
  return (n/Math.pow(1024,i)).toFixed(i?1:0) + ' ' + u[i];
}
function fmtTime(t) { return new Date(t).toLocaleTimeString(); }
async function refresh() {
  try {
    const res = await fetch('/api/swarms');
    const data = await res.json();
    document.getElementById('clock').textContent = '更新于 ' + new Date().toLocaleTimeString();
    const swarms = data.swarms || [];
    let totalPeers = 0, totalSeeders = 0;
    swarms.forEach(s => { totalPeers += s.peers.length; totalSeeders += s.seeders; });
    document.getElementById('stats').innerHTML =
      '<div class="stat-card"><div class="num">' + swarms.length + '</div><div class="label">Swarm 数</div></div>' +
      '<div class="stat-card"><div class="num">' + totalPeers + '</div><div class="label">在线节点</div></div>' +
      '<div class="stat-card"><div class="num">' + totalSeeders + '</div><div class="label">做种者</div></div>';
    if (!swarms.length) {
      document.getElementById('swarms').innerHTML = '<div class="empty">暂无活动的分发任务</div>';
      return;
    }
    document.getElementById('swarms').innerHTML = swarms.map(s => {
      const rows = s.peers.map(p => {
        const done = s.length ? (100 * (1 - p.left / s.length)) : 100;
        return '<tr>' +
          '<td class="mono">' + (p.peer_id || '').slice(0, 16) + '</td>' +
          '<td class="mono">' + p.ip + ':' + p.port + '</td>' +
          '<td class="' + (p.left === 0 ? 'role-seed' : 'role-leech') + '">' +
            (p.left === 0 ? '做种中' : '下载中') + '</td>' +
          '<td><div class="pbar"><div style="width:' + done.toFixed(1) + '%"></div></div></td>' +
          '<td>' + done.toFixed(1) + '%</td>' +
          '<td>' + fmtBytes(p.downloaded) + ' / ' + fmtBytes(p.uploaded) + '</td>' +
          '<td>' + fmtTime(p.last_seen) + '</td></tr>';
      }).join('');
      return '<div class="swarm"><div class="swarm-head">' +
        '<span class="name">' + (s.name || '(未命名)') + '</span>' +
        '<span class="hash">' + s.info_hash + '</span>' +
        '<span class="badge seed">做种 ' + s.seeders + '</span>' +
        '<span class="badge leech">下载 ' + s.leechers + '</span>' +
        '<span style="color:#7f8db0;font-size:12px">' + fmtBytes(s.length) + '</span>' +
        '</div><table><thead><tr><th>Peer ID</th><th>地址</th><th>角色</th>' +
        '<th>进度</th><th></th><th>下载/上传</th><th>最近汇报</th></tr></thead>' +
        '<tbody>' + rows + '</tbody></table></div>';
    }).join('');
  } catch (e) { console.error(e); }
}
refresh();
setInterval(refresh, 3000);
</script>
</body>
</html>
`
