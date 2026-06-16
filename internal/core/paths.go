package core

import (
	"os"
	"path/filepath"
)

// 路径约定：所有运行期数据统一放在用户主目录下的 ~/.sushiro（点开头表示隐藏目录）。
// 这样无论从哪个 CWD 启动都能找到同一份数据（区别于早期把文件散落在当前目录的写法）。
// 之所以不用 XDG/平台标准目录，是为了单二进制、零依赖、跨平台一致。
const (
	appDir      = ".sushiro"
	pidFile     = "sushiro.pid"
	logFileName = "sushiro.log"
)

// AppDirPath 返回本应用的数据目录（~/.sushiro）。os.UserHomeDir 失败时 home 为空，
// 会退化成相对路径 ".sushiro"（极少见，主要在 HOME 没设置的环境）。
func AppDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, appDir)
}

// PidFilePath 返回守护进程 PID 文件路径（~/.sushiro/sushiro.pid），用于单实例与进程管理。
func PidFilePath() string { return filepath.Join(AppDirPath(), pidFile) }

// StateFilePath 返回运行态文件路径（~/.sushiro/.sushiro_state.json）。
// 注意：这是「抓包/取号」链路的默认 state 路径；config.go 里 LoadSettings 的 state 文件
// 默认在 config.json 同目录，二者可能不同（手动配置 vs 默认）。
func StateFilePath() string { return filepath.Join(AppDirPath(), ".sushiro_state.json") }

// LogPath 返回运行日志文件路径（~/.sushiro/sushiro.log）。
func LogPath() string { return filepath.Join(AppDirPath(), logFileName) }
