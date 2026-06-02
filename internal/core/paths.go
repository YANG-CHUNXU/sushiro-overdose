package core

import (
	"os"
	"path/filepath"
)

const (
	appDir      = ".sushiro"
	pidFile     = "sushiro.pid"
	logFileName = "sushiro.log"
)

// AppDirPath 返回本应用的数据目录（~/.sushiro）。
func AppDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, appDir)
}

func PidFilePath() string   { return filepath.Join(AppDirPath(), pidFile) }
func StateFilePath() string { return filepath.Join(AppDirPath(), ".sushiro_state.json") }
func LogPath() string       { return filepath.Join(AppDirPath(), logFileName) }
