package adapters

import "github.com/healrun/healrun/pkg/types"

// GetAdapter returns the appropriate model adapter based on config
func GetAdapter(config *types.Config) (types.ModelAdapter, error) {
	provider := config.ModelProvider

	switch provider {
	case "openai":
		return NewOpenAIAdapter(config)
	case "ollama":
		return NewOllamaAdapter(config)
	case "dummy":
		return NewAnalyzerAdapter(), nil
	default:
		return NewAnalyzerAdapter(), nil
	}
}

// GetAdapterByName returns adapter by name
func GetAdapterByName(name string, config *types.Config) (types.ModelAdapter, error) {
	switch name {
	case "openai":
		return NewOpenAIAdapter(config)
	case "ollama":
		return NewOllamaAdapter(config)
	case "dummy", "analyzer":
		return NewAnalyzerAdapter(), nil
	default:
		return NewAnalyzerAdapter(), nil
	}
}
