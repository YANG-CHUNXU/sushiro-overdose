# 寿司郎排队助手

一个查寿司郎排队和叫号进度的桌面工具，用 Go 写的，跨平台。开箱不用登录，拿到排队号之后能算个大概几点叫到、几点该出门。

起因很简单：寿司郎拿号后小票上只有号码，门店大屏只显示当前叫到几号，前面还要等多久、什么时候该动身，全靠自己盯着算。这个工具就把"现在叫到几号、平均几分钟一个、前面还剩多少桌"这些本来就公开的数据，算成一个能照着执行的时间建议。

[![Latest Release](https://img.shields.io/github/v/release/Ryujoxys/sushiro-overdose?label=release)](https://github.com/Ryujoxys/sushiro-overdose/releases/latest)
[![Platforms](https://img.shields.io/badge/macOS%20%7C%20Windows%20%7C%20Linux-supported-2d9c4a)](#下载安装)

<p align="center">
  <img src="docs/screenshot-home.png" width="720" alt="首页" />
</p>

## 它能干什么

打开就能查门店开没开、前面排了几桌、当前叫到几号、这会儿挤不挤。挑两家常去的店盯着，叫号快了再出门。

<p align="center">
  <img src="docs/screenshot-queue.png" width="720" alt="实时排队面板" />
</p>

手里有号的话，把排队号填进去，工具会告诉你大概多久叫到、建议几点前后出发。估算依据是最近的叫号速度、前面剩余桌数和历史规律，会随着叫号进度不断更新：

> 1078 号，当前叫到 1051，预计 38-62 分钟后叫到（约 12:18-12:42）。建议 12:10 前后出发。

<p align="center">
  <img src="docs/screenshot-chart.png" width="720" alt="叫号进度与排队压力" />
</p>

快叫到了可以推一条通知，支持飞书、Telegram、Bark、Server酱，几个渠道能同时开，人不用一直守着屏幕。

部分门店支持预约。工具能查未来哪些天、哪些时段还能约，对于没放出来的热门时段，可以挂着等它一开放就自动抢。预约、远程取号、取消这些会动账号的操作，需要先从手机寿司郎小程序取一次"通行证"凭证；只看排队和叫号预测不用管这个，想约未来的时候再弄也不迟。

## 和直接看小程序的区别

小程序只告诉你"现在叫到几号"，不告诉你还要等多久、几点该走。这个工具多算了一步，给一个能照做的出发时间，顺便帮你盯着快到了提醒一下。排队和叫号信息本来就是公开的，工具只是读出来算一下，不上传也不碰别人账号。

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

## 怎么用

双击运行，会自动弹出网页界面。搜城市或门店名挑一家常去的店，没号就看排队压力，有号就把号填进去，它会直接告诉你几点出发。

想要到点提醒的话，去设置里填一个飞书 / Telegram / Bark 的通知地址就行。约未来需要先按界面提示拿一次通行证，大概 3 分钟。

只看排队和叫号预测不用登录，也不会动你的账号。取号、预约、取消这些会动手的操作，点之前都会再确认一次。

## 关于数据

排队和叫号信息本来就是公开的，小程序里也显示，工具只是读出来算一下。凭证只存在你自己电脑上（`~/.sushiro/`），不上传任何第三方服务器；软件开源，代码都能看。远程抢预约和取号走的是寿司郎官方接口，和你用手机操作一样。

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
