# Release Checklist: 2.15.0

## Scope

2.15.0 is the queue decision dashboard release. It makes the dashboard answer user-facing questions directly:

- "我这个号大概几点叫到？"
- "我应该几点前到店？"
- "现在哪个门店更好去？"
- "普通工作日、周末、节假日是否分开看？"

## Included Changes

- `/api/queue/dashboard` accepts `target_no` and returns an `advisor` object.
- `advisor.target_bucket` and `advisor.arrival_label` explain target-number timing.
- `advisor.milestones` explains key time points when no target number is entered.
- `advisor.store_choices` ranks current stores by open state, online ticket state, wait minutes, and waiting groups.
- Web Data Dashboard has a "我的号" input and renders the advisor card before charts/tables.
- README and release notes document the feature.
- Architecture guard keeps dashboard/read-only paths away from official mutation APIs.

## Verified Locally

Run from repository root:

```bash
go test ./...
go build ./...
go vet ./...
git diff --check
```

Expected result: all commands exit with code `0`.

## Local Binary

The local ignored `sushiro` binary can be rebuilt as:

```bash
go build -ldflags "-X main.Version=2.15.0" -o sushiro .
```

Sanity check:

```bash
strings sushiro | rg "2\\.15\\.0"
```

## Pre-Release Git Checks

Before publishing, confirm the worktree contains only intended 2.15 changes:

```bash
git status --short
git diff --stat
```

Review the release notes:

```bash
cat docs/release-notes-2.15.0.md
```

## Publish

Only run after the user explicitly confirms release:

```bash
git add .
git commit -m "feat: 发布 2.15 排队决策看板"
git tag v2.15.0
git push origin HEAD
git push origin v2.15.0
```

GitHub Actions should then create the Release through GoReleaser.

## Post-Release Verification

After CI completes, verify the Release contains:

- macOS DMG.
- Windows amd64 exe.
- Windows arm64 exe.
- Linux amd64 archive.
- Linux arm64 archive.
- checksums.

Also verify the Web UI update check sees `v2.15.0` as latest.

## Safety Notes

- Do not trigger booking, queue-ticket, or cancellation endpoints during release verification.
- Do not run a real Web UI process with the user's production `~/.sushiro` if there is an active timed queue-ticket plan.
- Use a temporary `HOME` for UI smoke tests when only validating rendering.
