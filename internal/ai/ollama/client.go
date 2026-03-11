package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
)

type Config struct {
	Host          string
	Port          int
	Model         string
	SystemMessage string
}

type OllamaClient struct {
	httpClient    *http.Client
	baseURL       string
	model         string
	systemMessage string
}

func NewClient(cfg Config) ai.Client {
	return &OllamaClient{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL:       fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
		model:         cfg.Model,
		systemMessage: cfg.SystemMessage,
	}
}

type GenerateRequest struct {
	Model     string  `json:"model"`
	Prompt    string  `json:"prompt"`
	Stream    bool    `json:"stream"`
	System    string  `json:"system,omitempty"`
	Format    string  `json:"format,omitempty"`
	KeepAlive *string `json:"keep_alive,omitempty"`
}

type GenerateResponse struct {
	Response string `json:"response"`
}

func (c *OllamaClient) Chat(ctx context.Context, prompt string) (string, error) {
	reqBody := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}

	if c.systemMessage != "" {
		reqBody.System = c.systemMessage
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return genResp.Response, nil
}
