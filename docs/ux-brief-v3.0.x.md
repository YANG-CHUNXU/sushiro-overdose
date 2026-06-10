# sushiro-overdose UX/代码问题全集（v3.0.3 / master @ d15d3ee）

> 本文档面向接手改进工作的 LLM/工程师，自包含、可直接执行。
> 所有行号已逐条对照 `internal/app/web_static.go`（共 1416 行）当前内容核实过；改动后行号会漂移，请以元素 id / 函数名为锚点。

## 项目背景

单二进制 Go Web 服务器，HTML/CSS/JS 全部以字符串常量嵌入 `internal/app/web_static.go`（1416 行，单一 `indexHTML` 常量），默认从 `localhost:8081` 提供服务（`web.go:28`）。用途：寿司郎中国排队管理与预约自动化桌面工具。没有外部框架，纯 Vanilla JS + SSE 实时更新。

**主要文件**
- `internal/app/web_static.go` — 全部前端代码（HTML/CSS/JS 字符串常量，无其他内容）
- `internal/app/web.go` 及 `web_*.go` — API handler 注册与实现
- `internal/core/` — 配置/token/偏好/门店/采样等核心逻辑
- `internal/notify/` — 飞书/Telegram/Bark/Server酱通知

**当前导航结构（5 个顶部 Tab，`web_static.go:477-481`）**
```
首页(home/da) | 现在去吃(eat/qt) | 我有号码(number/qd) | 约未来(book: ca 可约日历 / sn 自动抢) | 设置(settings/se)
```
另有 `我的单据` 页（`p-re`，711-713）和对应的 `mine` 导航组（852），但**没有顶部 nav 入口**，见问题 #1。子导航只在组内多于 1 页时显示（`renderSubnav`，856），目前只有「约未来」有。

---

## 已经做对的部分（不要改，不要退化）

- `alert()` / `confirm()` 全部替换为 `toast()`（824）+ `confirmDialog()`（825），样式完善
- `confirmDialog` 有危险语义自动检测（825，关键词 `/危险|不可恢复|卸载|清理本地|删除/`）
- 设置页用 `<details class="setting-fold">` 折叠，危险操作在嵌套 `<details class="btn-more danger">` 内（782）
- 首页 `setupCard` 4 项（通行证/常用门店/通知/预测）全 OK 时自动隐藏（896），不烦老用户
- 全国门店选择器 `openStorePicker()`（1257-1261）支持搜索、单/多选、回调
- authPill + healthPanel 状态面板（860-879）
- CSRF token 通过 `window.fetch` 拦截自动注入（807-821），403 且报文含 CSRF 时自动刷新页面（816-819）
- 页面专属吉祥物（`.pm`，`fillPageMascots` 1069）、传送带动画（`buildBelt` 1070-1077，CSS 465 尊重 `prefers-reduced-motion`）
- 每个会执行操作的入口（远程取号 1362、直接预约 1337、蹲号启动 1354、取号计划 1310、Routine 1309、取消排队号 1312、取消预约 1361）都已有 confirmDialog 二次确认
- 「每日提醒」「取号计划」的状态都有完整的动态渲染（`renderNetTicketRoutineStatus` 1277-1294、`renderNetTicketStatus` 1295-1307），按 status 分支给文案和操作按钮

---

## 问题清单

### 一、导航与信息架构

**#1 `我的单据` 没有稳定导航入口** `web_static.go:477-481` / `847-854`

`NAV_GROUPS`（847-854）定义了 `mine` 组（852，含 `re` 页），但 HTML nav 的 5 个 `<a>`（477-481）里没有它。现有入口全部是条件性、低可发现性的：

- 首页侧栏「当前偏好（人数 / 桌型 / 时段）」折叠内的「我的单据」按钮（548）——藏在一个标题完全不相关的折叠里
- ticket-hero 卡片的「查看单据」（951、954）——仅当有活跃票据时存在
- 引擎 success 态的主按钮「查看我的单据」（981）——仅成功后短暂可见
- 设置页状态总览的「看我的单据」（1029）——仅通行证就绪且未过期时显示；stale 时该按钮变成「重新认证」

结果：没有活跃票据、或通行证恰好过期的用户想查历史预约，没有任何可发现的路径。

目标：给 `mine` 一个稳定入口。可选方案：加入顶部 nav（第 5 个 Tab）；或并入「约未来」子导航（可约日历 / 自动抢 / 我的单据）；或把设置页通行证卡的「看我的单据」改为无条件常驻（re 页本身有完整的未认证兜底文案，711-712）。

---

**#2 `history.replaceState` 导致浏览器后退键失效** `web_static.go:858`

```js
if(location.hash.slice(1)!==n)history.replaceState(null,'','#'+n)  // 每次导航都覆盖，没有历史栈
```

全文件没有任何 `popstate` 监听（唯一的 window 级监听是 resize，1079）。用户按浏览器后退会直接离开 app。

目标：改为 `pushState` 并加 `window.popstate` 监听接管后退（popstate 回调里调 `go()` 时注意不要再次 pushState，避免死循环）。

---

**#3 任务卡标签与导航 Tab 不一致** `web_static.go:527`

首页任务卡写「**约未来 / 自动抢**」（527），点击落到可约日历（`go('ca')`，525）；顶部 Tab 叫「**约未来**」。子导航里再区分「可约日历 | 自动抢」。新用户看任务卡会以为「约未来」和「自动抢」是两个不同入口。

目标：任务卡标题统一为「约未来」，副文案说明点进去能同时访问日历和自动抢（卡片 p 文案 528 已经在解释这点，只差标题统一）。

---

### 二、首页视觉层级

**#4 双英雄区冲突** HTML `web_static.go:494-510`，逻辑 `936-941`

`renderActiveHome`（936-941）的隐藏条件（940）：

```js
hero.classList.toggle('hid', show && (es.status==='idle'||es.status==='success'))
// show = hc && activeTickets.length > 0
```

当有活跃票据 + 引擎在 capturing/booking/sniping/error 时，ticket-hero 卡片和 heroBox 两个大区块同时可见，堆叠在首页顶部，没有层级关系。

目标：引擎运行状态应作为 ticket-hero 卡片的附属信息（或只保留侧栏 `#eb` 引擎卡），而不是和 ticket-hero 平级的完整英雄区。

---

**#5 英雄区初始文案闪烁，且加载失败时永久卡住** `web_static.go:498-499` / `859`

页面加载时 `h1#heroTitle` 初始内容是「正在读取状态」，`p#heroCopy` 是「请稍等。」——`uD()`（958）在 `loadStatus()` 返回后才覆盖。每次打开 app 必看这行 copy 闪烁。更糟：`loadStatus` 失败时 catch 只把 ver 改成 offline（859），不调 `uD()`，英雄区永久停在「正在读取状态」，无重试入口。

目标：英雄区初始渲染用骨架屏（`.skeleton` 样式已存在，388-391）；`loadStatus` 失败时给明确的错误态 + 重试按钮。

---

**#6 版本号 badge 初始显示 "loading"** `web_static.go:484`

```html
<span class="ver" id="ver">loading</span>
```

黑色 pill 写着 "loading"，直到 `loadStatus()`（859）更新为 `v3.0.3` 或 `offline`。视觉突兀。

目标：初始值改为空并加 `hid`，有版本号后再显示。

---

**#7 `homeLive` 空态静默置空** `web_static.go:906-919`

两处直接置空：未选门店时（910）和实时接口全部失败时（914）。`homeWatchStores`（904）的兜底链是 localStorage → 偏好门店 → 凭证门店，全空的新用户首页这块什么都没有，也没有引导去选店。

目标：空态渲染轻量引导（如「选一家常去的门店，首页直接看排队」+ `openGuestStorePicker()` 按钮），失败态给重试；而不是 `innerHTML=''`。

---

**#8 `homeLive` 静默截断** `web_static.go:909`

```js
const ids = homeWatchStores().slice(0, 3);
```

用户选了 5 家门店，首页只显示 3 个，没有任何提示说还有门店没显示。

目标：超出时在区块末尾加「+N 家，去现在去吃看全部」溢出入口，或文案说明只展示前 3 家。

---

### 三、「我有号码」页（p-qd）密度

**#9 页面纵向密度过高** `web_static.go:585-638`

6 个区块全部默认展开（除 `qdEvidence` 外）：

| 区块 | 位置 | 样式 | 默认状态 |
|---|---|---|---|
| `qdAnswer` | 585 | `.answer-card` | 展开 |
| `qdAdvisor` | 586 | `.advisor-panel` | 展开 |
| `qdSamplingCard` | 587 | `.curve-sampling` | 展开（JS 重渲染后含状态 chips + 开关，1195）|
| `qdReminderCard` | 588-620 | `.curve-sampling` | 展开（含 2 Tab + 表单）|
| 时间换算卡 | 621-634 | `.curve-sampling` | 展开（含表单）|
| `qdEvidence` | 635-638 | `<details>` | 有曲线数据时自动展开（1203）|

三个 `.curve-sampling` 样式的卡片并排，视觉节奏单调，找不到重心。用户输入号码想看答案，要先划过采集卡。

目标：把采集卡（`qdSamplingCard`）和时间换算卡默认折叠（可复用 `qdEvidence` 的 `<details class="card adv">` 模式），让「输入号码→立即看答案」的主路径无需滚动。

---

**#10 `qdAnswer` 和 `qdAdvisor` 功能区分不清** `web_static.go:585-586`

两者 intro copy 几乎相同：
- `qdAnswer`（585）：「先选门店、输入你手里的号；**这里直接告诉你大概几点叫到、几点出发**」
- `qdAdvisor`（586）：「先选门店，再输入你手里的号；**这里会显示大概几点叫到和几点前到店**」

用户无法分辨两者区别。实际上 `qdAnswer` = 实时排队压力 + 即时 chips（`renderQueueAnswer`，1204），`qdAdvisor` = 历史曲线里程碑 + 到店建议（`renderDashboardAdvisor`，1244）。

目标：标题差异化（如 `qdAnswer` →「现在的排队压力」，`qdAdvisor` →「历史规律 · 到店建议」），或把两者合并为单一答案区。

---

**#11 每日提醒 Tab 的静态占位文案过时（小问题，已基本被动态渲染覆盖）** `web_static.go:616` / `1276-1294`

静态 HTML（616）写死「需要先配置通知渠道；开启后只是提醒你手动取号。」。实际上动态渲染已完整存在：`lQD` 会调 `loadNetTicketRoutine`（1276）→ `renderNetTicketRoutineStatus`（1277-1294）按 enabled/needs_notify/waiting_data 等 status 重写这块；`saveNetTicketRoutine`（1309）也会在未配置通知时阻止启用并跳设置。残留问题只有两个：(a) 接口返回前的短暂闪现；(b) `/api/queue/ticket/routine` 失败时 catch 静默（1276 行尾 `catch(e){}`），误导性静态文案永久残留。

目标：静态占位改为中性的「状态加载中…」，fetch 失败时渲染错误 + 重试（可复用 `loadErrBoxHTML`，841）。

---

### 四、设置页

**#12 通知渠道 6 个按钮平铺** `web_static.go:746`

```html
保存通知 / 测试全部 / 飞书 / Telegram / Bark / Server酱
```

4 个分渠道测试按钮属于调试级操作，和主操作「保存通知」平级显示，按钮行视觉噪声大。

目标：4 个分渠道测试按钮放进 `<details class="btn-more">`（模式已存在，314-322），一级只保留「保存通知」和「测试全部」。

---

**#13 历史洞察表格显示 `store_id` 裸数字** `web_static.go:1167`

```js
'<td data-label="门店">'+esc(storeName(r.store_id))+'<br><span class="mu">'+esc(r.store_id)+'</span></td>'
```

每行门店列同时显示门店名和 store_id（纯数字，用户无意义）。排障目的可以理解，但表格里显示过于喧闹。

目标：移除裸 ID，或仅在 `.debug-only` 模式（107 已有该机制）下展示。

---

**#14 设置页折叠顺序与使用频率不匹配** `web_static.go:721-786`

当前顺序：通行证（721）→ GitHub 登录（730）→ 通知渠道（739）→ 预测准确度（749）→ 历史洞察（767）→ 运行日志（774）→ 安全与维护（778）。

「通知渠道」的使用需求（叫号提醒）高于 GitHub 登录（可选增强），但排在第 3 位。

目标：调整为 通行证 → 通知渠道 → GitHub → 预测 → 历史洞察 → 运行日志 → 安全与维护。纯 HTML 块顺序调换，无逻辑改动。

---

### 五、「自动抢」页（p-sn）

**#15 `snPrefs` 折叠展开后内容过长** `web_static.go:677-704`

折叠内容包含：4 个快捷预设 + 人数/儿童/桌型/手机号 + 全国门店搜索 + 门店优先级列表 + 日期优先级/时段策略 + 工作日/周六/周日 3 套时段 + 保存按钮。展开后等于完整的偏好配置中心，summary 文字「抢号偏好：人数 / 门店优先级 / 时段」无法预告内容量。

目标：把「门店 + 人数基础配置」和「时段策略」拆为两个独立折叠，减轻单次展开的认知负担。注意 `openSnPrefs()`（1020）和 675 行的内联展开都直接引用 `snPrefs` id，拆分后要同步。

---

**#16 「现在就抢」和「蹲号」缺乏视觉分隔** `web_static.go:673-707`

`qbox` 现在就抢（673-676）→ `snPrefs` 折叠（677-704）→ 蹲号标题行（705）+ `snRows`/`snPlan`（706-707）在同一滚动区域内，没有 divider 或小节标题区分两种操作模式（已放出的立即抢 vs 未放出的蹲）。

目标：在「蹲还没放出的时段」（705）前加视觉分隔线或小节标题，明确两个模式的边界。

---

### 六、「现在去吃」页（p-qt）

**#17 「高级」折叠内 4 个操作按钮混排** `web_static.go:659-662`

```
启用 / 取消计划 / 恢复当前排队号 / 取消排队号
```

4 个按钮在同一 flex 容器里语义完全不同：启用/取消计划是规划操作，「取消排队号」是危险的即时操作（会取消小程序里的真实排队号），靠在一起容易误点。注意：`cancelNetTicket()`（1312）**已有**危险二次确认（「危险操作：取消当前排队号？…取消后不可恢复」），所以这是纯视觉分组问题，不缺保护。

目标：「取消排队号」从操作组单独分行/分区展示，并加 `.tag action` 警示标记。

---

**#18 `qtLive` 加载/失败态用 chip 样式** `web_static.go:650` / `1271`

初始 HTML（650）和 JS 加载态（1271）都用 `<div class="ci">实时排队待加载</div>`，失败态用 `.ci bad`。`.ci` 是状态 chip 样式，作为整块区域的 placeholder 语义不对，视觉上像一个孤立的小标签。

目标：加载态改为 `.skeleton` 骨架或 `.empty` 样式，失败态用 `loadErrBoxHTML`（841，自带重试按钮）。

---

### 七、交互细节

**#19 overlay 系列缺少 Escape 键处理** `web_static.go:825` 等

`confirmDialog`（825）创建后 `el('cfYes').focus()`，但没有 keydown Escape 监听，键盘用户只能 Tab 到取消再 Enter。其余 overlay 更弱：`openStorePicker`（1257）、`openHealthPanel`（869）、`openFirstUseWizard`（1084）、`openAuthWizard`（1105）既无 Escape 也无遮罩点击关闭（只有 confirmDialog 支持点遮罩关闭）。

目标：统一给 overlay 加 Escape 关闭（confirmDialog 走 `done(false)`，弹窗关闭时移除监听）；其余 overlay 至少补遮罩点击关闭。

---

**#20 `tN()` 静默先保存，toast 告知不够前置** `web_static.go:1392-1393`

```js
async function tN(ch){
  if(!await sN(true)) return;  // 静默保存整个通知表单
  // ...发送测试
  toast('已先保存当前表单，测试通知已发送')
}
```

行为本身合理（测试前要保存），但用户点的是「飞书」「Telegram」这类渠道名按钮，不会预期点一下会触发保存。toast 在事后才说。

目标：按钮 label 或就近文案改为「保存并测试」语义（与 #12 的折叠整理一起做）。

---

**#21 `go('sm')` 死链：3 处按钮静默跳回首页** `web_static.go:1167`（×2）/ `1293` / `858`

`sm`（旧的独立采样页）已不在 `NAV_GROUPS`（847-854）里，`go()`（858）对未知页静默回退到 `da`（首页）。受影响按钮：

- 历史洞察空态的两个「去预测准确度」按钮（1167，`go('sm', …)`）
- 每日提醒 Routine 状态卡的「提升预测准确度」按钮（1293，`go(&quot;sm&quot;)`）

用户点击后毫无解释地回到首页。正确目标页是设置页的 `fold-sm` 折叠，现成函数 `openSettingsFold('fold-sm')`（1021）在别处已大量使用。

附带的同源死代码：`cp==='sm'` 判断（1333、1412）和 `cp==='lo'` 判断（1411）永远为假——后者导致 SSE 实时日志在运行日志折叠里永不自动滚动（cp 此时是 `se`）。

目标：3 处 `go('sm')` 换成 `openSettingsFold('fold-sm')`；1411 的判断改为按 `fold-lo` 是否展开；清理 1333/1412 的死判断。
