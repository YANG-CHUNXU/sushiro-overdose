# Design System Audit / Extend / Handoff: Routine Reminder

## design-system audit

### Current problem

- The "现在去吃" page mixes read-only answers, notification setup, remote ticket actions, automatic ticket plans, sampling controls, and prediction charts in one visual weight level.
- Users cannot quickly tell which blocks are safe read-only, which blocks require notifications, and which blocks will submit official Sushiro actions.
- Routine was visually placed beside automatic ticket plans and used action-colored affordances, so it looked like another execution feature instead of a lightweight reminder.
- Settings still exposes important prerequisites and debug controls with similar priority, which makes the product feel busier than the main user journey requires.

### Product priority

- Primary question: "我几点取号、几点能吃上？"
- Secondary question: "我手里有号，几点会叫到？"
- Operational prerequisites: notifications, credential status, local/online prediction data.
- Advanced actions: remote ticket, automatic ticket plan, cancellation, recovery, debugging.

### Information hierarchy rule

- Read-only prediction should be visually first and low-friction.
- Reminder-only features must be labeled "只提醒 / 需要通知".
- Official API actions must be labeled "会执行操作" and stay inside advanced or confirmation-heavy areas.
- Debug/developer controls should only appear after debug mode is enabled.

## design-system extend

### Component: RequirementStrip

Use this near any feature that has hard prerequisites.

Props:

- `items`: list of `{label, state, actionLabel, action}`.
- `state`: `ok | warn | bad | neutral`.
- `density`: `compact | normal`.

Usage:

- Routine: `通知: bad -> 配置通知`, `预测样本: warn -> 提升预测准确度`.
- Auto ticket: `寿司郎凭证: bad -> 重新认证`, `通知: warn -> 配置通知`.

Copy rules:

- Bad states should describe the missing prerequisite, not the implementation.
- Example: "未配置通知，无法按时提醒取号。"

### Component: FocusTaskCard

Use for the top-level task cards that answer user intent.

States:

- `readonly`: can start without credentials.
- `notify-required`: reminder feature that will not submit official actions.
- `action-required`: will submit/cancel official requests after confirmation.

Visual rules:

- Only one primary red button per card.
- Secondary actions should be outline buttons.
- Each card must show exactly one clear outcome sentence.

### Component: RoutineReminderCard

Routine is now reminder-only.

Fields:

- Store.
- Target meal time.
- Travel minutes.
- Notify before minutes.

Status matrix:

- `idle`: not enabled.
- `waiting_data`: enabled but no reliable historical sample.
- `needs_notify`: enabled/configured but notification channel is missing.
- `armed`: reminder time is scheduled for today.
- `notified`: today's reminder has been sent.
- `missed`: today's recommended pickup window has passed.
- `done`: the user already has a ticket today.

Copy rules:

- Never say "自动取号" in this card.
- Always use "提醒你手动取号" or "不会自动向寿司郎提交".
- If samples are missing, say "样本不足，不会乱提醒".

### Component: QuietAdvancedPanel

Use for controls that are useful but not part of the primary journey.

Default placement:

- Auto ticket plan.
- Recover ticket status.
- Cancel current ticket.
- Raw sampling configuration.
- Cloud Worker endpoint.
- Diagnostic and debug flags.

Rules:

- Summary must include "高级" and, if relevant, "会执行操作".
- Destructive actions cannot appear as the first button.

### Component: StatusCallout

Use for stateful explanations under a card.

Content model:

- `headline`: one sentence with the user's current state.
- `detail`: one sentence with the practical consequence.
- `actions`: at most two visible buttons by default.

Example:

- Headline: "已开启：今天 11:50 提醒你取号"
- Detail: "太阳宫凯德店 · 目标 13:00 吃 · 建议取号 12:00 · 预计等 60-60 分钟。"

## design-handoff

### User journey

1. User opens "现在去吃".
2. User selects one or more stores and sees queue pressure first.
3. If they know a target meal time, they configure "每日取号提醒 Routine".
4. The app requires notification configuration before enabling Routine.
5. The app estimates the pickup window from historical/local/cloud data.
6. At the reminder time, the app sends a notification asking the user to manually take a number.
7. The app never auto-submits a ticket from Routine.

### Page-level changes

- Keep Routine near "现在去吃", but label it as reminder-only.
- Keep automatic remote ticket plans collapsed in "高级".
- Keep settings status cards ordered as authentication, online baseline, notification, prediction data.
- Do not add another curve just for local user data; local and online data should be fused into the existing queue pressure and prediction curve.

### Engineering constraints

- Keep the project single-binary and zero frontend build dependencies.
- Components are implemented as HTML/CSS/JS inside `internal/app/web_static.go`.
- Prefer state/copy cleanup before adding new visual density.
- Backend must enforce notification requirement; frontend validation is only a UX convenience.

### Acceptance checklist

- Routine POST with `enabled=true` and no notification returns 400.
- Routine no longer writes a `source=routine` auto-ticket plan.
- Existing non-terminal `source=routine` plans from v2.22 are retired on routine refresh/save.
- Routine UI never says it will auto take a ticket.
- Settings no longer shows Routine-specific daily credential requirements.
- Design handoff is reflected in copy and component states.
