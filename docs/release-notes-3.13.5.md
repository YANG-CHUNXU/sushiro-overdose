# v3.13.5

## fix: 换掉 Stream 推荐——改推 Reqable（抓得到 POST）

用户反馈 Stream 抓不到提交请求（POST body），拿不全凭证（缺微信ID/手机号）。

### 根因

Stream 不是"只抓 GET"，而是 HTTPS 解密 + 微信自有网络栈的兼容性问题——POST body 经常抓不全。
寿司郎小程序走标准 HTTPS（非 mmtls），换对工具就能稳定抓 POST。

### 改动（仅文案，不改后端）

- **iOS 分支**：Stream → **Reqable**（推荐，免费社区版够用），备选 HTTP Catcher。明确提示
  "别用 Stream——它抓微信小程序的提交请求（POST）经常抓不全"
- **安卓分支**：统一推 **Reqable**，备选 HttpCanary，提示安卓 7+ 需把 CA 装系统证书（Magisk）
- **第 3 步导出**：Stream 专属操作 → Reqable/HttpCanary 通用"长按 → 复制为 cURL"，加 HAR 备选

### 为什么不用改 parser

我们导入器已支持 cURL（`-d` body）/ JSON / 原始请求头，Reqable/HttpCanary/HTTP Catcher 都能
导出 cURL（含 POST body），直接喂进去。换工具即可，后端零改动。

> 仅改前端文案（web_static.go），逻辑无改动。
