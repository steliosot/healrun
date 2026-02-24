package context

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/healrun/healrun/pkg/types"
)

const (
	maxLogLines = 300

	OSTypeDarwin = "Darwin"
	OSTypeLinux  = "Linux"

	failurePatterns = "error,failed,not found,permission denied,segmentation fault"
)

// FactoryFunc creates a context based on system info
func FactoryFunc() func() *types.Context {
	return func() *types.Context {
		ctx := &types.Context{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
			Shell:        getShell(),
			CWD:          getWorkingDir(),
			InDocker:     isInDocker(),
			InCI:         isInCI(),
		}

		if runtime.GOOS == OSTypeLinux {
			ctx.Distro = getLinuxDistro()
			ctx.PackageManager = getLinuxPackageManager(ctx.Distro)
		} else if runtime.GOOS == OSTypeDarwin {
			if isCommandAvailable("brew") {
				ctx.PackageManager = "brew"
			}
		}

		return ctx
	}
}

// FromCollect creates context from a failed command result
func FromCollect(command string, exitCode int, logs string) *types.Context {
	factory := FactoryFunc()
	ctx := factory()

	ctx.Command = command
	ctx.ExitCode = exitCode
	ctx.Logs = truncateLogs(logs, maxLogLines)

	return ctx
}

// BuildPrompt constructs the LLM prompt from context
func BuildPrompt(ctx *types.Context) string {
	prompt := `You are an installation-repair agent that receives a failed shell command and its logs and must determine the correct fix in a deterministic and minimal way.
Your goal is to identify the root cause of the failure, not to guess randomly.
First, read the logs and classify the failure into one of these categories: missing system dependency, missing language library, permissions issue, network issue, incompatible package/version, or invalid command.
If the logs mention a specific missing binary, header, tool, or executable (for example: pg_config, gcc, make, python.h, node-gyp, apt package not found, command not found), infer the exact system package required and install only what is necessary.
Prefer minimal, direct fixes (e.g. install a required library, run apt-get update, install a build tool, install a missing runtime dependency) rather than reinstalling entire runtimes.
If the failure is due to an incompatible or deprecated package, or something that cannot be fixed automatically, return STOP_REPAIR instead of attempting random installs.
Never suggest reinstalling Python, Node, or the OS unless clearly required by logs.
Never repeat the same fix twice.
Never suggest destructive operations.
Output only executable shell commands required to repair the issue, in correct order, and nothing else.

Context:
OS: ` + ctx.OS + "\n"

	if ctx.Distro != "" {
		prompt += "Distro: " + ctx.Distro + "\n"
	}

	prompt += fmt.Sprintf("Architecture: %s\n", ctx.Architecture)
	prompt += fmt.Sprintf("Command: %s\n", ctx.Command)
	prompt += fmt.Sprintf("Exit code: %d\n", ctx.ExitCode)
	prompt += fmt.Sprintf("Shell: %s\n", ctx.Shell)

	if ctx.PackageManager != "" {
		prompt += fmt.Sprintf("Package manager: %s\n", ctx.PackageManager)
	}

	prompt += "Logs:\n" + ctx.Logs + "\n"

	return prompt
}

// DetectFailurePatterns checks if logs contain failure patterns
func DetectFailurePatterns(logs string) bool {
	patterns := strings.Split(failurePatterns, ",")
	logsLower := strings.ToLower(logs)

	for _, pattern := range patterns {
		if strings.Contains(logsLower, pattern) {
			return true
		}
	}

	return false
}

// CheckForFailure determines if a failure occurred
func CheckForFailure(exitCode int, logs string) (bool, string) {
	// Exit code check takes priority
	if exitCode != 0 {
		return true, fmt.Sprintf("Command failed with exit code %d", exitCode)
	}

	// Pattern detection should be more conservative
	// Only trigger on strong indicators like "error:" or "failed:" with context
	// Avoid false positives from normal output
	if DetectFailurePatternsWithContext(logs) {
		return true, "Failure patterns detected in logs"
	}

	return false, ""
}

// FailureSignature returns a stable signature for retry bucketing.
// The intent is: retries are per *same* underlying failure, but if the failure changes
// after applying a fix, we start a new retry bucket.
func FailureSignature(ctx *types.Context) string {
	logs := ctx.Logs
	logsLower := strings.ToLower(logs)

	// Command not found
	if ctx.ExitCode == 127 && (strings.Contains(logsLower, "command not found") || strings.Contains(logsLower, "not found")) {
		cmd := strings.ToLower(strings.TrimSpace(extractFirstWord(ctx.Command)))
		if cmd != "" {
			return "cmd_not_found:" + cmd
		}
		return "cmd_not_found"
	}

	// Python missing module
	if mod, ok := extractPythonMissingModule(logs); ok {
		return "py_missing_module:" + strings.ToLower(mod)
	}

	// Node missing module
	if mod, ok := extractNodeMissingModule(logs); ok {
		return "node_missing_module:" + strings.ToLower(mod)
	}

	// apt unable to locate package
	if pkg, ok := extractAptUnableToLocate(logs); ok {
		return "apt_unable_to_locate:" + strings.ToLower(pkg)
	}

	// npm tracker idealTree error
	if isNpmTrackerIdealTree(logs) {
		return "npm_tracker_idealTree"
	}

	// exec format error
	if isExecFormatError(logs) {
		return "exec_format_error"
	}

	// SSL/cert
	if strings.Contains(logsLower, "cannot verify") && strings.Contains(logsLower, "certificate") {
		return "ssl_cert_verify"
	}
	if strings.Contains(logsLower, "unable to locally verify") && strings.Contains(logsLower, "certificate") {
		return "ssl_cert_verify"
	}

	// Python 2 syntax in Python 3
	if strings.Contains(logsLower, "syntaxerror") && strings.Contains(logsLower, "missing parentheses") {
		return "py2_syntax"
	}

	// Permission
	if strings.Contains(logsLower, "permission denied") {
		return "permission_denied"
	}

	// Fallback
	line := extractFirstInterestingLine(logs)
	if line != "" {
		return fmt.Sprintf("exit_%d:%s", ctx.ExitCode, strings.ToLower(line))
	}
	return fmt.Sprintf("exit_%d", ctx.ExitCode)
}

// FailureSummary returns a human-readable summary for the UI.
func FailureSummary(ctx *types.Context) string {
	logsLower := strings.ToLower(ctx.Logs)

	if ctx.ExitCode == 127 {
		cmd := extractFirstWord(ctx.Command)
		if cmd != "" {
			return fmt.Sprintf("Missing command: %s", cmd)
		}
		return "Missing command"
	}

	if mod, ok := extractPythonMissingModule(ctx.Logs); ok {
		return fmt.Sprintf("Python module missing: %s", mod)
	}
	if mod, ok := extractNodeMissingModule(ctx.Logs); ok {
		return fmt.Sprintf("Node module missing: %s", mod)
	}
	if pkg, ok := extractAptUnableToLocate(ctx.Logs); ok {
		return fmt.Sprintf("apt package not found (missing apt index?): %s", pkg)
	}
	if isNpmTrackerIdealTree(ctx.Logs) {
		return "npm internal error: Tracker \"idealTree\" already exists"
	}
	if isExecFormatError(ctx.Logs) {
		return "exec format error (binary built for wrong OS/arch)"
	}

	if strings.Contains(logsLower, "cannot verify") && strings.Contains(logsLower, "certificate") {
		return "TLS/SSL certificate verification failed"
	}

	if strings.Contains(logsLower, "syntaxerror") && strings.Contains(logsLower, "missing parentheses") {
		return "Python 2-only package (syntax incompatible with Python 3)"
	}

	if strings.Contains(logsLower, "permission denied") {
		return "Permission denied"
	}

	return "Unknown failure"
}

func extractFirstWord(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func extractPythonMissingModule(logs string) (string, bool) {
	// Examples:
	// ModuleNotFoundError: No module named 'yaml'
	// ImportError: No module named yaml
	re := regexp.MustCompile(`(?i)(?:ModuleNotFoundError|ImportError):\s*No module named ['\"]?([^'\"\s]+)['\"]?`)
	if m := re.FindStringSubmatch(logs); len(m) == 2 {
		return m[1], true
	}
	return "", false
}

func extractNodeMissingModule(logs string) (string, bool) {
	re := regexp.MustCompile(`(?i)cannot\s+find\s+module\s+['\"]?([^'\"\s]+)['\"]?`)
	if m := re.FindStringSubmatch(logs); len(m) == 2 {
		return m[1], true
	}
	return "", false
}

func extractAptUnableToLocate(logs string) (string, bool) {
	re := regexp.MustCompile(`(?i)unable\s+to\s+locate\s+package\s+([^\s]+)`)
	if m := re.FindStringSubmatch(logs); len(m) == 2 {
		return m[1], true
	}
	return "", false
}

func isNpmTrackerIdealTree(logs string) bool {
	re := regexp.MustCompile(`(?i)tracker\s+"idealTree"\s+already\s+exists`)
	return re.MatchString(logs)
}

func isExecFormatError(logs string) bool {
	re := regexp.MustCompile(`(?i)exec\s+format\s+error`)
	return re.MatchString(logs)
}

func extractFirstInterestingLine(logs string) string {
	// Grab the first line that looks like an error headline.
	for _, line := range strings.Split(logs, "\n") {
		l := strings.TrimSpace(line)
		ll := strings.ToLower(l)
		if l == "" {
			continue
		}
		if strings.Contains(ll, "modulenotfounderror") || strings.Contains(ll, "importerror") || strings.Contains(ll, "syntaxerror") {
			return l
		}
		if strings.HasPrefix(ll, "error:") || strings.HasPrefix(ll, "failed:") {
			return l
		}
	}
	return ""
}

// DetectFailurePatternsWithContext checks for failure patterns with better context awareness
func DetectFailurePatternsWithContext(logs string) bool {
	logsLower := strings.ToLower(logs)

	// Only match if pattern appears in error context
	// Look for error indicators with colons, sentence endings, etc.
	errorIndicators := []string{
		"error:",
		"failed:",
		"error!",
		"failed!",
		" error ",
		" failed ",
		" not found: ",
		" permission denied: ",
	}

	for _, indicator := range errorIndicators {
		if strings.Contains(logsLower, indicator) {
			return true
		}
	}

	return false
}

// ParseFixCommands parses response into list of shell commands
func ParseFixCommands(response string) []string {
	response = strings.TrimSpace(response)

	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		if len(lines) > 2 {
			response = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var commands []string
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
			if strings.Contains(line, "&&") {
				parts := strings.Split(line, "&&")
				for _, part := range parts {
					cmd := strings.TrimSpace(part)
					if cmd != "" {
						commands = append(commands, cmd)
					}
				}
			} else {
				commands = append(commands, line)
			}
		}
	}

	return commands
}

// getShell returns the current shell
func getShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash"
}

// getWorkingDir returns the current working directory
func getWorkingDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

// IsInDocker checks if running inside Docker
func IsInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if content, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		if strings.Contains(string(content), "docker") {
			return true
		}
	}

	return false
}

// isInDocker checks if running inside Docker (internal)
func isInDocker() bool {
	return IsInDocker()
}

// isInCI checks if running in CI environment
func isInCI() bool {
	ciVar := []string{"CI", "CONTINUOUS_INTEGRATION", "JENKINS_URL", "GITHUB_ACTIONS"}
	for _, v := range ciVar {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}

// getLinuxDistro reads /etc/os-release to get distro name
func getLinuxDistro() string {
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			distro := strings.TrimPrefix(line, "ID=")
			distro = strings.Trim(distro, `"`)
			return strings.ToLower(distro)
		}
	}

	return "unknown"
}

// getLinuxPackageManager returns package manager for distro
func getLinuxPackageManager(distro string) string {
	if distro == "ubuntu" || distro == "debian" {
		return "apt"
	} else if distro == "fedora" || distro == "rhel" || distro == "centos" {
		if isCommandAvailable("dnf") {
			return "dnf"
		}
		return "yum"
	} else if distro == "arch" || distro == "archlinux" {
		return "pacman"
	}

	if isCommandAvailable("apt-get") {
		return "apt"
	} else if isCommandAvailable("dnf") {
		return "dnf"
	} else if isCommandAvailable("yum") {
		return "yum"
	}

	return ""
}

// isCommandAvailable checks if a command is available in PATH
func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// truncateLogs keeps only the last N lines of logs
func truncateLogs(logs string, maxLines int) string {
	lines := strings.Split(logs, "\n")
	if len(lines) <= maxLines {
		return logs
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

// SleepSeconds sleeps for specified seconds
func SleepSeconds(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}
