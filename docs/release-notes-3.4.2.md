# Release notes: v3.4.2

3.4.2 是一个修 bug 的小版本：堵住两个会导致数据写坏 / 重复取号的并发与重试问题，并补上几处一致性清理。

## 变化

### 排队观测写入串行化（防数据静默损坏）

- `queue_observations.jsonl` 现在用互斥锁串行化追加写。之前本机采样、线上基准采集等多个 goroutine 并发写入，会让 JSON 行交错 / 截断，下游逐行 `json.Unmarshal` 时静默丢观测数据，图表和预测会悄悄变空。
- `queue_baseline.jsonl` 的批量追加同样加了锁，避免将来出现第二个写者时踩同一个坑。
- 写法对齐既有的 `history.jsonl`（`historyMu`），三处本地 JSONL 现在都有写锁保护。

### 定时取号不再在服务端 5xx 时刷号

- 官方接口返回临时错误（5xx 等）时，定时取号之前会清掉当天占位、每 20 秒重发一次取号请求——服务器只要持续抖动，整个取号窗口内会反复提交，有刷号 / 重复取号风险。
- 现在限制为最多 `netTicketMaxServerRetries`（6）次连续重试，超过即转 `error`、保留当天占位不再触发，并推一条「已停止重试，可稍后手动取号」的通知。
- 新增持久化的 `server_retry_count` 字段，跨天后自动清零，不累积前一天的失败。

### 凭证健康监测一致性

- 抢号 / 狙击在 `GetTimeslots` 返回「需要刷新凭证」类错误时，现在也会喂给凭证健康监测（之前只有 `CreateReservation` 失败才喂）。这样只通过查时段暴露出来的软过期，也能正确触发 stale 提醒。

### 清理

- 删掉 `netticket_routine.go` 里无意义的 `strconvItoa` 包装，直接用 `strconv.Itoa`。

## 验证

- `gofmt` / `go vet ./...` / `go build ./...` 通过
- `go test ./...` 全绿，新增两条 `ServerRetryCount` 持久化与跨天清零的针对性测试
