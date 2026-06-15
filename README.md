# 🍣 寿司郎排队助手

> 拿到号之后，不用再傻站着干等了 —— 告诉你大概几点叫到、几点该出门。

[![Latest Release](https://img.shields.io/github/v/release/Ryujoxys/sushiro-overdose?label=release)](https://github.com/Ryujoxys/sushiro-overdose/releases/latest)
[![Platforms](https://img.shields.io/badge/macOS%20%7C%20Windows%20%7C%20Linux-supported-2d9c4a)](#下载安装)

---

## 这是啥？一句话版本

吃寿司郎总是要排队。这个工具帮你**算清楚两件事**：

1. **你手里那个号，大概几点能叫到？**
2. **那你几点出门最合适，不用去太早干等？**

它不是黄牛、不是代抢，就是把"现在叫到几号了、平均几分钟叫一个、前面还有多少桌"这些公开信息，算成一个**能看懂的时间建议**给你。

---

## 它能帮你做什么

<p align="center">
  <img src="docs/screenshot-home.png" width="720" alt="首页：三个场景入口" />
</p>

### 🥢 场景一：「我现在想去吃」

不用登录、不用输任何东西。打开就告诉你：这家店开着没、前面排了多少桌、大概要等多久、现在这会儿挤不挤。

<p align="center">
  <img src="docs/screenshot-queue.png" width="720" alt="实时排队面板" />
</p>

挑两家常去的店盯着，叫号快了、人少了，再出门。

### 🎫 场景二：「我已经拿到号了」（最常用）

手里攥着个小票，写着 `1078 号`，前面叫到 `1051` —— 那我到底要等多久？

把号输进去，它告诉你：

> 你是 1078 号，当前叫到 1051，预计 **38-62 分钟**后叫到（约 12:18-12:42）。**建议 12:10 前后出发。**

<p align="center">
  <img src="docs/screenshot-chart.png" width="720" alt="叫号进度与排队压力图" />
</p>

还能设提醒：快叫到你了，**飞书 / Telegram / Bark / 微信** 自动推一条，你可以去旁边星巴克坐着，不用一直盯着小票。

### 📅 场景三：「我想约下周六」

有的店可以预约。但好的时段一放出来就被秒光。工具可以帮你：

- 看未来哪些天、哪些时段还能约
- 没放出来的时段，让工具**蹲着**，一开放瞬间帮你抢（自动抢预约）

> 「通行证」是啥？就是从你手机寿司郎小程序里取一次登录凭证，让电脑能替你查预约、取号、取消。**只看排队和叫号预测完全不需要它**，想约未来或远程操作时再弄也不迟。

---

## 为什么不是直接看小程序？

小程序只告诉你"现在叫到几号"，但**不告诉你还要等多久、几点该走**。

这个工具多算了一步：根据最近的叫号速度、前面剩多少桌、历史规律，给你一个**可执行的建议**（几点出发），还顺手帮你盯着、快到了提醒你。

---

## 下载安装


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

## 快速开始（三步上手）

1. **打开** —— 双击运行，自动弹出网页界面。
2. **选店** —— 搜城市或门店名，挑你常去的那家。
3. **看建议** —— 没号就看排队压力；有号就输进去，直接告诉你几点出发。

想被提醒？去「设置」填一下飞书 / Telegram / Bark 任一个就行。想约未来？按界面提示拿一次通行证（约 3 分钟）。

> 放心：只看排队和叫号预测**完全不用登录**，也不会动你的账号。任何会"动手"的操作（取号、预约、取消），点之前都会再问你一次确认。

## 我的数据安全吗？

- 排队信息是**公开**的（小程序本来就显示），工具只是帮你读和算。
- 你的凭证只存在**你自己电脑**上（`~/.sushiro/`），不上传任何第三方服务器。
- 软件是**开源**的，每一行代码都能看。没有任何后台偷偷上传。
- 远程抢预约 / 取号用的是寿司郎官方接口，和你手机操作一模一样。

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
