# Feature Spec: Queue Chart Decision Markers & "Eat Now" Information Architecture

## Metadata

| Field | Value |
|---|---|
| Feature | `queue-chart-ux` |
| Version Target | `2.17.0` |
| Owner | `sushiro-overdose` |
| Date | `2026-06-05` |
| Affected Surfaces | `web / queue dashboard chart / eat-now page / navigation` |

## Problem

The 2.15 queue decision dashboard already computes a text advisor ("预计 18:40 叫到 893 号", "18:20 前到店"), but the called-number curve does not visualize those conclusions: there is no "now" marker, no "my number" marker, and the slow/fast uncertainty is drawn as two hard-to-read dashed lines. On phones the chart is forced into horizontal scroll by a fixed `min-width`.

Separately, the "现在去吃" (eat-now) page inverts usage frequency: the high-frequency live queue readout sits **below** low-frequency configuration (ticket plan, nationwide baseline collection, call alerts). The low-frequency "本机收集" (local collection) page is also a sibling tab of "看排队" inside the eat-now group, instead of living under settings.

## Goal

Make the called-number curve directly carry the arrival decision (now line, my-number line, highlighted intersection, readable uncertainty band, mobile-friendly) and reorder the eat-now page by usage frequency. All changes are read-only front-end edits in `internal/app/web_static.go`; no mutation API, backend struct, or interface contract changes.

## User Stories

| ID | Story | Priority |
|---|---|---|
| US-1 | As a user, I can see a "now" vertical line on the called-number curve so I know where the current time falls. | P0 |
| US-2 | As a user, I can see my queue number as a horizontal line with the intersection bucket highlighted, matching the advisor text. | P0 |
| US-3 | As a user, I can read the slow~fast uncertainty as a single shaded band instead of two dashed lines. | P0 |
| US-4 | As a user on a phone, the called-number and trend charts fit the screen without horizontal scrolling. | P0 |
| US-5 | As a user opening "现在去吃", I see live queue data first, with ticket plan / baseline collection / alerts folded into an advanced section. | P0 |
| US-6 | As a user, I find "数据收集" under 设置 instead of mixed into the eat-now group. | P1 |

## Scope

### In Scope

- Read-only front-end rendering and CSS in `internal/app/web_static.go`.
- Pass the existing `advisor` object into the curve renderer to reuse `target_bucket`.
- Confidence band polygon, now line, my-number line, intersection highlight, legend updates.
- Mobile responsive CSS for both the dashboard chart and trend chart.
- Reorder eat-now DOM and move the collection page into the settings nav group.

### Out Of Scope

- Backend `queue_dashboard.go` data structures and API contracts (advisor fields already suffice).
- Any booking / ticket / cancellation mutation path; no change near `architecture_guard_test` boundaries.
- Trend-page annotation (now/my-number markers stay on the main dashboard chart only).
- Introducing a charting library; charts stay hand-written zero-dependency SVG.
- Changing the fixed 10:00–22:00 window or existing bucket granularity.
- Changing any element `id` or `onclick` handler during the eat-now reorder.

## Behavior

| State/Input | Expected Behavior | User Copy |
|---|---|---|
| Curve has slow & fast values | Draw one shaded band between fast and slow, typical line on top | `保守-偏快区间` legend |
| Local clock within 10:00–22:00 | Draw a red vertical "now" line with a top label | `现在 HH:MM` |
| Local clock outside window | No now line drawn | N/A |
| Target number entered and within curve max | Draw red dashed horizontal line at the number; highlight intersection bucket dot | reuses existing tooltip |
| Intersection bucket | Prefer `advisor.target_bucket`; fallback to first bucket where typical ≥ target | matches advisor card |
| Narrow viewport (≤600px) | Chart scales to fit, no horizontal scroll | N/A |
| Eat-now page load | Live queue readout appears first; config folded under advanced details | `高级 · 取号计划 / 基准采集 / 叫号提醒` |
| Open "数据收集" | Loads under settings group via existing `sm:lSm` dispatch | N/A |

## Data Rules

| Data | Source | Retention | Can Write Local State | Can Call Official Mutation |
|---|---|---|---|---|
| Called curve + advisor | `/api/queue/dashboard` response | request lifetime | No | No |
| Target number | `qdTargetNo` web input | request lifetime | No | No |
| Current time | client `new Date()` | render lifetime | No | No |

## Acceptance Criteria

- [x] `renderDashboardCalledCurve` receives the advisor object and uses `target_bucket` for the intersection.
- [x] Slow/fast dashed lines are replaced by a single shaded confidence band drawn beneath the typical line.
- [x] A red "now" vertical line with `现在 HH:MM` label is drawn only when the local clock is within 10:00–22:00.
- [x] A red dashed "my number" horizontal line is drawn when a valid target ≤ curve max is entered, with the intersection dot highlighted.
- [x] Legend shows the band, now, and my-number entries.
- [x] At ≤600px both the dashboard chart and trend chart fit without horizontal scrolling.
- [x] Eat-now page shows live queue first, with ticket plan / baseline collection / alerts inside an advanced `<details>`.
- [x] "数据收集" appears under the 设置 group and loads correctly; it no longer appears under 现在去吃.
- [x] No element `id` or `onclick` handler changed during the eat-now reorder; all queue handlers still resolve their elements.
- [x] No read-only entrypoint calls official mutation APIs.

## Open Decisions

- Trend chart annotations (now/my-number) are intentionally deferred to keep scope tight.
- X-axis label thinning on very narrow screens is optional and not required for acceptance.
