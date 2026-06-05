# Implementation Plan: Queue Decision Dashboard

## Architecture Impact

| Module | Change | Reason |
|---|---|---|
| `internal/app/queue_dashboard.go` | Add read-only advisor structs and builder | Convert called curve into user decisions |
| `internal/app/web_queue_trends.go` | Parse `target_no` query param | Support "my number" from Web UI |
| `internal/app/web_static.go` | Add target input and advisor rendering | Make the result visible and understandable |
| `internal/app/queue_dashboard_test.go` | Add advisor tests | Lock behavior for covered, passed, and uncovered target numbers |
| `specs/003-queue-decision-dashboard/` | Track phase scope and acceptance | Keep 2.15 work reviewable |

## API Contract

| Endpoint/Function | Method | Read/Write | Request | Response | Errors |
|---|---|---|---|---|---|
| `/api/queue/dashboard` | GET | Read | `store/stores`, `scope`, `date_type`, `bucket`, `window`, `target_no` | Existing dashboard response plus `advisor` | Empty advisor if data missing |
| `BuildQueueDashboardWithContext` | Function | Read | `QueueDashboardQuery{TargetNo}` | `QueueDashboardResponse{Advisor}` | No mutation side effects |

## State Contract

| File/State | Writer | Reader | Expiry/Cleanup | Notes |
|---|---|---|---|---|
| `~/.sushiro/queue_observations.jsonl` | existing collector | dashboard | existing cleanup rules | Read-only for advisor |
| `~/.sushiro/queue_stats.json` / remote baseline cache | existing baseline loader | dashboard | existing cache rules | Read-only fallback |
| `~/.sushiro/holidays.json` | user | dashboard/trends | permanent until edited | Read-only date type model |

## State Transitions

| From | Event | To | Side Effects |
|---|---|---|---|
| No target input | Dashboard refresh | Milestone advisor | None |
| Target covered by curve | Dashboard refresh | Target estimate advisor | None |
| Target already passed | Dashboard refresh | Passed warning advisor | None |
| No curve data | Dashboard refresh | Empty advisor | None |

## Safety Analysis

- Cancellation risk: none; the feature only reads existing local/public data.
- Duplicate booking/ticket risk: none; no booking or queue-ticket mutation is introduced.
- Phone-app state drift: no official reservation state is inferred from dashboard data.
- Official API stale/error behavior: public baseline errors already fall back to local data and warnings.
- Sampling/dashboard interference: dashboard does not start or stop collectors.

## Test Plan

| Test | Type | What It Proves |
|---|---|---|
| Target covered advisor | unit | A number maps to the first matching called-number bucket |
| Already passed advisor | unit | Latest called number takes precedence over historical curve |
| Uncovered target advisor | unit | User gets a clear no-coverage message |
| Milestone advisor | unit | No target still returns useful time-number rows |
| Existing dashboard tests | regression | Called curve, holiday exclusion, and remote fallback stay intact |

## Rollout

- Version: `2.15.0`.
- Migration: none.
- Manual verification: launch local Web UI, open Data Dashboard, select store 3006, input a target number, verify advisor cards and tooltip table.
- Rollback: remove advisor rendering and ignore `target_no`; existing dashboard response remains backward compatible.
