# Feature Spec: Queue Decision Dashboard

## Metadata

| Field | Value |
|---|---|
| Feature | `queue-decision-dashboard` |
| Version Target | `2.15.0` |
| Owner | `sushiro-overdose` |
| Date | `2026-06-05` |
| Affected Surfaces | `web / queue dashboard / public baseline / local observations` |

## Problem

The queue dashboard can already draw queue and called-number data, but users do not make decisions from raw metrics. The most important user question is: "If I have or want a number, roughly when will the store call it, and when should I arrive?"

## Goal

Turn local and public queue baseline data into direct, user-readable decisions: what number is likely called at each time, when a target number is likely called, when to arrive, and which data source/confidence supports the advice.

## User Stories

| ID | Story | Priority |
|---|---|---|
| US-1 | As a user, I can enter my queue number and see roughly what time it will be called. | P0 |
| US-2 | As a user, I can see a plain-language arrival suggestion instead of interpreting P50/P80 or raw samples. | P0 |
| US-3 | As a user, I can scan a 10-minute table from 10:00 to 22:00 showing what number is usually called at each time. | P0 |
| US-4 | As a user, I can compare selected stores and understand which store is easier to visit right now. | P1 |
| US-5 | As a user, weekends, holidays, and compensatory workdays use separate baselines so special days do not pollute normal days. | P1 |

## Scope

### In Scope

- Add a read-only queue decision advisor to `/api/queue/dashboard`.
- Support `target_no` query input for "my number".
- Return direct copy such as "预计 18:40 左右叫到 893 号" and "建议 18:20 前到店".
- Keep called-number curve/table fixed to 10:00-22:00 and bucketed by selected 5/10/15/30 minutes.
- Reuse existing local observation and Turso public baseline fallback.
- Keep the date type model aligned with existing `queueTrendDateType`, including Friday 16:30 through Sunday 22:00 as weekend.

### Out Of Scope

- Automatically taking queue tickets.
- Automatically cancelling reservation or queue tickets.
- Writing to official Sushiro state.
- Replacing the booking calendar or sniper flow.
- Guaranteeing arrival time; advice is a data-driven estimate.

## Behavior

| State/Input | Expected Behavior | User Copy |
|---|---|---|
| Target number exists in curve | Return the earliest bucket where typical called number reaches target | `预计 18:40 左右叫到 893 号` |
| Target number already passed by latest called number | Mark as already passed/current | `当前已经叫到 910 号，893 号可能已经过号` |
| Target number is beyond current curve | Return no exact bucket and explain sample range | `今天 10:00-22:00 的样本还没覆盖到这个号` |
| No target number | Return readable milestones for now, +60m, +120m, and evening/closing when available | `19:00 一般叫到 760 号` |
| No called curve data | Return an empty advisor with next action | `先开启信息收集，或选择有线上基准的门店` |
| Read-only dashboard refresh | Must not call booking, ticket, or cancellation APIs | N/A |

## Data Rules

| Data | Source | Retention | Can Write Local State | Can Call Official Mutation |
|---|---|---|---|---|
| Called curve | `queue_observations.jsonl`, local baseline, Turso public baseline | existing retention | No | No |
| Target number | Web query only | request lifetime | No | No |
| Advisor output | Derived response | request lifetime | No | No |
| Holiday/workday model | `~/.sushiro/holidays.json` + weekend window rule | existing retention | No | No |

## Acceptance Criteria

- [x] `/api/queue/dashboard?target_no=893` returns an `advisor` object.
- [x] Advisor returns `target_bucket`, `target_label`, and `arrival_label` when the target number is covered.
- [x] Advisor states "already passed" when latest called number is greater than or equal to target number.
- [x] Advisor returns milestone rows even when no target number is entered.
- [x] Web dashboard has a target number input and renders plain-language cards.
- [x] 10:00-22:00 and selected bucket granularity remain enforced.
- [x] Date type options stay aligned with existing weekend/holiday model.
- [x] No read-only entrypoint calls official mutation APIs.
- [x] Reservation and queue-ticket cancellation paths remain separate.

## Open Decisions

- Store comparison in `2.15.0` uses a combined current-effort score: open/online-ticket status first, then lower wait minutes and lower waiting groups.
- Whether target-number input should also be reused by queue alerts.
