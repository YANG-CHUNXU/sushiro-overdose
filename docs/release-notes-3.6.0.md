# Release notes: v3.6.0

v3.6.0 是一次**代码可读性**专项：多智能体并行给 6 个业务域的重要位置补中文注释，零逻辑改动。

## 做了什么

用 6 个智能体（各自隔离 worktree、互不影响）并行审查 + 补注释，共 **225 条「做什么+为什么」级中文注释，覆盖 40 个文件**。原则：只加注释、不改逻辑/签名/变量名/测试，不确定的标注 TODO 不瞎编。

## 各域补的关键位置

### 定时取号 / 后台采样（38 条）
- `netticket_routine.go`：routine 状态机（waiting_data/needs_notify/notified/missed/armed 转换）、节流窗口、SafetyMinutes 遗留迁移
- `sampling.go`：后台 loop 调度、startWithConfig 双重检查锁、samplingInActiveWindow 跨午夜、samplingBlockedReason 四类冲突优先级、pauseSamplingForMainFlow 自杀保护
- `sampling_cli.go`：守护进程生命周期、PID 文件自愈、自杀保护
- `netticket.go`：补个别遗漏（FiredDate 跨天判定、终态自动失能等）

### 自动抢号 / 引擎（31 条）
- `engine.go`：EngineStatus 6 状态含义与互斥、runBooking 主循环（100ms 轮询、errStreak≥5 退避、authErrors≥3 终止依据）、setState/finishRun done 指针比对的并发坑
- `engine_sniper.go` / `sniper.go`：50ms 高频抢约、30 天放号规则、一目标成功即停其余（同账号只允许一个有效预约）
- `sniper_plan.go`：refreshSniperPlanTarget 9 条状态转换、sniperPlanMu 防 lost-update

### 排队预测 / 趋势（26 条）
- `queue_advisor.go`：estimateWaitRange 三档回退、officialWait 抬高低界的为什么、computeQueueEta 过号/封顶钳制分支、waitTimeCap 封顶
- `queue_trends.go`：queueTrendWeekendWindow（周五 16:30 算周末的时间窗口逻辑）、分位数、叫号推进异常过滤（diff≤500 防跨天重置误算）
- `queue_baseline*.go`：Rollup 字段语义、远程基准缓存键、Turso schema 迁移兼容（先带叫号列、不行回退）

### Web API / MITM 代理（30 条）
- `web.go`：webSecurityMiddleware（Host 白名单防 DNS rebinding + 写请求双重校验）、validWebCSRF 恒定时间比较、validWebOrigin 三道关、只绑 127.0.0.1 的网络层隔离
- `web_engine.go`：各 handler 写前 refreshWebClient、kind=reservation 安全保护、跨天失效判定
- `proxy.go`：allowedProxyTarget（只解密 sushiro、防开放代理）、sanitizedProxyURL 脱敏、MITM 流程
- `cert.go`：CA 生成/信任、有效期

### 共享内核 / 平台（最薄弱域，重点补）
- `internal/core/*`：Slot/StoreInfo/ReservationRecord/Config/SamplingConfig 结构体关键字段语义、各 API client 方法（URL + 用途）、SanitizeDiagnosticLine 脱敏白名单、atomicWriteFile 原子写约定、AppDirPath 路径约定、FirstAvailableLocalPort 端口探测、discovery 默认关 + 只记元数据
- `internal/api/api.go`：每个接口方法的端点和用途

### 云端认证 / 手机抓包 / 通知
- `cloud_auth.go`：OAuth state 一次性票据（TTL + 单次消费防 CSRF）、缓存键、配置加载优先级（env > 磁盘）
- `mobile_auth_capture.go` / `mobile_ua_capture.go`：0.0.0.0 绑定 + token 随机性 + TTL 自动过期
- `auth_health.go`：noteAuthResult 状态机（stale/healthy/unknown 判定）
- `auth_import.go`：凭证解析（JSON/cURL/原始头）
- `notify/*`：MultiNotifier 聚合 + 各渠道 API 端点

## 验证

- `gofmt` / `go build ./...` / `go vet ./...` / `go test ./...` 全绿。
- 确认零逻辑改动：所有删除行都是 gofmt 重新对齐（常量/字段加注释后列宽变化的重排），函数签名/控制流/import 未动。
- build/vet/test 在各 worktree 内也已验证。
