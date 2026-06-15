# Release notes: v3.4.4

v3.4.4 是多智能体并行审查后的一轮修复：6 个维度（前端 JS、后端并发、Web API、安全、CSS、数据一致性）各自隔离审查，共发现 48 条问题，逐条复核后修复了其中的真实缺陷，包括用户报告的「选门店后 p is not defined」。

## 关键修复

### 🔴 「选门店后 p is not defined」（用户报告的崩溃）

- **根因**：`renderPressureChart` 的排序回调 `.sort((a,b)=>hhmmMinute(p.time)-...)` 误用了上面 `.filter(p=>...)` 的形参 `p`（已离开作用域），应为 `a`。排队面板(qd)选门店/填号、排队趋势(qt)选门店后，只要压力曲线有数据点就抛 `ReferenceError: p is not defined`，导致整张图渲染中断。
- 改为 `hhmmMinute(a.time)`。
- 已用真实门店数据验证 qd / qt 两条路径，修复后零异常。

### 🔴 定时取号 5xx 退避失效（上一版引入的回归）

- v3.4.2 加的 `ServerRetryCount` 重试上限，因为「跨天清零」的判断只看计数非零、没判断日期，导致每个 tick 都清零、6 次上限永远不生效——官方服务持续 5xx 时仍会每 20 秒重发取号请求。
- 新增 `retry_date` 字段记录计数所属日期，仅**真正跨天**才清零；同一天内计数正确累积到上限后转 `error` 并通知。

### 🔴 其它高优修复

- **切换「本机持续采集」开关误报失败**：`toggleDashboardSampling` 末尾调用了不存在的 `uSamplingSummary()`，在 try 块内抛错被 catch，于是采集其实已成功却弹「采集开关失败」。删除该无效调用。
- **取号规划区间倒序**：`buildQueueMealPlan` 的推荐取号区间可能 `Early>Late`（前端显示成 `12:30-12:00`）。校正为早→晚。

### 并发 / 数据一致性

- **sniper plan 读-改-写竞态**：`UpdateSniperPlanTarget` / `StopRemainingSniperPlanTargetsAfterSuccess` 现在用 `sniperPlanMu` 串行化，避免引擎 goroutine（每 50ms 写）与 UI 保存互相覆盖、把「已成功」状态抹掉。`SaveSniperPlan` 改用原子写，避免并发读读到半截 JSON。
- **全国门店缓存持锁拉取**：`CachedAllStores` 不再全程持锁做 15s 的 HTTP 拉取，改为「锁内查缓存→锁外拉取→锁内双检写回」，消除门店选择器/基准采集被一次慢请求串行卡住。
- **dashboard 慢请求覆盖新数据**：`loadQueueDashboard` 加了递增 token 守卫（与 `loadQueueAdvisorCard` 一致），快速切换门店时先发的慢请求不会再用旧门店数据覆盖 `qdDashboardData`。

### Web API

- `handleReservations` / `handleCalendar` 补 GET 方法校验：之前 POST（带 CSRF 即过中间件）会触发原本只读的列表接口执行写副作用（SaveState / noteAuthHealthy / appendHistory / bus.publish）。
- 删掉 `redirectCloudAuthResult` 里恒被覆盖的死代码 `target := "/#se"`。

### 安全加固

- MITM 代理日志的 query 脱敏白名单扩充：除 token/auth/phone/wechat 外，新增 `code`/`openid`/`unionid`/`session`/`secret`/`ticket`/`sign`/`key`/`sid`，避免微信/通用接口的敏感参数明文落日志。

### CSS / 响应式

- 修正上一版「胶囊统一 38px」引入的错位：`.dash-target` 内嵌 input 恢复内陷样式（不再顶满容器、聚焦态正常）；`.switch/.check/.chip` 与 40px 输入对齐，`.preset` 保留 42px。
- 补 input/select/textarea 的 disabled 样式；`.dash-tip` tooltip 提到 z-index:30，不再被 sticky header 遮挡。
- `--mute` 灰色加深到 `#76716B`，次要文字对比度达到 WCAG AA（原 `#9B9691` 仅 2.9:1）。
- 新增 600-768px 中等宽度断点：多列网格（首页决策卡、机制图、狙击行）降为 2 列，不再拥挤；移动端主图不再固定 680px 宽顶破窄屏，改为自适应缩放 + 横向滚动兜底。

### 防御性

- `fillSamplingForm` 对 `el()` 返回值判空，元素缺失不再抛 `Cannot set properties of null`。
- `netTimeDisp` 对超长/含非数字的 HHMM 输入先规整再切片，避免畸形值显示成错误时间。

## 审查中识别但未改的（设计固有风险 / 低收益）

- 手机抓包服务绑 0.0.0.0、OAuth token 走 URL 回传、通知密钥 GET 回显、CSRF 全局静态 token——属本地单用户 MITM 工具的设计权衡，token 随机性 + TTL + 127.0.0.1 绑定已构成合理防护。
- `netTicketTick` 持锁做取号 HTTP + 通知发送的延迟问题（非正确性 bug）未重构，避免回归风险，留作后续。
- 暗色模式、SSE 心跳等体验增强项未纳入本次。

## 验证

- `gofmt` / `go build ./...` / `go vet ./...` / `go test ./...` 全绿。
- CDP 真机验证：qd 选门店 + 填号触发 `renderPressureChart` 全路径、qt 选门店触发 `refreshQueueView` 全链路，修复后均零 JS 异常。
- 移动端 390px 压测（叫号 10888 / 长店名 / 180 分钟等待）仍无横向溢出。
