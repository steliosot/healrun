package types

// CommandResult represents the result of running a command
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Success  bool
}

// Context represents system environment context for LLM analysis
type Context struct {
	OS             string `json:"os"`
	Distro         string `json:"distro,omitempty"`
	Architecture   string `json:"architecture"`
	Shell          string `json:"shell"`
	PackageManager string `json:"package_manager,omitempty"`
	Command        string `json:"command"`
	ExitCode       int    `json:"exit_code"`
	Logs           string `json:"logs"`
	CWD            string `json:"cwd"`
	InDocker       bool   `json:"in_docker"`
	InCI           bool   `json:"in_ci"`
}

// Config represents the agent configuration
type Config struct {
	AutoApprove      bool
	DryRun           bool
	Debug            bool
	MaxRetries       int
	ModelProvider    string
	OpenAIAPIKey     string
	OpenAIModel      string
	OllamaHost       string
	OllamaModel      string
	ForceAutoApprove bool
	InDocker         bool
	Policies         Policies
	ConfigPath       string
}

// Policies represents additional allow/block rules from config.
type Policies struct {
	Allowed []string
	Blocked []string
}

// ModelAdapter is the interface for different model providers
type ModelAdapter interface {
	SuggestFix(ctx *Context) ([]string, error)
}

// RepairStatus represents the status of a repair attempt
type RepairStatus int

const (
	RepairStatusUnknown RepairStatus = iota
	RepairStatusSuccess
	RepairStatusFailed
	RepairStatusCancelled
)
