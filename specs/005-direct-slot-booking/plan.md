# Implementation Plan: Direct Slot Booking

## Architecture Impact

| Module | Change | Reason |
|---|---|---|
| `internal/app/engine.go` | `BookingEngine` 加 `pinnedSlot *TargetSlot` 字段；`StartBookingSlot()` 入口；`StartBooking` 按 pinned 选门店；`runBooking` 内加一次性 pinned 分支；`finishRun` 清 pinned | 在已授权入口内实现"直接预约确切时段"，不扩大 mutation 边界 |
| `internal/app/web_engine.go` | `handleEngineBooking` 解析可选 `{store,date,start,end}`，有则走 `StartBookingSlot` | 让前端能直接预约某个时段 |
| `internal/app/web_static.go` | 日历可约时段→「预约这个时段」(`bookSlotDirect`)，已满/不可约→「加入狙击」；新增 `bookSlotDirect` JS | 区分"直接预约"与"狙击" |
| `specs/005-direct-slot-booking/` | 规格与验收 | 预约行为改动留痕可审 |
| `docs/release-notes-2.18.0.md` | 用户可见说明 | 沟通行为变化 |

`CreateReservation` 的调用点不新增、不外移：pinned 逻辑写在 `runBooking` 方法体内（一次性分支），因此 `architecture_guard_test` 白名单**不需要改动**，官方 mutation 边界不变。

## API Contract

| Endpoint/Function | Method | Read/Write | Request | Response | Errors |
|---|---|---|---|---|---|
| `/api/engine/booking` | POST | Write（创建预约） | 可选 JSON `{store,date,start,end}`；空 body = 原自动抢 | `{ok,message}` | 409 引擎运行中；参数不全回退自动抢或报错 |
| `BookingEngine.StartBookingSlot` | 方法 | Write | `storeID,date,start,end` | `error` | 同 `StartBooking`（认证/参数/运行中） |

`start`/`end` 规范成 HHMMSS（4 位补 `00`），与 `CreateReservation`/`TargetSlot` 一致。

## State Contract

| File/State | Writer | Reader | Expiry/Cleanup | Notes |
|---|---|---|---|---|
| `e.pinnedSlot`（内存） | `StartBookingSlot` 设置 | `StartBooking`/`runBooking` 读 | `finishRun` 清空 | 仅单次运行有效，不持久化 |
| 本地预约记录 | 既有 `onBookingSuccess` | 「我的单据」 | 既有留存 | 与自动抢成功一致 |

## State Transitions

| From | Event | To | Side Effects |
|---|---|---|---|
| idle | POST booking 带时段 | booking(pinned) | 设 pinnedSlot，校验认证 |
| booking(pinned) | CreateReservation 成功 | success | 保存预约、通知、清 pinned |
| booking(pinned) | 时段不可约/业务错误 | error | 提示并停止、清 pinned（不循环） |
| booking(pinned) | 认证失效 | error | 既有认证失效处理 |
| idle | POST booking 空 body | booking(自动) | 维持原行为 |

## Safety Analysis

- 取消风险：无；不触碰任何取消接口。
- 重复预约：直接预约成功即停；已有进行中预约时走既有 `ErrActiveReservationExists` 提示并停止，不会叠单。
- 手机端状态漂移：与自动抢一致，以小程序最终记录为准；不臆测官方状态。
- 官方 API 失败：复用 `runBooking` 既有错误映射（认证/5xx/已满）。
- 采样/后台干扰：复用 `pauseSamplingForMainFlow`，与自动抢同路径。
- 架构守卫：`CreateReservation` 仍只在 `runBooking` 等四个入口；pinned 分支在 `runBooking` 内，守卫不变、保持通过。

## Test Plan

| Test | Type | What It Proves |
|---|---|---|
| `TestOfficialMutationCallsStayInApprovedEntrypoints` | 回归 | pinned 分支没把 `CreateReservation` 移出授权入口 |
| `Embedded*`（前端守卫） | 回归 | 新 `bookSlotDirect` onclick 有定义、id 引用完整 |
| `go build/vet/test` | 构建/回归 | 后端改动不破坏既有 |
| 手动：可约时段直接预约 | manual | 成功落「我的单据」，文案一致 |
| 手动：满/不可约时段 | manual | 显示「加入狙击」 |

## Rollout

- Version：`2.18.0`。
- Migration：无。
- 手动验证：本地 Web，预约未来选可约时段→「预约这个时段」→确认→看「我的单据」；满时段→「加入狙击」。
- Rollback：`handleEngineBooking` 忽略时段参数、日历恢复「加入狙击」即可；空 body 行为始终向后兼容。
