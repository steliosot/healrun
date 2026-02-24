package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/healrun/healrun/pkg/adapters"
	"github.com/healrun/healrun/pkg/context"
	"github.com/healrun/healrun/pkg/logger"
	"github.com/healrun/healrun/pkg/runner"
	"github.com/healrun/healrun/pkg/safety"
	"github.com/healrun/healrun/pkg/types"
)

// Agent represents the self-healing agent
type Agent struct {
	config       *types.Config
	adapter      types.ModelAdapter
	appliedFixes map[string]bool
}

func shouldReplaceExecutable(original, fix string) bool {
	// Compare using a cheap lexer. This is sufficient for our CLI use cases
	// (and preserves quoted tokens consistently between original/fix).
	origParts := strings.Fields(original)
	fixParts := strings.Fields(fix)
	if len(origParts) < 2 || len(fixParts) < 2 {
		return false
	}
	// executable differs but the rest matches
	if origParts[0] == fixParts[0] {
		return false
	}
	if len(origParts) != len(fixParts) {
		return false
	}
	for i := 1; i < len(origParts); i++ {
		if origParts[i] != fixParts[i] {
			return false
		}
	}
	return true
}

func isCommandRewrite(original, fix string) bool {
	origParts := strings.Fields(strings.TrimSpace(original))
	fixParts := strings.Fields(strings.TrimSpace(fix))
	if len(origParts) == 0 || len(fixParts) <= len(origParts) {
		return false
	}
	for i := 0; i < len(origParts); i++ {
		if origParts[i] != fixParts[i] {
			return false
		}
	}
	return true
}

func shouldRewriteCommand(original, fix string) bool {
	orig := strings.TrimSpace(original)
	fx := strings.TrimSpace(fix)
	if orig == "" || fx == "" {
		return false
	}
	// Docker apt-get update rewrite
	if strings.HasPrefix(orig, "docker run") && strings.HasPrefix(fx, "docker run") {
		if strings.Contains(orig, "npm install") && strings.Contains(fx, "npm install") {
			return true
		}
		origImg, origCmd, ok1 := dockerImageAndCmd(orig)
		fixImg, _, ok2 := dockerImageAndCmd(fx)
		if ok1 && ok2 && origImg == fixImg {
			if strings.Contains(fx, origCmd) && strings.Contains(fx, "apt-get update") {
				return true
			}
			if strings.Contains(fx, origCmd) && strings.Contains(fx, "npm cache clean --force") {
				return true
			}
			if strings.Contains(fx, origCmd) && strings.Contains(fx, "npm_config_cache=") {
				return true
			}
		}
		// Allow npm rewrites that change the image (e.g. node:18-slim -> node:18)
		if ok1 && ok2 && strings.Contains(fx, origCmd) && strings.Contains(fx, "npm install") {
			return true
		}
	}
	return false
}

func dockerImageAndCmd(cmd string) (string, string, bool) {
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
	if idx+1 >= len(parts) {
		return image, "", false
	}
	containerCmd := strings.Join(parts[idx+1:], " ")
	return image, containerCmd, true
}

// NewAgent creates a new repair agent
func NewAgent(config *types.Config) (*Agent, error) {
	adapter, err := adapters.GetAdapter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	return &Agent{
		config:       config,
		adapter:      adapter,
		appliedFixes: make(map[string]bool),
	}, nil
}

// RunWithAutoRepair runs command and auto-repairs on failure
func (a *Agent) RunWithAutoRepair(command string) (*types.CommandResult, error) {
	logger.Init()

	logger.Printf("Running: %s\n", command)
	logger.Write("START", "Running command: "+command)

	result, err := runner.RunCommand(command)
	if err != nil {
		logger.Errorln("Error running command:", err)
		return result, err
	}

	combinedLogs := result.Stdout + result.Stderr
	hasFailure, failureReason := context.CheckForFailure(result.ExitCode, combinedLogs)

	if !hasFailure {
		logger.Println("✔ Success")
		logger.Write("SUCCESS", fmt.Sprintf("Command succeeded: %s", command))
		return result, nil
	}

	logger.Printf("❌ %s\n", failureReason)
	logger.Write("FAILURE", fmt.Sprintf("%s: %s", failureReason, command))

	// Retry bucketing is per failure signature.
	// If a fix changes the failure, we start a new bucket (up to MaxRetries for that signature).
	sigAttempts := make(map[string]int)
	totalRounds := 0
	globalMaxRounds := a.config.MaxRetries * 10

	for totalRounds < globalMaxRounds {
		totalRounds++

		ctx := context.FromCollect(command, result.ExitCode, combinedLogs)
		sig := context.FailureSignature(ctx)
		sigAttempts[sig]++
		attempt := sigAttempts[sig]
		if attempt > a.config.MaxRetries {
			logger.Printf("\n❌ Max retries (%d) reached for this failure\n", a.config.MaxRetries)
			logger.Write("MAX_RETRIES", fmt.Sprintf("Max retries reached for failure signature: %s", sig))
			return result, nil
		}

		if a.config.InDocker {
			logger.Printf("[healrun] self-healing activated (attempt %d/%d)\n", attempt, a.config.MaxRetries)
		} else {
			logger.Printf("\n🤖 Self-healing activated (attempt %d/%d)\n", attempt, a.config.MaxRetries)
		}

		summary := context.FailureSummary(ctx)
		if summary != "" {
			logger.Printf("Detected: %s\n", summary)
		}

		if a.config.Debug {
			logger.Debugf("Collecting context for repair...")
		}

		if a.config.Debug {
			logger.Debugf("Requesting fix from model adapter...")
		}

		fixCommands, err := a.adapter.SuggestFix(ctx)
		if err != nil {
			if err.Error() == "repair stopped by model - no applicable fix" {
				logger.Printf("⚠️  This failure cannot be automatically repaired\n")
				logger.Printf("ℹ️  The package may be deprecated, incompatible, or already in standard library\n")
				logger.Write("STOP_REPAIR", "Model indicated no applicable fix exists")
			} else {
				logger.Printf("⚠️  Error getting fix suggestions: %v\n", err)
				logger.Write("ERROR", fmt.Sprintf("Model adapter error: %v", err))
			}
			return result, nil
		}

		if len(fixCommands) == 0 {
			logger.Printf("❌ No fixes suggested\n")
			logger.Write("NO_FIXES", "No fix commands suggested")
			return result, nil
		}

		if a.config.InDocker {
			logger.Printf("[healrun] Suggested fixes:\n")
			for _, fix := range fixCommands {
				logger.Printf("[healrun]   - %s\n", fix)
			}
		} else {
			logger.Printf("\nSuggested fixes:\n")
			for i, fix := range fixCommands {
				logger.Printf("  %d. %s\n", i+1, fix)
			}
		}

		if a.config.DryRun {
			logger.Printf("\n(Dry run - skipping fix application)\n")
			return result, nil
		}

		approved := true
		if !a.config.InDocker && !a.config.AutoApprove {
			logger.Printf("\n")
			approved = safety.GetConfirmation("Apply these fixes?", a.config.InDocker)
		}

		if !approved {
			logger.Printf("❌ Repairs cancelled\n")
			logger.Write("CANCELLED", "Repairs cancelled by user")
			return result, nil
		}

		logger.Printf("\nApplying fixes...\n")

		allApplied := true
		appliedAny := false
		for _, fix := range fixCommands {
			if a.appliedFixes[fix] {
				continue
			}

			// If the "fix" is a modified version of the original command (e.g. adds flags),
			// update the command we retry so we don't immediately fail again.
			if strings.HasPrefix(fix, command+" ") || isCommandRewrite(command, fix) {
				logger.Write("CMD_UPDATE", fmt.Sprintf("Updating command: %s -> %s", command, fix))
				command = fix
				a.appliedFixes[fix] = true
				appliedAny = true
				continue
			}

			// Heuristic: if a fix only swaps the executable but keeps the same args (common for python venv),
			// treat it as a command replacement.
			if shouldReplaceExecutable(command, fix) {
				logger.Write("CMD_UPDATE", fmt.Sprintf("Updating command: %s -> %s", command, fix))
				command = fix
				a.appliedFixes[fix] = true
				appliedAny = true
				continue
			}

			// Heuristic: treat certain fixes as a rewrite of the original command.
			if shouldRewriteCommand(command, fix) {
				logger.Write("CMD_UPDATE", fmt.Sprintf("Updating command: %s -> %s", command, fix))
				command = fix
				a.appliedFixes[fix] = true
				appliedAny = true
				continue
			}

			cwd, _ := os.Getwd()
			if !safety.ApplyFix(fix, cwd, a.config.AutoApprove, a.config.InDocker) {
				allApplied = false
				continue
			}
			// We are attempting a fix command now
			appliedAny = true

			if a.config.InDocker {
				logger.Printf("[healrun] Applying: %s\n", fix)
			} else {
				logger.Printf("  → %s", fix)
			}

			fixResult, err := runner.RunCommand(fix)
			if err != nil {
				logger.Printf("  ⚠️  Error running fix: %v\n", err)
				allApplied = false
				logger.Write("FIX_FAILED", fmt.Sprintf("Fix failed: %s", fix))
				continue
			}

			if !fixResult.Success {
				logger.Printf("  ⚠️  Fix command failed\n")
				allApplied = false
				logger.Write("FIX_FAILED", fmt.Sprintf("Fix failed: %s", fix))
			} else {
				if a.config.InDocker {
					logger.Printf("[healrun] Fix applied: %s\n", fix)
				} else {
					logger.Printf("  ✔ Fix applied\n")
				}
				a.appliedFixes[fix] = true
				logger.Write("FIX_APPLIED", fmt.Sprintf("Fix applied: %s", fix))
			}
		}

		if !appliedAny {
			logger.Printf("\n❌ No fixes could be applied safely\n")
			logger.Write("NO_FIX_APPLIED", "No fix commands were applied")
			return result, nil
		}
		if !allApplied {
			logger.Printf("\n⚠️  Some fixes failed; retrying anyway\n")
			logger.Write("FIX_PARTIAL", "Some fix commands failed")
		}

		logger.Printf("\n")

		if a.config.Debug {
			logger.Debugf("Waiting 1 second before retry...")
		}

		time.Sleep(1 * time.Second)

		logger.Printf("Retrying command (attempt %d/%d)...\n", attempt, a.config.MaxRetries)
		logger.Write("RETRY", fmt.Sprintf("Retrying command (attempt %d): %s", attempt, command))

		result, err = runner.RunCommand(command)
		if err != nil {
			logger.Write("ERROR", fmt.Sprintf("Error running command: %v", err))
			continue
		}

		combinedLogs = result.Stdout + result.Stderr
		hasFailure, failureReason = context.CheckForFailure(result.ExitCode, combinedLogs)

		if !hasFailure {
			logger.Printf("✔ Success after repair\n")
			logger.Write("SUCCESS", fmt.Sprintf("Command succeeded after repair: %s", command))
			return result, nil
		} else {
			logger.Printf("❌ Still failing: %s\n", failureReason)
		}
	}

	logger.Printf("\n❌ Max repair rounds reached\n")
	logger.Write("MAX_ROUNDS", fmt.Sprintf("Max rounds reached for: %s", command))
	return result, nil
}
