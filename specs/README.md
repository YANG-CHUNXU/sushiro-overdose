# Specs

Large features in this repository use spec-driven development. A feature should start here before code changes, especially when it touches reservation, queue ticket, collection, dashboard, or local state.

## Workflow

1. Copy `specs/_template/` to `specs/NNN-short-name/`.
2. Fill `spec.md` with user-visible behavior and acceptance criteria.
3. Fill `plan.md` with architecture impact, data contracts, state rules, and tests.
4. Fill `tasks.md` with implementation tasks that can be checked off.
5. Implement only what is in scope, then update tests.

## Required For

- New reservation or queue-ticket behavior.
- Any API request to `api_auth`.
- Any local state file migration or cleanup.
- Any dashboard or prediction feature that changes user decisions.
- Any background process, scheduler, collector, or automation.

## Safety Baseline

Read `.specify/memory/constitution.md` before drafting a feature. The constitution is the architecture baseline for LLM agents and human reviewers.
