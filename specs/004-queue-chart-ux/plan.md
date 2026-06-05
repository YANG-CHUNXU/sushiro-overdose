# Implementation Plan: Queue Chart Decision Markers & "Eat Now" Information Architecture

## Architecture Impact

| Module | Change | Reason |
|---|---|---|
| `internal/app/web_static.go` | Front-end SVG rendering + CSS + DOM reorder + nav groups | Visualize advisor decisions and reorder eat-now by frequency |
| `specs/004-queue-chart-ux/` | Track phase scope and acceptance | Keep 2.17 work reviewable |
| `docs/release-notes-2.17.0.md` | User-facing release notes | Communicate the UX upgrade |

No backend (`queue_dashboard.go`), state, or API contract changes. The advisor already returns `target_no` / `target_bucket` / `target_label` / `arrival_label`.

## API Contract

No new or changed endpoints. The existing `/api/queue/dashboard` (GET, read-only) response already contains the `advisor` object consumed by the chart.

## State Contract

| File/State | Writer | Reader | Expiry/Cleanup | Notes |
|---|---|---|---|---|
| none | — | — | — | Pure front-end render of existing read-only response + client clock |

## State Transitions

| From | Event | To | Side Effects |
|---|---|---|---|
| Curve without markers | Dashboard refresh / target input | Curve with band + now + my-number markers | None (render only) |
| Eat-now config-first layout | Page render | Live-first layout, config under advanced | None (DOM order only) |

## Safety Analysis

- Cancellation risk: none; charts and reorder only read existing data and move DOM.
- Duplicate booking/ticket risk: none; no booking or ticket mutation touched. All eat-now ids/onclick handlers preserved verbatim, so `saveNetTicketPlan` / `saveQueueBaseline` / `addQueueAlert` keep their confirm-gated behavior.
- Phone-app state drift: none; no official reservation state inferred or written.
- Official API stale/error behavior: unchanged; existing fallbacks remain.
- Sampling/dashboard interference: none; rendering does not start/stop collectors.
- Architecture guard: no read-only entrypoint gains a mutation; `architecture_guard_test` boundaries untouched.

## Test Plan

| Test | Type | What It Proves |
|---|---|---|
| `go build ./...` | build | Front-end string template still compiles into the binary |
| `go vet ./...` | static | No vet regressions |
| `go test ./...` | regression | `web_security_test` and `queue_dashboard_test` stay green |
| `git diff --check` | lint | No whitespace/conflict markers |
| Manual dashboard | manual | Band, now line, my-number line, highlighted intersection match advisor text |
| Manual mobile | manual | Charts fit ≤600px with no horizontal scroll |
| Manual eat-now | manual | Live-first order; config folded; 数据收集 under settings |

## Rollout

- Version: `2.17.0`.
- Migration: none.
- Manual verification: launch local Web UI on 8081, open 我有号码, select store 3006, enter a target number, verify markers; narrow the window to ≤600px; open 现在去吃 and 设置 → 数据收集.
- Rollback: revert the `web_static.go` rendering/CSS/DOM changes; response payloads remain backward compatible.
