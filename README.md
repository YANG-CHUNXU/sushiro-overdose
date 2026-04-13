# SUSHIRO 寿司郎 Overdose

全自动抢号的寿司郎预约工具。通过本地 MITM 代理捕获微信小程序认证参数，然后自动轮询预约目标时段。

零外部依赖，纯 Go 标准库实现。支持 macOS / Windows / Linux。

**双击运行，浏览器操作，不需要任何编程知识。**

---

## 下载安装

从 [GitHub Releases](https://github.com/Ryujoxys/sushiro-overdose/releases) 下载最新版本：

| 平台 | 下载文件 | 使用方式 |
|------|---------|---------|
| **macOS** | `Sushiro Overdose-*-macOS.zip` | 解压得到 .app，双击运行 |
| **Windows** | `sushiro-overdose_*_windows_amd64.zip` | 解压得到 .exe，双击运行 |
| **Linux** | `sushiro-overdose_*_linux_amd64.tar.gz` | 解压后终端运行 |

### 从源码构建

```bash
git clone https://github.com/Ryujoxys/sushiro-overdose.git
cd sushiro-overdose
go build -o sushiro-overdose .
```

---

## 使用教程

### 第一次使用

1. **运行程序** → 自动打开浏览器
2. **设置向导** → 安装证书（按提示确认即可）
3. **捕获参数** → 在 PC 微信中打开寿司郎小程序，进行一次排队/预约操作
4. **设置偏好** → 选择人数、桌型、目标时段
5. **开始抢号** → 点击「开始抢号」按钮

> **注意：** 必须使用 **PC 版微信** 中的小程序，手机端无效。

### 日常使用

认证参数捕获后自动保存，下次运行无需重复捕获（过期时会自动提示）。

打开程序 → 点击「开始抢号」→ 成功后自动通知。

---

## 全部命令

程序默认启动 Web UI。高级用户可使用命令行：

```
sushiro-overdose                 启动 Web UI（默认，推荐）
sushiro-overdose cli             终端交互模式（高级）
sushiro-overdose start           后台静默运行
sushiro-overdose status          查看运行状态
sushiro-overdose exit            停止后台进程

sushiro-overdose calendar        查看近 7 天可预约时段
sushiro-overdose sniper          狙击模式 - 提前锁定未开放时段
sushiro-overdose list            查看当前预约
sushiro-overdose cancel <id>     取消预约

sushiro-overdose trends          分析时段可用率趋势
sushiro-overdose recommend       智能推荐最佳时段

sushiro-overdose config                          查看通知配置
sushiro-overdose config feishu <webhook>         配置飞书通知
sushiro-overdose config telegram <token> <id>    配置 Telegram
sushiro-overdose config bark <url> <key>         配置 Bark
sushiro-overdose config serverchan <key>         配置 Server酱
sushiro-overdose config store add <id> <name>    添加门店昵称
```

---

## 通知配置

支持多个通知渠道，可在 Web UI 的「设置」页面配置，也可用命令行：

- **飞书** — 群机器人 Webhook
- **Telegram** — Bot Token + Chat ID
- **Bark** — iOS 推送
- **Server酱** — 微信推送

---

## 数据文件

所有数据统一存放在 `~/.sushiro/` 目录：

| 文件 | 说明 |
|------|------|
| `config.json` | 认证参数 |
| `preferences.json` | 用户偏好（人数/桌型/时段） |
| `notify.json` | 通知渠道配置 |
| `stores.json` | 门店昵称 |
| `history.jsonl` | 历史时段数据 |
| `sushiro.log` | 后台模式日志 |

---

## 工作原理

```
┌──────────┐     HTTPS (MITM)     ┌──────────────┐
│  PC 微信  │ ──────────────────→ │ 寿司郎服务器    │
│  小程序   │   ←── 本地代理 ──→   │              │
└──────────┘     捕获认证参数      └──────────────┘
       │                                    │
       └── 捕获完成后，清理代理，直连抢号 ──┘
```

1. 启动本地 HTTPS 代理 (MITM)，仅拦截寿司郎域名
2. 设置系统代理（退出时自动恢复）
3. 捕获认证参数后清理代理，直连 API 抢号
4. 后台每 5 分钟验证 Token 有效性

---

## 开发

```bash
go build -o sushiro-overdose .       # 构建
go vet ./...                         # 静态检查
./sushiro-overdose                   # 运行（Web UI）
./sushiro-overdose cli               # 运行（终端模式）
```

### 发布新版本

```bash
git tag v1.2.0
git push origin v1.2.0
# GitHub Actions 自动构建所有平台并发布 Release
```

> 详细的架构文档和打包流程见 [AGENTS.md](AGENTS.md)。

## License

MIT
