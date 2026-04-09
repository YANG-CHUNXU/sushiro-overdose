# SUSHIRO 寿司郎重度依赖

全自动抢号的寿司郎预约工具。通过本地 MITM 代理捕获微信小程序认证参数，然后自动轮询预约目标时段。

零外部依赖，纯 Go 标准库实现。支持 macOS / Windows / Linux。

---

## 安装

### macOS

**一键安装（推荐）：**

```bash
curl -sSL https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install.sh | bash
```

**手动安装：**

```bash
# 从 GitHub Releases 下载最新版
# https://github.com/Ryujoxys/sushiro-overdose/releases

# 解压并安装
tar xzf sushiro-overdose_*_darwin_arm64.tar.gz
sudo cp sushiro-overdose /usr/local/bin/
```

**从源码构建：**

```bash
git clone https://github.com/Ryujoxys/sushiro-overdose.git
cd sushiro-overdose
go build -o sushiro-overdose .
sudo cp sushiro-overdose /usr/local/bin/
```

### Windows

**一键安装（PowerShell）：**

```powershell
irm https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install.ps1 | iex
```

> 安装后**重启终端**，`sushiro-overdose` 命令即可使用。

**手动安装：**

1. 从 [GitHub Releases](https://github.com/Ryujoxys/sushiro-overdose/releases) 下载 `sushiro-overdose_*_windows_amd64.zip`
2. 解压到任意目录（如 `%APPDATA%\sushiro\`）
3. 将该目录加入系统 PATH

### Linux

**一键安装：**

```bash
curl -sSL https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install.sh | bash
```

**从源码构建：**

```bash
git clone https://github.com/Ryujoxys/sushiro-overdose.git
cd sushiro-overdose
go build -o sushiro-overdose .
sudo cp sushiro-overdose /usr/local/bin/
```

---

## 使用教程

### 第一次使用

```
1. 运行 sushiro-overdose → 安装 CA 证书（首次，需要输入密码确认）
2. PC 微信打开寿司郎小程序 → 浏览门店 → 随便进一个预约/排队页面
3. 程序自动捕获认证参数 → 选择门店 → 配置时段偏好
4. 开始自动抢号 → 成功后弹出通知
```

#### macOS 用户

1. 打开终端，运行 `sushiro-overdose`
2. 首次运行会提示安装 CA 证书，输入登录密码确认
3. 程序会自动设置系统代理（退出时自动恢复）
4. 打开 **PC 微信** → 搜索「寿司郎」小程序 → 打开
5. 在小程序中浏览门店、查看预约页面
6. 终端会实时显示捕获进度，全部 ✅ 后自动进入下一步
7. 选择门店 → 配置工作日/周末时段偏好 → 开始抢号

#### Windows 用户

1. 打开 PowerShell 或 CMD，运行 `sushiro-overdose`
2. 首次运行会提示安装 CA 证书（弹窗确认）
3. 程序会自动设置系统代理（退出时自动恢复）
4. 打开 **PC 微信** → 搜索「寿司郎」小程序 → 打开
5. 在小程序中浏览门店、查看预约页面
6. 终端显示捕获进度，全部 ✅ 后自动进入下一步
7. 选择门店 → 配置时段 → 开始抢号

#### Linux 用户

1. 打开终端，运行 `sushiro-overdose`
2. 首次运行会安装 CA 证书到系统信任链（可能需要 sudo）
3. 打开 **PC 微信** → 操作同上
4. 如果使用桌面环境，会发送桌面通知

> **注意：** 必须使用 **PC 版微信** 中的小程序，手机端无效。

### 日常使用

认证参数捕获后会自动保存，下次运行无需重复捕获（过期时会自动提示重新获取）。

```
sushiro-overdose              # 前台交互模式（默认）
sushiro-overdose start        # 后台静默运行
sushiro-overdose status       # 查看运行状态和最近日志
sushiro-overdose exit         # 停止后台进程
```

---

## 全部命令

```
sushiro-overdose              前台交互模式（默认）
sushiro-overdose start        后台静默运行
sushiro-overdose status       查看运行状态
sushiro-overdose exit         停止后台进程

sushiro-overdose calendar     查看近 7 天可预约时段
sushiro-overdose sniper       狙击模式 - 提前锁定未开放时段
sushiro-overdose list         查看当前预约
sushiro-overdose cancel <id>  取消预约

sushiro-overdose web          启动 Web UI（浏览器操作）
sushiro-overdose trends       分析时段可用率趋势
sushiro-overdose recommend    智能推荐最佳时段

sushiro-overdose config                          查看和配置通知（交互式）
sushiro-overdose config feishu <webhook>         配置飞书通知
sushiro-overdose config telegram <token> <id>    配置 Telegram 通知
sushiro-overdose config bark <url> <key>         配置 Bark 推送
sushiro-overdose config serverchan <key>         配置 Server酱
sushiro-overdose config store                    查看门店昵称
sushiro-overdose config store add <id> <name>    添加门店昵称
sushiro-overdose config store remove <id>        移除门店昵称
```

---

## 功能详解

### 日历视图

```bash
sushiro-overdose calendar
```

查看近 7 天所有可预约时段，包括网格概览和详细列表。

### 狙击模式

```bash
# 交互式选择
sushiro-overdose sniper

# 命令行快速指定
sushiro-overdose sniper --date 20260503 --time 1930-2030 --store 12345
```

寿司郎提前 30 天开放预约，且严格对应开放时间。狙击模式会在开放时刻精确发起请求，50ms 高速轮询抢号。

### Web UI

```bash
sushiro-overdose web
```

自动打开浏览器，可视化查看日历、预约、配置通知。

### 智能推荐

```bash
sushiro-overdose trends       # 查看各时段可用率
sushiro-overdose recommend    # 基于历史数据推荐最优时段
```

程序自动记录每次查询的时段数据，积累后可分析哪些时段最容易抢到。

### 门店昵称

```bash
sushiro-overdose config store add 12345 "家门口那家"
sushiro-overdose config store add 67890 "公司旁边"
```

设置后，日历、狙击、选择门店等所有界面都会显示昵称。

---

## 通知配置

支持多个通知渠道，可同时启用：

### 飞书

1. 打开飞书群聊 → 群设置 → **群机器人** → **添加机器人** → **自定义机器人**
2. 复制 Webhook 地址
3. `sushiro-overdose config feishu https://open.feishu.cn/open-apis/bot/v2/hook/xxx`

### Telegram

1. 和 [@BotFather](https://t.me/BotFather) 创建 Bot，获取 Token
2. 给 Bot 发一条消息，然后访问 `https://api.telegram.org/bot<TOKEN>/getUpdates` 获取 Chat ID
3. `sushiro-overdose config telegram <bot_token> <chat_id>`

### Bark (iOS)

1. App Store 安装 Bark
2. `sushiro-overdose config bark https://api.day.app <device_key>`

### Server酱

1. [sct.ftqq.com](https://sct.ftqq.com) 登录获取 SendKey
2. `sushiro-overdose config serverchan <send_key>`

---

## 时段配置

启动时可配置每种日期的目标时段：

- **19:30-20:30** — 晚饭黄金时段
- **20:00 前** — 任意 20 点前的时段
- **10:30-13:00** — 午饭时段
- **不预约** — 跳过该日期类型

分别对工作日、周六、周日进行配置。

---

## 工作原理

```
┌──────────┐     HTTPS (MITM)     ┌──────────────┐
│  PC 微信  │ ──────────────────→ │ 寿司郎服务器    │
│  小程序   │   ←── 本地代理 8080 ──→ │              │
└──────────┘     捕获认证参数      └──────────────┘
       │                                    │
       └── 捕获完成后，清理代理，直连抢号 ──┘
```

1. 启动本地 HTTPS 代理 (MITM)
2. 设置系统代理指向本地
3. 仅拦截寿司郎域名流量，其他流量直接放行
4. 捕获 X-App-Code、Authorization 等认证参数
5. 捕获完成后清理代理，直连 API 抢号
6. 后台每 5 分钟验证 Token 有效性

---

## 文件结构

```
main.go                   入口，CLI 命令分发，抢号循环
api.go                    HTTP API 客户端
booking.go                预约管理（list/cancel）
calendar.go               日历视图
cert.go                   CA 证书生成
config.go                 Settings 配置
health.go                 Token 健康检测
history.go                历史数据记录 + 趋势分析
notifier.go               通知接口
notifier_feishu.go        飞书通知
notifier_telegram.go      Telegram 通知
notifier_bark.go          Bark 推送
notifier_serverchan.go    Server酱通知
notify.go                 通知工具函数
platform.go               跨平台抽象
platform_darwin.go        macOS 实现
platform_windows.go       Windows 实现
platform_linux.go         Linux 实现
proxy.go                  MITM 代理，参数捕获
recommend.go              智能推荐
slot.go                   时段工具函数
sniper.go                 狙击模式
state.go                  状态持久化
store.go                  门店昵称管理
watchdog.go               代理清理安全网
web.go                    Web UI 服务器
web_handlers.go           Web API 处理
web_static.go             内嵌前端
```

---

## 数据文件

| 文件 | 说明 |
|------|------|
| `.sushiro_local.json` | 认证参数（当前目录） |
| `.sushiro_state.json` | 预约状态（当前目录） |
| `~/.sushiro/feishu.json` | 飞书通知配置 |
| `~/.sushiro/notify.json` | 全部通知渠道配置 |
| `~/.sushiro/stores.json` | 门店昵称 |
| `~/.sushiro/history.jsonl` | 历史时段数据 |
| `~/.sushiro/sushiro.log` | 后台模式日志 |
| `~/.sushiro/sushiro.pid` | 后台进程 PID |
| `~/.sushiro-proxy/` | CA 证书目录 |

---

## 注意事项

- 程序会修改系统代理设置，**退出时自动恢复**。如果异常退出，下次启动会自动清除残留代理
- CA 证书存储在 `~/.sushiro-proxy/`（macOS/Linux）或 `%APPDATA%\sushiro-proxy\`（Windows）
- 认证参数会过期，过期后自动提示重新捕获
- 抢号成功后程序自动退出
- 后台模式日志存储在 `~/.sushiro/sushiro.log`
- **必须使用 PC 微信小程序**，手机端无法捕获

## 开发

```bash
go build -o sushiro-overdose .                              # 本地构建
GOOS=windows GOARCH=amd64 go build -o sushiro.exe .         # Windows
GOOS=linux GOARCH=amd64 go build -o sushiro .               # Linux

# 发布新版本
git tag v1.1.0
git push origin v1.1.0
# GitHub Actions 自动构建并发布 Release
```

## License

MIT
