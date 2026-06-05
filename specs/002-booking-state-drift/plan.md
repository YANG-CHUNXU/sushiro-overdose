# Implementation Plan: Booking State Drift Error Handling

## Architecture Impact

| Module | Change | Reason |
|---|---|---|
| `internal/api/api.go` | Add `ErrActiveReservationExists` and text classifier | Convert official conflict into typed error |
| `internal/app/booking_errors.go` | Add user-friendly message for active-reservation conflict | Avoid generic failure copy |
| `internal/app/engine.go` | Stop normal booking on active-reservation conflict | Prevent infinite retry loop |
| `internal/app/engine_sniper.go` | Stop sniper and mark target error on active-reservation conflict | Prevent misleading running state |
| `internal/app/booking_strategy_test.go` | Add parser and create-reservation tests | Lock behavior |

## API Contract

| Endpoint/Function | Method | Read/Write | Request | Response | Errors |
|---|---|---|---|---|---|
| `CreateReservation` | POST | Official mutation | store/date/time/people/auth | reservation record | `ErrNoReservationAvailable`, `ErrActiveReservationExists`, HTTP/auth errors |
| `GetReservations` | POST | Official read | auth | reservation list | 404 means unavailable, not empty |

## State Contract

| File/State | Writer | Reader | Expiry/Cleanup | Notes |
|---|---|---|---|---|
| Engine state | booking/sniper engines | Web UI/SSE | run lifecycle | active-reservation conflict sets `error` |
| Sniper plan | sniper engine | Web UI | plan refresh | target gets `error` and `last_error` |
| `.sushiro_state.json` | success/fallback only | reservations UI | stale cleanup | conflict does not write fake state |

## State Transitions

| From | Event | To | Side Effects |
|---|---|---|---|
| `booking` | `ErrActiveReservationExists` | `error` | log clear message, stop run |
| `sniping/running` | `ErrActiveReservationExists` | `error` | mark target error, stop run |
| `booking` | `ErrNoReservationAvailable` | `booking` | mark slot tried/full |
| `booking` | HTTP 500 / E010 | `booking` | temporary skip current slot |

## Safety Analysis

- Cancellation risk: no cancellation endpoint is called; user must cancel manually in official mini-program or explicit cancel UI.
- Duplicate booking risk: active-reservation conflict stops retrying instead of hammering.
- Phone-app state drift: conflict message explains official state may lag after manual cancellation.
- Official API stale/error behavior: 404 current-list does not imply empty; 409 E052 is authoritative conflict.
- Sampling/dashboard interference: no changes.

## Test Plan

| Test | Type | What It Proves |
|---|---|---|
| `TestIsActiveReservationText` | unit | E052 and translated messages are recognized |
| `TestReservationBusinessError` | unit | business error maps to typed error |
| `TestCreateReservationMapsActiveReservationError` | unit | HTTP 409 E052 maps to `ErrActiveReservationExists` |
| `go test ./...` | CI | behavior and architecture guards pass |

## Rollout

- Version: `2.12.2`
- Migration: none
- Manual verification: trigger safe diagnostics only; do not call cancellation APIs.
- Rollback: restore generic error handling if official response changes, then add new classifier tests.
