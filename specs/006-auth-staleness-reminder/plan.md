# Implementation Plan: Auth Staleness Reminder

> 本轮只出方案，待批准后再实现。

## Architecture Impact

| Module | Change | Reason |
|---|---|---|
| `internal/app/`（新增 `auth_health.go` 或并入 `auth_probe.go`） | 内存级 `authHealth` 状态 + `markAuthStale(reason)` / `markAuthHealthy()` / `getAuthHealth()` | 统一记录认证健康，进程内存、不落盘 |
| `internal/app/web_engine.go` | `handleStatus` 增加 `auth_health` 字段 | 前端可读认证健康 |
| 读取/操作请求路径（`web_engine.go` 等：`handleReservations`、取号/取消、必要时 calendar/queue） | 命中 `isAuthError`/401/403 时调用 `markAuthStale`；成功时可 `markAuthHealthy` | 被动检测，不新增探活 |
| 认证成功处（捕获保存 / `auth/import` / `mobile-auth` 完成） | 调 `markAuthHealthy()` | 重新捕获后清除 stale |
| `internal/app/web_static.go` | 首页常驻横幅（读 `auth_health`）+ 点击进入 `startAuth()` | 用户可见提醒 |
| `internal/app/main.go`（或就近） | ok→stale 跃迁时 `sendNotification` 一次（去重） | 离屏也能收到提醒 |

无新增后台 goroutine/定时器；不改 `api` 包；不动 mutation 边界。

## 关键设计点

1. **只被动、不主动**：复用既有请求结果判定，绝不新增定时探活（避免加剧会话互斥）。
2. **状态机**：`unknown(初始/未认证) → ok（有过成功请求/刚捕获）→ stale（命中认证失败）→ ok（重新捕获成功）`。`has_config=false` 时固定 `unknown`，不报 stale，避免对只读用户误报。
3. **通知去重**：维护 `lastNotifiedStaleAt`；只在 `ok/unknown → stale` 跃迁时推一次，stale 周期内不再推；`markAuthHealthy` 重置去重标志。
4. **线程安全**：`authHealth` 用 `sync.RWMutex` 保护（多 handler 并发读写）。

## API Contract

| Endpoint/Function | Method | Read/Write | 变更 |
|---|---|---|---|
| `/api/status` | GET | Read | 响应新增 `auth_health: {status, reason, checked_at}` |
| `markAuthStale(reason string)` / `markAuthHealthy()` / `getAuthHealth()` | 函数 | 内存 | 新增，纯内存状态，无副作用、无官方调用 |

## State Contract

| File/State | Writer | Reader | Cleanup | Notes |
|---|---|---|---|---|
| `authHealth`（内存） | 各请求路径 / 捕获成功 | `handleStatus`、通知判定 | 进程退出即失，不落盘 | 不持久化，避免重启误报 |

## Safety Analysis

- 取消/重复预约风险：无；不触碰任何 mutation。
- 会话互斥：本方案**不主动发任何官方请求**，不会额外顶掉手机会话——这是核心约束。
- 误报：`has_config=false` 不报 stale；只读功能不触发检测。
- 通知噪音：跃迁去重，stale 周期内最多一条。
- 架构守卫：不新增 `CreateReservation/CreateNetTicket/Cancel*` 调用；边界不变。

## Test Plan

| Test | Type | What It Proves |
|---|---|---|
| authHealth 状态机单测 | unit | ok→stale→ok 跃迁、has_config=false 不报、通知只跃迁推一次 |
| `handleStatus` 含 auth_health | unit/契约 | 字段存在且随状态变化 |
| 前端守卫 `Embedded*` | 回归 | 新横幅 onclick/id 有定义、引用完整 |
| `go build/vet/test ./...` | 回归 | 架构守卫、既有测试保持绿 |
| 手动 | manual | 触发一次认证失败→看横幅+收到一次通知→重新捕获→横幅消失 |

## Rollout

- Version：`2.19.0`。
- Migration：无（纯内存状态）。
- 手动验证：令牌失效后访问「我的单据」→ 顶部出现横幅 + 通知一条 → 重新捕获 → 横幅与提醒清除。
- Rollback：移除横幅渲染与 `auth_health` 字段即可；其余请求行为不变，向后兼容。

## 待你确认的取舍（实现前）

1. 横幅展示范围：**全局顶部一处常驻**（推荐）vs 每页就近提示。
2. 通知文案是否点明"手机用过小程序后电脑认证会失效"这层因果（推荐点明，校正预期）。
3. 是否同时在顶栏放一个小的认证状态徽标（之前第二梯队提过的"全局状态 pill"），把认证健康一起显示——可与本功能合并做，也可留到以后。
