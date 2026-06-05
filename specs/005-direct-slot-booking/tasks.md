# Tasks: Direct Slot Booking

## Phase 0: Contracts

- [x] T001 写 2.18 直接预约 spec。
- [x] T002 确认 `CreateReservation` 调用边界与守卫白名单，确定 pinned 逻辑放 `runBooking` 内不动守卫。

## Phase 1: Booking Engine

- [x] T003 `BookingEngine` 加 `pinnedSlot *TargetSlot` 字段。
- [x] T004 加 `StartBookingSlot(storeID,date,start,end)` 入口（规范 HHMMSS，设 pinned 后走 StartBooking 校验链）。
- [x] T005 `StartBooking` 按 pinned 选门店（pinned 时用该门店，否则维持 prefs）。
- [x] T006 `runBooking` 加一次性 pinned 分支：成功→success；不可约/业务错误→error 停止；认证失效→既有处理；复用成功落库/通知。
- [x] T007 `finishRun` 清空 `pinnedSlot`。

## Phase 2: Web Handler

- [x] T008 `handleEngineBooking` 解析可选 `{store,date,start,end}`，齐全则 `StartBookingSlot`，否则 `StartBooking`。

## Phase 3: Web UI

- [x] T009 日历可约时段渲染「预约这个时段」→ `bookSlotDirect`；已满/不可约渲染「加入狙击」。
- [x] T010 新增 `bookSlotDirect`：会执行操作确认 → POST 时段 → 成功 toast + 引导「我的单据」。
- [x] T011 顺手校对首页第 4 步与相关文案，避免再把可约说成"狙击"。

## Phase 4: Verification

- [x] T012 `go build ./...`。
- [x] T013 `go vet ./...`。
- [x] T014 `go test ./...`（含架构守卫 + 前端守卫保持绿）。
- [x] T015 `git diff --check`。
- [ ] T016 手动验证：可约直接预约成功落单；满时段显示加入狙击；空 body 仍自动抢。

## Phase 5: Release

- [x] T017 写 2.18.0 release notes。
- [ ] T018 CI 通过后再 tag/发布。
