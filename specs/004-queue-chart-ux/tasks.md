# Tasks: Queue Chart Decision Markers & "Eat Now" Information Architecture

## Phase 0: Contracts

- [x] T001 Add feature spec for 2.17 queue chart UX.
- [x] T002 Confirm advisor fields (`target_bucket`/`target_label`/`arrival_label`) already satisfy the chart needs.

## Phase 1: Chart Decision Markers

- [x] T003 Pass `d.advisor` into `renderDashboardCalledCurve` (call site + signature).
- [x] T004 Replace slow/fast dashed polylines with a single shaded confidence band polygon drawn beneath the typical line.
- [x] T005 Draw a red "now" vertical line + `现在 HH:MM` label when local clock is within 10:00–22:00.
- [x] T006 Draw a red dashed "my number" horizontal line and highlight the intersection bucket dot (advisor.target_bucket, fallback typical ≥ target).
- [x] T007 Update the legend (band / now / my-number) and add swatch CSS classes.
- [x] T008 Add `@media(max-width:600px)` rules so the dashboard and trend charts fit without horizontal scroll.

## Phase 2: Eat-Now Information Architecture

- [x] T009 Reorder `eat` nav group to `[['qt','门店排队']]` and move `['sm','数据收集']` into `settings`.
- [x] T010 Move the live queue readout (`qtLive`) above the configuration blocks.
- [x] T011 Wrap ticket plan / baseline collection / call alerts in a new advanced `<details class="adv">`.
- [x] T012 Verify no element id or onclick handler changed during the reorder.

## Phase 3: Verification

- [x] T013 Run `go build ./...`.
- [x] T014 Run `go vet ./...`.
- [x] T015 Run `go test ./...`.
- [x] T016 Run `git diff --check`.
- [x] T017 Manually verify markers, mobile fit, and eat-now reorder.

## Phase 4: Release

- [x] T018 Update user-facing release notes for `2.17.0`.
- [ ] T019 Tag and publish only after CI passes.
