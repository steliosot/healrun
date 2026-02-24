package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/healrun/healrun/pkg/agent"
	"github.com/healrun/healrun/pkg/config"
	"github.com/healrun/healrun/pkg/context"
	"github.com/healrun/healrun/pkg/logger"
	"github.com/healrun/healrun/pkg/safety"
	"github.com/healrun/healrun/pkg/types"
)

var (
	autoApprove   = flag.Bool("auto-approve", false, "Automatically apply fixes without confirmation")
	assumeYes     = flag.Bool("y", false, "Assume yes to all fix prompts")
	dryRun        = flag.Bool("dry-run", false, "Show suggested fixes but do not execute")
	debug         = flag.Bool("debug", false, "Enable verbose debug logging")
	maxRetries    = flag.Int("max-retries", 3, "Maximum number of repair attempts")
	modelProvider = flag.String("model", "", "Model provider (openai, ollama, dummy)")
	version       = "0.1.0"
)

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := flag.Arg(0)

	fileConfig, configPath, err := config.Load("")
	if err != nil {
		logger.Errorln("Error loading config:", err)
		os.Exit(1)
	}

	appConfig := &types.Config{
		AutoApprove:   *autoApprove || *assumeYes,
		DryRun:        *dryRun,
		Debug:         *debug,
		MaxRetries:    *maxRetries,
		ModelProvider: *modelProvider,
		InDocker:      context.IsInDocker(),
		ConfigPath:    configPath,
	}

	if appConfig.ModelProvider == "" && fileConfig.Model.Provider != "" {
		appConfig.ModelProvider = fileConfig.Model.Provider
	}
	if appConfig.OpenAIAPIKey == "" && fileConfig.APIKeys.OpenAI != "" {
		appConfig.OpenAIAPIKey = fileConfig.APIKeys.OpenAI
	}
	if appConfig.OpenAIModel == "" && fileConfig.Model.OpenAIModel != "" {
		appConfig.OpenAIModel = fileConfig.Model.OpenAIModel
	}
	if appConfig.OllamaHost == "" && fileConfig.Model.OllamaHost != "" {
		appConfig.OllamaHost = fileConfig.Model.OllamaHost
	}
	if appConfig.OllamaModel == "" && fileConfig.Model.OllamaModel != "" {
		appConfig.OllamaModel = fileConfig.Model.OllamaModel
	}
	appConfig.Policies = types.Policies{
		Allowed: append([]string{}, fileConfig.Policies.Allowed...),
		Blocked: append([]string{}, fileConfig.Policies.Blocked...),
	}

	if envProvider := os.Getenv("HEALRUN_MODEL_PROVIDER"); envProvider != "" {
		appConfig.ModelProvider = envProvider
	}
	if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" {
		appConfig.OpenAIAPIKey = envKey
	}
	if envModel := os.Getenv("HEALRUN_OPENAI_MODEL"); envModel != "" {
		appConfig.OpenAIModel = envModel
	}
	if envHost := os.Getenv("HEALRUN_OLLAMA_HOST"); envHost != "" {
		appConfig.OllamaHost = envHost
	}
	if envModel := os.Getenv("HEALRUN_OLLAMA_MODEL"); envModel != "" {
		appConfig.OllamaModel = envModel
	}

	if appConfig.InDocker || os.Getenv("HEALRUN_AUTO_APPROVE") == "true" {
		appConfig.AutoApprove = true
	}

	// If stdin is not a TTY (e.g., Docker build), auto-approve to avoid blocking prompts.
	if fi, err := os.Stdin.Stat(); err == nil {
		if (fi.Mode() & os.ModeCharDevice) == 0 {
			appConfig.AutoApprove = true
		}
	}

	if os.Getenv("HEALRUN_DEBUG") == "true" {
		appConfig.Debug = true
	}

	if appConfig.ModelProvider == "" {
		appConfig.ModelProvider = "dummy"
	}

	if len(appConfig.Policies.Allowed) > 0 || len(appConfig.Policies.Blocked) > 0 {
		safety.ConfigurePolicies(appConfig.Policies)
	}

	if appConfig.Debug {
		logger.Debugf("Starting healrun v%s", version)
		logger.Debugf("Config: %+v", appConfig)
	}

	a, err := agent.NewAgent(appConfig)
	if err != nil {
		logger.Errorln("Error creating agent:", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sigTerm := make(chan bool, 1)

	go func() {
		<-sigChan
		sigTerm <- true
	}()

	go func() {
		<-sigTerm
		logger.Println("\n❌ Interrupted by user")
		os.Exit(130)
	}()

	result, err := a.RunWithAutoRepair(command)
	if err != nil {
		logger.Errorln("Error:", err)
		os.Exit(1)
	}

	os.Exit(result.ExitCode)
}

func printUsage() {
	fmt.Printf("healrun v%s - Self-healing installation agent\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  healrun \"<command>\"")
	fmt.Println("  healrun [flags] \"<command>\"")
	fmt.Println("\nFlags:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  healrun \"pip install torch\"")
	fmt.Println("  healrun --auto-approve \"npm install\"")
	fmt.Println("  healrun -y \"npm install\"")
	fmt.Println("  healrun --model=openai \"docker build .\"")
	fmt.Println("\nEnvironment Variables:")
	fmt.Println("  HEALRUN_MODEL_PROVIDER=openai|ollama|dummy")
	fmt.Println("  HEALRUN_AUTO_APPROVE=true")
	fmt.Println("  HEALRUN_DEBUG=true")
	fmt.Println("  HEALRUN_FAKE_MODE=true")
}
