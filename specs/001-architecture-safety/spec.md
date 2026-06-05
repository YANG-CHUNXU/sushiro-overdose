# Feature Spec: Architecture Safety Baseline

## Metadata

| Field | Value |
|---|---|
| Feature | `architecture-safety` |
| Version Target | `2.12.x` |
| Owner | `sushiro-overdose` |
| Date | `2026-06-05` |
| Affected Surfaces | `repo / ci / tests / docs` |

## Problem

The project now has booking, queue ticket, public queue collection, prediction, dashboard, and local state. LLM-assisted changes can easily mix these boundaries, especially around cancellation and state refresh. We need a repository-level baseline that makes architectural rules explicit and testable.

## Goal

Create a spec-driven development framework that gives LLM agents and human reviewers a stable architecture baseline before implementation starts.

## User Stories

| ID | Story | Priority |
|---|---|---|
| US-1 | As a maintainer, I can point agents to a constitution that defines non-negotiable architecture rules. | P0 |
| US-2 | As a reviewer, I can see a feature spec/plan/tasks before large changes. | P0 |
| US-3 | As a user, I am protected from accidental reservation or queue cancellation caused by read-only refreshes. | P0 |

## Scope

### In Scope

- Add repository-level constitution for LLM and human development.
- Add reusable feature spec templates.
- Add one CI-level architecture guard for cancellation boundaries.
- Document the required workflow for future large features.

### Out Of Scope

- Installing external Spec Kit tooling.
- Refactoring current app architecture.
- Rewriting existing features into full specs retroactively.

## Behavior

| State/Input | Expected Behavior | User Copy |
|---|---|---|
| New large feature | Create `spec.md`, `plan.md`, `tasks.md` before implementation | N/A |
| Read-only handler | Must not call official mutation APIs | N/A |
| Cancellation path | Reservation cancellation and queue-ticket cancellation stay separate | N/A |
| CLI cancellation | `cmdCancel` remains an explicit reservation-only cancellation command | N/A |
| CI run | Architecture guard fails if official mutation APIs are called outside approved explicit entrypoints | N/A |

## Data Rules

| Data | Source | Retention | Can Write Local State | Can Call Official Mutation |
|---|---|---|---|---|
| Spec docs | repo | permanent | No | No |
| Architecture guard | source scan | test runtime | No | No |

## Acceptance Criteria

- [ ] Constitution exists at `.specify/memory/constitution.md`.
- [ ] Feature templates exist under `specs/_template/`.
- [ ] A first spec exists under `specs/001-architecture-safety/`.
- [ ] CI runs a test that prevents cancellation API calls outside explicit cancel entrypoints.
- [ ] No read-only entrypoint calls official mutation APIs.
- [ ] Reservation and queue-ticket cancellation paths remain separate.

## Open Decisions

- Whether to install GitHub Spec Kit CLI later or keep the repo-local framework lightweight.
