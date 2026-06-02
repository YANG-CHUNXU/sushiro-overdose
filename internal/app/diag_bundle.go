package app

import . "github.com/Ryujoxys/sushiro-overdose/internal/proxy"

import . "github.com/Ryujoxys/sushiro-overdose/internal/core"

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const diagBundleLogLines = 2000

// buildDiagBundle assembles a zip of evidence files (diagnostics JSON, sanitized
// logs, PAC fetch result, and on Windows the WeChat process + cert store dump)
// and returns the zip bytes plus a suggested filename. The bundle is meant to
// be sent back by a failing user so we can root-cause without guessing.
func buildDiagBundle() ([]byte, string, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	add := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			return
		}
		_, _ = io.WriteString(w, content)
	}

	add("README.txt", diagBundleReadme())

	diag := CollectDiagnostics()
	if data, err := json.MarshalIndent(diag, "", "  "); err == nil {
		add("diagnostics.json", string(data))
	} else {
		add("diagnostics.json.error", err.Error())
	}

	add("engine-log.txt", renderEngineLogs(engine.GetLogs()))
	add("sushiro-log.txt", readSanitizedFile(LogPath(), diagBundleLogLines))
	add("sampling-log.txt", readSanitizedFile(SamplingLogPath(), diagBundleLogLines))

	add("pac-fetch.txt", probeLocalPAC())

	if runtime.GOOS == "windows" {
		add("wechat-processes.txt", collectWeChatProcessInfo())
		thumb, _ := LocalCACertSHA1Thumbprint()
		add("cert-stores-raw.txt", collectWindowsCertStoreDump(thumb))
	}

	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("sushiro-diag-%s.zip", time.Now().Format("20060102-150405"))
	return buf.Bytes(), name, nil
}

func diagBundleReadme() string {
	return strings.Join([]string{
		"sushiro-overdose 诊断包",
		"",
		"复现步骤建议：",
		"  1) 运行本程序，进入 Web UI 等捕获页面停留 30 秒。",
		"  2) 完全退出 PC 微信（任务管理器里 kill WeChat.exe / WeChatAppEx.exe）。",
		"  3) 重新打开 PC 微信，进入寿司郎小程序并尝试登录/点一次排队。",
		"  4) 等失败提示出现后，立即生成本诊断包并发送。",
		"",
		"文件说明：",
		"  diagnostics.json     完整只读诊断（证书、端口、系统代理、网络、配置）",
		"  engine-log.txt       运行期引擎日志（含每个 CONNECT 命中的域名 + MITM/透传 模式）",
		"  sushiro-log.txt      程序文件日志尾部（脱敏）",
		"  sampling-log.txt     后台信息收集日志尾部（脱敏，可能不存在）",
		"  pac-fetch.txt        本地 PAC URL 抓取结果（验证 PAC 是否提供给微信）",
		"  wechat-processes.txt 当前运行的 WeChat 进程版本与启动时间（Windows）",
		"  cert-stores-raw.txt  本机 Root/Disallowed 证书存储中我们的指纹原始记录（Windows）",
		"",
		"诊断包内容只做脱敏后的本机证据收集，不上传任何认证 token、Cookie 或预约数据。",
	}, "\n") + "\n"
}

func renderEngineLogs(entries []LogEntry) string {
	if len(entries) == 0 {
		return "(engine log empty — 引擎还未启动或未捕获到任何请求)\n"
	}
	var b strings.Builder
	for _, e := range entries {
		level := e.Level
		if level == "" {
			level = "info"
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", e.Time, level, SanitizeDiagnosticLine(e.Message))
	}
	return b.String()
}

func readSanitizedFile(path string, maxLines int) string {
	if path == "" {
		return "(no path)\n"
	}
	lines, err := readSanitizedLogTail(path, maxLines)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("(missing: %s)\n", path)
		}
		return fmt.Sprintf("(read error: %v)\n", err)
	}
	if len(lines) == 0 {
		return "(empty)\n"
	}
	return strings.Join(lines, "\n") + "\n"
}

func probeLocalPAC() string {
	url := fmt.Sprintf("http://127.0.0.1:%d/proxy.pac", defaultWebPort)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "request build failed: " + err.Error() + "\n"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("GET %s\nerror: %v\n", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	return fmt.Sprintf("GET %s\nstatus: %d %s\ncontent-type: %s\nbody:\n%s\n",
		url, resp.StatusCode, resp.Status, resp.Header.Get("Content-Type"), string(body))
}

func collectWeChatProcessInfo() string {
	script := `
$procs = Get-Process | Where-Object { $_.ProcessName -match '^(WeChat|WeChatAppEx|Weixin|WeChatPlayer)$' }
if (-not $procs) { Write-Output "(no WeChat processes running)"; return }
foreach ($p in $procs) {
  $v = ""
  try { $v = $p.MainModule.FileVersionInfo.FileVersion } catch {}
  $path = ""
  try { $path = $p.MainModule.FileName } catch {}
  Write-Output ("Name={0} PID={1} StartTime={2} FileVersion={3} Path={4}" -f $p.ProcessName, $p.Id, $p.StartTime, $v, $path)
}
`
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("powershell error: %v\n%s\n", err, strings.TrimSpace(string(out)))
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return "(no WeChat processes running)\n"
	}
	return text + "\n"
}

func collectWindowsCertStoreDump(thumbprint string) string {
	if thumbprint == "" {
		return "(no thumbprint available — CA may not be generated yet)\n"
	}
	script := `
$thumb = $args[0].ToUpperInvariant()
$stores = @(
  'Cert:\CurrentUser\Root',
  'Cert:\LocalMachine\Root',
  'Cert:\CurrentUser\Disallowed',
  'Cert:\LocalMachine\Disallowed',
  'Cert:\CurrentUser\CA',
  'Cert:\LocalMachine\CA'
)
foreach ($s in $stores) {
  $items = Get-ChildItem -Path $s -ErrorAction SilentlyContinue | Where-Object { $_.Thumbprint -eq $thumb }
  if ($items) {
    foreach ($i in $items) {
      Write-Output ("STORE={0} Subject={1} NotBefore={2} NotAfter={3} HasPrivateKey={4}" -f $s, $i.Subject, $i.NotBefore, $i.NotAfter, $i.HasPrivateKey)
    }
  } else {
    Write-Output ("STORE={0} (not present)" -f $s)
  }
}
`
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script, thumbprint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("powershell error: %v\n%s\n", err, strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("Thumbprint: %s\n%s\n", thumbprint, strings.TrimSpace(string(out)))
}

func cmdDiagBundle() {
	data, name, err := buildDiagBundle()
	if err != nil {
		fmt.Fprintln(os.Stderr, "诊断包生成失败:", err)
		os.Exit(1)
	}
	outDir := AppDirPath()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "无法创建输出目录:", err)
		os.Exit(1)
	}
	outPath := filepath.Join(outDir, name)
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "写入诊断包失败:", err)
		os.Exit(1)
	}
	fmt.Println("诊断包已生成:", outPath)
	fmt.Println("请把这个文件发回给维护者，便于定位 Windows 捕获失败原因。")
}

func handleDiagBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	data, name, err := buildDiagBundle()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "诊断包生成失败: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}
