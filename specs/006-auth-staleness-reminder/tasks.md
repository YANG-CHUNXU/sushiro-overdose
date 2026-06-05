# Tasks: Auth Staleness Reminder

## Phase 1: 状态与检测（后端）

- [x] T001 新增 `auth_health.go`：内存状态机 `markAuthStale/markAuthHealthy/noteAuthResult/getAuthHealth`，ok→stale 跃迁去重通知。
- [x] T002 `handleStatus` 暴露 `auth_health`。
- [x] T003 被动检测挂入认证请求：`handleReservations`、`handleQueueTicket`、取消排队号/预约（失败→noteAuthResult，成功→markAuthHealthy）。
- [x] T004 重新捕获/导入认证成功 → markAuthHealthy（auth_import / mobile_auth_capture / engine capture）。

## Phase 2: Web UI

- [x] T005 顶栏认证状态徽标（只读模式 / 已认证 / 认证可能过期），点击进入获取认证或我的单据。
- [x] T006 stale 时首页常驻横幅 + 点明"手机用过小程序后电脑认证会失效"因果 + 重新获取认证按钮。
- [x] T007 我的单据加载失败后刷新状态，让横幅/徽标即时反映。

## Phase 3: 验证

- [x] T008 `auth_health` 状态机单测。
- [x] T009 `go build ./...` / `go vet ./...` / `go test ./...`（含架构守卫、前端守卫）。
- [x] T010 内嵌 JS `node --check`、`git diff --check`。
- [ ] T011 手动验证：令牌失效→横幅+通知一次→重新捕获→清除。

## 不做

- 不新增任何后台定时探活（避免加剧会话互斥）。
- 不解决 Issue 2（手机端报错的会话互斥根因，先天限制，已在 spec Context 记录）。
