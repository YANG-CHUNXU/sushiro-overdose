# SUSHIRO 寿司郎重度依赖

全自动抢号的寿司郎预约工具。通过本地 MITM 代理捕获微信小程序认证参数，然后自动轮询预约目标时段。

零外部依赖，纯 Go 标准库实现。

## 安装

```bash
git clone https://github.com/Ryujoxys/sushiro.git
cd sushiro
go build -o sushiro .
sudo cp sushiro /usr/local/bin/
```

安装后 `sushiro` 命令全局可用。

## 命令

```
sushiro              # 前台交互模式（默认）
sushiro start        # 后台静默运行
sushiro status       # 查看运行状态和日志
sushiro exit         # 停止后台进程
sushiro setting      # 配置飞书通知等设置（交互式）
sushiro config feishu <webhook_url>   # 直接配置飞书通知
sushiro config feishu --clear         # 清除飞书通知
```

## 使用流程

```
1. 运行 sushiro → 安装 CA 证书（首次）→ 启动本地代理
2. PC 微信打开寿司郎小程序 → 浏览门店和预约页面
3. 程序自动捕获认证参数 → 选择门店 → 配置时段偏好
4. 自动抢号 → 成功后通知（macOS 通知 + 飞书）
```

## 首次运行

首次运行需要安装 CA 证书到 macOS 系统信任链（用于拦截 HTTPS 流量），程序会提示输入登录密码。只需一次。

## 飞书通知配置（可选）

运行时程序会询问是否配置飞书通知。也可以手动配置：

1. 打开飞书群聊 → 群设置 → **群机器人** → **添加机器人** → **自定义机器人**
2. 复制 Webhook 地址
3. 运行 `sushiro config feishu https://open.feishu.cn/open-apis/bot/v2/hook/xxx`

预约成功或认证过期时会自动发送飞书通知。

## 时段配置

启动时可配置每种日期的目标时段：

- **19:30-20:30** — 晚饭黄金时段
- **20:00 前** — 任意 20 点前的时段
- **10:30-13:00** — 午饭时段
- **不预约** — 跳过该日期类型

分别对工作日、周六、周日进行配置。

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
2. 设置 macOS 系统代理指向本地
3. 仅拦截寿司郎域名流量，其他流量直接放行
4. 捕获 X-App-Code、Authorization 等认证参数
5. 捕获完成后清理代理，直连 API 抢号

## 文件结构

```
main.go       入口，CLI 命令分发，抢号循环
cert.go       CA 证书生成和 macOS 信任安装
proxy.go      MITM 代理，参数捕获，时段配置
api.go        HTTP API 客户端
config.go     Settings 配置
notify.go     通知（飞书卡片）
slot.go       时段工具函数
state.go      状态持久化
```

## 注意事项

- 程序会修改 macOS 系统代理设置，退出时自动恢复
- CA 证书存储在 `~/.sushiro-proxy/`
- 认证参数保存在 `.sushiro_local.json`，过期后自动提示重新捕获
- 抢号成功后程序自动退出
- 后台模式日志存储在 `~/.sushiro/sushiro.log`
- 开机自启：已配置 launchd (`~/Library/LaunchAgents/com.ryujo.sushiro.plist`)
