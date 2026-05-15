# sushiro-overdose 架构分层

项目仍保持单 `package main` 和零外部依赖，方便单二进制发布；代码按文件职责分层。

## 层次

- **入口与 CLI**：`main.go`、`calendar.go`、`booking.go`、`sniper.go`
- **应用编排**：`engine.go`、`engine_sniper.go`、`health.go`
- **领域与策略**：`slot.go`、`preferences.go`、`sniper_plan.go`、`insights.go`
- **外部服务**：`api.go`、`proxy.go`、`cert.go`
- **平台适配**：`platform.go`、`platform_darwin.go`、`platform_windows.go`、`platform_linux.go`
- **Web 层**：`web.go`、`web_handlers.go`、`web_calendar.go`、`web_engine.go`、`web_preferences.go`、`web_sniper.go`、`web_events.go`、`web_static.go`
- **通知与维护**：`notifier*.go`、`diagnostics.go`、`maintenance.go`、`watchdog.go`
- **数据辅助**：`history.go`、`recommend.go`、`state.go`、`store.go`

## 约定

- 新 Web API 优先按职责放到 `web_*.go`，`web_handlers.go` 只保留通用响应工具和首页。
- 新后台生命周期逻辑放在 `engine_*.go`，避免继续扩大 `engine.go`。
- 可复用算法先做成纯函数并配测试，再从 Web/CLI 调用。
- 平台相关能力必须通过 `platform.go` 暴露，业务代码不直接调用平台命令。
- 用户数据路径统一通过现有 path helper，不在业务逻辑中拼硬编码路径。
