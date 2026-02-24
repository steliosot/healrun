package safety

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/healrun/healrun/pkg/types"
)

var (
	blocklistRegexes  []*regexp.Regexp
	allowlistPrefixes []string
	safeCommands      []string
)

func init() {
	initializeBlocklist()
	initializeAllowlist()
}

// initializeBlocklist initializes the dangerous command blocklist
func initializeBlocklist() {
	blocklistRegexes = append(blocklistRegexes,
		regexp.MustCompile(`(?i)rm\s+-rf\s+\/`),
		regexp.MustCompile(`(?i)rm\s+-rf\s+\/[^\s]+`),
		regexp.MustCompile(`(?i)rm\s+-rf\s+[^\s]+\s*`),
		regexp.MustCompile(`(?i)rm\s+-f\s+\/[^\s]+`),
		regexp.MustCompile(`(?i)dd\s+if=\/dev\/zero`),
		regexp.MustCompile(`(?i)dd\s+if=\/dev\/random`),
		regexp.MustCompile(`(?i)mkfs\.?\s`),
		regexp.MustCompile(`(?i)shutdown\s+(now|restart|reboot)\b`),
		regexp.MustCompile(`(?i)^reboot`),
		regexp.MustCompile(`(?i)^poweroff`),
		regexp.MustCompile(`(?i)^halt`),
		regexp.MustCompile(`(?i)userdel\s+`),
		regexp.MustCompile(`(?i)groupdel\s+`),
		regexp.MustCompile(`(?i)chmod\s+777\s+\/\w*`),
		regexp.MustCompile(`(?i)chmod\s+-x\s+\/`))
}

// initializeAllowlist initializes the safe command list
func initializeAllowlist() {
	allowlistPrefixes = append(allowlistPrefixes,
		"pip install", "pip uninstall", "pip update", "pip3 install",
		"apt-get install", "apt-get update", "apt-get upgrade",
		"apt install", "apt update", "apt upgrade",
		"dnf install", "dnf update", "dnf upgrade",
		"yum install", "yum update", "yum upgrade",
		"pacman -S", "pacman -U",
		"brew install", "brew uninstall", "brew upgrade",
		"npm install", "npm update", "yarn install", "yarn add",
		"docker build", "docker pull", "docker run",
		"python -m pip install", "python3 -m pip install",
		"export ", "echo ", "touch ", "mkdir ", "mkdir -p ",
		"git clone", "git init",
		"make", "cmake", "gcc", "g++", "clang")

	safeCommands = append(safeCommands,
		"pip", "pip3", "apt-get", "apt", "dnf", "yum", "pacman",
		"brew", "npm", "yarn", "cargo", "gem", "docker",
		"git", "curl", "wget", "make", "cmake", "gcc", "g++", "clang", "go",
		"python", "python3", "node", "bash", "sh", "zsh")
}

// ConfigurePolicies adds additional allow/block rules from config.
func ConfigurePolicies(policies types.Policies) {
	for _, entry := range policies.Allowed {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		allowlistPrefixes = append(allowlistPrefixes, trimmed)
	}

	for _, entry := range policies.Blocked {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		pattern := "(?i)" + regexp.QuoteMeta(trimmed)
		blocklistRegexes = append(blocklistRegexes, regexp.MustCompile(pattern))
	}
}

// IsBlockedCommand checks if a command is in the blocklist
func IsBlockedCommand(command string) (bool, string) {
	trimmedCmd := strings.ToLower(strings.TrimSpace(command))
	if isSafeRmRf(trimmedCmd) {
		return false, ""
	}

	for _, re := range blocklistRegexes {
		if re.MatchString(trimmedCmd) {
			return true, fmt.Sprintf("Blocked dangerous command: matched pattern")
		}
	}

	dangerousPaths := []string{"/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/", "/etc/passwd", "/etc/shadow"}
	for _, path := range dangerousPaths {
		if strings.Contains(command, path) {
			if strings.Contains(command, "echo") || strings.Contains(command, "cat") || strings.Contains(command, "tee") {
				if strings.Contains(command, ">") || strings.Contains(command, ">>") {
					return true, fmt.Sprintf("Blocked write to system path: %s", path)
				}
			}
		}
	}

	return false, ""
}

func isSafeRmRf(command string) bool {
	parts := strings.Fields(command)
	if len(parts) < 3 {
		return false
	}
	if parts[0] != "rm" {
		return false
	}
	hasR := false
	hasF := false
	paths := []string{}
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		if strings.HasPrefix(p, "-") {
			if strings.Contains(p, "r") {
				hasR = true
			}
			if strings.Contains(p, "f") {
				hasF = true
			}
			continue
		}
		paths = append(paths, p)
	}
	if !hasR || !hasF || len(paths) == 0 {
		return false
	}
	for _, p := range paths {
		if strings.HasPrefix(p, "/tmp/") || strings.HasPrefix(p, "/var/tmp/") {
			continue
		}
		if strings.HasPrefix(p, "/root/.npm") || strings.HasPrefix(p, "/root/.cache/npm") {
			continue
		}
		if strings.HasPrefix(p, "~/.npm") || strings.HasPrefix(p, "~/.cache/npm") {
			continue
		}
		if strings.HasPrefix(p, "/home/") || strings.HasPrefix(p, "/users/") {
			if strings.Contains(p, "/.npm") || strings.Contains(p, "/.cache/npm") {
				continue
			}
		}
		return false
	}
	return true
}

// IsAllowedCommand checks if a command is in the allowlist
func IsAllowedCommand(command, cwd string) (bool, string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false, "empty command"
	}

	baseCmd := parts[0]

	if strings.HasPrefix(command, "./") || strings.HasPrefix(command, "../") {
		return true, ""
	}

	// Allow executing a relative path that resolves within cwd (e.g. .healrun-venv/bin/python)
	if strings.Contains(baseCmd, "/") && !strings.HasPrefix(baseCmd, "/") {
		cwdAbs, err1 := filepath.Abs(cwd)
		cmdAbs, err2 := filepath.Abs(filepath.Join(cwd, baseCmd))
		if err1 == nil && err2 == nil {
			cwdAbs = filepath.Clean(cwdAbs)
			cmdAbs = filepath.Clean(cmdAbs)
			prefix := cwdAbs + string(os.PathSeparator)
			if cmdAbs == cwdAbs || strings.HasPrefix(cmdAbs, prefix) {
				return true, ""
			}
		}
	}

	for _, prefix := range allowlistPrefixes {
		if strings.HasPrefix(command, prefix) {
			return true, ""
		}
	}

	for _, cmd := range safeCommands {
		if baseCmd == cmd {
			return true, ""
		}
	}

	if strings.Contains(command, cwd) {
		return true, ""
	}

	return false, fmt.Sprintf("Command not in allowlist: %s", baseCmd)
}

// IsSafeCommand performs comprehensive safety check
func IsSafeCommand(command, cwd string, autoApprove bool) (bool, string) {
	blocked, reason := IsBlockedCommand(command)
	if blocked {
		return false, reason
	}

	allowed, reason := IsAllowedCommand(command, cwd)
	if allowed {
		return true, ""
	}

	if autoApprove {
		return false, reason
	}

	return false, fmt.Sprintf("Command requires manual confirmation: %s", reason)
}

// GetConfirmation prompts user for confirmation (returns false in Docker mode)
func GetConfirmation(prompt string, inDocker bool) bool {
	if inDocker {
		return true
	}

	// Check if stdin is a TTY before trying to read
	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		// Non-interactive environment, skip confirmation
		return true
	}

	fmt.Printf("❌ %s", prompt)
	fmt.Printf(" [y/N]: ")

	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// ApplyFix determines if a fix should be applied
func ApplyFix(command, cwd string, autoApprove, inDocker bool) bool {
	blocked, reason := IsBlockedCommand(command)
	if blocked {
		fmt.Printf("⚠️  Blocked: %s\n", reason)
		return false
	}

	allowed, reason := IsAllowedCommand(command, cwd)
	if !allowed {
		if !autoApprove && !inDocker {
			return GetConfirmation(fmt.Sprintf("%s\nApply fix?", reason), inDocker)
		}
		return false
	}

	return true
}
