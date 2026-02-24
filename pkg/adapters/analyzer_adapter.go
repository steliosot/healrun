package adapters

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/healrun/healrun/pkg/logger"
	"github.com/healrun/healrun/pkg/types"
)

// AnalyzerAdapter provides intelligent error analysis for common failures
type AnalyzerAdapter struct{}

// NewAnalyzerAdapter creates a new analyzer adapter
func NewAnalyzerAdapter() *AnalyzerAdapter {
	return &AnalyzerAdapter{}
}

// SuggestFix analyzes the error and suggests appropriate fixes
func (a *AnalyzerAdapter) SuggestFix(ctx *types.Context) ([]string, error) {
	errorType, details := a.classifyError(ctx)

	logger.Debugf("Error classified as: %s - %s", errorType, details)

	switch errorType {
	case ErrorCommandNotFound:
		return a.suggestCommandInstall(ctx)
	case ErrorCertificate:
		return a.suggestCertificateFix(ctx, details)
	case ErrorPythonSyntax:
		return a.suggestPythonFix(ctx, details)
	case ErrorPackageNotFound:
		return a.suggestPackageNotFoundFix(ctx)
	case ErrorPermission:
		return a.suggestPermissionFix(ctx)
	case ErrorMissingDeps:
		return a.suggestDepsInstall(ctx)
	case ErrorPythonModuleMissing:
		return a.suggestPythonModuleInstall(ctx, details)
	case ErrorNodeModuleMissing:
		return a.suggestNodeModuleInstall(ctx, details)
	case ErrorAptUnableToLocate:
		return a.suggestAptUpdateThenInstall(ctx, details)
	case ErrorNpmTrackerIdealTree:
		return nil, fmt.Errorf("repair stopped - npm tracker idealTree error")
	case ErrorExecFormat:
		return a.suggestExecFormatFix(ctx)
	case ErrorNetwork:
		return a.suggestNetworkFix(ctx, details)
	case ErrorTerminal:
		return nil, fmt.Errorf("repair stopped - %s", details)
	case ErrorUnrepairable:
		return nil, fmt.Errorf("repair stopped - %s", details)
	case ErrorNone:
		if shouldStopRepair(ctx) {
			return nil, fmt.Errorf("repair stopped - no applicable fix")
		}
		return a.getSuggestionBasedOnCommand(ctx)
	}

	return nil, fmt.Errorf("repair stopped - unknown error type")
}

// ErrorType represents different categories of errors
type ErrorType int

const (
	ErrorNone ErrorType = iota
	ErrorCommandNotFound
	ErrorCertificate
	ErrorPythonSyntax
	ErrorPackageNotFound
	ErrorPermission
	ErrorMissingDeps
	ErrorPythonModuleMissing
	ErrorNodeModuleMissing
	ErrorAptUnableToLocate
	ErrorNpmTrackerIdealTree
	ErrorExecFormat
	ErrorNetwork
	ErrorTerminal
	ErrorUnrepairable
)

// classifyError analyzes logs and command to determine error type
func (a *AnalyzerAdapter) classifyError(ctx *types.Context) (ErrorType, string) {
	logs := strings.ToLower(ctx.Logs)
	command := strings.ToLower(ctx.Command)

	// Command not found (exit code 127)
	if ctx.ExitCode == 127 {
		if strings.Contains(logs, "command not found") || strings.Contains(logs, "not found") {
			baseCmd := extractBaseCommand(ctx.Command)
			return ErrorCommandNotFound, fmt.Sprintf(" '%s' command not found", baseCmd)
		}
	}

	// Certificate verification errors
	if strings.Contains(logs, "cannot verify") && strings.Contains(logs, "certificate") {
		return ErrorCertificate, "SSL certificate verification failed"
	}
	if strings.Contains(logs, "certificate") && strings.Contains(logs, "verify") {
		return ErrorCertificate, "SSL certificate issue"
	}

	// Python 2 syntax errors
	if strings.Contains(logs, "syntaxerror") && strings.Contains(logs, "missing parentheses") {
		return ErrorPythonSyntax, "Python 2 syntax detected in Python 3 environment"
	}

	// Python missing module
	if mod, ok := extractPythonMissingModule(ctx.Logs); ok {
		return ErrorPythonModuleMissing, mod
	}

	// Node.js missing module
	if mod, ok := extractNodeMissingModule(ctx.Logs); ok {
		return ErrorNodeModuleMissing, mod
	}

	// apt-get: Unable to locate package
	if pkg, ok := extractAptUnableToLocate(ctx.Logs); ok {
		return ErrorAptUnableToLocate, pkg
	}

	// npm internal tracker error
	if isNpmTrackerIdealTree(ctx.Logs) {
		return ErrorNpmTrackerIdealTree, "npm tracker idealTree error"
	}

	// Exec format error (wrong binary for OS/arch)
	if isExecFormatError(ctx.Logs) {
		return ErrorExecFormat, "exec format error"
	}

	// Package not found (404)
	if strings.Contains(logs, "error: could not find a version") && strings.Contains(logs, "from versions: none") {
		return ErrorPackageNotFound, "Package not available or doesn't exist"
	}

	// Permission denied
	if strings.Contains(logs, "permission denied") || strings.Contains(logs, "cannot open") {
		return ErrorPermission, "Permission denied"
	}

	// Build dependencies errors
	if strings.Contains(logs, "build dependency") || strings.Contains(logs, "installing build dependencies") {
		return ErrorMissingDeps, "Missing build dependencies"
	}

	// pg_config missing (psycopg2, libpq-dev)
	if strings.Contains(logs, "pg_config executable not found") || strings.Contains(logs, "pg_config is required") {
		return ErrorMissingDeps, "Missing pg_config (PostgreSQL dev headers)"
	}

	// Network errors
	if strings.Contains(logs, "network") || strings.Contains(logs, "connection") || strings.Contains(logs, "timeout") {
		return ErrorNetwork, "Network connectivity issue"
	}

	// Terminal errors
	if strings.Contains(logs, "error opening terminal") || strings.Contains(logs, "unknown terminal") {
		return ErrorTerminal, "Interactive terminal required - cannot be tested in non-interactive mode"
	}

	// Python stdlib packages
	stdlibPackages := []string{"logging", "multiprocessing", "threading", "queue", "socket", "hashlib", "time", "os", "sys", "json"}
	for _, pkg := range stdlibPackages {
		if strings.Contains(command, fmt.Sprintf("pip install %s", pkg)) {
			return ErrorUnrepairable, fmt.Sprintf("'%s' is in Python standard library", pkg)
		}
	}

	return ErrorNone, "Unknown error type"
}

func mapExecutableToPackage(baseCmd string) string {
	execToPkg := map[string]string{
		"convert":  "imagemagick",
		"magick":   "imagemagick",
		"gs":       "ghostscript",
		"rg":       "ripgrep",
		"fd":       "fd",
		"wget":     "wget",
		"htop":     "htop",
		"tree":     "tree",
		"watch":    "watch",
		"parallel": "parallel",
		"ffmpeg":   "ffmpeg",
		"sqlite3":  "sqlite",
		"python3":  "python3",
		"pip3":     "python3-pip",
		"pip":      "python3-pip",
	}
	if mapped, ok := execToPkg[strings.ToLower(baseCmd)]; ok {
		return mapped
	}
	return baseCmd
}

func extractPythonMissingModule(logs string) (string, bool) {
	re := regexp.MustCompile(`(?i)(?:ModuleNotFoundError|ImportError):\s*No module named ['\"]?([^'\"\s]+)['\"]?`)
	if m := re.FindStringSubmatch(logs); len(m) == 2 {
		return m[1], true
	}
	return "", false
}

func extractNodeMissingModule(logs string) (string, bool) {
	// Example:
	// Error: Cannot find module 'axios'
	re := regexp.MustCompile(`(?i)cannot\s+find\s+module\s+['\"]([^'\"]+)['\"]`)
	if m := re.FindStringSubmatch(logs); len(m) == 2 {
		return m[1], true
	}
	return "", false
}

func extractAptUnableToLocate(logs string) (string, bool) {
	// Example:
	// E: Unable to locate package curl
	re := regexp.MustCompile(`(?i)unable\s+to\s+locate\s+package\s+([^\s]+)`)
	if m := re.FindStringSubmatch(logs); len(m) == 2 {
		return m[1], true
	}
	return "", false
}

func isNpmTrackerIdealTree(logs string) bool {
	// Example:
	// npm error Tracker "idealTree" already exists
	re := regexp.MustCompile(`(?i)tracker\s+"idealTree"\s+already\s+exists`)
	return re.MatchString(logs)
}

func isExecFormatError(logs string) bool {
	// Example:
	// /bin/sh: 1: healrun: Exec format error
	re := regexp.MustCompile(`(?i)exec\s+format\s+error`)
	return re.MatchString(logs)
}

func (a *AnalyzerAdapter) suggestExecFormatFix(ctx *types.Context) ([]string, error) {
	cmd := strings.TrimSpace(ctx.Command)
	if strings.Contains(cmd, "docker build") || strings.Contains(cmd, "docker run") {
		return []string{
			"CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags=\"-s -w\" -o healrun ./cmd/healrun",
		}, nil
	}
	return []string{
		"CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags=\"-s -w\" -o healrun ./cmd/healrun",
	}, nil
}

func (a *AnalyzerAdapter) suggestPythonModuleInstall(ctx *types.Context, module string) ([]string, error) {
	// Map import name -> pip distribution name
	importToDist := map[string]string{
		"yaml":    "pyyaml",
		"pil":     "pillow",
		"cv2":     "opencv-python",
		"crypto":  "pycryptodome",
		"sklearn": "scikit-learn",
		"bs4":     "beautifulsoup4",
		"lxml":    "lxml",
	}

	dist := module
	if v, ok := importToDist[strings.ToLower(module)]; ok {
		dist = v
	}

	py := extractBaseCommand(ctx.Command)
	if py == "" {
		py = "python3"
	}
	if !strings.HasPrefix(py, "python") {
		py = "python3"
	}

	// On macOS, system Python often enforces PEP 668. The most robust automated fix is a local venv
	// in the current working directory, then retry the original command with the venv interpreter.
	if runtime.GOOS == "darwin" {
		orig := strings.TrimSpace(ctx.Command)
		base := extractBaseCommand(orig)
		rest := strings.TrimSpace(strings.TrimPrefix(orig, base))
		venv := ".healrun-venv"
		pyVenv := venv + "/bin/python"
		cmdRetry := pyVenv
		if rest != "" {
			cmdRetry = cmdRetry + " " + rest
		}
		return []string{
			fmt.Sprintf("%s -m venv %s 2>/dev/null || true", py, venv),
			fmt.Sprintf("%s -m pip install %s", pyVenv, dist),
			cmdRetry,
		}, nil
	}

	// Linux fallback: prefer user install (avoids sudo)
	return []string{fmt.Sprintf("%s -m pip install --user %s", py, dist)}, nil
}

func (a *AnalyzerAdapter) suggestNodeModuleInstall(ctx *types.Context, module string) ([]string, error) {
	name := strings.TrimSpace(module)
	if name == "" {
		return nil, fmt.Errorf("repair stopped - cannot determine node module name")
	}
	// Install into the current working directory.
	return []string{fmt.Sprintf("npm install %s", name)}, nil
}

func (a *AnalyzerAdapter) suggestNpmCacheClean(ctx *types.Context) ([]string, error) {
	return nil, fmt.Errorf("repair stopped - npm tracker idealTree error is not deterministic to fix")
}

func rewriteDockerRunWithNpmCacheClean(cmd string) (string, bool) {
	parts := strings.Fields(cmd)
	if len(parts) < 5 {
		return "", false
	}
	idx := 2
	for idx < len(parts) {
		p := parts[idx]
		if strings.HasPrefix(p, "-") {
			idx++
			continue
		}
		break
	}
	if idx >= len(parts) {
		return "", false
	}
	containerCmdParts := parts[idx+1:]
	if len(containerCmdParts) == 0 {
		return "", false
	}
	containerCmd := strings.Join(containerCmdParts, " ")
	if !strings.Contains(containerCmd, "npm install") {
		return "", false
	}
	if strings.Contains(containerCmd, "npm cache clean --force") {
		return "", false
	}
	script := "npm cache clean --force && " + containerCmd
	script = strings.ReplaceAll(script, "\"", "\\\"")
	newParts := append([]string{}, parts[:idx+1]...)
	newParts = append(newParts, "bash", "-c", fmt.Sprintf("\"%s\"", script))
	return strings.Join(newParts, " "), true
}

func extractNpmInstallPackage(cmd string) (string, bool) {
	re := regexp.MustCompile(`(?i)npm\s+install\s+([@\w./-]+)`) // minimal match
	if m := re.FindStringSubmatch(cmd); len(m) == 2 {
		return m[1], true
	}
	return "", false
}

func extractDockerImageAndNpmPackage(cmd string) (string, string, bool) {
	parts := strings.Fields(cmd)
	if len(parts) < 5 {
		return "", "", false
	}
	if parts[0] != "docker" || parts[1] != "run" {
		return "", "", false
	}
	idx := 2
	for idx < len(parts) {
		p := parts[idx]
		if strings.HasPrefix(p, "-") {
			idx++
			continue
		}
		break
	}
	if idx >= len(parts) {
		return "", "", false
	}
	image := parts[idx]
	containerCmd := strings.Join(parts[idx+1:], " ")
	pkg, ok := extractNpmInstallPackage(containerCmd)
	if !ok {
		return "", "", false
	}
	return image, pkg, true
}

func normalizeNodeImage(image string) string {
	// If image is node:<ver>-slim, try node:<ver>
	if strings.Contains(image, "node:") && strings.Contains(image, "-slim") {
		return strings.Replace(image, "-slim", "", 1)
	}
	return image
}

func (a *AnalyzerAdapter) suggestAptUpdateThenInstall(ctx *types.Context, pkg string) ([]string, error) {
	cmd := strings.TrimSpace(ctx.Command)
	// If this is inside docker run, rewrite container command to run apt-get update first.
	if strings.HasPrefix(cmd, "docker run") {
		rewritten, ok := rewriteDockerRunWithAptUpdate(cmd)
		if ok {
			return []string{rewritten}, nil
		}
	}

	// Inside Dockerfile/container or generic apt-get install: update first, then install.
	if strings.Contains(cmd, "apt-get install") {
		return []string{"apt-get update", cmd}, nil
	}

	_ = pkg
	return nil, fmt.Errorf("repair stopped - apt package index missing; cannot rewrite command safely")
}

func rewriteDockerRunWithAptUpdate(cmd string) (string, bool) {
	parts := strings.Fields(cmd)
	if len(parts) < 5 {
		return "", false
	}
	// Find image token: first token after 'docker run' options that doesn't start with '-'
	idx := 2 // after docker run
	for idx < len(parts) {
		p := parts[idx]
		if strings.HasPrefix(p, "-") {
			idx++
			continue
		}
		break
	}
	if idx >= len(parts) {
		return "", false
	}
	containerCmdParts := parts[idx+1:]
	if len(containerCmdParts) == 0 {
		return "", false
	}
	containerCmd := strings.Join(containerCmdParts, " ")
	if !strings.Contains(containerCmd, "apt-get install") {
		return "", false
	}
	script := "apt-get update && " + containerCmd
	script = strings.ReplaceAll(script, "\"", "\\\"")

	newParts := append([]string{}, parts[:idx+1]...)
	newParts = append(newParts, "bash", "-c", fmt.Sprintf("\"%s\"", script))
	return strings.Join(newParts, " "), true
}

// suggestCommandInstall suggests installing missing commands
func (a *AnalyzerAdapter) suggestCommandInstall(ctx *types.Context) ([]string, error) {
	baseCmd := extractBaseCommand(ctx.Command)
	if baseCmd == "" {
		return nil, fmt.Errorf("repair stopped - cannot determine command name")
	}

	// Map common executables to the package/formula name.
	baseCmd = mapExecutableToPackage(baseCmd)

	switch runtime.GOOS {
	case "linux":
		pm := ctx.PackageManager
		if pm == "" {
			pm = "apt"
		}
		if pm == "apt" || pm == "dnf" || pm == "yum" || pm == "pacman" {
			pmCmd := pm
			if pm == "dnf" || pm == "yum" {
				pmCmd = fmt.Sprintf("%s install -y", pm)
			} else if pm == "pacman" {
				pmCmd = "pacman -S"
			} else {
				pmCmd = "apt-get install -y"
			}
			return []string{fmt.Sprintf("%s %s", pmCmd, baseCmd)}, nil
		}
	case "darwin":
		return []string{fmt.Sprintf("brew install %s", baseCmd)}, nil
	}

	return nil, fmt.Errorf("repair stopped - unable to suggest package manager")
}

// suggestCertificateFix suggests fixes for SSL certificate errors
func (a *AnalyzerAdapter) suggestCertificateFix(ctx *types.Context, details string) ([]string, error) {
	// If command supports --no-check-certificate
	if strings.Contains(ctx.Command, "wget") || strings.Contains(ctx.Command, "curl") {
		if strings.Contains(ctx.Command, "--no-check-certificate") {
			return nil, fmt.Errorf("repair stopped - certificate verification already bypassed but still failing")
		}
		// Add flag to the command
		newCommand := ctx.Command + " --no-check-certificate"
		return []string{newCommand}, nil
	}

	return nil, fmt.Errorf("repair stopped - cannot fix certificate error automatically")
}

// suggestPythonFix handles Python package errors
func (a *AnalyzerAdapter) suggestPythonFix(ctx *types.Context, details string) ([]string, error) {
	return nil, fmt.Errorf("repair stopped - %s (package is Python 2 only, use Python 3 equivalent or remove)", details)
}

// suggestPackageNotFoundFix handles missing packages
func (a *AnalyzerAdapter) suggestPackageNotFoundFix(ctx *types.Context) ([]string, error) {
	return nil, fmt.Errorf("repair stopped - package not available or doesn't exist")
}

// suggestPermissionFix handles permission errors
func (a *AnalyzerAdapter) suggestPermissionFix(ctx *types.Context) ([]string, error) {
	cwd, _ := os.Getwd()
	return []string{
		fmt.Sprintf("chmod +w %s 2>/dev/null || sudo %s", cwd, ctx.Command),
	}, nil
}

// suggestDepsInstall handles missing build dependencies
func (a *AnalyzerAdapter) suggestDepsInstall(ctx *types.Context) ([]string, error) {
	logs := strings.ToLower(ctx.Logs)
	if strings.HasPrefix(ctx.Command, "pip install") || strings.HasPrefix(ctx.Command, "pip3 install") {
		if strings.Contains(logs, "pg_config") {
			switch runtime.GOOS {
			case "linux":
				pm := ctx.PackageManager
				if pm == "" {
					pm = "apt"
				}
				switch pm {
				case "dnf", "yum":
					return []string{fmt.Sprintf("%s install -y postgresql-devel python3-devel gcc", pm)}, nil
				case "pacman":
					return []string{"pacman -S postgresql base-devel"}, nil
				default:
					return []string{"apt-get install -y libpq-dev python3-dev build-essential"}, nil
				}
			case "darwin":
				return []string{"brew install postgresql"}, nil
			}
		}
		switch runtime.GOOS {
		case "linux":
			pm := ctx.PackageManager
			if pm == "" {
				pm = "apt"
			}
			return []string{fmt.Sprintf("apt-get install -y python3-dev build-essential")}, nil
		case "darwin":
			return []string{"brew install python"}, nil
		}
		return nil, fmt.Errorf("repair stopped - cannot determine dependency type")
	}
	return nil, fmt.Errorf("repair stopped - cannot determine dependency type")
}

// suggestNetworkFix handles network errors
func (a *AnalyzerAdapter) suggestNetworkFix(ctx *types.Context, details string) ([]string, error) {
	return []string{
		"echo 'Network issue - check connectivity'",
		"ping -c 3 8.8.8.8",
	}, nil
}

// getSuggestionBasedOnCommand provides fallback suggestions
func (a *AnalyzerAdapter) getSuggestionBasedOnCommand(ctx *types.Context) ([]string, error) {
	// Default: try installing the base command
	baseCmd := extractBaseCommand(ctx.Command)
	if baseCmd != "" {
		baseCmd = mapExecutableToPackage(baseCmd)
		switch runtime.GOOS {
		case "darwin":
			return []string{fmt.Sprintf("brew install %s", baseCmd)}, nil
		case "linux":
			pm := ctx.PackageManager
			if pm == "" {
				pm = "apt"
			}
			return []string{fmt.Sprintf("%s install -y %s", pm, baseCmd)}, nil
		}
	}

	return nil, fmt.Errorf("repair stopped - no applicable fix")
}

// extractBaseCommand extracts the base command from a command string
func extractBaseCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// shouldStopRepair checks if the failure cannot be fixed
func shouldStopRepair(ctx *types.Context) bool {
	logs := strings.ToLower(ctx.Logs)
	command := strings.ToLower(ctx.Command)

	// Interactive tools that require terminal
	interactiveTools := []string{"htop", "vi", "vim", "nano", "top", "less", "more"}
	for _, tool := range interactiveTools {
		if extractBaseCommand(ctx.Command) == tool {
			if strings.Contains(logs, "error opening terminal") {
				return true
			}
		}
	}

	// Python 2 syntax errors on Python 3
	if strings.Contains(logs, "syntaxerror") && strings.Contains(logs, "missing parentheses") {
		return true
	}

	// Package not found (404)
	if strings.Contains(logs, "error: could not find a version") && strings.Contains(logs, "from versions: none") {
		return true
	}

	// Terminal errors (cannot be fixed)
	if strings.Contains(logs, "error opening terminal") || strings.Contains(logs, "unknown terminal") {
		return true
	}

	// Python packages that are in stdlib
	stdlibPackages := []string{"logging", "multiprocessing", "threading", "queue", "socket", "hashlib", "time", "os", "sys", "json"}
	for _, pkg := range stdlibPackages {
		if strings.Contains(command, fmt.Sprintf("pip install %s", pkg)) {
			return true
		}
	}

	return false
}
