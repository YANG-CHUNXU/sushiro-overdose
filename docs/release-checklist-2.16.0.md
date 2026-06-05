# Release Checklist: 2.16.0

2.16.0 is the product experience rebuild release. It changes the Web UI entry model from feature modules to user tasks while keeping booking and queue mutation boundaries intact.

## Required Checks

```bash
go test ./...
go build ./...
git diff --check
perl -0777 -ne 'if(/<script>\n(.*)\n<\/script>/s){print $1}' internal/app/web_static.go > /tmp/sushiro-web.js
node --check /tmp/sushiro-web.js
go build -ldflags "-X main.Version=2.16.0" -o sushiro .
```

## Manual Smoke

- Open the app with an isolated `HOME`.
- Confirm the homepage renders without console errors.
- Confirm the new top navigation contains 首页、现在去吃、我有号码、预约未来、自动蹲号、我的单据、设置.
- Click 打开新手向导 and confirm the scenario chooser opens.
- Navigate to 我有号码 and confirm the page explains “输入手里的号，判断几点到店”.
- Navigate to 现在去吃 and confirm the page marks read-only queue viewing.
- Navigate to 设置 and confirm 常用设置 and 高级工具 are separated.

## Safety Checks

- Starting auto booking shows a confirmation that it may create a reservation and will not cancel existing orders.
- Starting sniper shows a confirmation that it may create a reservation.
- Enabling auto queue ticket shows a confirmation that it may remotely take a ticket.
- Cancel reservation and cancel queue ticket confirmations explicitly say the official order/ticket will be cancelled and cannot be restored.

## Publish

```bash
git status
git tag v2.16.0
git push origin master
git push origin v2.16.0
```
