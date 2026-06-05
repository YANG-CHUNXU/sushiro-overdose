# Feature Spec: Booking State Drift Error Handling

## Metadata

| Field | Value |
|---|---|
| Feature | `booking-state-drift` |
| Version Target | `2.12.2` |
| Owner | `sushiro-overdose` |
| Date | `2026-06-05` |
| Affected Surfaces | `web booking / sniper / api error parsing / engine state` |

## Problem

Users may cancel a reservation manually in the phone mini-program, then immediately retry booking in this tool. The official current-reservations endpoint may be unavailable or stale, while `createReservation` can still return a business error saying an active reservation exists. If the tool treats that as a generic failure, the user cannot tell whether the system is rate-limited, stale, full, or still blocked by official state.

## Goal

When the official API says the account already has a reservation, stop the current booking/sniper run and show a clear state-drift message instead of looping with generic failures.

## User Stories

| ID | Story | Priority |
|---|---|---|
| US-1 | As a user, I can understand that the official system still thinks I have an active reservation after phone-side cancellation. | P0 |
| US-2 | As a user, I am not misled into thinking the tool is locally fused or permanently blocked. | P0 |
| US-3 | As a maintainer, I can distinguish full slots, active-reservation conflicts, temporary server errors, and auth failures in tests. | P0 |

## Scope

### In Scope

- Classify official `E052` / active-reservation business errors.
- Stop normal booking and Web sniper runs on active-reservation conflicts.
- Keep `E044` / full-slot behavior as continue-to-next-target.
- Keep `E010` / server error behavior as temporary retry/skip.
- Add tests for business-error parsing and engine/sniper error handling branches.

### Out Of Scope

- Calling cancellation APIs automatically.
- Polling phone mini-program state beyond existing official endpoints.
- Changing public queue dashboard behavior.
- Changing net-ticket cancellation semantics.

## Behavior

| State/Input | Expected Behavior | User Copy |
|---|---|---|
| `createReservation` returns `E044` or full message | Mark slot unavailable and continue trying other targets | 名额已满，继续尝试 |
| `createReservation` returns `E052` or already-reserved message | Stop current booking/sniper run with error | 官方仍认为当前账号已有预约；如果你刚在手机上取消，请等小程序状态同步后再抢，或重新打开寿司郎小程序确认“我的预约”已清空 |
| `createReservation` returns `E010` or HTTP 500 | Temporarily skip current slot and retry later | 官方接口临时异常，稍后重试 |
| Auth API returns auth failure | Stop and ask user to recapture auth | 预约认证参数已失效 |
| Current reservations endpoint returns 404 | Do not infer user has no reservation | 当前预约列表不可用，不代表小程序没有预约 |

## Data Rules

| Data | Source | Retention | Can Write Local State | Can Call Official Mutation |
|---|---|---|---|---|
| Active reservation conflict | `createReservation` response | engine runtime log/state | No | No |
| Current reservation fallback | local state file | until stale/cleared | Yes, local only | No |
| Booking success | `createReservation` response | local state/history | Yes | Already performed by user action |

## Acceptance Criteria

- [x] `E052` and equivalent messages map to `ErrActiveReservationExists`.
- [x] Normal booking stops with a clear error on `ErrActiveReservationExists`.
- [x] Web sniper stops and marks the target `error` on `ErrActiveReservationExists`.
- [x] `E044` still maps to `ErrNoReservationAvailable`.
- [x] No cancellation API is called automatically.
- [x] Reservation and queue-ticket cancellation paths remain separate.

## Open Decisions

- Whether to add a dedicated Web UI banner suggesting “wait 1-2 minutes after phone cancellation” before retrying.
