# Tasks: Booking State Drift Error Handling

## Phase 1: Contracts

- [x] T001 Define behavior for E052 / active reservation conflict.
- [x] T002 Define state transitions for normal booking and sniper.
- [x] T003 Define safety rule that no cancellation happens automatically.

## Phase 2: Implementation

- [x] T004 Add `ErrActiveReservationExists`.
- [x] T005 Add active-reservation text classifier.
- [x] T006 Stop normal booking on active-reservation conflict.
- [x] T007 Stop Web sniper on active-reservation conflict.
- [x] T008 Add user-facing friendly error copy.

## Phase 3: Verification

- [x] T009 Add classifier and API mapping tests.
- [x] T010 Run `go test ./...`.
- [x] T011 Run `go build ./...`.
- [x] T012 Run `go vet ./...`.
- [x] T013 Run `git diff --check`.

## Phase 4: Release

- [ ] T014 Decide whether to publish `v2.12.2`.
