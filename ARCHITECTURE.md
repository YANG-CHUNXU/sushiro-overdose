# sushiro-overdose 架构分层

项目零外部依赖（纯 Go 标准库），单二进制发布。代码按职责拆分到 `internal/` 下的若干包，根目录只保留 `main.go` 作为入口。

## 包结构

```
main.go                入口：注入版本号 (ldflags) 后调用 internal/app.Run()
internal/
  app/        应用编排 + CLI 命令 + Web 层（最上层，依赖其余所有包）
  core/       共享内核：配置、令牌、时段、偏好、门店、状态、抓包数据结构
  api/        寿司郎 API 客户端（时段查询、预约、排队取号、门店信息）
  proxy/      MITM 抓包代理 + CA 证书生成/信任
  platform/   系统/OS 适配（macOS / Linux / Windows 的代理设置、进程、证书信任）
  notify/     通知渠道（飞书 / Telegram / Bark / Server酱）
```

依赖方向自上而下：`app` → (`api`, `proxy`, `platform`, `notify`, `core`)；`core` 不依赖其他内部包，作为公共底座。`app` 通过点导入（dot import）聚合下层包的导出符号。

## 各包职责

- **app/**
  - 入口与 CLI：`main.go`（`Run()` 命令分发 + `printUsage`）、`calendar.go`、`booking.go`、`sniper.go`、`daemon.go`、`sampling_cli.go`
  - 后台编排：`engine.go`、`engine_sniper.go`、`health.go`、`watchdog.go`、`activity.go`
  - 领域与策略：`sniper_plan.go`、`insights.go`、`recommend.go`、`queue_trends.go`、`queue_alerts.go`、`booking_errors.go`
  - 排队取号：`netticket.go`（定时取号调度器 + 按日期独占锁）、`queue_live.go`、`queue_live_panel.go`
  - 采样与历史：`sampling.go`、`history.go`
  - Web 层：`web.go`（路由注册）+ `web_*.go`（按功能拆分的 handler：calendar/engine/sniper/preferences/sampling/queue_live/queue_trends/events/pac/static）、`web_handlers.go`（通用响应工具 + 首页）
  - 诊断与维护：`diagnostics.go`、`diag_bundle.go`、`auth_probe.go`、`maintenance.go`、`update_check.go`

- **core/**：`config.go`、`tokens.go`、`slot.go`、`preferences.go`、`store.go`、`state.go`、`runtime.go`、`capture.go`、`discovery.go`（接口发现调试）、`paths.go`、`ports.go`、`redact.go`（脱敏）、`util.go`

- **api/**：`api.go` —— 封装官方接口调用与认证头注入。

- **proxy/**：`proxy.go`（只对寿司郎 API 域名做 TLS 解密，其余 CONNECT 透传）、`cert.go`（CA 证书）。

- **platform/**：`platform.go`（统一接口）+ `platform_{darwin,linux,windows}.go`（平台实现）+ `processes.go`。

- **notify/**：`notifier.go`（`MultiNotifier` 聚合）+ 各渠道 `notifier_*.go`。

## 约定

- 新 Web API 优先按职责放到 `internal/app/web_*.go`；`web_handlers.go` 只保留通用响应工具和首页。
- 新后台生命周期逻辑放在 `engine_*.go`，避免继续扩大 `engine.go`。
- 可复用算法先在 `core/` 做成纯函数并配测试，再从 Web/CLI 调用。
- 平台相关能力必须通过 `platform/` 暴露，业务代码不直接调用平台命令。
- 用户数据路径统一通过 `core` 的 path helper（`paths.go`），不在业务逻辑中拼硬编码路径。
- `core` 保持无内部依赖；不要让它反向依赖 `app`/`api`/`proxy`。

> 更详细的架构说明与打包流程见 [AGENTS.md](AGENTS.md)。
