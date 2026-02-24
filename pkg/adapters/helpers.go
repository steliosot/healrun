package adapters

import (
	"fmt"

	"github.com/healrun/healrun/pkg/types"
)

const (
	StopRepairCommand = "STOP_REPAIR"
)

// buildPrompt builds the LLM prompt
func buildPrompt(ctx *types.Context) string {
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

	if len(ctx.RepairHistory) > 0 {
		prompt += "\nPrevious repair attempts:\n"
		for i, attempt := range ctx.RepairHistory {
			status := "FAILED"
			if attempt.Success {
				status = "SUCCESS"
			}
			prompt += fmt.Sprintf("  %d. %s [%s]\n", i+1, attempt.Command, status)
			if !attempt.Success && (attempt.ErrorMessage != "" || attempt.Output != "") {
				if attempt.ErrorMessage != "" {
					prompt += fmt.Sprintf("     Error: %s\n", attempt.ErrorMessage)
				}
				if attempt.Output != "" {
					prompt += fmt.Sprintf("     Output: %s\n", truncateOutput(attempt.Output, 200))
				}
			}
		}
		prompt += "\n"
	}

	prompt += "Logs:\n" + ctx.Logs + "\n"

	return prompt
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// parseCommands parses LLM response into a list of shell commands
func parseCommands(content string) ([]string, error) {
	lines := splitLines(content)
	var commands []string

	for _, line := range lines {
		trimmed := trimSpace(line)
		if trimmed == StopRepairCommand {
			return nil, fmt.Errorf("repair stopped by model - no applicable fix")
		}
		if trimmed != "" && !isComment(trimmed) {
			commands = append(commands, trimmed)
		}
	}

	return commands, nil
}

// ParseParseCommands parses commands and returns error if STOP_REPAIR is detected
func ParseCommands(content string) ([]string, error) {
	return parseCommands(content)
}

// splitLines splits string by newline
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

// trimSpace trims whitespace from string
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// isComment checks if line is a comment
func isComment(s string) bool {
	if len(s) >= 1 && s[0] == '#' {
		return true
	}
	if len(s) >= 2 && s[0] == '/' && s[1] == '/' {
		return true
	}
	if len(s) >= 3 && s[0] == '`' && s[1] == '`' && s[2] == '`' {
		return true
	}
	return false
}
