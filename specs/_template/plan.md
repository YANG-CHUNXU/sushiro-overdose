# Implementation Plan: <feature name>

## Architecture Impact

| Module | Change | Reason |
|---|---|---|
| `internal/app/...` | ... | ... |

## API Contract

| Endpoint/Function | Method | Read/Write | Request | Response | Errors |
|---|---|---|---|---|---|
| ... | GET | Read | ... | ... | ... |

## State Contract

| File/State | Writer | Reader | Expiry/Cleanup | Notes |
|---|---|---|---|---|
| `~/.sushiro/...` | ... | ... | ... | ... |

## State Transitions

| From | Event | To | Side Effects |
|---|---|---|---|
| ... | ... | ... | ... |

## Safety Analysis

- Cancellation risk:
- Duplicate booking/ticket risk:
- Phone-app state drift:
- Official API stale/error behavior:
- Sampling/dashboard interference:

## Test Plan

| Test | Type | What It Proves |
|---|---|---|
| ... | unit | ... |

## Rollout

- Version:
- Migration:
- Manual verification:
- Rollback:
