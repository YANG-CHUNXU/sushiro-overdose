# Release notes: v2.23.0

2.23.0 把「每日到店 Routine」改成更安全、也更符合用户心智的「每日取号提醒 Routine」。

## 变化

- Routine 不再自动向寿司郎提交取号请求，只会按目标就餐时间倒推取号窗口，并提前提醒用户手动取号。
- 启用 Routine 前必须先配置通知渠道；否则后端会拒绝保存，前端也会引导去设置通知。
- 兼容 2.22 旧配置：历史 `source=routine` 的未执行自动取号计划会被退役为 idle，避免升级后误触发。
- Routine 状态改为 `waiting_data` / `needs_notify` / `armed` / `notified` / `missed` / `done`，去掉 Routine 专属的今日认证要求。
- 设置页认证状态回归为「官方操作凭证」状态，不再把 Routine 绑定到当天重新认证。
- 增加设计系统摸底与交接文档，明确 read-only、提醒、会执行操作三个层级。

## 用户影响

- 如果之前开启过 2.22 的 Routine，升级后它不会再自动取号。
- 想继续使用 Routine，需要先配置飞书、Telegram、Bark 或 Server酱任一通知渠道。
- 自动取号计划仍保留在高级区域，启用前仍会再次确认。

## 验证

- 覆盖 Routine 提醒规划、无通知拒绝、手动计划保护、旧 Routine 自动计划退役、样本不足不提醒。
- 覆盖前端文案与状态渲染的本地 smoke 检查。
