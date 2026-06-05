# Tasks: Architecture Safety Baseline

## Phase 1: Contracts

- [x] T001 Add repository constitution.
- [x] T002 Add reusable feature templates.
- [x] T003 Add first architecture safety spec.

## Phase 2: Implementation

- [x] T004 Add CI-covered architecture guard test for cancellation API boundaries.
- [x] T005 Add future guards for sampling/state/dashboard boundaries.

## Phase 3: Verification

- [x] T006 Run `go test ./...`.
- [x] T007 Run `go build ./...`.
- [x] T008 Run `go vet ./...`.
- [x] T009 Verify existing dirty changes are not reverted.

## Phase 4: Release

- [ ] T010 Decide whether to include in `2.12.2` with the current reservation error-classification fix.
