package platform

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	. "github.com/Ryujoxys/sushiro-overdose/internal/core"
)

func killRelatedAppProcessesByPGrep(excludePID int) []MaintenanceResult {
	out, err := exec.Command("pgrep", "-f", "sushiro|Sushiro").Output()
	if err != nil {
		return []MaintenanceResult{{
			Name:   "related_processes",
			Action: "kill_by_name",
			Status: MaintenanceStatusMissing,
		}}
	}
	results := []MaintenanceResult{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || pid <= 0 || pid == excludePID {
			continue
		}
		result := MaintenanceResult{
			Name:   "related_process",
			Action: "kill_by_name",
			Status: MaintenanceStatusOK,
			Error:  fmt.Sprintf("pid %d", pid),
		}
		if err := KillProcess(pid); err != nil {
			result.Status = MaintenanceStatusError
			result.Error = fmt.Sprintf("pid %d: %s", pid, err.Error())
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		results = append(results, MaintenanceResult{
			Name:   "related_processes",
			Action: "kill_by_name",
			Status: MaintenanceStatusMissing,
		})
	}
	return results
}

type relatedProcessKillResult struct {
	PID    int    `json:"pid"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

func parseRelatedProcessKillOutput(out string) []MaintenanceResult {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	results := []MaintenanceResult{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item relatedProcessKillResult
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			results = append(results, MaintenanceResult{
				Name:   "related_process",
				Action: "kill_by_name",
				Status: MaintenanceStatusError,
				Error:  err.Error() + ": " + line,
			})
			continue
		}
		status := MaintenanceStatusOK
		if item.Status == MaintenanceStatusError || item.Status == "error" {
			status = MaintenanceStatusError
		}
		name := item.Name
		if name == "" {
			name = "related_process"
		}
		result := MaintenanceResult{
			Name:   name,
			Action: "kill_by_name",
			Path:   item.Path,
			Status: status,
			Error:  fmt.Sprintf("pid %d", item.PID),
		}
		if item.Error != "" {
			result.Error += ": " + item.Error
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		results = append(results, MaintenanceResult{
			Name:   "related_processes",
			Action: "kill_by_name",
			Status: MaintenanceStatusMissing,
		})
	}
	return results
}
