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

// OpenAIAdapter provides OpenAI model integration
type OpenAIAdapter struct {
	apiKey string
	model  string
	client *http.Client
}

// OpenAIRequest represents OpenAI API request
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

// OpenAIMessage represents a message in OpenAI API
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents OpenAI API response
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// NewOpenAIAdapter creates a new OpenAI adapter
func NewOpenAIAdapter(config *types.Config) (*OpenAIAdapter, error) {
	apiKey := ""
	model := ""
	if config != nil {
		apiKey = config.OpenAIAPIKey
		model = config.OpenAIModel
	}

	if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" {
		apiKey = envKey
	}
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}

	if envModel := os.Getenv("HEALRUN_OPENAI_MODEL"); envModel != "" {
		model = envModel
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &OpenAIAdapter{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// SuggestFix requests fix suggestions from OpenAI
func (o *OpenAIAdapter) SuggestFix(ctx *types.Context) ([]string, error) {
	prompt := buildPrompt(ctx)

	req := OpenAIRequest{
		Model: o.model,
		Messages: []OpenAIMessage{
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
		MaxTokens:   500,
		Temperature: 0.7,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Debugf("OpenAI error response: %s", string(body))
		return nil, fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := apiResp.Choices[0].Message.Content
	cmds, err := parseCommands(content)
	if err != nil {
		return nil, err
	}
	return cmds, nil
}
