package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/healrun/healrun/pkg/logger"
	"github.com/healrun/healrun/pkg/types"
)

// OllamaAdapter provides Ollama local model integration
type OllamaAdapter struct {
	host   string
	model  string
	client *http.Client
}

// OllamaRequest represents Ollama API request
type OllamaRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Options  map[string]interface{} `json:"options,omitempty"`
	Stream   bool                   `json:"stream"`
}

// OllamaMessage represents a message in Ollama API
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaResponse represents Ollama API response
type OllamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error,omitempty"`
}

// NewOllamaAdapter creates a new Ollama adapter
func NewOllamaAdapter(config *types.Config) (*OllamaAdapter, error) {
	host := ""
	model := ""
	if config != nil {
		host = config.OllamaHost
		model = config.OllamaModel
	}

	if envHost := os.Getenv("HEALRUN_OLLAMA_HOST"); envHost != "" {
		host = envHost
	}
	if host == "" {
		host = "http://localhost:11434"
	}

	if envModel := os.Getenv("HEALRUN_OLLAMA_MODEL"); envModel != "" {
		model = envModel
	}
	if model == "" {
		model = "llama3.2"
	}

	return &OllamaAdapter{
		host:  host,
		model: model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// SuggestFix requests fix suggestions from Ollama
func (o *OllamaAdapter) SuggestFix(ctx *types.Context) ([]string, error) {
	prompt := buildPrompt(ctx)

	req := OllamaRequest{
		Model: o.model,
		Messages: []OllamaMessage{
			{
				Role: "system",
				Content: "You are an installation-repair agent. Follow the user instructions exactly. " +
					"Return only executable shell commands or STOP_REPAIR, no explanations.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Options: map[string]interface{}{
			"num_predict": 300,
			"temperature": 0.7,
		},
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", o.host)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Debugf("Ollama error response: %s", string(body))
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Error != "" {
		return nil, fmt.Errorf("Ollama API error: %s", apiResp.Error)
	}

	content := apiResp.Message.Content
	cmds, err := parseCommands(content)
	if err != nil {
		return nil, err
	}
	return cmds, nil
}
