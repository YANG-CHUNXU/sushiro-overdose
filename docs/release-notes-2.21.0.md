# 2.21.0 Release Notes

2.21.0 是云端数据登录与叫号曲线升级版本。

## 新增

- 设置页新增“云端数据”：填写 Cloudflare Worker URL 后，可以用 GitHub OAuth 登录云端数据服务。
- 本地新增 `~/.sushiro/cloud_auth.json`，只保存 Worker URL 和 Worker 签发的应用 session，不保存 Turso token。
- 新增 Cloudflare Worker 工程 `cloudflare/sushiro-cloud/`：负责 GitHub OAuth、HMAC session、Turso secrets 和固定白名单查询。
- 远程基准加载支持两种来源：开发环境仍可用本机 Turso 直连凭证；普通用户可用 Cloudflare Worker 会话代理线上 Turso。
- “我有号码”主曲线优先展示，并融合本机采样、线上基准、排队压力维度。

## 修复

- 多段叫号提醒命中后自动删除，删除规则时同步清理去重状态，避免“阅后即焚”提醒长期残留。
- 线上基准在缺少叫号列的 schema 下自动回退，不影响等待分钟和等位桌数基准读取。
- 官方返回 `E010/error.server` 时改为提示凭证大概率需要刷新；定时取号遇到该错误也会触发重新认证提醒。

## 注意

- 当前线上 Turso schema 尚未提供用户贡献表，也没有叫号聚合列；本版本不上传用户采样。
- 如需让用户采样进入线上库，需要先设计贡献表、迁移 schema、配置写入权限和防滥用策略。
- Cloudflare Worker secrets 必须通过 Wrangler 或 Cloudflare Dashboard 配置，不能写进仓库。
