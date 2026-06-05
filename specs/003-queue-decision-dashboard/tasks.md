# Tasks: Queue Decision Dashboard

## Phase 1: Contracts

- [x] T001 Add feature spec for 2.15 queue decision dashboard.
- [x] T002 Define read-only API contract for `target_no`.
- [x] T003 Add unit tests for advisor target mapping.

## Phase 2: Decision Advisor

- [x] T004 Extend `QueueDashboardQuery` with `TargetNo`.
- [x] T005 Add `QueueDashboardAdvisor` response object.
- [x] T006 Build milestone rows for no-target mode.
- [x] T007 Build target estimate and arrival suggestion for target-number mode.
- [x] T008 Return clear empty/passed/uncovered states.

## Phase 3: Web UI

- [x] T009 Add target number input to Data Dashboard controls.
- [x] T010 Render plain-language advisor cards before the chart.
- [x] T011 Keep chart/table tooltips at selected bucket granularity from 10:00 to 22:00.
- [x] T012 Make the panel usable on mobile.

## Phase 4: Store Comparison And Calendar Modeling

- [x] T013 Keep date type filters aligned with `queueTrendDateType`.
- [x] T014 Add store comparison copy that answers which selected store is easier right now.
- [x] T015 Keep holidays/workdays separated from normal weekday/weekend baselines.

## Phase 5: Verification

- [x] T016 Run `go test ./...`.
- [x] T017 Run `go build ./...`.
- [x] T018 Run `go vet ./...`.
- [x] T019 Run `git diff --check`.
- [x] T020 Verify no read-only path calls official mutation APIs.

## Phase 6: Release

- [x] T021 Update user-facing release notes for `2.15.0`.
- [ ] T022 Tag and publish only after CI passes.
