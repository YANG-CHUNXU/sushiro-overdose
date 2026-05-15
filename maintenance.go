package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maintenanceStatusOK          = "ok"
	maintenanceStatusMissing     = "missing"
	maintenanceStatusError       = "error"
	maintenanceStatusSkipped     = "skipped"
	maintenanceStatusWouldRemove = "would_remove"
)

type MaintenanceReport struct {
	Action  string              `json:"action"`
	OK      bool                `json:"ok"`
	Results []MaintenanceResult `json:"results"`
}

type MaintenanceResult struct {
	Name   string `json:"name"`
	Action string `json:"action"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type UninstallOptions struct {
	All          bool `json:"all"`
	Config       bool `json:"config"`
	Notify       bool `json:"notify"`
	Feishu       bool `json:"feishu"`
	Preferences  bool `json:"preferences"`
	Stores       bool `json:"stores"`
	State        bool `json:"state"`
	History      bool `json:"history"`
	PID          bool `json:"pid"`
	ProxyMarker  bool `json:"proxy_marker"`
	Certificates bool `json:"certificates"`
	SystemCert   bool `json:"system_cert"`
	DryRun       bool `json:"dry_run"`
}

type maintenanceTarget struct {
	name     string
	path     string
	selected bool
}

func RepairProxy() MaintenanceReport {
	report := MaintenanceReport{Action: "repair_proxy"}

	if err := ClearSystemProxy(); err != nil {
		report.Results = append(report.Results, MaintenanceResult{
			Name:   "system_proxy",
			Action: "clear_system_proxy",
			Status: maintenanceStatusError,
			Error:  err.Error(),
		})
	} else {
		report.Results = append(report.Results, MaintenanceResult{
			Name:   "system_proxy",
			Action: "clear_system_proxy",
			Status: maintenanceStatusOK,
		})
	}

	markProxyInactive()
	markerPath := proxyStatePath()
	markerResult := MaintenanceResult{
		Name:   "proxy_marker",
		Action: "remove_file",
		Path:   markerPath,
		Status: maintenanceStatusOK,
	}
	if _, err := os.Stat(markerPath); err == nil {
		markerResult.Status = maintenanceStatusError
		markerResult.Error = "proxy marker still exists after cleanup"
	} else if !os.IsNotExist(err) {
		markerResult.Status = maintenanceStatusError
		markerResult.Error = err.Error()
	}
	report.Results = append(report.Results, markerResult)

	report.OK = maintenanceReportOK(report.Results)
	return report
}

func UninstallLocalData(options UninstallOptions) MaintenanceReport {
	report := MaintenanceReport{Action: "uninstall_local_data"}
	targets := uninstallTargets(options)

	selected := 0
	for _, target := range targets {
		if !target.selected {
			continue
		}
		selected++
		report.Results = append(report.Results, removeMaintenanceFile(target.name, target.path, options.DryRun))
	}
	if options.SystemCert {
		selected++
		report.Results = append(report.Results, uninstallSystemCertificate(options.DryRun))
	}
	if selected == 0 {
		report.Results = append(report.Results, MaintenanceResult{
			Name:   "selection",
			Action: "remove_file",
			Status: maintenanceStatusSkipped,
			Error:  "no local data item selected",
		})
	}

	report.OK = maintenanceReportOK(report.Results)
	return report
}

func uninstallSystemCertificate(dryRun bool) MaintenanceResult {
	result := MaintenanceResult{
		Name:   "system_certificate",
		Action: "uninstall_cert",
	}
	if dryRun {
		result.Status = maintenanceStatusWouldRemove
		return result
	}
	if err := UninstallCert(); err != nil {
		result.Status = maintenanceStatusError
		result.Error = err.Error()
		return result
	}
	result.Status = maintenanceStatusOK
	return result
}

func uninstallTargets(options UninstallOptions) []maintenanceTarget {
	certDir := certDirPath()
	return []maintenanceTarget{
		{name: "config", path: localConfigPath(), selected: options.All || options.Config},
		{name: "notify", path: notifyConfigPath(), selected: options.All || options.Notify},
		{name: "feishu", path: feishuConfigPath(), selected: options.All || options.Feishu},
		{name: "preferences", path: preferencesPath(), selected: options.All || options.Preferences},
		{name: "stores", path: storeRegistryPath(), selected: options.All || options.Stores},
		{name: "state", path: stateFilePath(), selected: options.All || options.State},
		{name: "history", path: historyPath(), selected: options.All || options.History},
		{name: "pid", path: pidFilePath(), selected: options.All || options.PID},
		{name: "proxy_marker", path: proxyStatePath(), selected: options.All || options.ProxyMarker},
		{name: "ca_cert", path: filepath.Join(certDir, "ca.crt"), selected: options.All || options.Certificates},
		{name: "ca_key", path: filepath.Join(certDir, "ca.key"), selected: options.All || options.Certificates},
	}
}

func removeMaintenanceFile(name, path string, dryRun bool) MaintenanceResult {
	result := MaintenanceResult{
		Name:   name,
		Action: "remove_file",
		Path:   path,
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			result.Status = maintenanceStatusMissing
			return result
		}
		result.Status = maintenanceStatusError
		result.Error = err.Error()
		return result
	}

	if dryRun {
		result.Status = maintenanceStatusWouldRemove
		return result
	}

	if err := os.Remove(path); err != nil {
		result.Status = maintenanceStatusError
		result.Error = err.Error()
		return result
	}
	result.Status = maintenanceStatusOK
	return result
}

func maintenanceReportOK(results []MaintenanceResult) bool {
	for _, result := range results {
		if result.Status == maintenanceStatusError {
			return false
		}
	}
	return true
}

func cmdRepairProxy() {
	printMaintenanceReport(os.Stdout, RepairProxy())
}

func cmdUninstall(args []string) {
	options := parseUninstallOptions(args)
	if options.DryRun {
		printMaintenanceReport(os.Stdout, dryRunRepairProxyReport())
	} else {
		printMaintenanceReport(os.Stdout, RepairProxy())
	}
	printMaintenanceReport(os.Stdout, UninstallLocalData(options))
}

func parseUninstallOptions(args []string) UninstallOptions {
	var options UninstallOptions
	selection := false
	for _, arg := range args {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case "--all", "all":
			options.All = true
			selection = true
		case "--config":
			options.Config = true
			selection = true
		case "--notify":
			options.Notify = true
			selection = true
		case "--feishu":
			options.Feishu = true
			selection = true
		case "--preferences":
			options.Preferences = true
			selection = true
		case "--stores":
			options.Stores = true
			selection = true
		case "--state":
			options.State = true
			selection = true
		case "--history":
			options.History = true
			selection = true
		case "--pid":
			options.PID = true
			selection = true
		case "--proxy-marker":
			options.ProxyMarker = true
			selection = true
		case "--certificates", "--certs":
			options.Certificates = true
			selection = true
		case "--system-cert":
			options.SystemCert = true
			selection = true
		case "--dry-run":
			options.DryRun = true
		}
	}
	if !selection {
		options.All = true
		options.Certificates = true
		options.SystemCert = true
	}
	return options
}

func printMaintenanceReport(w io.Writer, report MaintenanceReport) {
	status := "OK"
	if !report.OK {
		status = "FAILED"
	}
	fmt.Fprintf(w, "%s: %s\n", report.Action, status)
	for _, result := range report.Results {
		fmt.Fprintf(w, "  - %s [%s]", result.Name, result.Status)
		if result.Path != "" {
			fmt.Fprintf(w, " %s", result.Path)
		}
		if result.Error != "" {
			fmt.Fprintf(w, " %s", result.Error)
		}
		fmt.Fprintln(w)
	}
}

func dryRunRepairProxyReport() MaintenanceReport {
	return MaintenanceReport{
		Action: "repair_proxy",
		OK:     true,
		Results: []MaintenanceResult{{
			Name:   "system_proxy",
			Action: "clear_system_proxy",
			Status: maintenanceStatusWouldRemove,
		}},
	}
}
