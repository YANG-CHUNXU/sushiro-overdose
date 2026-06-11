# SUSHIRO 寿司郎助手（sushiro-overdose）

[![Latest Release](https://img.shields.io/github/v/release/Ryujoxys/sushiro-overdose?label=release)](https://github.com/Ryujoxys/sushiro-overdose/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![Platforms](https://img.shields.io/badge/macOS%20%7C%20Windows%20%7C%20Linux-supported-2d9c4a)](#下载安装)

**sushiro-overdose** 是面向中国大陆寿司郎（SUSHIRO / 寿司郎 / すしロー）的排队叫号、到店预测、取号提醒和预约助手。

它解决三件事：

- **现在想去吃**：不用登录，查看门店营业、等位、当前叫号、排队压力和到店建议。
- **已经拿到排队号**：输入手里的号，估算几点叫到、几点出发，并设置多段提醒。
- **想约未来或远程操作**：获取一次微信小程序凭证后，可查可约日历、自动抢预约、远程取号、取消排队号。

项目名、命令名和 Release 产物仍保持 `sushiro-overdose` 不变。

## 下载安装

### Windows

推荐下载 [latest release](https://github.com/Ryujoxys/sushiro-overdose/releases/latest) 里的 `Sushiro-Overdose-*-windows-amd64.exe`，双击运行。Windows ARM 设备下载 `windows-arm64.exe`。

也可以用 PowerShell 一行安装：

```powershell
irm https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install/install.ps1 | iex
```

首次运行如果遇到 SmartScreen，点击「更多信息」再选择「仍要运行」。

### macOS

下载 [latest release](https://github.com/Ryujoxys/sushiro-overdose/releases/latest) 里的 `Sushiro-Overdose-*-macOS.dmg`，打开后拖到 Applications。

当前 DMG 默认未签名。首次打开如果提示无法验证开发者，请在「系统设置 -> 隐私与安全性」允许打开，或右键 App 选择「打开」。

### Linux

下载 `sushiro-overdose_*_linux_amd64.tar.gz` 并解压运行，或执行：

```bash
curl -fsSL https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install/install.sh | bash
```

### 从源码构建

```bash
git clone https://github.com/Ryujoxys/sushiro-overdose.git
cd sushiro-overdose
go build -o sushiro .
./sushiro
```

## 快速开始

1. 双击运行程序，默认打开 Web UI。
2. 在首页选择常用门店，先查看实时排队和叫号趋势。
3. 如果已经取到排队号，进入「我有号码」，输入号码后查看 ETA 和出发建议。
4. 需要提醒时，先在「设置」配置飞书、Telegram、Bark 或 Server酱通知。
5. 需要预约、远程取号、取消排队号或读取我的单据时，再按向导获取「通行证」。

看排队、看叫号预测、看排队压力不需要登录，也不会影响账号。提交官方操作前界面会再次确认。

## 核心功能

| 场景 | 功能 |
|------|------|
| 现在去吃 | 实时排队、当前叫号、等待桌数、排队压力、门店推荐 |
| 我有号码 | 叫号 ETA、建议出发时间、整合走势大图、多段到店提醒 |
| 约未来 | 可约日历、门店多选、午餐/晚餐筛选、自动抢预约 |
| 官方操作 | 远程取号、定时取号、取消排队号、读取本机可识别单据 |
| 通知 | 飞书、Telegram、Bark、Server酱，多渠道可同时启用 |
| 数据 | 本机采样、线上只读基准、历史规律、排队压力曲线 |

## 通行证与凭证过期

「通行证」指从寿司郎微信小程序请求中提取的凭证参数。它只用于需要官方身份的操作，例如预约、取号、取消、读取我的单据。

凭证会过期，也可能被手机重新登录小程序后顶掉。遇到这些情况时，建议在设置里重置认证并重新获取通行证：

- 官方接口返回 `E010/error.server`
- 返回 401 / 403
- 远程取号或自动预约突然失败
- 手机端重新打开过寿司郎小程序后，电脑端旧凭证失效

Windows 上通常使用手机抓包导入凭证；macOS 可优先尝试 PC 微信自动捕获。向导会按步骤提示。

## 数据与隐私

本机数据默认存放在 `~/.sushiro/`：

- `config.json`：寿司郎凭证参数
- `notify.json`：通知渠道配置
- `preferences.json`：用户偏好
- `history.jsonl`：预约时段历史
- `queue_observations.jsonl`：实时排队快照
- `queue_sessions.jsonl`：真实取号等待记录
- `cloud_auth.json`：Cloudflare Worker URL 与 GitHub 登录 session，不包含 Turso token

公开排队和本机采样默认只保存在本机。若你配置自建 Cloudflare Worker + Turso，只读数据库 token 应保存在 Worker Secrets 中；客户端只保存 Worker URL 和登录 session。读取线上基准时应按选中门店查询，避免无意义全量读取。

## 常用命令

```bash
sushiro                 # 启动 Web UI
sushiro cli             # 终端交互模式
sushiro doctor          # 只读诊断
sushiro repair-proxy    # 恢复系统代理
sushiro uninstall       # 清理本地敏感数据和证书

sushiro calendar        # 查看可预约时段
sushiro list            # 查看当前预约
sushiro cancel <id>     # 取消预约
sushiro sample once     # 采集一次排队/时段数据
```

更多命令可运行：

```bash
sushiro help
```

## 排障

- **打不开页面**：重新运行程序，端口冲突时会自动换端口。
- **系统代理异常**：运行 `sushiro repair-proxy`，或在设置页点击代理修复。
- **通知收不到**：先在设置页点「测试通知」，确认 Webhook 或 Token 正确。
- **取号失败 E010**：通常先重置认证，再重新获取通行证。
- **macOS 无法打开 App**：到「系统设置 -> 隐私与安全性」允许打开。
- **Windows 被拦截**：选择「仍要运行」，或将 exe 加入杀毒白名单。

诊断信息可运行：

```bash
sushiro doctor
```

## 开发

```bash
go build ./...
go test ./...
go vet ./...
```

架构、文件职责和发布细节见 [AGENTS.md](AGENTS.md) 与 [ARCHITECTURE.md](ARCHITECTURE.md)。

发布新版本：

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

GitHub Actions 会自动构建 macOS、Windows、Linux 产物并创建 Release。

## License

MIT
