package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileConfig represents the YAML configuration file.
type FileConfig struct {
	APIKeys  APIKeys  `yaml:"api_keys"`
	Model    Model    `yaml:"model"`
	Policies Policies `yaml:"policies"`
}

type APIKeys struct {
	OpenAI string `yaml:"openai"`
}

type Model struct {
	Provider    string `yaml:"provider"`
	OpenAIModel string `yaml:"openai_model"`
	OllamaHost  string `yaml:"ollama_host"`
	OllamaModel string `yaml:"ollama_model"`
}

type Policies struct {
	Allowed []string `yaml:"allowed"`
	Blocked []string `yaml:"blocked"`
}

// DefaultPath returns the default config path (~/.healrun/config.yaml).
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".healrun", "config.yaml"), nil
}

// Load reads the config file from path (or default path when empty).
// Missing config is not an error.
func Load(path string) (*FileConfig, string, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, "", err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileConfig{}, path, nil
		}
		return nil, path, err
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, path, err
	}

	return &cfg, path, nil
}
