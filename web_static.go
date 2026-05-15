package main

const logoBase64 = "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"

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="sushiro-csrf" content="{{CSRF_TOKEN}}">
<title>SUSHIRO Overdose</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --red:#B81C22;--red-dark:#9F1419;--red-soft:#FFF1F1;
  --ink:#191817;--text:#282522;--sub:#66615C;--mute:#9B9691;
  --paper:#FFFFFF;--wash:#F5F3F1;--line:#E5E0DB;--line-strong:#D5CEC7;
  --green:#21823F;--green-soft:#ECF7EF;--yellow:#B67800;--yellow-soft:#FFF5D8;
  --blue:#2B5B83;--blue-soft:#EEF5FA;--shadow:0 12px 34px rgba(42,35,28,.08);
  --font:"PingFang SC","Hiragino Sans GB","Microsoft YaHei",-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
}
body{min-height:100vh;background:linear-gradient(180deg,#fff 0,#f7f5f2 260px,var(--wash) 100%);color:var(--text);font-family:var(--font);-webkit-font-smoothing:antialiased}
button,input,select{font:inherit}
.topline{height:4px;background:var(--red)}
.shell{max-width:1120px;margin:0 auto;padding:0 24px}
.hdr{position:sticky;top:0;z-index:20;background:rgba(255,255,255,.92);backdrop-filter:blur(18px);border-bottom:1px solid var(--line)}
.hdr-in{height:72px;display:flex;align-items:center;gap:18px}
.brand{display:flex;align-items:center;gap:12px;min-width:0}
.brand img{width:42px;height:42px;border-radius:50%}
.brand strong{display:block;font-size:16px;line-height:1;color:var(--ink)}
.brand span{display:block;margin-top:4px;font-size:11px;color:var(--mute);letter-spacing:.08em;text-transform:uppercase}
.nav{margin-left:auto;display:flex;gap:4px;padding:5px;background:#F0EDEA;border:1px solid var(--line);border-radius:999px}
.nav a{display:inline-flex;align-items:center;height:34px;padding:0 14px;border-radius:999px;color:var(--sub);font-size:13px;font-weight:700;text-decoration:none;white-space:nowrap}
.nav a.on{background:var(--paper);color:var(--red);box-shadow:0 2px 10px rgba(32,25,18,.08)}
.ver{margin-left:6px;padding:7px 11px;border-radius:999px;background:var(--ink);color:#fff;font-size:11px;font-weight:700}
.wrap{padding:30px 0 80px}
.grid{display:grid;grid-template-columns:minmax(0,1fr) 320px;gap:18px;align-items:start}
.hero{min-height:250px;background:var(--paper);border:1px solid var(--line);border-radius:10px;padding:30px;box-shadow:var(--shadow);position:relative;overflow:hidden}
.hero:before{content:"";position:absolute;inset:0 0 auto 0;height:6px;background:var(--red)}
.eyebrow{display:inline-flex;align-items:center;gap:8px;padding:6px 10px;border-radius:999px;background:var(--red-soft);color:var(--red);font-size:12px;font-weight:800}
.hero h1{margin-top:22px;font-size:34px;line-height:1.15;letter-spacing:0;color:var(--ink);max-width:560px}
.hero p{margin-top:12px;color:var(--sub);font-size:15px;line-height:1.8;max-width:620px}
.actions{display:flex;gap:10px;flex-wrap:wrap;margin-top:26px}
.bt{display:inline-flex;align-items:center;justify-content:center;height:42px;padding:0 20px;border:0;border-radius:999px;cursor:pointer;font-size:14px;font-weight:800;text-decoration:none;transition:transform .14s,box-shadow .14s,background .14s,border-color .14s}
.bt:hover{transform:translateY(-1px)}
.bt:disabled{opacity:.45;cursor:not-allowed;transform:none}
.bt-l{height:48px;padding:0 28px;font-size:15px}
.bt-s{height:34px;padding:0 14px;font-size:12px}
.bt-r{background:var(--red);color:#fff;box-shadow:0 10px 22px rgba(184,28,34,.22)}
.bt-r:hover{background:var(--red-dark)}
.bt-y{background:#F5BA24;color:#2C2418;box-shadow:0 8px 18px rgba(190,128,0,.16)}
.bt-o{background:var(--paper);color:var(--red);border:1px solid rgba(184,28,34,.35)}
.bt-w{background:var(--paper);color:var(--text);border:1px solid var(--line-strong)}
.side{display:flex;flex-direction:column;gap:14px}
.card,.cd{background:var(--paper);border:1px solid var(--line);border-radius:10px;padding:20px;box-shadow:0 8px 24px rgba(42,35,28,.05);margin-bottom:18px}
.card h2,.cd-t{font-size:12px;letter-spacing:.09em;text-transform:uppercase;color:var(--mute);font-weight:900;margin-bottom:14px}
.engine{border-radius:10px;border:1px solid var(--line);padding:16px;background:#FBFAF8}
.engine .row{display:flex;align-items:center;gap:10px}
.dot{width:9px;height:9px;border-radius:50%;background:var(--mute);box-shadow:0 0 0 4px rgba(155,150,145,.12)}
.engine strong{font-size:15px;color:var(--ink)}
.engine p{margin-top:8px;color:var(--sub);font-size:12px;line-height:1.6}
.engine.capturing .dot,.step.active .mark{background:var(--yellow)}
.engine.booking .dot,.engine.sniping .dot{background:var(--blue)}
.engine.success .dot,.step.done .mark{background:var(--green)}
.engine.error .dot{background:var(--red)}
.track{display:grid;gap:10px}
.step{display:grid;grid-template-columns:28px 1fr;gap:12px;align-items:start;padding:14px;border:1px solid var(--line);border-radius:10px;background:#FBFAF8}
.step .mark{width:28px;height:28px;border-radius:999px;background:#D8D2CC;color:#fff;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:900}
.step b{display:block;color:var(--ink);font-size:14px;margin-bottom:3px}
.step span{display:block;color:var(--sub);font-size:12px;line-height:1.5}
.step.active{border-color:#E4C05E;background:var(--yellow-soft)}
.step.done{border-color:#B9DEC2;background:var(--green-soft)}
.notice{margin-top:16px;padding:13px 14px;border-radius:10px;background:var(--yellow-soft);border:1px solid #ECD681;color:#6F4B00;font-size:13px;line-height:1.6}
.summary{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin-top:18px}
.mini{padding:16px;border:1px solid var(--line);border-radius:10px;background:var(--paper)}
.mini span{display:block;color:var(--mute);font-size:12px;font-weight:800;margin-bottom:8px}
.mini strong{display:block;color:var(--ink);font-size:20px}
.mini p{margin-top:7px;color:var(--sub);font-size:12px;line-height:1.6}
.ps{font-size:13px;line-height:1.9;color:var(--sub)}
.ps b{color:var(--ink)}
.ps .line{display:block;margin-top:4px}
.cg{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:8px}
.ci{display:flex;align-items:center;gap:8px;min-height:38px;padding:9px 10px;border-radius:8px;background:#F3F0ED;color:var(--sub);font-size:12px;font-weight:700}
.ci:before{content:"";width:8px;height:8px;border-radius:50%;background:#C9C1BA}
.ci.ok{background:var(--green-soft);color:var(--green)}
.ci.ok:before{background:var(--green)}
.ci.bad{background:var(--red-soft);color:var(--red)}
.ci.bad:before{background:var(--red)}
.ci.warn{background:var(--yellow-soft);color:var(--yellow)}
.ci.warn:before{background:var(--yellow)}
.fg{margin-bottom:16px}
.fg label{display:block;margin-bottom:6px;color:var(--sub);font-size:12px;font-weight:800}
.fr{display:flex;gap:12px;flex-wrap:wrap}
input[type=number],input[type=text],select{width:100%;height:40px;padding:0 12px;background:#fff;border:1px solid var(--line-strong);border-radius:8px;color:var(--ink);font-size:14px}
input[type=number]{width:86px}
input:focus,select:focus{outline:0;border-color:var(--red);box-shadow:0 0 0 3px rgba(184,28,34,.08)}
.settings-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:18px}
.tl{display:flex;flex-direction:column;gap:8px}
.tr{display:flex;align-items:center;gap:8px}
.tr input{width:82px;text-align:center}
.tr .sp{color:var(--mute)}
.tr .x{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;border-radius:999px;color:var(--red);cursor:pointer;background:var(--red-soft);font-weight:900}
.at{display:inline-flex;margin-top:8px;color:var(--green);font-size:13px;font-weight:800;cursor:pointer}
.chips{display:flex;gap:8px;flex-wrap:wrap}
.chip{display:inline-flex;align-items:center;min-height:34px;padding:0 13px;border:1px solid var(--line-strong);border-radius:999px;background:#fff;color:var(--sub);font-size:12px;font-weight:800;cursor:pointer}
.chip.on{background:var(--red);border-color:var(--red);color:#fff}
.check{display:inline-flex;align-items:center;gap:8px;height:40px;padding:0 12px;border:1px solid var(--line-strong);border-radius:999px;background:#fff;color:var(--sub);font-size:13px;font-weight:800;cursor:pointer}
.check input{width:auto;height:auto}
.store-list{display:grid;gap:8px}
.store-row{display:grid;grid-template-columns:auto minmax(0,1fr) auto auto;align-items:center;gap:8px;padding:10px 12px;border:1px solid var(--line);border-radius:10px;background:#FBFAF8}
.store-row b{font-size:13px;color:var(--ink);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.store-row span{font-size:11px;color:var(--mute)}
.ico{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;border:1px solid var(--line-strong);border-radius:999px;background:#fff;color:var(--sub);cursor:pointer;font-size:13px;font-weight:900}
.metric{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px;margin-bottom:14px}
.sn-row{display:grid;grid-template-columns:1.1fr 1fr 1fr 1.1fr auto;gap:8px;align-items:end;margin-bottom:8px}
.sn-row input,.sn-row select{height:38px}
.inline-err{grid-column:1/-1;padding:8px 10px;border-radius:8px;background:var(--red-soft);color:var(--red);font-size:12px;font-weight:800}
.tbl{width:100%;border-collapse:collapse;font-size:13px}
.tbl th,.tbl td{padding:9px 8px;border-bottom:1px solid var(--line);text-align:left;vertical-align:top}
.tbl th{color:var(--mute);font-size:11px;text-transform:uppercase;letter-spacing:.06em}
.db{display:flex;gap:8px;overflow-x:auto;padding-bottom:8px;margin:14px 0 18px}
.dc{flex:0 0 auto;min-width:76px;padding:10px 12px;border:1px solid var(--line);border-radius:10px;background:#FBFAF8;cursor:pointer;text-align:center}
.dc.on{background:var(--red);border-color:var(--red);color:#fff}
.dc .dw{font-size:11px;color:var(--mute)}.dc.on .dw{color:rgba(255,255,255,.72)}
.dc .dd{margin-top:2px;font-size:16px;font-weight:900}
.dc .dv{margin-top:3px;font-size:11px;font-weight:800}.dc .dv.h{color:var(--green)}.dc .dv.n{color:var(--red)}.dc.on .dv{color:#fff}
.sg{display:grid;grid-template-columns:repeat(auto-fill,minmax(126px,1fr));gap:9px}
.sl{padding:13px;border:1px solid var(--line);border-radius:10px;background:#F7F4F1}
.sl.av{background:var(--green-soft);border-color:#B9DEC2}
.sl.fu{opacity:.52}
.sl .tm{font-size:15px;font-weight:900;color:var(--ink)}
.sl .ss{margin-top:4px;font-size:12px;color:var(--sub)}
.lg{max-height:430px;overflow:auto;padding:14px;border-radius:10px;background:#181614;color:#E8E1DA;font-family:"SF Mono",Menlo,Consolas,monospace;font-size:12px;line-height:1.75}
.ll{display:flex;gap:10px;border-bottom:1px solid rgba(255,255,255,.06);padding:2px 0}
.ll .lt{color:#9F988F;flex:0 0 auto}.ll.er .lm{color:#FFB7B7}
.empty{padding:32px;border:1px dashed var(--line-strong);border-radius:10px;text-align:center;color:var(--mute);background:#FBFAF8}
.errbox{margin-bottom:12px;padding:12px;border:1px solid #F0B7B9;border-radius:10px;background:var(--red-soft);color:var(--red);font-size:13px;line-height:1.6}
.diag-detail{margin-top:12px;padding:14px;border:1px solid var(--line);border-radius:10px;background:#FBFAF8;color:var(--sub);font-size:12px;line-height:1.7}
.diag-detail b{color:var(--ink)}
.diag-detail code{display:inline-block;max-width:100%;overflow:auto;padding:2px 5px;border-radius:6px;background:#EEE9E4;color:var(--ink)}
.ft{padding:26px 0 46px;text-align:center;color:var(--mute);font-size:12px}.ft a{color:var(--red);text-decoration:none}
.hid{display:none!important}.mu{color:var(--mute)}.tc{text-align:center}.tg{color:var(--green)}.tre{color:var(--red)}
.mt8{margin-top:8px}.mt16{margin-top:16px}.mb16{margin-bottom:16px}
.fl{display:flex}.g8{gap:8px}.g12{gap:12px}.ai{align-items:center}.jb{justify-content:space-between}.fw{flex-wrap:wrap}
@media(max-width:900px){
  .grid,.settings-grid,.sn-row{grid-template-columns:1fr}
  .summary{grid-template-columns:1fr}
  .hdr-in{height:auto;min-height:70px;flex-wrap:wrap;padding:12px 0}
  .nav{order:3;width:100%;overflow:auto}
  .nav a{flex:1;justify-content:center}
  .ver{margin-left:auto}
}
@media(max-width:600px){
  .shell{padding:0 14px}.wrap{padding-top:18px}
  .hero{padding:24px 18px}.hero h1{font-size:27px}
  .actions .bt{width:100%}.side{gap:10px}
  .card,.cd{padding:16px}
}
</style>
</head>
<body>
<div class="topline"></div>
<header class="hdr">
  <div class="shell hdr-in">
    <div class="brand">
      <img src="data:image/png;base64,` + logoBase64 + `" alt="SUSHIRO">
      <div><strong>SUSHIRO Overdose</strong><span>reservation assistant</span></div>
    </div>
    <nav class="nav">
      <a href="#" class="on" onclick="go('da',this)">首页</a>
      <a href="#" onclick="go('ca',this)">日历</a>
      <a href="#" onclick="go('in',this)">洞察</a>
      <a href="#" onclick="go('sn',this)">狙击</a>
      <a href="#" onclick="go('re',this)">预约</a>
      <a href="#" onclick="go('se',this)">设置</a>
      <a href="#" onclick="go('lo',this)">日志</a>
    </nav>
    <span class="ver" id="ver">loading</span>
  </div>
</header>

<main class="shell wrap">
  <section id="p-da">
    <div class="grid">
      <div>
        <div class="hero">
          <div class="eyebrow" id="heroBadge">当前步骤</div>
          <h1 id="heroTitle">正在读取状态</h1>
          <p id="heroCopy">请稍等。</p>
          <div class="actions">
            <button class="bt bt-r bt-l" id="bm" onclick="mA()">开始</button>
            <button class="bt bt-o hid" id="bs" onclick="sE()">停止</button>
            <button class="bt bt-w" id="bc" onclick="sC()">重新获取认证</button>
          </div>
          <div id="nc" class="notice hid"></div>
        </div>
        <div id="cb" class="card hid mt16">
          <h2>认证获取进度</h2>
          <div id="cg" class="cg"></div>
        </div>
        <div class="summary">
          <div class="mini"><span>人数</span><strong id="sumPeople">2 成人</strong><p id="sumTable">桌位</p></div>
          <div class="mini"><span>选号策略</span><strong id="sumSlot">未设置</strong><p>时段范围在设置中调整</p></div>
          <div class="mini"><span>运行</span><strong id="sumRun">就绪</strong><p id="sumRunSub">等待下一步</p></div>
        </div>
      </div>
      <aside class="side">
        <div id="eb" class="engine idle"><div class="row"><span class="dot"></span><strong>就绪</strong></div><p>等待操作。</p></div>
        <div class="card">
          <h2>新手路径</h2>
          <div class="track">
            <div class="step" id="step-capture"><div class="mark">1</div><div><b>获取认证</b><span>用 PC 微信打开寿司郎小程序后完成。</span></div></div>
            <div class="step" id="step-prefs"><div class="mark">2</div><div><b>确认偏好</b><span>人数、桌型、工作日和周末时段。</span></div></div>
            <div class="step" id="step-booking"><div class="mark">3</div><div><b>开始抢号</b><span>后台持续查询，成功后通知。</span></div></div>
          </div>
        </div>
        <div class="card">
          <h2>当前偏好</h2>
          <div class="ps" id="ps"></div>
          <button class="bt bt-w bt-s mt16" onclick="go('se',document.querySelector('[onclick*=se]'))">修改设置</button>
        </div>
      </aside>
    </div>
  </section>

  <section id="p-ca" class="hid">
    <div class="cd">
      <div class="fl ai jb mb16 fw g8">
        <div><div class="cd-t">预约日历</div></div>
        <div class="fl g8 fw"><button class="bt bt-w bt-s" onclick="saveCalendarStoresAsPrefs()">保存为抢号门店</button><button class="bt bt-w bt-s" onclick="rC()">刷新</button><select id="ar" onchange="setAR()" style="width:auto"><option value="0">不自动刷新</option><option value="15">15 秒</option><option value="30">30 秒</option><option value="60">60 秒</option></select></div>
      </div>
      <div class="fg"><label>门店</label><div id="storeChoices" class="chips"><span class="mu">加载中</span></div></div>
      <div class="fl g8 fw mb16">
        <label class="check"><input type="checkbox" id="avOnly" onchange="rC()">只看可预约</label>
        <select id="period" onchange="rC()" style="width:auto"><option value="all">全部时段</option><option value="lunch">午餐</option><option value="dinner">晚餐</option></select>
      </div>
      <div class="db" id="dbar"></div>
      <div id="sc"><div class="empty">选择门店查看时段</div></div>
    </div>
  </section>

  <section id="p-in" class="hid">
    <div class="cd">
      <div class="fl ai jb mb16 fw g8"><div class="cd-t" style="margin-bottom:0">历史洞察</div><button class="bt bt-w bt-s" onclick="lI()">刷新</button></div>
      <div id="ic"><div class="empty">加载中</div></div>
    </div>
  </section>

  <section id="p-sn" class="hid">
    <div class="cd">
      <div class="fl ai jb mb16 fw g8"><div class="cd-t" style="margin-bottom:0">Web 狙击计划器</div><div class="fl g8 fw"><button class="bt bt-w bt-s" onclick="addSn()">添加目标</button><button class="bt bt-r bt-s" onclick="saveSn()">保存计划</button><button class="bt bt-y bt-s" onclick="startSn()">启动狙击</button></div></div>
      <div id="snRows"></div>
      <div id="snPlan" class="mt16"><div class="empty">暂无计划</div></div>
    </div>
  </section>

  <section id="p-re" class="hid">
    <div class="cd"><div class="cd-t">我的预约</div><div id="rc"><div class="empty">加载中</div></div></div>
  </section>

  <section id="p-se" class="hid">
    <div class="settings-grid">
      <div class="cd">
        <div class="cd-t">预约偏好</div>
        <div class="fr mb16">
          <div class="fg"><label>成人</label><input type="number" id="pa" min="0" max="10" value="2"></div>
          <div class="fg"><label>儿童</label><input type="number" id="pc" min="0" max="10" value="0"></div>
          <div class="fg"><label>桌型</label><select id="pt"><option value="T">桌位</option><option value="C">吧台</option></select></div>
        </div>
        <div class="fg"><label>抢号门店与优先级</label><div id="bookingStores" class="store-list"><span class="mu">完成认证后可选择门店</span></div></div>
        <div class="fr mb16">
          <div class="fg"><label>日期优先级</label><select id="ppm"><option value="date">按日期优先</option><option value="weekend_first">周末优先</option><option value="weekday_first">工作日优先</option></select></div>
          <div class="fg"><label>时段策略</label><select id="pst"><option value="earliest">最早可约</option><option value="latest">最晚可约</option><option value="closest">接近目标时间</option></select></div>
          <div class="fg"><label>目标时间</label><input type="text" id="ptm" placeholder="1930"></div>
        </div>
        <div class="fg"><label>工作日时段</label><div id="wd" class="tl"></div><span class="at" onclick="aT('wd')">添加时段</span></div>
        <div class="fg"><label>周六时段</label><div id="sa" class="tl"></div><span class="at" onclick="aT('sa')">添加时段</span></div>
        <div class="fg"><label>周日时段</label><div id="su" class="tl"></div><span class="at" onclick="aT('su')">添加时段</span></div>
        <button class="bt bt-r mt8" onclick="sP()">保存偏好</button>
      </div>
      <div class="cd">
        <div class="cd-t">通知渠道</div>
        <div class="fg"><label>飞书 Webhook</label><input type="text" id="nf" placeholder="https://open.feishu.cn/..."></div>
        <div class="fr"><div class="fg" style="flex:1"><label>Telegram Token</label><input type="text" id="ntt" placeholder="123456:ABC..."></div><div class="fg" style="flex:1"><label>Chat ID</label><input type="text" id="ntc" placeholder="-100..."></div></div>
        <div class="fr"><div class="fg" style="flex:1"><label>Bark URL</label><input type="text" id="nbu" placeholder="https://api.day.app"></div><div class="fg" style="flex:1"><label>Bark Key</label><input type="text" id="nbk"></div></div>
        <div class="fg"><label>Server 酱 Key</label><input type="text" id="ns" placeholder="SCT..."></div>
        <div class="fl g8 fw mt8"><button class="bt bt-r" onclick="sN()">保存通知</button><button class="bt bt-w" onclick="tN('all')">测试全部</button><button class="bt bt-w" onclick="tN('feishu')">飞书</button><button class="bt bt-w" onclick="tN('telegram')">Telegram</button><button class="bt bt-w" onclick="tN('bark')">Bark</button><button class="bt bt-w" onclick="tN('serverchan')">Server酱</button></div>
      </div>
      <div class="cd" style="grid-column:1/-1">
        <div class="fl ai jb mb16 fw g8"><div class="cd-t" style="margin-bottom:0">本机诊断</div><div class="fl g8 fw"><button class="bt bt-w bt-s" onclick="lD()">刷新</button><button class="bt bt-w bt-s" onclick="repairP()">修复代理</button><button class="bt bt-o bt-s" onclick="uninstallAll()">卸载清理</button></div></div>
        <div id="dg" class="cg"><div class="ci">尚未加载</div></div>
        <div id="ddetail" class="diag-detail hid"></div>
      </div>
    </div>
  </section>

  <section id="p-lo" class="hid">
    <div class="cd"><div class="cd-t">运行日志</div><div class="lg" id="lv"></div></div>
  </section>
</main>
<footer class="ft">由 <a href="https://github.com/Ryujoxys/sushiro-overdose">sushiro-overdose</a> 驱动 · 非官方工具，仅供学习</footer>

<script>
let cp='da',es={status:'idle'},hc=0,as=[],sd='',pr={},pf='',cE=null,stores=[],selStores=[],calErrs=[],arTimer=null,lastDiag=null;
const W=['日','一','二','三','四','五','六'];
const need=['x_app_code','query_auth','reservation_auth','user_agent','referer','wechat_id','phone_number','store_ids'];
const csrfToken=document.querySelector('meta[name="sushiro-csrf"]')?.content||'';
const rawFetch=window.fetch.bind(window);
function sameOriginRequest(input){
  try{
    const target=input instanceof Request?input.url:String(input);
    return new URL(target,location.href).origin===location.origin;
  }catch(e){return true}
}
window.fetch=(input,init)=>{
  const opt=init?{...init}:{};
  const method=String(opt.method||(input&&input.method)||'GET').toUpperCase();
  if((method==='POST'||method==='PUT')&&sameOriginRequest(input)){
    const h=new Headers(opt.headers||(input&&input.headers)||{});
    h.set('X-Sushiro-CSRF',csrfToken);
    opt.headers=h;
  }
  return rawFetch(input,opt);
};
function el(id){return document.getElementById(id)}
function esc(s){const d=document.createElement('div');d.textContent=s==null?'':String(s);return d.innerHTML}
function escA(s){return esc(s).replaceAll('"','&quot;')}
function go(n,e){document.querySelectorAll('.wrap>section[id^="p-"]').forEach(p=>p.classList.add('hid'));el('p-'+n).classList.remove('hid');document.querySelectorAll('.nav a').forEach(a=>a.classList.remove('on'));if(e)e.classList.add('on');cp=n;({ca:lC,in:lI,sn:lSn,re:lR,se:lS,lo:lL})[n]?.();return false}
async function loadStatus(){try{const r=await(await fetch('/api/status')).json();el('ver').textContent='v'+r.version;hc=!!r.has_config;pf=r.platform||'';es=r.engine||{status:'idle'};uE();uD();}catch(e){el('ver').textContent='offline';}}
async function init(){await loadStatus();await lP();sse();}
function isRun(){return ['capturing','booking','sniping'].includes(es.status)}
function setStep(id,state){const x=el(id);x.classList.remove('active','done');if(state)x.classList.add(state)}
function uD(){
  const b=el('bm'),bc=el('bc'),nc=el('nc'),title=el('heroTitle'),copy=el('heroCopy'),badge=el('heroBadge');
  const run=isRun();
  b.disabled=run;bc.classList.toggle('hid',es.status==='capturing');
  nc.classList.add('hid');nc.textContent='';
  setStep('step-capture',hc?'done':'active');setStep('step-prefs',hc?'active':'');setStep('step-booking',hc&&es.status==='booking'?'active':es.status==='success'?'done':'');
  if(es.status==='capturing'){
    badge.textContent='正在获取认证';title.textContent='保持 PC 微信打开';copy.textContent='在寿司郎小程序中进行一次排队或预约操作，捕获完成后会自动进入下一步。';
    b.textContent='获取中';b.className='bt bt-y bt-l';b.onclick=sC;
  }else if(es.status==='booking'||es.status==='sniping'){
    badge.textContent='正在运行';title.textContent='正在为你查询目标时段';copy.textContent=es.message||'页面可以保持打开，成功后会保存预约并发送通知。';
    b.textContent='运行中';b.className='bt bt-r bt-l';b.onclick=sB;
  }else if(es.status==='success'){
    badge.textContent='已成功';title.textContent='预约成功';copy.textContent=es.message||'预约信息已保存。';
    b.textContent='查看预约';b.className='bt bt-r bt-l';b.onclick=()=>go('re',document.querySelector('[onclick*=re]'));
  }else if(es.status==='error'){
    badge.textContent='需要处理';title.textContent='运行遇到问题';copy.textContent=es.message||'请查看日志后重试。';
    b.textContent=hc?'重新开始':'重新获取认证';b.className='bt bt-y bt-l';b.onclick=hc?sB:sC;
    nc.classList.remove('hid');nc.innerHTML='先打开设置页刷新本机诊断；如果看到代理残留或证书未信任，优先点击“修复代理”后重新获取认证。';
  }else if(!hc){
    badge.textContent='首次设置';title.textContent='先获取认证参数';copy.textContent='准备 PC 微信，点击开始后按页面状态完成一次寿司郎小程序操作。';
    b.textContent='开始获取认证';b.className='bt bt-y bt-l';b.onclick=sC;
    nc.classList.remove('hid');nc.textContent='完成认证后，再确认人数、桌型和目标时段，就可以开始抢号。';
  }else{
    badge.textContent='准备就绪';title.textContent='确认偏好后开始抢号';copy.textContent='当前认证已保存。你可以直接开始，也可以先查看日历或调整偏好。';
    b.textContent='开始抢号';b.className='bt bt-r bt-l';b.onclick=sB;
  }
}
function uE(){
  const box=el('eb'),bs=el('bs'),s=es||{status:'idle'};
  const label={idle:'就绪',capturing:'正在获取认证',booking:'正在抢号',sniping:'狙击中',success:'预约成功',error:'需要处理'}[s.status]||s.status;
  const desc=s.message||({idle:'等待下一步。',capturing:'等待小程序请求。',booking:'正在查询目标时段。',sniping:'高频窗口运行中。',success:'已保存预约信息。',error:'请查看日志。'}[s.status]||'');
  box.className='engine '+s.status;box.innerHTML='<div class="row"><span class="dot"></span><strong>'+esc(label)+'</strong></div><p>'+esc(desc)+'</p>';
  if(s.status==='booking'&&s.attempts)box.innerHTML+='<p>已查询 '+s.attempts+' 次</p>';
  bs.classList.toggle('hid',!isRun());
  el('sumRun').textContent=label;el('sumRunSub').textContent=desc;
  if(s.status==='capturing'&&s.capture){el('cb').classList.remove('hid');rG(s.capture)}else if(s.status!=='capturing'){el('cb').classList.add('hid')}
}
function rG(c){el('cg').innerHTML=need.map(k=>'<div class="ci '+(c[k]?'ok':'')+'">'+fieldName(k)+'</div>').join('')}
function fieldName(k){return {x_app_code:'App Code',query_auth:'查询认证',reservation_auth:'预约认证',user_agent:'设备信息',referer:'小程序来源',wechat_id:'微信 ID',phone_number:'手机号',store_ids:'门店'}[k]||k}
async function sC(){try{const d=await(await fetch('/api/engine/capture',{method:'POST'})).json();if(d.error)alert(d.error);await loadStatus();}catch(e){alert('启动失败')}}
async function sB(){try{const d=await(await fetch('/api/engine/booking',{method:'POST'})).json();if(d.error)alert(d.error);await loadStatus();}catch(e){alert('启动失败')}}
async function sE(){try{await fetch('/api/engine/stop',{method:'POST'});await loadStatus();}catch(e){}}
function mA(){hc?sB():sC()}

async function lC(){await ensureStores();if(!stores.length){el('storeChoices').innerHTML='<span class="mu">先完成认证获取</span>';el('sc').innerHTML='<div class="empty">先完成认证获取，再查看日历</div>';return}if(!selStores.length)selStores=stores.map(s=>String(s.id));rStoreChoices();rC()}
function rStoreChoices(){const c=el('storeChoices');c.innerHTML=stores.map(s=>'<button class="chip '+(selStores.includes(String(s.id))?'on':'')+'" data-store="'+escA(String(s.id))+'">'+esc(s.nickname||s.name||s.id)+'</button>').join('');c.querySelectorAll('.chip').forEach(b=>b.onclick=()=>togStore(b.dataset.store))}
function togStore(id){selStores=selStores.includes(id)?selStores.filter(x=>x!==id):selStores.concat(id);if(!selStores.length&&stores[0])selStores=[String(stores[0].id)];rStoreChoices();sd='';rC()}
async function rC(){if(!selStores.length)return;el('sc').innerHTML='<div class="empty">加载中</div>';const q='stores='+encodeURIComponent(selStores.join(','))+'&available='+(el('avOnly').checked?'1':'0')+'&period='+encodeURIComponent(el('period').value||'all');try{const d=await(await fetch('/api/calendar?'+q)).json();if(d.error){el('sc').innerHTML='<div class="empty">'+esc(d.error)+'</div>';return}as=[];calErrs=[];(d.stores||[]).forEach(st=>{if(st.error)calErrs.push({store:st.store_name||st.store_id,error:st.error});(st.slots||[]).forEach(s=>as.push({...s,store_name:st.store_name,store_id:st.store_id}))});rDB()}catch(e){el('sc').innerHTML='<div class="empty">加载失败</div>'}}
function setAR(){if(arTimer){clearInterval(arTimer);arTimer=null}const sec=+el('ar').value||0;if(sec>0)arTimer=setInterval(()=>{if(cp==='ca')rC()},sec*1000)}
function fD(d){return parseInt(d.substring(4,6),10)+'/'+parseInt(d.substring(6,8),10)}
function fT(t){return t&&t.length>=4?t.substring(0,2)+':'+t.substring(2,4):t||''}
function calendarErrHTML(){return calErrs.length?'<div class="errbox">'+calErrs.map(x=>'<b>'+esc(x.store)+'</b>：'+esc(x.error)).join('<br>')+'<div class="mt8"><button class="bt bt-o bt-s" onclick="sC()">重新获取认证</button></div></div>':''}
function rDB(){const g={};as.forEach(s=>{if(!g[s.date])g[s.date]=[];g[s.date].push(s)});const ds=Object.keys(g).sort(),b=el('dbar');b.innerHTML='';if(!ds.length){el('sc').innerHTML=calendarErrHTML()+'<div class="empty">暂无时段</div>';return}if(!sd||!ds.includes(sd))sd=ds[0];ds.forEach(d=>{const sl=g[d],av=sl.filter(s=>s.availability==='AVAILABLE').length,dt=new Date(d.substring(0,4)+'-'+d.substring(4,6)+'-'+d.substring(6,8)),c=document.createElement('div');c.className='dc'+(d===sd?' on':'');c.innerHTML='<div class="dw">周'+W[dt.getDay()]+'</div><div class="dd">'+fD(d)+'</div><div class="dv '+(av>0?'h':'n')+'">'+(av>0?'可约 '+av:'已满')+'</div>';c.onclick=()=>{sd=d;rDB()};b.appendChild(c)});rS(sd)}
function rS(d){const sl=as.filter(s=>s.date===d).sort((a,b)=>(a.store_name||'').localeCompare(b.store_name||'')||(a.start||'').localeCompare(b.start||'')),c=el('sc');if(!sl.length){c.innerHTML=calendarErrHTML()+'<div class="empty">无时段</div>';return}const ac=sl.filter(s=>s.availability==='AVAILABLE').length;c.innerHTML=calendarErrHTML()+'<div class="sg">'+sl.map(s=>{const a=s.availability==='AVAILABLE';return'<div class="sl '+(a?'av':'fu')+'"><div class="tm">'+esc(fT(s.start))+'-'+esc(fT(s.end))+'</div><div class="ss">'+(a?'可预约':'已满')+' · '+esc(s.store_name||s.store_id||'')+'</div></div>'}).join('')+'</div><p class="mu mt8">'+sl.length+' 个时段 · '+ac+' 个可预约 · '+selStores.length+' 家门店</p>'}

async function lI(){await ensureStores();const c=el('ic');c.innerHTML='<div class="empty">分析中</div>';try{const d=await(await fetch('/api/insights?top=12')).json();if(d.error){c.innerHTML='<div class="empty">'+esc(d.error)+'</div>';return}const rec=d.recommendations||[];const metrics='<div class="metric">'+chip('历史样本',d.valid_snapshots||0,'ok')+chip('跳过样本',d.skipped_snapshots||0,(d.skipped_snapshots||0)?'warn':'ok')+chip('推荐数量',rec.length,'ok')+'</div>';const rows=rec.map(r=>'<tr><td>'+esc(storeName(r.store_id))+'<br><span class="mu">'+esc(r.store_id)+'</span></td><td>'+esc(r.weekday_name)+'</td><td>'+esc(fT(r.start))+'-'+esc(fT(r.end))+'</td><td>'+Math.round((r.availability_rate||0)*100)+'%</td><td>'+(r.sold_out_minutes==null?'-':Math.round(r.sold_out_minutes)+' 分')+'</td><td>'+esc(r.observations)+'</td></tr>').join('');c.innerHTML=metrics+(rows?'<table class="tbl"><thead><tr><th>门店</th><th>星期</th><th>时段</th><th>开放概率</th><th>售罄速度</th><th>样本</th></tr></thead><tbody>'+rows+'</tbody></table>':'<div class="empty">暂无历史数据。<div class="mt8"><button class="bt bt-w bt-s" onclick="go(\'ca\',document.querySelector(\'[onclick*=ca]\'))">去日历刷新一次</button></div></div>')}catch(e){c.innerHTML='<div class="empty">洞察加载失败</div>'}}

async function lSn(){await ensureStores();if(!el('snRows').children.length)addSn();await loadSnPlan()}
async function ensureStores(){if(stores.length)return;try{stores=await(await fetch('/api/stores')).json();selStores=stores.map(s=>String(s.id));}catch(e){}}
function storeOpts(v){return stores.map(s=>'<option value="'+escA(String(s.id))+'" '+(String(s.id)===String(v)?'selected':'')+'>'+esc(s.nickname||s.name||s.id)+'</option>').join('')}
function dateInputValue(v){v=String(v||'');return /^\d{8}$/.test(v)?v.slice(0,4)+'-'+v.slice(4,6)+'-'+v.slice(6,8):v}
function timeInputValue(v){v=String(v||'');return /^\d{6}$/.test(v)?v.slice(0,2)+':'+v.slice(2,4):/^\d{4}$/.test(v)?v.slice(0,2)+':'+v.slice(2,4):v}
function compactDate(v){v=String(v||'').trim();return /^\d{4}-\d{2}-\d{2}$/.test(v)?v.replaceAll('-',''):v}
function compactTime(v){v=String(v||'').trim();return /^\d{2}:\d{2}$/.test(v)?v.replace(':',''):v.replaceAll(':','')}
function validDate8(v){if(!/^\d{8}$/.test(v))return false;const d=new Date(v.slice(0,4)+'-'+v.slice(4,6)+'-'+v.slice(6,8));return !Number.isNaN(d.getTime())&&d.toISOString().slice(0,10).replaceAll('-','')===v}
function timeSec(v){v=compactTime(v);if(!/^(?:\d{4}|\d{6})$/.test(v))return -1;const h=+v.slice(0,2),m=+v.slice(2,4),s=v.length===6?+v.slice(4,6):0;return h>=0&&h<=23&&m>=0&&m<=59&&s>=0&&s<=59?h*3600+m*60+s:-1}
function addSnErr(row,msg){const d=document.createElement('div');d.className='inline-err';d.textContent=msg;row.appendChild(d)}
function addSn(t={}){const c=el('snRows'),d=document.createElement('div');d.className='sn-row';d.innerHTML='<div class="fg"><label>日期</label><input type="date" value="'+escA(dateInputValue(t.date||''))+'"></div><div class="fg"><label>最早</label><input type="time" value="'+escA(timeInputValue(t.start_after||'1930'))+'"></div><div class="fg"><label>最晚</label><input type="time" value="'+escA(timeInputValue(t.start_before||'2030'))+'"></div><div class="fg"><label>门店</label><select>'+storeOpts(t.store_id||stores[0]?.id||'')+'</select></div><button class="bt bt-o bt-s" onclick="this.parentElement.remove()">删除</button>';c.appendChild(d)}
function readSnTargets(){const rows=Array.from(el('snRows').children);let ok=true;const targets=[];rows.forEach(r=>{r.querySelector('.inline-err')?.remove();const i=r.querySelectorAll('input'),s=r.querySelector('select'),date=compactDate(i[0].value),start=compactTime(i[1].value),end=compactTime(i[2].value),ss=timeSec(start),es=timeSec(end);if(!date&&!start&&!end&&!s.value)return;if(!validDate8(date)){ok=false;addSnErr(r,'日期无效');return}if(ss<0||es<0){ok=false;addSnErr(r,'时间无效');return}if(ss>=es){ok=false;addSnErr(r,'最晚时间必须晚于最早时间');return}if(!s.value){ok=false;addSnErr(r,'请选择门店');return}targets.push({date,start_after:start,start_before:end,store_id:s.value})});return{ok,targets}}
function snTargets(){const r=readSnTargets();return r.ok?r.targets:[]}
async function saveSn(){const read=readSnTargets();if(!read.ok)return;if(!read.targets.length){alert('请至少添加一个有效目标');return}try{const d=await(await fetch('/api/sniper/plan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({targets:read.targets})})).json();if(d.error){alert(d.error);return}renderSnPlan(d.plan);alert('已保存狙击计划')}catch(e){alert('保存失败')}}
async function loadSnPlan(){try{const d=await(await fetch('/api/sniper/plan')).json();if(d.targets?.length){el('snRows').innerHTML='';d.targets.forEach(addSn)}renderSnPlan(d)}catch(e){}}
function renderSnPlan(p){const c=el('snPlan'),ts=p?.targets||[];if(!ts.length){c.innerHTML='<div class="empty">暂无计划</div>';return}c.innerHTML='<table class="tbl"><thead><tr><th>目标</th><th>开放窗口</th><th>状态</th><th>尝试</th><th>最后错误</th></tr></thead><tbody>'+ts.map(t=>'<tr><td>'+esc(t.store_id)+'<br>'+esc(t.date)+' '+esc(fT(t.start_after))+'-'+esc(fT(t.start_before))+'</td><td>'+esc(t.open_at?new Date(t.open_at).toLocaleString():'-')+'<br>'+(t.countdown_seconds>0?Math.ceil(t.countdown_seconds/60)+' 分钟后':'窗口内/已结束')+'</td><td>'+esc(t.status||'-')+'</td><td>'+esc(t.attempts||0)+'</td><td>'+esc(t.last_error||'')+'</td></tr>').join('')+'</tbody></table>'}
async function startSn(){const read=readSnTargets();if(!read.ok)return;if(!read.targets.length){alert('请至少添加一个有效目标');return}try{const d=await(await fetch('/api/sniper/start',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({targets:read.targets})})).json();if(d.error){alert(d.error);return}await loadStatus();await loadSnPlan();alert('狙击计划已启动')}catch(e){alert('启动失败')}}

async function lR(){try{const d=await(await fetch('/api/reservations')).json(),c=el('rc');if(d.error){c.innerHTML='<div class="empty">'+esc(d.error)+'</div>';return}if(!d.length){c.innerHTML='<div class="empty">暂无预约</div>';return}c.innerHTML='<div class="sg">'+d.map(r=>'<div class="sl av"><div class="tm">'+esc(r.number||'-')+'</div><div class="ss">'+esc(r.status||'-')+'</div><div class="mu mt8">#'+esc(r.ticketId||'')+'</div></div>').join('')+'</div>'}catch(e){el('rc').innerHTML='<div class="empty">加载失败</div>'}}

async function lP(){try{pr=await(await fetch('/api/preferences')).json();fF(pr);dP(pr);renderBookingStores();uD()}catch(e){}}
function fF(p){el('pa').value=p.adult||2;el('pc').value=p.child||0;el('pt').value=p.table_type||'T';el('ppm').value=p.day_priority_mode||'date';el('pst').value=p.slot_strategy||'earliest';el('ptm').value=p.target_time||'1930';rT('wd',p.weekday_slots||[]);rT('sa',p.saturday_slots||[]);rT('su',p.sunday_slots||[])}
function rangeText(rs){return !rs||!rs.length?'不预约':rs.map(r=>fT(String(r.start||''))+'-'+fT(String(r.end||''))).join('、')}
function priText(v){return v==='weekend_first'?'周末优先':v==='weekday_first'?'工作日优先':'按日期优先'}
function stratText(v,t){return v==='latest'?'最晚可约':v==='closest'?'接近 '+fT(t||'1930'):'最早可约'}
function dP(p){const people=(p.adult||2)+' 成人'+((p.child||0)>0?' · '+p.child+' 儿童':'');const table=(p.table_type||'T')==='C'?'吧台':'桌位',pri=priText(p.day_priority_mode),str=stratText(p.slot_strategy,p.target_time);el('sumPeople').textContent=people;el('sumTable').textContent=table;el('sumSlot').textContent=pri+' · '+str;el('ps').innerHTML='<b>'+esc(people)+'</b> · '+esc(table)+'<span class="line">优先级：'+esc(pri)+' · '+esc(str)+'</span><span class="line">工作日：'+esc(rangeText(p.weekday_slots))+'</span><span class="line">周六：'+esc(rangeText(p.saturday_slots))+'</span><span class="line">周日：'+esc(rangeText(p.sunday_slots))+'</span>'}
function storeName(id){const s=stores.find(x=>String(x.id)===String(id));return s?(s.nickname||s.name||s.id):id}
function orderedStoreIDs(){const all=stores.map(s=>String(s.id)),sel=(pr.selected_stores||[]).map(String).filter(id=>all.includes(id)),base=(pr.store_priority||[]).map(String).filter(id=>all.includes(id));let order=[];base.forEach(id=>{if(!order.includes(id))order.push(id)});sel.forEach(id=>{if(!order.includes(id))order.push(id)});all.forEach(id=>{if(!order.includes(id))order.push(id)});return{all,selected:sel.length?sel:all,order}}
function renderBookingStores(){const box=el('bookingStores');if(!box)return;if(!stores.length){box.innerHTML='<span class="mu">完成认证后可选择门店</span>';return}const data=orderedStoreIDs(),set=new Set(data.selected);box.innerHTML=data.order.map(id=>'<div class="store-row" data-store="'+escA(id)+'"><input type="checkbox" '+(set.has(id)?'checked':'')+'><div><b>'+esc(storeName(id))+'</b><span>'+esc(id)+'</span></div><button type="button" class="ico" onclick="moveStoreRow(this,-1)">↑</button><button type="button" class="ico" onclick="moveStoreRow(this,1)">↓</button></div>').join('')}
function moveStoreRow(btn,dir){const r=btn.closest('.store-row'),p=r.parentElement;if(dir<0&&r.previousElementSibling)p.insertBefore(r,r.previousElementSibling);if(dir>0&&r.nextElementSibling)p.insertBefore(r.nextElementSibling,r)}
function bookingStoresFromUI(){const rows=Array.from(document.querySelectorAll('#bookingStores .store-row')),selected=[];rows.forEach(r=>{if(r.querySelector('input').checked)selected.push(r.dataset.store)});return{selected_stores:selected,store_priority:selected}}
function rT(k,rs){const c=el(k);c.innerHTML='';(rs||[]).forEach(r=>{const d=document.createElement('div');d.className='tr';d.innerHTML='<input type="text" value="'+escA(r.start||'')+'" placeholder="1930"><span class="sp">至</span><input type="text" value="'+escA(r.end||'')+'" placeholder="2030"><span class="x" onclick="this.parentElement.remove()">×</span>';c.appendChild(d)});if(!rs||!rs.length)c.innerHTML='<span class="mu">不预约</span>'}
function aT(k){const c=el(k);if(c.querySelector('.mu'))c.innerHTML='';const d=document.createElement('div');d.className='tr';d.innerHTML='<input type="text" placeholder="1930"><span class="sp">至</span><input type="text" placeholder="2030"><span class="x" onclick="this.parentElement.remove()">×</span>';c.appendChild(d)}
function gT(k){const ip=document.querySelectorAll('#'+k+' input'),r=[];for(let i=0;i<ip.length;i+=2){const s=ip[i].value.trim(),e=ip[i+1]?ip[i+1].value.trim():'';if(s||e)r.push({start:s,end:e})}return r}
function prefsPayload(){const st=bookingStoresFromUI();return{adult:+el('pa').value||2,child:+el('pc').value||0,table_type:el('pt').value||'T',selected_stores:st.selected_stores,store_priority:st.store_priority,day_priority_mode:el('ppm').value||'date',day_priority:pr.day_priority||['saturday','sunday','weekday'],slot_strategy:el('pst').value||'earliest',target_time:el('ptm').value.trim()||'1930',weekday_slots:gT('wd'),saturday_slots:gT('sa'),sunday_slots:gT('su')}}
async function savePrefsPayload(b,quiet){try{const d=await(await fetch('/api/preferences',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)})).json();if(d.error){if(!quiet)alert(d.error);return false}pr=d.preferences||b;fF(pr);dP(pr);renderBookingStores();uD();if(!quiet)alert('已保存');return true}catch(e){if(!quiet)alert('保存失败');return false}}
async function sP(){const b=prefsPayload();if(stores.length&&!b.selected_stores.length){alert('请至少选择一家抢号门店');return false}return savePrefsPayload(b,false)}
async function saveCalendarStoresAsPrefs(){if(!selStores.length){alert('请先选择门店');return}await lP();const b={...pr,selected_stores:selStores.slice(),store_priority:selStores.slice()};if(await savePrefsPayload(b,true))alert('已保存为抢号门店优先级')}
async function lS(){await lP();await ensureStores();renderBookingStores();try{const c=await(await fetch('/api/config')).json();el('nf').value=c.feishu?.webhook||'';el('ntt').value=c.telegram?.token||'';el('ntc').value=c.telegram?.chat_id||'';el('nbu').value=c.bark?.url||'';el('nbk').value=c.bark?.key||'';el('ns').value=c.server_chan?.key||''}catch(e){}lD()}
async function sN(quiet){const b={feishu:{webhook:el('nf').value.trim()},telegram:{token:el('ntt').value.trim(),chat_id:el('ntc').value.trim()},bark:{url:el('nbu').value.trim(),key:el('nbk').value.trim()},server_chan:{key:el('ns').value.trim()}};try{const d=await(await fetch('/api/config',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)})).json();if(d.error){if(!quiet)alert(d.error);return false}if(!quiet)alert('已保存');return true}catch(e){if(!quiet)alert('保存失败');return false}}
async function tN(ch){if(!await sN(true))return;try{const r=await fetch('/api/notifications/test',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({channel:ch||'all'})}),d=await r.json();if(d.error){alert(d.error);return}const bad=(d.results||[]).filter(x=>!x.ok).map(x=>x.channel+': '+x.error);alert(bad.length?'已先保存当前表单，部分发送失败：\n'+bad.join('\n'):'已先保存当前表单，测试通知已发送')}catch(e){alert('发送失败')}}
function chip(t,s,c){return'<div class="ci '+c+'">'+esc(t)+'：'+esc(s)+'</div>'}
function diagDetail(d){const cfg=d.config||{},cert=d.certificate||{},pm=d.proxy_marker||{},sp=d.system_proxy||{},net=d.network||{},logs=d.engine_log_tail||[];const badPorts=(d.ports||[]).filter(p=>!p.available).map(p=>p.name+': '+(p.error||'占用'));let html='<b>下一步建议</b><br>';if(!cfg.complete)html+='先重新获取认证参数。<br>';if(cert.cert_exists&&!cert.trusted)html+='证书已生成但未信任，请在系统证书管理中信任，或重新获取认证触发安装。<br>';if(pm.stale||pm.active)html+='发现代理状态，请先点“修复代理”。<br>';if(!net.reachable)html+='寿司郎网络不可达，先确认网络或稍后重试。<br>';html+='<br><b>证书</b>：<code>'+esc(cert.cert_path||'-')+'</code>'+(cert.trust_error?'<br>'+esc(cert.trust_error):'');if(badPorts.length)html+='<br><b>端口</b>：'+esc(badPorts.join('；'));if((sp.summary||[]).length)html+='<br><b>系统代理</b>：'+esc(sp.summary.join('；'));if(logs.length)html+='<br><b>最近日志</b><br>'+logs.slice(-5).map(l=>esc((l.time||'')+' '+(l.message||''))).join('<br>');return html}
async function lD(){const box=el('dg'),detail=el('ddetail');if(!box)return;box.innerHTML='<div class="ci">诊断中</div>';if(detail)detail.classList.add('hid');try{const d=await(await fetch('/api/diagnostics')).json();lastDiag=d;const cfg=d.config||{},cert=d.certificate||{},pm=d.proxy_marker||{},sp=d.system_proxy||{},eng=d.engine||{},net=d.network||{};const miss=(cfg.missing||[]).join('、');const ports=(d.ports||[]).filter(p=>p.in_use).map(p=>p.name).join('、');const items=[];items.push(chip('认证参数',cfg.complete?'完整':(miss||'未捕获'),cfg.complete?'ok':'bad'));items.push(chip('门店',cfg.store_count?cfg.store_count+' 个':'未选择',cfg.store_count?'ok':'bad'));items.push(chip('证书',cert.trusted?'已信任':cert.cert_exists?'未信任':'未生成',cert.trusted?'ok':'bad'));items.push(chip('端口',ports||'默认端口可用',ports?'warn':'ok'));items.push(chip('代理残留',pm.stale?'发现残留':pm.active?'运行中':'未发现',pm.stale?'bad':pm.active?'warn':'ok'));items.push(chip('系统代理',sp.available?'可读取':'不可读取',sp.available?'ok':'warn'));items.push(chip('网络',net.reachable?'寿司郎可达':'不可达',net.reachable?'ok':'bad'));items.push(chip('通知',cfg.notification_channels?.length?cfg.notification_channels.join('、'):'未配置',cfg.notification_channels?.length?'ok':'warn'));items.push(chip('引擎',eng.status||'idle',eng.status==='error'?'bad':(eng.status==='booking'||eng.status==='capturing'||eng.status==='sniping')?'warn':'ok'));box.innerHTML=items.join('');if(detail){detail.innerHTML=diagDetail(d);detail.classList.remove('hid')}}catch(e){box.innerHTML='<div class="ci bad">诊断加载失败</div>'}}
async function repairP(){try{const d=await(await fetch('/api/repair-proxy',{method:'POST'})).json();alert(d.ok?'代理已恢复':'修复失败，请看 doctor');lD()}catch(e){alert('修复失败')}}
async function uninstallAll(){if(!confirm('将恢复代理、移除证书并清理本地敏感数据。继续？'))return;try{const d=await(await fetch('/api/uninstall',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({all:true,certificates:true,system_cert:true})})).json();alert(d.ok?'已清理':'部分清理失败，请看 doctor');lD()}catch(e){alert('清理失败')}}

async function lL(){try{const ls=await(await fetch('/api/engine/logs')).json(),v=el('lv');v.innerHTML=(ls||[]).map(l=>'<div class="ll '+(l.level==='error'?'er':'')+'"><span class="lt">'+esc(l.time)+'</span><span class="lm">'+esc(l.message)+'</span></div>').join('');v.scrollTop=v.scrollHeight}catch(e){}}
function aL(e){const v=el('lv');if(!v)return;const d=document.createElement('div');d.className='ll '+(e.level==='error'?'er':'');d.innerHTML='<span class="lt">'+esc(e.time)+'</span><span class="lm">'+esc(e.message)+'</span>';v.appendChild(d);if(cp==='lo')v.scrollTop=v.scrollHeight}
function sse(){if(cE)cE.close();const s=new EventSource('/api/events');cE=s;s.onopen=()=>{loadStatus()};s.addEventListener('engine',e=>{try{es=JSON.parse(e.data);uE();uD();if(cp==='sn')loadSnPlan();if(['idle','success','error'].includes(es.status))loadStatus()}catch(x){}});s.addEventListener('log',e=>{try{aL(JSON.parse(e.data))}catch(x){}});s.addEventListener('calendar',e=>{try{const d=JSON.parse(e.data);if(cp==='ca'){as=[];(d.stores||[]).forEach(st=>(st.slots||[]).forEach(x=>as.push({...x,store_name:st.store_name,store_id:st.store_id})));if(as.length)rDB()}}catch(x){}});s.addEventListener('ping',()=>{});s.onerror=()=>{s.close();cE=null;setTimeout(sse,3000)}}
init();
</script>
</body></html>
`
