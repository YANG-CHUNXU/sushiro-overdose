# Release notes: v3.5.4

v3.5.4 给 macOS DMG 和 Windows EXE 加上 sushiro 图标，并加强「首次打开被系统拦截」的用户提示。

## 变化

### macOS DMG / .app 图标

- 之前 `.app` 和 DMG 都是系统默认图标（`Info.plist` 没有 `CFBundleIconFile`）。
- 新增 `assets/icons/sushiro.icns`（全尺寸，从 sushiro.png 放大到 1024 生成）。
- `scripts/bundle-macos.sh` 现在把 icns 嵌进 `.app/Contents/Resources/`，plist 加 `CFBundleIconFile`。安装后 App 图标和 DMG 打开后的图标都是 sushiro。

### Windows EXE 图标 + DPI 感知

- 之前 Windows exe 是裸 `go build`，无图标、无 DPI 感知（高 DPI 屏模糊）。
- 新增 `resource_windows_amd64.syso` / `resource_windows_arm64.syso`（按架构自动链接），内含 sushiro 图标 + manifest（PerMonitorV2 DPI 感知 + Win10 兼容 + Common Controls v6）。
- 现在下载的 exe 任务栏、文件管理器、窗口图标都是 sushiro，高 DPI 屏不再模糊。

### 「首次打开被拦」提示加强

用户反馈 macOS 打不开 App。README 已有提示但不够醒目、步骤不全，现在改成两种放行方式 + 终端兜底命令：
- macOS：系统设置→隐私与安全性→仍要打开（推荐）；或右键 App→打开；或 `xattr -dr com.apple.quarantine "/Applications/Sushiro Overdose.app"`。
- Windows：SmartScreen 点「更多信息」→「仍要运行」。
- 故障表同步更新。

> DMG/exe 仍是未签名（没有付费开发者证书）。这是开源工具的常态，不是安全问题——首次放行一次后永久可用。

## 验证

- `gofmt` / `go build ./...`（darwin/linux/windows×amd64/arm64）/ `go vet` / `go test ./...` 全绿。
- `bash -n scripts/bundle-macos.sh` 通过。
- Windows exe 本地交叉编译确认：syso 按架构自动选择、图标和 manifest 嵌入成功（strings 检测到 PerMonitorV2/Common-Controls）。

> DMG 图标和 exe 图标的最终视觉效果要等 CI 跑完 release 产物后下载确认。
