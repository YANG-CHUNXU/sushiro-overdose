package main

const logoBase64 = "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"

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>SUSHIRO Overdose</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --red:#B81C22;--red-dk:#9a1519;--red-lt:#fef0f0;
  --bg:#f5f5f5;--white:#fff;--text:#222;--sub:#666;--mute:#aaa;
  --bdr:#e5e5e5;--bdr2:#ddd;
  --green:#1a8c3a;--green-bg:#eef8f0;
  --yellow:#d4a017;--yellow-bg:#fef9ec;
  --shadow:0 2px 8px rgba(0,0,0,.06);
  --shadow-lg:0 8px 24px rgba(0,0,0,.08);
  --f:'PingFang SC',-apple-system,BlinkMacSystemFont,'Segoe UI','Microsoft YaHei',sans-serif;
}
body{font-family:var(--f);background:var(--bg);color:var(--text);-webkit-font-smoothing:antialiased}

/* ── Header ── */
.hdr{background:var(--white);border-bottom:1px solid var(--bdr);position:sticky;top:0;z-index:50}
.hdr-top{height:3px;background:var(--red)}
.hdr-main{max-width:860px;margin:0 auto;padding:14px 24px;display:flex;align-items:center;gap:12px}
.hdr-main img{width:36px;height:36px}
.hdr-main .brand{font-size:15px;font-weight:700;color:var(--text);letter-spacing:.5px}
.hdr-main .brand em{font-style:normal;color:var(--red);font-weight:800;margin-left:4px;font-size:13px}
.hdr-main .tag{margin-left:auto;font-size:11px;color:var(--mute);background:var(--bg);padding:3px 10px;border-radius:99px}
/* ── Nav ── */
.nav{background:var(--white);border-bottom:1px solid var(--bdr)}
.nav-in{max-width:860px;margin:0 auto;display:flex;padding:0 24px;gap:0}
.nav-in a{padding:13px 20px;font-size:13px;font-weight:600;color:var(--sub);text-decoration:none;border-bottom:2px solid transparent;transition:.15s}
.nav-in a:hover{color:var(--text)}
.nav-in a.on{color:var(--red);border-bottom-color:var(--red)}

/* ── Container ── */
.wrap{max-width:860px;margin:0 auto;padding:28px 24px 80px}

/* ── Card ── */
.cd{background:var(--white);border-radius:12px;padding:24px;margin-bottom:18px;box-shadow:var(--shadow);border:1px solid var(--bdr)}
.cd-t{font-size:11px;font-weight:700;color:var(--mute);letter-spacing:1px;text-transform:uppercase;margin-bottom:14px;padding-bottom:10px;border-bottom:1px solid var(--bdr)}

/* ── Banner ── */
.bn{display:flex;align-items:center;gap:10px;padding:14px 18px;border-radius:10px;margin-bottom:18px;font-size:14px;font-weight:500}
.bn.idle{background:var(--bg);color:var(--sub);border:1px solid var(--bdr)}
.bn.capturing{background:var(--yellow-bg);color:var(--yellow);border:1px solid #ecd47a}
.bn.booking{background:var(--green-bg);color:var(--green);border:1px solid #aad6b2}
.bn.success{background:var(--green-bg);color:var(--green);border:1px solid var(--green);font-weight:700}
.bn.error{background:var(--red-lt);color:var(--red);border:1px solid #e4aaaa}
.d{width:7px;height:7px;border-radius:50%;flex-shrink:0}
.d-g{background:#bbb}.d-gr{background:var(--green)}.d-r{background:var(--red)}.d-y{background:var(--yellow)}
.bn .tail{margin-left:auto;font-size:11px;opacity:.6;font-weight:400}

/* ── Buttons ── */
.bt{display:inline-flex;align-items:center;justify-content:center;gap:6px;padding:10px 24px;border:none;border-radius:99px;cursor:pointer;font-size:13px;font-weight:700;font-family:var(--f);transition:.2s;text-decoration:none;line-height:1}
.bt-l{padding:14px 32px;font-size:15px}
.bt-s{padding:7px 16px;font-size:12px}
.bt-r{background:var(--red);color:var(--white);box-shadow:0 2px 8px rgba(184,28,34,.2)}
.bt-r:hover{background:var(--red-dk);transform:translateY(-1px);box-shadow:0 4px 12px rgba(184,28,34,.25)}
.bt-o{background:var(--white);color:var(--red);border:1.5px solid var(--red)}
.bt-o:hover{background:var(--red-lt)}
.bt-w{background:var(--white);color:var(--sub);border:1.5px solid var(--bdr2)}
.bt-w:hover{border-color:var(--sub);color:var(--text)}
.bt-y{background:#F5BA24;color:#333;border:2px solid #333}
.bt:disabled{opacity:.3;cursor:not-allowed;transform:none!important}

/* ── Capture ── */
.cg{display:grid;grid-template-columns:repeat(auto-fill,minmax(140px,1fr));gap:6px;margin:12px 0}
.ci{padding:7px 10px;background:var(--bg);border-radius:6px;font-size:12px;display:flex;align-items:center;gap:5px}
.ci.ok{background:var(--green-bg);color:var(--green);font-weight:600}

/* ── Form ── */
.fg{margin-bottom:14px}
.fg label{display:block;font-size:11px;color:var(--sub);margin-bottom:4px;font-weight:600;letter-spacing:.3px}
.fr{display:flex;gap:12px;flex-wrap:wrap}
input[type=number],input[type=text],select{padding:9px 12px;background:var(--white);border:1px solid var(--bdr);border-radius:8px;color:var(--text);font-size:13px;font-family:var(--f);width:100%;transition:.15s}
input:focus,select:focus{outline:0;border-color:var(--red);box-shadow:0 0 0 3px rgba(184,28,34,.06)}
input[type=number]{width:72px}

/* ── Slots ── */
.sg{display:grid;grid-template-columns:repeat(auto-fill,minmax(110px,1fr));gap:6px}
.sl{padding:10px;background:var(--bg);border-radius:8px;font-size:12px;border:1px solid transparent;transition:.15s}
.sl.av{background:var(--green-bg);border-color:#c8e6cc;cursor:pointer}
.sl.av:hover{box-shadow:var(--shadow);transform:translateY(-1px)}
.sl.fu{opacity:.4}
.sl .tm{font-weight:700;font-size:14px}
.sl .ss{font-size:10px;margin-top:2px}
.sl.av .ss{color:var(--green)}.sl.fu .ss{color:var(--red)}

/* ── Date bar ── */
.db{display:flex;gap:5px;overflow-x:auto;padding-bottom:6px;margin-bottom:14px}
.dc{flex-shrink:0;padding:8px 12px;background:var(--white);border:1px solid var(--bdr);border-radius:8px;cursor:pointer;font-size:12px;text-align:center;min-width:60px;transition:.15s}
.dc:hover{border-color:var(--bdr2);background:var(--bg)}
.dc.on{background:var(--red);color:var(--white);border-color:var(--red);box-shadow:0 2px 8px rgba(184,28,34,.2)}
.dc .dw{font-size:10px;color:var(--mute);margin-bottom:1px}.dc.on .dw{color:rgba(255,255,255,.7)}
.dc .dd{font-weight:700}
.dc .dv{font-size:9px;margin-top:1px}.dc .dv.h{color:var(--green)}.dc .dv.n{color:var(--red)}.dc.on .dv{color:rgba(255,255,255,.65)}

/* ── Time ranges ── */
.tl{display:flex;flex-direction:column;gap:6px}
.tr{display:flex;align-items:center;gap:6px}
.tr input{width:72px;text-align:center}
.tr .sp{color:var(--mute);font-size:12px}
.tr .x{cursor:pointer;color:var(--red);font-size:16px;line-height:1}
.at{color:var(--green);cursor:pointer;font-size:12px;margin-top:4px;font-weight:600}

/* ── Log ── */
.lg{max-height:400px;overflow-y:auto;font-family:'SF Mono',Menlo,Consolas,monospace;font-size:11px;line-height:1.9;padding:14px;background:var(--bg);border-radius:8px}
.ll{display:flex;gap:8px}.ll .lt{color:var(--mute);flex-shrink:0}.ll.er .lm{color:var(--red)}

/* ── Wizard ── */
.wo{position:fixed;inset:0;background:rgba(0,0,0,.3);z-index:100;display:flex;align-items:center;justify-content:center;backdrop-filter:blur(4px)}
.wz{background:var(--white);border-radius:16px;padding:36px;max-width:460px;width:92%;max-height:85vh;overflow-y:auto;box-shadow:var(--shadow-lg)}
.wz h2{text-align:center;font-size:18px;margin-bottom:4px}
.wz .su{text-align:center;color:var(--sub);margin-bottom:24px;font-size:13px}
.wz .ds{display:flex;gap:6px;justify-content:center;margin-bottom:24px}
.wz .wd{width:28px;height:3px;border-radius:2px;background:var(--bdr);transition:.3s}
.wz .wd.a{background:var(--red);width:42px}.wz .wd.d{background:var(--green)}
.wz .sc{min-height:160px}
.wz .sa{display:flex;justify-content:space-between;margin-top:24px}
.wz .gb{background:var(--bg);border-radius:8px;padding:14px;margin:10px 0;font-size:12px;line-height:1.8}
.wz .gb ol{padding-left:18px}.wz .gb li{margin-bottom:3px}

/* ── Prefs summary ── */
.ps{font-size:13px;line-height:2;color:var(--sub)}
.ps b{color:var(--text);font-weight:600}

/* ── Footer ── */
.ft{text-align:center;padding:20px;font-size:11px;color:var(--mute)}
.ft a{color:var(--red);text-decoration:none}

/* ── Utils ── */
.hid{display:none!important}.mu{color:var(--mute)}.tc{text-align:center}
.tg{color:var(--green)}.tre{color:var(--red)}
.mt8{margin-top:8px}.mt16{margin-top:16px}.mb16{margin-bottom:16px}
.fl{display:flex}.g8{gap:8px}.g12{gap:12px}.ai{align-items:center}.jb{justify-content:space-between}.fw{flex-wrap:wrap}

@media(max-width:640px){
  .nav-in{overflow-x:auto}.nav-in a{padding:10px 14px;font-size:12px;white-space:nowrap}
  .wrap{padding:16px 16px 60px}.hdr-main .tag{display:none}
}
</style>
</head>
<body>
<div class="hdr">
  <div class="hdr-top"></div>
  <div class="hdr-main">
    <img src="data:image/png;base64,` + logoBase64 + `" alt="SUSHIRO">
    <span class="brand">SUSHIRO<em>Overdose</em></span>
    <span class="tag" id="ver">loading</span>
  </div>
</div>
<div class="nav"><div class="nav-in">
  <a href="#" class="on" onclick="go('da',this)">控制台</a>
  <a href="#" onclick="go('ca',this)">预约日历</a>
  <a href="#" onclick="go('re',this)">我的预约</a>
  <a href="#" onclick="go('se',this)">设置</a>
  <a href="#" onclick="go('lo',this)">日志</a>
</div></div>
<div class="wrap">
  <div id="p-da">
    <div id="eb" class="bn idle"><span class="d d-g"></span><span>就绪</span></div>
    <div id="cb" class="cd hid"><div class="cd-t">参数捕获进度</div><div id="cg" class="cg"></div><p class="mu mt8" style="font-size:12px">请在 PC 微信中打开寿司郎小程序，进行一次排队/预约操作</p></div>
    <div class="cd"><div class="cd-t">操作</div>
      <div class="fl g12 fw"><button class="bt bt-r bt-l" id="bm" onclick="mA()">开始抢号</button><button class="bt bt-o hid" id="bs" onclick="sE()">停止</button><button class="bt bt-w" id="bc" onclick="sC()">重新捕获参数</button></div>
      <div id="nc" class="hid" style="margin-top:14px;font-size:12px;background:var(--yellow-bg);padding:12px 16px;border-radius:8px;border:1px solid #ecd47a;color:var(--yellow)">尚未获取认证参数。请先点击「开始捕获参数」完成首次设置。</div>
    </div>
    <div class="cd"><div class="cd-t">当前配置</div><div class="ps" id="ps"></div></div>
  </div>
  <div id="p-ca" class="hid"><div class="cd"><div class="fl ai jb mb16 fw g8"><select id="ss" onchange="oSC()" style="width:auto;min-width:180px"></select><button class="bt bt-w bt-s" onclick="rC()">刷新</button></div><div class="db" id="dbar"></div><div id="sc"><p class="mu tc">选择门店查看时段</p></div></div></div>
  <div id="p-re" class="hid"><div class="cd"><div id="rc"><p class="mu tc">加载中...</p></div></div></div>
  <div id="p-se" class="hid">
    <div class="cd"><div class="cd-t">预约偏好</div>
      <div class="fr mb16"><div class="fg"><label>成人</label><input type="number" id="pa" min="0" max="10" value="2"></div><div class="fg"><label>儿童</label><input type="number" id="pc" min="0" max="10" value="0"></div><div class="fg"><label>桌型</label><select id="pt"><option value="T">桌位 (T)</option><option value="C">吧台 (C)</option></select></div></div>
      <div class="fg"><label>工作日时段</label><div id="wd" class="tl"></div><span class="at" onclick="aT('wd')">+ 添加</span></div>
      <div class="fg"><label>周六时段</label><div id="sa" class="tl"></div><span class="at" onclick="aT('sa')">+ 添加</span></div>
      <div class="fg"><label>周日时段</label><div id="su" class="tl"></div><span class="at" onclick="aT('su')">+ 添加</span></div>
      <button class="bt bt-r mt8" onclick="sP()">保存偏好</button></div>
    <div class="cd"><div class="cd-t">通知渠道</div>
      <div class="fg"><label>飞书 Webhook</label><input type="text" id="nf" placeholder="https://open.feishu.cn/..."></div>
      <div class="fr"><div class="fg" style="flex:1"><label>Telegram Token</label><input type="text" id="ntt" placeholder="123456:ABC..."></div><div class="fg" style="flex:1"><label>Chat ID</label><input type="text" id="ntc" placeholder="-100..."></div></div>
      <div class="fr"><div class="fg" style="flex:1"><label>Bark URL</label><input type="text" id="nbu" placeholder="https://api.day.app"></div><div class="fg" style="flex:1"><label>Bark Key</label><input type="text" id="nbk"></div></div>
      <div class="fg"><label>Server酱 Key</label><input type="text" id="ns" placeholder="SCT..."></div>
      <button class="bt bt-r mt8" onclick="sN()">保存通知</button></div>
  </div>
  <div id="p-lo" class="hid"><div class="cd"><div class="lg" id="lv"></div></div></div>
</div>
<div class="ft">由 <a href="https://github.com/Ryujoxys/sushiro-overdose">sushiro-overdose</a> 驱动 · 非官方工具，仅供学习</div>

<div class="wo hid" id="wo"><div class="wz">
  <div class="tc mb16"><img src="data:image/png;base64,` + logoBase64 + `" style="width:52px;height:52px"></div>
  <h2>欢迎使用 SUSHIRO Overdose</h2>
  <p class="su">首次使用需要完成简单设置</p>
  <div class="ds"><div class="wd a" id="w0"></div><div class="wd" id="w1"></div><div class="wd" id="w2"></div></div>
  <div class="sc" id="wc"></div><div class="sa" id="wa"></div>
</div></div>

<script>
let cp='da',es={status:'idle'},hc=0,as=[],sd='',pr={},pf='';
const W=['日','一','二','三','四','五','六'];
function go(n,e){document.querySelectorAll('.wrap>div[id^="p-"]').forEach(p=>p.classList.add('hid'));document.getElementById('p-'+n).classList.remove('hid');document.querySelectorAll('.nav-in a').forEach(a=>a.classList.remove('on'));if(e)e.classList.add('on');cp=n;({ca:lC,re:lR,se:lS,lo:lL})[n]?.();}
async function init(){try{const r=await(await fetch('/api/status')).json();document.getElementById('ver').textContent='v'+r.version;hc=r.has_config;pf=r.platform||'';es=r.engine||{status:'idle'};uE();uD();if(!hc)sW();}catch(e){document.getElementById('ver').textContent='offline';}lP();sse();}
function uD(){if(hc){document.getElementById('nc').classList.add('hid');const b=document.getElementById('bm');b.textContent='开始抢号';b.className='bt bt-r bt-l';b.onclick=sB;}else{document.getElementById('nc').classList.remove('hid');const b=document.getElementById('bm');b.textContent='开始捕获参数';b.className='bt bt-y bt-l';b.onclick=sC;}}
function uE(){const b=document.getElementById('eb'),bs=document.getElementById('bs'),bm=document.getElementById('bm'),cb=document.getElementById('cb'),s=es;b.className='bn '+s.status;const m={idle:['d-g','就绪 — 等待操作'],capturing:['d-y','正在捕获认证参数...'],booking:['d-gr',s.message||'正在抢号...'],sniping:['d-gr','狙击中...'],success:['d-gr',s.message||'预约成功！'],error:['d-r',s.message||'出错了']};const[d,l]=m[s.status]||['d-g',s.status];b.innerHTML='<span class="d '+d+'"></span><span>'+l+'</span>';if(s.status==='booking'&&s.attempts)b.innerHTML+='<span class="tail">第'+s.attempts+'次</span>';const run=s.status==='capturing'||s.status==='booking'||s.status==='sniping';bs.classList.toggle('hid',!run);bm.disabled=run;if(s.status==='capturing'&&s.capture){cb.classList.remove('hid');rG(s.capture);}else cb.classList.add('hid');}
function rG(c){const i=[['X-App-Code',c.x_app_code],['查询认证',c.query_auth],['预约认证',c.reservation_auth],['UA',c.user_agent],['Referer',c.referer],['微信ID',c.wechat_id],['手机号',c.phone_number]];document.getElementById('cg').innerHTML=i.map(([n,o])=>'<div class="ci'+(o?' ok':'')+'">'+(o?'✅':'⏳')+' '+n+'</div>').join('');}
async function sC(){try{const d=await(await fetch('/api/engine/capture',{method:'POST'})).json();if(d.error)alert(d.error);}catch(e){alert('失败');}}
async function sB(){try{const d=await(await fetch('/api/engine/booking',{method:'POST'})).json();if(d.error)alert(d.error);}catch(e){alert('失败');}}
async function sE(){try{await fetch('/api/engine/stop',{method:'POST'});}catch(e){}}
function mA(){hc?sB():sC();}

async function lC(){const s=document.getElementById('ss');if(!s.options.length){try{const st=await(await fetch('/api/stores')).json();s.innerHTML='';if(!st.length){s.innerHTML='<option>暂无</option>';return;}st.forEach(x=>{const o=document.createElement('option');o.value=x.id;o.textContent=x.nickname||x.name;s.appendChild(o);});}catch(e){return;}}rC();}
async function rC(){const id=document.getElementById('ss').value;if(!id)return;document.getElementById('sc').innerHTML='<p class="mu tc">加载中...</p>';try{const d=await(await fetch('/api/calendar?store='+id)).json();if(d.error){document.getElementById('sc').innerHTML='<p class="mu tc">'+d.error+'</p>';return;}as=d.slots||[];rDB();}catch(e){document.getElementById('sc').innerHTML='<p class="mu tc">加载失败</p>';}}
function oSC(){sd='';rC();}
function fD(d){return parseInt(d.substring(4,6))+'/'+parseInt(d.substring(6,8));}
function fT(t){return t&&t.length>=4?t.substring(0,2)+':'+t.substring(2,4):t;}
function rDB(){const g={};as.forEach(s=>{if(!g[s.date])g[s.date]=[];g[s.date].push(s);});const ds=Object.keys(g).sort(),b=document.getElementById('dbar');b.innerHTML='';if(!ds.length){document.getElementById('sc').innerHTML='<p class="mu tc">无可用时段</p>';return;}ds.forEach(d=>{const sl=g[d],av=sl.filter(s=>s.availability==='AVAILABLE').length,dt=new Date(d.substring(0,4)+'-'+d.substring(4,6)+'-'+d.substring(6,8)),c=document.createElement('div');c.className='dc'+(d===sd?' on':'');c.innerHTML='<div class="dw">周'+W[dt.getDay()]+'</div><div class="dd">'+fD(d)+'</div><div class="dv '+(av>0?'h':'n')+'">'+(av>0?'✓'+av:'✗')+'</div>';c.onclick=()=>{sd=d;rDB();rS(d);};b.appendChild(c);});if(!sd||!ds.includes(sd)){sd=ds[0];rDB();}rS(sd);}
function rS(d){const sl=as.filter(s=>s.date===d).sort((a,b)=>(a.start||'').localeCompare(b.start||'')),c=document.getElementById('sc');if(!sl.length){c.innerHTML='<p class="mu tc">无时段</p>';return;}const ac=sl.filter(s=>s.availability==='AVAILABLE').length;c.innerHTML='<div class="sg">'+sl.map(s=>{const a=s.availability==='AVAILABLE';return'<div class="sl '+(a?'av':'fu')+'"><div class="tm">'+fT(s.start)+'-'+fT(s.end)+'</div><div class="ss">'+(a?'✓可预约':'✗已满')+'</div></div>';}).join('')+'</div><p class="mu mt8" style="font-size:11px">'+sl.length+'个时段 · '+ac+'个可预约</p>';}

async function lR(){try{const d=await(await fetch('/api/reservations')).json(),c=document.getElementById('rc');if(d.error){c.innerHTML='<p class="mu tc">'+d.error+'</p>';return;}if(!d.length){c.innerHTML='<p class="mu tc">暂无预约</p>';return;}c.innerHTML='<div class="sg">'+d.map(r=>'<div class="sl av"><div class="tm">'+(r.number||'-')+'</div><div class="ss">'+(r.status||'-')+'</div><div style="font-size:10px;color:var(--mute);margin-top:2px">#'+(r.ticketId||'')+'</div></div>').join('')+'</div>';}catch(e){document.getElementById('rc').innerHTML='<p class="mu tc">失败</p>';}}

async function lP(){try{pr=await(await fetch('/api/preferences')).json();fF(pr);dP(pr);}catch(e){}}
function fF(p){document.getElementById('pa').value=p.adult||2;document.getElementById('pc').value=p.child||0;document.getElementById('pt').value=p.table_type||'T';rT('wd',p.weekday_slots||[]);rT('sa',p.saturday_slots||[]);rT('su',p.sunday_slots||[]);}
function dP(p){const f=s=>!s||!s.length?'<span class="mu">不预约</span>':s.map(r=>fT(r.start+'00')+'–'+fT(r.end+'00')).join('、');document.getElementById('ps').innerHTML='<b>'+(p.adult||2)+'</b> 成人 · <b>'+(p.child||0)+'</b> 儿童 · 桌型 <b>'+(p.table_type||'T')+'</b><br>工作日 '+f(p.weekday_slots)+'<br>周六 '+f(p.saturday_slots)+'<br>周日 '+f(p.sunday_slots);}
function rT(k,rs){const c=document.getElementById(k);c.innerHTML='';(rs||[]).forEach((r,i)=>{const d=document.createElement('div');d.className='tr';d.innerHTML='<input type=text value="'+(r.start||'')+'" placeholder="1930"><span class="sp">—</span><input type=text value="'+(r.end||'')+'" placeholder="2030"><span class="x" onclick="this.parentElement.remove()">×</span>';c.appendChild(d);});if(!rs||!rs.length)c.innerHTML='<span class="mu" style="font-size:12px">不预约</span>';}
function aT(k){const c=document.getElementById(k);if(c.querySelector('.mu'))c.innerHTML='';const d=document.createElement('div');d.className='tr';d.innerHTML='<input type=text placeholder="1930"><span class="sp">—</span><input type=text placeholder="2030"><span class="x" onclick="this.parentElement.remove()">×</span>';c.appendChild(d);}
function gT(k){const ip=document.querySelectorAll('#'+k+' input'),r=[];for(let i=0;i<ip.length;i+=2){const s=ip[i].value.trim(),e=ip[i+1]?ip[i+1].value.trim():'';if(s||e)r.push({start:s,end:e});}return r;}
async function sP(){const b={adult:+document.getElementById('pa').value||2,child:+document.getElementById('pc').value||0,table_type:document.getElementById('pt').value||'T',selected_stores:pr.selected_stores||[],weekday_slots:gT('wd'),saturday_slots:gT('sa'),sunday_slots:gT('su')};try{const d=await(await fetch('/api/preferences',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)})).json();if(d.error){alert(d.error);return;}pr=d.preferences||b;dP(pr);alert('已保存');}catch(e){alert('失败');}}
async function lS(){await lP();try{const c=await(await fetch('/api/config')).json();document.getElementById('nf').value=c.feishu?.webhook||'';document.getElementById('ntt').value=c.telegram?.token||'';document.getElementById('ntc').value=c.telegram?.chat_id||'';document.getElementById('nbu').value=c.bark?.url||'';document.getElementById('nbk').value=c.bark?.key||'';document.getElementById('ns').value=c.server_chan?.key||'';}catch(e){}}
async function sN(){const b={feishu:{webhook:document.getElementById('nf').value.trim()},telegram:{token:document.getElementById('ntt').value.trim(),chat_id:document.getElementById('ntc').value.trim()},bark:{url:document.getElementById('nbu').value.trim(),key:document.getElementById('nbk').value.trim()},server_chan:{key:document.getElementById('ns').value.trim()}};try{const d=await(await fetch('/api/config',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)})).json();if(d.error){alert(d.error);return;}alert('已保存');}catch(e){alert('失败');}}

async function lL(){try{const ls=await(await fetch('/api/engine/logs')).json(),v=document.getElementById('lv');v.innerHTML=(ls||[]).map(l=>'<div class="ll'+(l.level==='error'?' er':'')+'"><span class="lt">'+l.time+'</span><span class="lm">'+esc(l.message)+'</span></div>').join('');v.scrollTop=v.scrollHeight;}catch(e){}}
function aL(e){const v=document.getElementById('lv');if(!v)return;const d=document.createElement('div');d.className='ll'+(e.level==='error'?' er':'');d.innerHTML='<span class="lt">'+e.time+'</span><span class="lm">'+esc(e.message)+'</span>';v.appendChild(d);if(cp==='lo')v.scrollTop=v.scrollHeight;}
function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML;}

let ws=0;
const wz=[
  ()=>{const ci=pf==='windows'?'<li>程序会自动安装证书到 Windows</li><li>弹出安全提示请点击「是」</li>':pf==='darwin'?'<li>程序会安装证书到钥匙串</li><li>可能需要输入登录密码</li>':'<li>程序会安装证书到系统</li>';document.getElementById('wc').innerHTML='<p style="margin-bottom:14px">本工具通过拦截微信小程序请求来获取认证参数：</p><div class="gb"><ol>'+ci+'<li>确保 PC 版微信已登录</li><li>准备好要预约的门店</li></ol></div><p class="mu mt8" style="font-size:12px">约 1–2 分钟完成</p>';document.getElementById('wa').innerHTML='<div></div><button class="bt bt-r" onclick="wN()">开始 →</button>';},
  ()=>{document.getElementById('wc').innerHTML='<div id="ws"></div><div class="gb"><ol><li>点击「开始捕获」</li><li>在 PC 微信打开寿司郎小程序</li><li>做一次排队或预约操作</li><li>自动捕获所需参数</li></ol></div><div id="wg" class="cg mt16"></div>';document.getElementById('wa').innerHTML='<button class="bt bt-w" onclick="wP()">← 返回</button><button class="bt bt-y" id="wb" onclick="wC()">开始捕获</button>';},
  ()=>{document.getElementById('wc').innerHTML='<p style="margin-bottom:14px">设置偏好（之后可在设置页修改）：</p><div class="fr mb16"><div class="fg"><label>成人</label><input type=number id="wa-a" min=0 max=10 value="'+(pr.adult||2)+'"></div><div class="fg"><label>儿童</label><input type=number id="wa-c" min=0 max=10 value="'+(pr.child||0)+'"></div><div class="fg"><label>桌型</label><select id="wa-t"><option value=T>桌位</option><option value=C>吧台</option></select></div></div><p class="mu" style="font-size:12px">时段可在设置页自定义</p>';document.getElementById('wa').innerHTML='<button class="bt bt-w" onclick="wP()">← 返回</button><button class="bt bt-r" onclick="wF()">完成 ✓</button>';}
];
function sW(){ws=0;document.getElementById('wo').classList.remove('hid');rW();}
function rW(){for(let i=0;i<3;i++)document.getElementById('w'+i).className='wd'+(i===ws?' a':i<ws?' d':'');wz[ws]();}
function wN(){ws<2&&(ws++,rW());}function wP(){ws>0&&(ws--,rW());}
async function wC(){document.getElementById('wb').disabled=true;document.getElementById('wb').textContent='捕获中...';try{const d=await(await fetch('/api/engine/capture',{method:'POST'})).json();if(d.error){alert(d.error);document.getElementById('wb').disabled=false;return;}document.getElementById('ws').innerHTML='<p style="color:var(--yellow)">⏳ 等待中，请操作微信...</p>';const iv=setInterval(async()=>{try{const s=await(await fetch('/api/engine/state')).json();if(s.capture){const g=document.getElementById('wg');if(g){const i=[['X-App-Code',s.capture.x_app_code],['查询认证',s.capture.query_auth],['预约认证',s.capture.reservation_auth],['UA',s.capture.user_agent],['Referer',s.capture.referer],['微信ID',s.capture.wechat_id],['手机号',s.capture.phone_number]];g.innerHTML=i.map(([n,o])=>'<div class="ci'+(o?' ok':'')+'">'+(o?'✅':'⏳')+' '+n+'</div>').join('');}}if(s.status!=='capturing'){clearInterval(iv);if(s.capture?.complete){document.getElementById('ws').innerHTML='<p class="tg" style="font-weight:700">✅ 捕获完成！</p>';hc=1;setTimeout(()=>{ws=2;rW();},600);}else if(s.status==='error'){document.getElementById('ws').innerHTML='<p class="tre">❌ '+(s.message||'失败')+'</p>';document.getElementById('wb').disabled=false;document.getElementById('wb').textContent='重试';}else{hc=1;setTimeout(()=>{ws=2;rW();},400);}}}catch(e){}},1000);}catch(e){alert('失败');}}
async function wF(){const b={adult:+document.getElementById('wa-a').value||2,child:+document.getElementById('wa-c').value||0,table_type:document.getElementById('wa-t').value||'T',selected_stores:pr.selected_stores||[],weekday_slots:pr.weekday_slots||[{start:'1930',end:'2030'}],saturday_slots:pr.saturday_slots||[{start:'1030',end:'1300'},{start:'1930',end:'2030'}],sunday_slots:pr.sunday_slots||[{start:'1030',end:'1300'},{start:'1930',end:'2030'}]};try{await fetch('/api/preferences',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)});}catch(e){}document.getElementById('wo').classList.add('hid');pr=b;dP(pr);uD();}

let cE;
function sse(){cE?.close();const s=new EventSource('/api/events');cE=s;s.addEventListener('engine',e=>{try{es=JSON.parse(e.data);uE();if(es.status==='idle'||es.status==='success'){hc=1;uD();}}catch(x){}});s.addEventListener('log',e=>{try{aL(JSON.parse(e.data));}catch(x){}});s.addEventListener('calendar',e=>{try{if(cp==='ca'){as=JSON.parse(e.data).slots||[];rDB();}}catch(x){}});s.addEventListener('ping',()=>{});s.onerror=()=>{s.close();cE=null;setTimeout(sse,3000);};}
init();
</script>
</body></html>
`
