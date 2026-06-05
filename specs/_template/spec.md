# Feature Spec: <feature name>

## Metadata

| Field | Value |
|---|---|
| Feature | `<short-name>` |
| Version Target | `<version>` |
| Owner | `<owner>` |
| Date | `<YYYY-MM-DD>` |
| Affected Surfaces | `<web / cli / api / collector / state>` |

## Problem

Describe the user problem in concrete terms. Avoid implementation details here.

## Goal

One sentence describing the successful outcome.

## User Stories

| ID | Story | Priority |
|---|---|---|
| US-1 | As a user, I can ... | P0 |

## Scope

### In Scope

- ...

### Out Of Scope

- ...

## Behavior

| State/Input | Expected Behavior | User Copy |
|---|---|---|
| Normal | ... | ... |
| Empty | ... | ... |
| Official API 404 | ... | ... |
| Official API 409 | ... | ... |
| Official API 500 | ... | ... |

## Data Rules

| Data | Source | Retention | Can Write Local State | Can Call Official Mutation |
|---|---|---|---|---|
| ... | ... | ... | No | No |

## Acceptance Criteria

- [ ] ...
- [ ] No read-only entrypoint calls official mutation APIs.
- [ ] Reservation and queue-ticket cancellation paths remain separate.

## Open Decisions

- ...
