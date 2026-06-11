# SUSHIRO 寿司郎排队叫号与预约助手（sushiro-overdose）

[![Latest Release](https://img.shields.io/github/v/release/Ryujoxys/sushiro-overdose?label=release)](https://github.com/Ryujoxys/sushiro-overdose/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![Platforms](https://img.shields.io/badge/macOS%20%7C%20Windows%20%7C%20Linux-supported-2d9c4a)](#下载安装)

**sushiro-overdose** 是面向中国大陆寿司郎（SUSHIRO / 寿司郎 / すしロー）的排队叫号、到店预测、取号提醒和预约助手。

最新 3.x 版本已经从“工具箱”收敛成普通用户能直接理解的场景流：不登录先看排队、输入号码算几点叫到、按饭点设置每日取号提醒；需要官方操作时再获取一次“通行证”（微信小程序凭证），用于约未来、远程取号、自动抢和读取我的单据。

English: a desktop helper for Sushiro China queue status, called-number prediction, arrival reminders, WeChat mini program credential capture, remote ticketing, and reservation automation.

零外部依赖，纯 Go 标准库实现。支持 macOS / Windows / Linux。

**双击运行，独立应用窗口操作，不需要任何编程知识。**

> 关键词：寿司郎、Sushiro、SUSHIRO 中国、寿司郎排队、寿司郎叫号、寿司郎取号、寿司郎预约、寿司郎小程序、微信小程序、实时排队、排队叫号、叫号预测、到店提醒、自动抢号、自动预约、飞书通知、Telegram 通知。

## 这个项目解决什么问题

- **现在想去吃寿司郎**：不用登录，直接看全国门店营业、等位、叫号和排队压力。
- **已经拿到排队号**：输入手里的号，估算几点叫到、几点前出发，并支持多段到店提醒。
- **想按饭点取号**：配置每日取号提醒，按“想几点吃”倒推取号窗口，到点前提醒你手动取号，不自动提交。
- **想约未来时段**：查看可预约日历；目标明确时进入“自动抢”，已放出的直接抢，没放出的蹲开放窗口。
- **需要官方操作**：远程取号、定时取号、取消排队号、读取我的单据和自动预约都会要求通行证，并在执行前再次确认。
- **凭证过期了**：遇到 `E010/error.server`、401/403 或手机端登录顶掉凭证时，引导重置认证并重新获取凭证。

## 功能速览

| 场景 | 功能 |
|------|------|
| 现在去吃 | 全国门店实时排队、营业状态、等位桌数、线上取号状态、门店推荐 |
| 我有号码 | 我的号码 ETA、几点出发、历史规律、排队压力、叫号曲线 |
| 提醒 | 当次多段叫号提醒、每日取号提醒、通知渠道强校验、手动删除 |
| 约未来 | 可约日历、门店多选、午餐/晚餐筛选、保存为抢号门店 |
| 自动抢 | 已放出时段按偏好开抢、未放出时段蹲开放窗口、抢到即停 |
| 官方操作 | 远程取号、定时取号、一开放就取号、恢复/取消当前排队号 |
| 通行证 | 手机抓包导入、PC 微信 MITM、凭证健康检测、认证重置 |
| 体验层 | 三态首页、健康胶囊、12 只寿司宝宝、回转寿司背景视觉 |

---

## 下载安装

### Windows 用户（推荐：一键安装）

任选一种方式：

**方式 A：双击 install.bat（最简单）**

1. 下载 [install.bat](https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install.bat)（右键「另存为」）
2. 双击运行，自动完成下载、解压、加入 PATH、创建桌面快捷方式
3. 安装完成后，双击桌面「Sushiro Overdose」图标即可使用

**方式 B：PowerShell 一行命令**

打开 PowerShell（按 `Win+X` → 选「终端」或「PowerShell」），粘贴执行：

```powershell
irm https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install/install.ps1 | iex
```

**方式 C：手动下载 .exe**

从 [GitHub Releases](https://github.com/Ryujoxys/sushiro-overdose/releases/latest) 下载 `Sushiro-Overdose-*-windows-amd64.exe`，双击即可。ARM 设备下载 `Sushiro-Overdose-*-windows-arm64.exe`。

> Windows 注意事项：
> - 首次运行 SmartScreen 可能弹窗提示「Windows 已保护你的电脑」，点击「更多信息」→「仍要运行」即可。
> - 推荐下载 `Sushiro-Overdose-*.exe` 直下版，双击会打开独立应用窗口且不显示终端黑框。压缩包内的 `sushiro.exe` 保留给高级用户命令行使用。
> - 程序会自动安装一张本地 MITM 证书并临时设置系统代理，**退出时自动恢复**。
> - 如杀毒软件误报，请将 `%LOCALAPPDATA%\sushiro\sushiro.exe` 加入白名单。
> - 抓包阶段只解密寿司郎 API 域名；其他站点流量保持 CONNECT 透传，不读取或解密内容。

### macOS / Linux 用户

| 平台 | 下载文件 | 使用方式 |
|------|---------|---------|
| **macOS** | `Sushiro-Overdose-*-macOS.dmg` | 双击打开 DMG，将 App 拖到 Applications 后运行，独立窗口优先 |
| **Linux** | `sushiro-overdose_*_linux_amd64.tar.gz` | 解压后终端运行 |

> macOS 注意事项：当前 Release 默认未签名/未公证。首次打开如果提示无法验证开发者，请在「系统设置 → 隐私与安全性」中允许打开，或右键 App 选择「打开」。

也可使用一键脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install/install.sh | bash
```

### 从源码构建

```bash
git clone https://github.com/Ryujoxys/sushiro-overdose.git
cd sushiro-overdose
go build -o sushiro .       # macOS / Linux
# Windows (PowerShell):
# go build -o sushiro.exe .
```

---

## 使用教程

### 第一次使用

1. **运行程序** → 自动打开独立应用窗口（无法打开时回退浏览器）
2. **先选常用门店** → 欢迎页选一家你常去的店，马上看到实时排队；选过的店会被记住，之后看排队、叫号预测、约号都自动带入
3. **先用只读功能** → 「现在去吃」看门店排队，「我有号码」算几点叫到；这些不需要通行证，也不会影响账号
4. **需要提醒时先配通知** → 飞书、Telegram、Bark、Server酱任选一个；当次多段提醒和每日取号提醒都依赖通知
5. **需要官方操作再拿通行证** → 抢预约、远程取号、取消排队号、读取我的单据时，按五步向导「拿通行证」（选设备 → 抓一次 → 传到电脑 → 粘贴解析 → 验证），中途关闭可断点续做
6. **约未来 / 自动抢** → 先看可约日历；目标明确时进入「约未来 → 自动抢」，已放出的时段直接抢，没放出的时段设置蹲号目标
7. **执行前确认** → 远程取号、自动抢、取消预约和取消排队号都会有明确确认；提醒类功能只推送通知，不会替你操作
8. **已有排队号 / 预约时** → 首页自动置顶你的单据卡：当前叫号、预计几点叫到、一键设提醒

> **用手机拿通行证（Windows 主路径，macOS 也可用）：** Windows 上 PC 微信抓不到凭证，需要用手机拿一次（macOS 可在向导第 1 步直接选「PC 微信自动抓」）。向导五步中手机相关的核心就三件事：
>
> 1. **手机上**：用本机抓包工具（iPhone 推荐 Stream，安卓可用 Reqable / HttpCanary）按其说明装好并信任证书，开启抓包后打开微信寿司郎小程序，点一次门店/排队/预约，再点一次「我的预约」，找到 `crm-cn-prd.sushiro.com.cn` 的请求导出成 cURL 或原始请求头。
> 2. **把内容发到电脑**：最稳的办法是手机微信搜「**文件传输助手**」，把复制的内容发给它，电脑微信打开同一会话即可复制——手机电脑不在同一网络也能用。
> 3. **粘贴到向导**：在电脑上把内容粘贴进向导，点「解析并保存」。建议至少含一次门店/查询请求和一次排队/预约请求；保存后会自动带入门店并可用「测试基础接口」验证。
>
> 程序会从中解析 `X-App-Code`、查询凭证、预约凭证、User-Agent、Referer、微信 ID、手机号和门店，全程只在本机处理。

> **同 Wi-Fi 无隔离时的备选（自动代理抓）：** 如果手机和电脑在同一 Wi-Fi 且路由器没开 AP（客户端）隔离，向导里也可改用「自动代理抓」：手机扫码打开引导页，装并信任 CA，把手机 Wi-Fi HTTP 代理指向电脑显示的 IP 和端口，再用手机打开寿司郎小程序点一次门店/排队/预约，电脑会自动捕获并保存，完成后请立刻关闭手机 Wi-Fi 代理。一开代理就网络不佳通常就是 AP 隔离，退回上面的手动导入即可。

### 日常使用

凭证参数捕获后自动保存，下次运行无需重复捕获（过期时会自动提示）。

打开程序 → 首页先看常用门店和活跃单据 → 现在就想吃进「现在去吃」 → 手里有号进「我有号码」 → 想约未来进「约未来」 → 目标明确时用「自动抢」 → 成功、异常或提醒命中后通过已配置渠道通知。

## 常见问题

### 只看寿司郎实时排队需要登录吗？

不需要。查看寿司郎门店实时排队、等位桌数、营业状态、可线上取号状态、叫号预测和到店建议时，默认使用公开门店数据，不需要微信小程序凭证，也不会影响你的账号。

### 什么时候需要获取寿司郎微信小程序凭证？

只有在读取“我的单据”、远程取号、定时取号、取消排队号、自动抢未来预约或蹲未放出的预约时段时才需要凭证。凭证只保存在本机 `~/.sushiro/config.json`，不会上传。

### `E010/error.server` 是什么？

在寿司郎官方接口里，`E010/error.server` 很多时候不是普通网络错误，而是本机保存的凭证已经过期，或被手机端重新打开寿司郎小程序后顶掉。遇到这个提示时，先在工具里重置认证，再重新获取凭证。

### 这能帮我判断寿司郎几点能吃上吗？

可以。进入「我有号码」输入你的排队号，系统会结合实时叫号、等位桌数、本机采样和线上基准，估算大概几点叫到、几点前到店，并支持提前多段提醒。

### 每日取号提醒会自动帮我取号吗？

不会。每日提醒只做一件事：按你想吃饭的时间倒推取号窗口，并提前几分钟通过通知提醒你手动取号。它要求先配置通知渠道，样本不足时不会乱提醒。真正会提交官方取号请求的是「现在去吃」里的高级自动取号计划，启用前会再次确认。

### 预约优先级

Web UI 的「设置」页面支持明确控制选号策略：

- **日期优先级**：按日期优先、周末优先、工作日优先
- **时段策略**：最早可约、最晚可约、接近目标时间
- **目标时间**：当选择「接近目标时间」时使用，例如 `1930`
- **每日时段范围**：工作日、周六、周日可分别配置多个时间段

同一天同一时间有多个门店时，会按已选择门店的优先顺序尝试。

---

## 全部命令

程序默认启动 Web UI。高级用户可使用命令行：

```
sushiro                 启动 Web UI（默认，推荐，独立窗口优先）
sushiro cli             终端交互模式（高级）
sushiro start           后台静默运行
sushiro status          查看运行状态
sushiro exit            停止后台进程

sushiro calendar        查看近 7 天可预约时段
sushiro sniper          狙击模式 - 提前锁定未开放时段
sushiro list            查看当前预约
sushiro cancel <id>     取消预约

sushiro trends          分析时段可用率趋势
sushiro recommend       智能推荐最佳时段
sushiro sample once     信息收集一次，写入本地历史数据
sushiro sample run      前台持续信息收集，不抢号
sushiro sample start    后台静默信息收集
sushiro sample autostart on|off   配置系统开机自启动信息收集
sushiro sample stop     停止后台信息收集
sushiro doctor          打印只读诊断信息
sushiro diag-bundle     导出脱敏证据包（zip），便于排障反馈
sushiro auth-probe      用已保存凭证测试官方基础接口连通性
sushiro repair-proxy    一键恢复系统代理
sushiro uninstall       恢复代理、移除证书并清理本地敏感数据
sushiro stop-processes  停止本应用相关进程，便于删除 exe

sushiro config                          查看通知配置
sushiro config feishu <webhook>         配置飞书通知
sushiro config telegram <token> <id>    配置 Telegram
sushiro config bark <url> <key>         配置 Bark
sushiro config serverchan <key>         配置 Server酱
sushiro config store add <id> <name>    添加门店昵称
```

---

## 通知配置

支持多个通知渠道，可在 Web UI 的「设置」页面配置，也可用命令行：

- **飞书** — 群机器人 Webhook
- **Telegram** — Bot Token + Chat ID
- **Bark** — iOS 推送
- **Server酱** — 微信推送

Web UI 的「设置」页面可点击「测试全部」，也可单独测试飞书、Telegram、Bark、Server酱。

## Web 增强功能

- **三态首页**：没有通行证时先引导看排队；有通行证后展示准备清单和任务入口；有活跃排队号/预约时，首页直接置顶单据卡。
- **场景化导航**：主路径收敛为「现在去吃」「我有号码」「约未来」「我的单据」「设置」。自动抢放在「约未来」子页里，不再让用户先理解“狙击/采集/基准”等内部概念。
- **现在去吃**：按关注门店展示实时营业、等位、当前叫号、近 15 分钟叫号和排队压力；高级区保留远程取号、定时取号、一开放就取号和取消排队号。
- **我有号码**：输入手里的排队号，先给“现在大概几点叫到、几点出发”的答案，再展开历史规律、时间换算和整合走势大图。
- **提醒合并**：提醒统一放在「我有号码」页。支持“当次快叫到我时”多段提醒，也支持“每日该取号了”提醒；提醒只推送通知，不会替你取号。
- **约未来**：可约日历支持门店多选、只看可预约、午餐/晚餐过滤、自动刷新，并可一键保存为抢号门店。
- **自动抢**：已放出的时段按偏好立即开抢；还没放出的日期/门店/时间窗可加入蹲号目标。添加目标、保存计划、启动蹲号都贴在同一个操作区。
- **通行证向导**：五步状态机（选设备 → 抓一次 → 传到电脑 → 粘贴解析 → 验证），粘贴步实时点亮字段捕获进度，中途关闭可断点续做。
- **健康胶囊**：右上角集中显示通行证、通知、预测数据三项前置条件；任何页面点开都能跳到对应修复入口。
- **安全分级**：界面明确标注「只读 · 直接用」「需要通行证」「会执行操作」。看排队和叫号预测不会影响账号；取号、抢预约、取消类动作会再次确认。
- **预测数据**：常用门店的公开排队曲线默认在本机记录，无需通行证；拿到通行证后可额外采集预约可用性。数据默认只留在本机。
- **线上基准**：支持通过自建 Cloudflare Worker + GitHub 登录读取 Turso 只读基准库；公开下载包默认不内置数据库 URL/token。
- **设置页瘦身**：通行证、通知、GitHub、预测、历史洞察、运行日志、安全维护按折叠区组织；低频/调试项默认收起。
- **寿司家族与背景视觉**：寿司宝宝从 8 只扩编到 12 只（三文鱼、金枪鱼、玉子烧、甜虾、鳗鱼、鱼子军舰、海苔卷、黄瓜卷、鲭鱼、章鱼、扇贝、海胆军舰），配合回转寿司传送带、价位盘配色和极淡芝麻点背景。
- **本机诊断**：检查证书信任、端口占用、代理状态、配置完整性和寿司郎网络连通性；可导出脱敏诊断包。
- **一键修复/卸载**：设置页可恢复代理、停止本应用相关进程；`uninstall` 可移除本地凭证、通知配置、历史、证书文件并尝试移除系统信任证书。
- **更新提醒**：Web UI 可检查 GitHub 最新 Release，提示可下载的新版本。

排障时可运行 `sushiro doctor`，或在 Web UI 服务启动后访问 `GET /api/diagnostics` 获取只读、脱敏的诊断信息。

---

## 数据文件

所有数据统一存放在 `~/.sushiro/` 目录：

| 文件 | 说明 |
|------|------|
| `config.json` | 凭证参数 |
| `preferences.json` | 用户偏好（人数/桌型/时段/优先级） |
| `notify.json` | 通知渠道配置 |
| `stores.json` | 门店昵称 |
| `sampling.json` | 信息收集配置 |
| `cloud_auth.json` | 云端数据配置：Cloudflare Worker URL 与 GitHub 登录后的应用 session，不含 Turso token |
| `holidays.json` | 可选节假日/调休工作日本地表，用于到店预测分类 |
| `history.jsonl` | 历史时段数据 |
| `queue_observations.jsonl` | 实时排队/公开叫号快照，本地私有 |
| `queue_sessions.jsonl` | 用户真实取号等待 session，本地私有 |
| `queue_stats.json` | 本地聚合后的排队统计缓存 |
| `netticket_plan.json` | 取号计划（目标门店/触发方式/时间/状态/号码） |
| `netticket_routine.json` | 每日取号提醒配置（门店/目标就餐时间/提前提醒分钟/状态） |
| `queue_baseline.json` / `queue_baseline.jsonl` | 全国门店基准采集开关与全量门店快照记录 |
| `api_discovery.json` / `api_discovery.jsonl` | 接口发现调试模式的开关配置与接口摘要记录 |
| `sushiro.log` | 后台模式日志 |
| `sampling.log` | 后台信息收集日志 |
| `sampling.pid` / `sampling.lock` | 后台信息收集进程状态与互斥锁 |
| `main_active.json` | 抢号/捕获/狙击运行标记，信息收集会自动避让 |

---

## 信息收集与到店预测

到店预测是纯本地能力，不上传数据。建议先选择自己常去或准备去的门店持续信息收集；每家店会单独计算趋势，预测只作为选择到店时间和预约优先级的参考。

- **叫号时间表**：「我有号码」会把本机记录和线上公开基准转成 10:00-22:00 的叫号表，回答「几点大概叫到多少号」。
- **我的号估算**：输入自己的排队号后，系统会找出最早能覆盖该号码的时间点，并提前 20 分钟给出到店建议。
- **门店参考**：当多个门店有实时排队数据时，会优先推荐营业中、可线上取号、前面桌数和等待分钟更低的门店。
- **实际过号数**：只在记录到 `called_no_when_user_called` 和 `display_called_no_at_take` 时计算，不用自己的号和当前叫号做简单推断。
- **全局过号数**：来自同一门店连续公开叫号快照的正向增量。
- **推荐时段**：优先参考等待更短、样本更多、过号风险更低的门店和时段；低样本会显示为继续观察。
- **节假日**：可在 `~/.sushiro/holidays.json` 写入 `{"holidays":["2026-05-01"],"workdays":["2026-05-09"]}`，未配置时只按自然工作日/周末分类。

## 工作原理

```
┌──────────┐     HTTPS (MITM)     ┌──────────────┐
│  PC 微信  │ ──────────────────→ │ 寿司郎服务器    │
│  小程序   │   ←── 本地代理 ──→   │              │
└──────────┘     捕获凭证参数      └──────────────┘
       │                                    │
       └── 捕获完成后，清理代理，直连抢号 ──┘
```

1. 启动本地 HTTPS 代理 (MITM)，只对寿司郎 API 域名做 TLS 解密，其他域名保持 CONNECT 透传
2. 设置系统代理（退出时自动恢复）
3. 捕获凭证参数后清理代理，直连 API 抢号
4. 后台每 5 分钟验证 Token 有效性

---

## 开发

代码分层说明见 [ARCHITECTURE.md](ARCHITECTURE.md)。

```bash
go build -o sushiro .       # 构建
go vet ./...                # 静态检查
./sushiro                   # 运行（Web UI，独立窗口优先）
./sushiro cli               # 运行（终端模式）
```

### 发布新版本

```bash
git tag vX.Y.Z          # 替换为新版本号，如 v3.1.3
git push origin vX.Y.Z
# GitHub Actions 自动构建所有平台并发布 Release
```

> 详细的架构文档和打包流程见 [AGENTS.md](AGENTS.md)。

## License

MIT
