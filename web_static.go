package main

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>寿司郎重度依赖</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0f0f0f; color: #e0e0e0; }
  .header { background: linear-gradient(135deg, #1a1a2e, #16213e); padding: 20px 30px; border-bottom: 1px solid #2a2a3e; }
  .header h1 { font-size: 22px; color: #ff6b6b; }
  .header .version { font-size: 12px; color: #666; margin-top: 4px; }
  .container { max-width: 900px; margin: 0 auto; padding: 20px; }
  .card { background: #1a1a1a; border: 1px solid #2a2a2a; border-radius: 12px; padding: 20px; margin-bottom: 16px; }
  .card h2 { font-size: 16px; color: #ff6b6b; margin-bottom: 12px; }
  .tabs { display: flex; gap: 8px; margin-bottom: 20px; }
  .tab { padding: 8px 16px; background: #1a1a1a; border: 1px solid #2a2a2a; border-radius: 8px; cursor: pointer; color: #999; font-size: 14px; }
  .tab.active { background: #ff6b6b; color: #fff; border-color: #ff6b6b; }
  .slot-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 8px; }
  .slot { padding: 10px 12px; background: #252525; border-radius: 8px; font-size: 13px; cursor: default; }
  .slot.available { border-left: 3px solid #4ecdc4; cursor: pointer; }
  .slot.available:hover { background: #2a3a3a; }
  .slot.full { border-left: 3px solid #ff6b6b; opacity: 0.5; }
  .slot .time { font-weight: 600; font-size: 15px; }
  .slot .status { font-size: 11px; margin-top: 4px; }
  .slot.available .status { color: #4ecdc4; }
  .slot.full .status { color: #ff6b6b; }
  .btn { padding: 8px 16px; border: none; border-radius: 8px; cursor: pointer; font-size: 13px; font-weight: 600; }
  .btn-primary { background: #ff6b6b; color: #fff; }
  .btn-secondary { background: #333; color: #ccc; }
  .btn-small { padding: 6px 12px; font-size: 12px; }
  .loading { text-align: center; padding: 40px; color: #666; }
  .store-select, .date-select { padding: 8px 12px; background: #252525; border: 1px solid #333; border-radius: 8px; color: #e0e0e0; font-size: 14px; }
  #log { max-height: 300px; overflow-y: auto; font-family: monospace; font-size: 12px; color: #888; line-height: 1.6; }

  .date-bar { display: flex; gap: 6px; overflow-x: auto; padding-bottom: 8px; margin-bottom: 16px; }
  .date-chip {
    flex-shrink: 0; padding: 8px 14px; background: #252525; border: 1px solid #333;
    border-radius: 8px; cursor: pointer; font-size: 13px; text-align: center; min-width: 70px;
    transition: all 0.15s;
  }
  .date-chip:hover { background: #333; }
  .date-chip.active { background: #ff6b6b; color: #fff; border-color: #ff6b6b; }
  .date-chip .dw { font-size: 11px; color: #999; margin-bottom: 2px; }
  .date-chip.active .dw { color: #fff; }
  .date-chip .dd { font-weight: 600; }
  .date-chip .dc { font-size: 10px; margin-top: 2px; }
  .date-chip .dc.has-avail { color: #4ecdc4; }
  .date-chip .dc.all-full { color: #ff6b6b; }

  .toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 16px; flex-wrap: wrap; }
</style>
</head>
<body>
<div class="header">
  <h1>寿司郎重度依赖</h1>
  <div class="version" id="version">loading...</div>
</div>
<div class="container">
  <div class="tabs">
    <div class="tab active" onclick="showTab('calendar')">日历</div>
    <div class="tab" onclick="showTab('reservations')">预约</div>
    <div class="tab" onclick="showTab('config')">设置</div>
    <div class="tab" onclick="showTab('log')">日志</div>
  </div>

  <div id="tab-calendar">
    <div class="card">
      <div class="toolbar">
        <h2 style="margin:0">时段</h2>
        <select class="store-select" id="store-select" onchange="onStoreChange()"></select>
        <button class="btn btn-primary btn-small" onclick="refreshData()">刷新</button>
      </div>
      <div class="date-bar" id="date-bar"></div>
      <div id="slot-content"><div class="loading">选择日期查看时段</div></div>
    </div>
  </div>

  <div id="tab-reservations" style="display:none">
    <div class="card">
      <h2>当前预约</h2>
      <div id="reservations-content"><div class="loading">加载中...</div></div>
    </div>
  </div>

  <div id="tab-config" style="display:none">
    <div class="card">
      <h2>通知设置</h2>
      <div id="config-content"><div class="loading">加载中...</div></div>
    </div>
  </div>

  <div id="tab-log" style="display:none">
    <div class="card">
      <h2>实时日志</h2>
      <div id="log"></div>
    </div>
  </div>
</div>

<script>
let allSlots = [];
let selectedDate = '';
const weekdays = ['日','一','二','三','四','五','六'];

function showTab(name) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('[id^="tab-"]').forEach(t => t.style.display = 'none');
  event.target.classList.add('active');
  document.getElementById('tab-' + name).style.display = 'block';
  if (name === 'reservations') loadReservations();
  if (name === 'config') loadConfig();
}

async function loadStatus() {
  try {
    const r = await fetch('/api/status');
    const d = await r.json();
    document.getElementById('version').textContent = 'v' + d.version + (d.running ? ' · 运行中 PID ' + d.pid : ' · 未运行');
  } catch(e) {}
}

async function loadStores() {
  try {
    const r = await fetch('/api/stores');
    const stores = await r.json();
    const sel = document.getElementById('store-select');
    sel.innerHTML = '';
    if (!stores.length) { sel.innerHTML = '<option>暂无门店</option>'; return false; }
    stores.forEach(s => {
      const opt = document.createElement('option');
      opt.value = s.id;
      opt.textContent = s.nickname || s.name;
      sel.appendChild(opt);
    });
    return true;
  } catch(e) { return false; }
}

async function refreshData() {
  const storeID = document.getElementById('store-select').value;
  if (!storeID || storeID === '暂无门店') return;
  document.getElementById('date-bar').innerHTML = '';
  document.getElementById('slot-content').innerHTML = '<div class="loading">加载中...</div>';
  try {
    const r = await fetch('/api/calendar?store=' + storeID);
    const d = await r.json();
    if (d.error) { document.getElementById('slot-content').innerHTML = '<div class="loading">' + d.error + '</div>'; return; }
    allSlots = d.slots || [];
    renderDateBar();
  } catch(e) {
    document.getElementById('slot-content').innerHTML = '<div class="loading">加载失败</div>';
  }
}

function onStoreChange() {
  selectedDate = '';
  refreshData();
}

function fmtDate(d) {
  // d = "20260409" → "4/9"
  const m = parseInt(d.substring(4,6));
  const day = parseInt(d.substring(6,8));
  return m + '/' + day;
}

function fmtTime(t) {
  if (!t || t.length < 4) return t;
  return t.substring(0,2) + ':' + t.substring(2,4);
}

function renderDateBar() {
  const grouped = {};
  allSlots.forEach(s => {
    if (!grouped[s.date]) grouped[s.date] = [];
    grouped[s.date].push(s);
  });
  const dates = Object.keys(grouped).sort();

  const bar = document.getElementById('date-bar');
  bar.innerHTML = '';

  if (!dates.length) {
    document.getElementById('slot-content').innerHTML = '<div class="loading">无可用时段</div>';
    return;
  }

  dates.forEach(date => {
    const slots = grouped[date];
    const avail = slots.filter(s => s.availability === 'AVAILABLE').length;
    const d = new Date(date.substring(0,4)+'-'+date.substring(4,6)+'-'+date.substring(6,8));
    const dw = weekdays[d.getDay()];

    const chip = document.createElement('div');
    chip.className = 'date-chip' + (date === selectedDate ? ' active' : '');
    chip.innerHTML = '<div class="dw">周' + dw + '</div><div class="dd">' + fmtDate(date) + '</div>'
      + '<div class="dc ' + (avail > 0 ? 'has-avail' : 'all-full') + '">' + (avail > 0 ? '✓' + avail : '✗') + '</div>';
    chip.onclick = () => { selectedDate = date; renderDateBar(); renderSlots(date); };
    bar.appendChild(chip);
  });

  // Auto-select first date if none selected
  if (!selectedDate || !dates.includes(selectedDate)) {
    selectedDate = dates[0];
    renderDateBar();
    renderSlots(selectedDate);
  } else {
    renderSlots(selectedDate);
  }
}

function renderSlots(date) {
  const slots = allSlots.filter(s => s.date === date);
  const cont = document.getElementById('slot-content');

  if (!slots.length) {
    cont.innerHTML = '<div class="loading">该日无时段数据</div>';
    return;
  }

  // Sort by start time
  slots.sort((a,b) => (a.start||'').localeCompare(b.start||''));

  let html = '<div class="slot-grid">';
  slots.forEach(s => {
    const avail = s.availability === 'AVAILABLE';
    html += '<div class="slot ' + (avail ? 'available' : 'full') + '">'
      + '<div class="time">' + fmtTime(s.start) + '-' + fmtTime(s.end) + '</div>'
      + '<div class="status">' + (avail ? '✓ 可预约' : '✗ 已满') + '</div>'
      + '</div>';
  });
  html += '</div>';

  const availCount = slots.filter(s => s.availability === 'AVAILABLE').length;
  html += '<div style="margin-top:12px;font-size:12px;color:#666">' + slots.length + ' 个时段，' + availCount + ' 个可预约</div>';

  cont.innerHTML = html;
}

async function loadReservations() {
  try {
    const r = await fetch('/api/reservations');
    const data = await r.json();
    if (data.error) { document.getElementById('reservations-content').innerHTML = '<div class="loading">' + data.error + '</div>'; return; }
    if (!data.length) {
      document.getElementById('reservations-content').innerHTML = '<div class="loading">暂无预约</div>';
      return;
    }
    let html = '<div class="slot-grid">';
    data.forEach(r => {
      html += '<div class="slot available">'
        + '<div>号码: ' + (r.number||'-') + '</div>'
        + '<div>状态: ' + (r.status||'-') + '</div>'
        + '<div>Ticket: #' + (r.ticketId||'-') + '</div></div>';
    });
    html += '</div>';
    document.getElementById('reservations-content').innerHTML = html;
  } catch(e) {
    document.getElementById('reservations-content').innerHTML = '<div class="loading">获取失败</div>';
  }
}

async function loadConfig() {
  try {
    const r = await fetch('/api/config');
    const d = await r.json();
    let html = '<div style="font-size:14px;line-height:2.2">';
    html += '飞书: ' + (d.feishu?.webhook ? '✓ 已配置' : '✗ 未配置') + '<br>';
    html += 'Telegram: ' + (d.telegram?.token ? '✓ 已配置' : '✗ 未配置') + '<br>';
    html += 'Bark: ' + (d.bark?.key ? '✓ 已配置' : '✗ 未配置') + '<br>';
    html += 'Server酱: ' + (d.server_chan?.key ? '✓ 已配置' : '✗ 未配置') + '<br>';
    html += '</div>';
    html += '<div style="margin-top:12px;color:#666;font-size:12px">CLI 配置: sushiro-overdose config telegram/bark/serverchan</div>';
    document.getElementById('config-content').innerHTML = html;
  } catch(e) {
    document.getElementById('config-content').innerHTML = '<div class="loading">获取失败</div>';
  }
}

// SSE
let currentES = null;
function connectSSE() {
  if (currentES) currentES.close();
  const es = new EventSource('/api/events');
  currentES = es;
  es.addEventListener('calendar', e => {
    try {
      const d = JSON.parse(e.data);
      allSlots = d.slots || [];
      renderDateBar();
    } catch(err) {}
  });
  es.addEventListener('ping', () => {});
  es.onerror = () => { es.close(); currentES = null; setTimeout(connectSSE, 3000); };
}

// Init
loadStatus();
loadStores().then(ok => { if(ok) refreshData(); });
connectSSE();
</script>
</body>
</html>
`
