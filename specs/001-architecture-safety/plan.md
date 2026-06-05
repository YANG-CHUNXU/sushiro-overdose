# Implementation Plan: Architecture Safety Baseline

## Architecture Impact

| Module | Change | Reason |
|---|---|---|
| `.specify/memory/constitution.md` | Add architecture constitution | Make LLM constraints explicit |
| `specs/_template/` | Add `spec.md`, `plan.md`, `tasks.md` templates | Standardize feature planning |
| `specs/001-architecture-safety/` | Add first feature spec | Demonstrate workflow |
| `internal/app/architecture_guard_test.go` | Add source-level guard test | Enforce cancellation boundary in CI |

## API Contract

| Endpoint/Function | Method | Read/Write | Request | Response | Errors |
|---|---|---|---|---|---|
| `handleCancelReservation` | POST | Official mutation | `{ticket_id, kind:"reservation"}` | `{ok:true}` | rejects non-reservation kind |
| `cmdCancel` | CLI | Official mutation | `ticket_id` argument | terminal output | official API error |
| `handleCancelNetTicket` | POST | Official mutation | current auth state | `{ok:true}` | official API error |

## State Contract

| File/State | Writer | Reader | Expiry/Cleanup | Notes |
|---|---|---|---|---|
| `.specify/memory/constitution.md` | maintainers | agents/reviewers | manual | Canonical rules |
| `specs/**` | feature owner | agents/reviewers | manual | Feature source of truth |

## State Transitions

| From | Event | To | Side Effects |
|---|---|---|---|
| No spec | Large feature begins | Draft spec | No runtime side effects |
| Draft spec | Plan accepted | Tasks ready | No runtime side effects |
| Code changed | CI runs | Pass/fail architecture guard | No runtime side effects |

## Safety Analysis

- Cancellation risk: addressed by a source-level guard that whitelists only explicit cancel entrypoints.
- Duplicate booking/ticket risk: future specs must classify official 409/500 states before implementation.
- Phone-app state drift: future specs must define behavior when official list/status endpoints are stale.
- Official API stale/error behavior: required in every spec behavior table.
- Sampling/dashboard interference: prohibited by constitution and future tests.

## Test Plan

| Test | Type | What It Proves |
|---|---|---|
| `TestOfficialMutationCallsStayInApprovedEntrypoints` | unit/source guard | Booking, queue-ticket, and cancellation mutation calls only happen in approved entrypoints |
| `go test ./...` | CI | guard is part of normal checks |

## Rollout

- Version: `2.12.x`
- Migration: none
- Manual verification: inspect docs and run tests
- Rollback: remove docs/test if the framework blocks legitimate changes, then update constitution first
