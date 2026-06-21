# v3.11.0

## feat: 凭证采集向导全套 UX 重构 —— 阶段进度 + 错误人话化

把"拿通行证"从"一行提示 + 信号灯"升级为**傻瓜式、全程可见进度、错误都说人话**的体验。

### 背景

"拿通行证"是新人最大门槛。调研确认采集原理（MITM 抓包）无法绕过——token 只在小程序请求头里，
二维码扫不到、MCP 也没有特权通道；Windows UAC、iPhone 证书信任是 OS 行为无法消除。但现有向导
的**体验**远没达到这套原理能达到的最简：PC 抓路径没阶段进度、证书装没装上靠正则猜 message、
错误是技术堆栈、手机引导页与主应用割裂、安卓引导两边矛盾。

### 本次（把现有件串成傻瓜体验，不换原理）

1. **结构化阶段进度条**（`engine.go` + `web_static.go`）
   - 后端 `runCapture` 推 `stage` 枚举：装证书→起代理→设系统代理→抓包→自检
   - 前端 `captureProgressHTML` 横向进度条，已完成打绿勾、当前高亮、未来灰
   - PC 抓路径也显示"已抓到 X/8 字段，还差：…"（之前只有手机 auto 路径有字段勾选）
   - waiting 阶段每秒更新字段计数

2. **错误全说人话**（`engine.go` + `web_static.go`）
   - 弃用 `explainMsg` 正则猜 message（脆弱），改用 `ErrorKind` 枚举驱动
   - 每类错误一条人话 + 一个出路按钮：
     - `cert_uac_declined`："刚才弹的系统窗口你没点是，重装时务必点是"
     - `cert_locked`："钥匙串锁住了，security unlock-keychain 解锁后重试"
     - `proxy_failed`："先到设置页修复代理再重试"
     - `quic_block_failed`："微信可能走旁路了，重启微信再试"
     - `auth_stale` / `network` 等
   - Windows 装 LocalMachine 证书前**预告 UAC**（不再装失败了才解释）

3. **QUIC 屏蔽透明化**（`platform_windows.go` + `platform.go`）
   - `blockSushiroQUIC` 失败不再静默降级（仅日志），改回传非致命 warning 到前端提示用户
   - 新增 `RecordProxyWarning` / `DrainProxyWarnings` 收集器，engine 读后推 `EngineState.Warning`

4. **手机引导页重写**（`mobile_auth_capture.go`）
   - 复用主应用设计语言（red/green-soft/yellow-soft 配色体系），不再脱节的内联 style
   - iPhone/安卓**分 tab 各自一套清晰步骤**，消除原来"手机页说安卓改用电脑、电脑页说装 Reqable"的矛盾
   - iPhone 突出"证书信任设置"易漏步骤 + 完成回电脑验证的提示
   - 步骤编号 + 进度感

5. **macOS keychain 锁定检测**（`platform_darwin.go`）
   - `add-trusted-cert` 遇锁定（"User interaction is not allowed"）给出明确关键词，
     `classifyCertError` 归为 `cert_locked` 并提示解锁

### 系统限制（不可突破，只做预期管理）

- Windows 装 LocalMachine 证书必弹 UAC → 装之前预告
- iPhone 装完证书要去设置开完全信任 → 引导页直通路径 + 验证回环
- 微信 QUIC 绕过 TCP 代理 → 屏蔽失败时告知用户

### 不在范围
- 不换采集原理（仍 MITM，不做 MCP——那是 LLM 对话工具，与采集门槛无关）
- 不做"二维码自动采集"（物理不可行）
- 不动凭证过期机制（同账号单会话互斥是服务端行为）

> 仅改采集向导 + 平台证书/代理 + 错误处理；抢号/预约/排队逻辑零改动。
