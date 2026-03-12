package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
)

type Config struct {
	Host          string
	Port          int
	Model         string
	SystemMessage string
}

type OllamaClient struct {
	client        *api.Client
	model         string
	systemMessage string
}

func NewClient(cfg Config) ai.Client {
	baseURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
	}

	return &OllamaClient{
		client:        api.NewClient(baseURL, &http.Client{Timeout: 120 * time.Second}),
		model:         cfg.Model,
		systemMessage: cfg.SystemMessage,
	}
}

func (c *OllamaClient) Chat(ctx context.Context, prompt string) (string, error) {
	stream := false
	req := &api.GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: &stream,
		Think:  &api.ThinkValue{Value: false},
	}

	if c.systemMessage != "" {
		req.System = c.systemMessage
	}

	var response string
	err := c.client.Generate(ctx, req, func(r api.GenerateResponse) error {
		response = r.Response
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate: %w", err)
	}

	if response == "" {
		return "", fmt.Errorf("empty response from LLM")
	}

	return response, nil
}
