package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/robotjoosen/ai-assistant-driver/internal/llm"
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

func NewClient(cfg Config) llm.Client {
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

func (c *OllamaClient) Chat(ctx context.Context, prompt string, conversationContext string, tools []llm.Tool) (string, []llm.ToolCall, error) {
	messages := []api.Message{}

	if c.systemMessage != "" {
		messages = append(messages, api.Message{
			Role:    "system",
			Content: c.systemMessage,
		})
	}

	if conversationContext != "" {
		messages = append(messages, api.Message{
			Role:    "user",
			Content: conversationContext,
		})
	}

	messages = append(messages, api.Message{
		Role:    "user",
		Content: prompt,
	})

	ollamaTools := c.convertTools(tools)

	req := &api.ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   func(b bool) *bool { return &b }(false),
		Tools:    ollamaTools,
	}

	var responseText string
	var toolCalls []llm.ToolCall

	err := c.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		responseText = resp.Message.Content

		if len(resp.Message.ToolCalls) > 0 {
			for _, tc := range resp.Message.ToolCalls {
				argsMap := make(map[string]any)
				for k, v := range tc.Function.Arguments.ToMap() {
					argsMap[k] = v
				}

				toolCalls = append(toolCalls, llm.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: argsMap,
				})
			}
		}
		return nil
	})

	if err != nil {
		return "", nil, fmt.Errorf("failed to chat: %w", err)
	}

	if responseText == "" && len(toolCalls) == 0 {
		return "", nil, fmt.Errorf("empty response from LLM")
	}

	return responseText, toolCalls, nil
}

func (c *OllamaClient) convertTools(tools []llm.Tool) api.Tools {
	if len(tools) == 0 {
		return nil
	}

	ollamaTools := make(api.Tools, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters: api.ToolFunctionParameters{
					Type:       "object",
					Properties: convertToolParams(tool.Parameters()),
					Required:   extractRequiredParams(tool.Parameters()),
				},
			},
		}
	}

	return ollamaTools
}

func convertToolParams(params map[string]any) *api.ToolPropertiesMap {
	if params == nil {
		return nil
	}

	props := api.NewToolPropertiesMap()

	if obj, ok := params["properties"].(map[string]any); ok {
		for name, prop := range obj {
			if propMap, ok := prop.(map[string]any); ok {
				tp := api.ToolProperty{}
				if t, ok := propMap["type"].(string); ok {
					tp.Type = api.PropertyType{t}
				}
				if desc, ok := propMap["description"].(string); ok {
					tp.Description = desc
				}
				if enum, ok := propMap["enum"].([]any); ok {
					tp.Enum = enum
				}
				props.Set(name, tp)
			}
		}
	}

	return props
}

func extractRequiredParams(params map[string]any) []string {
	if req, ok := params["required"].([]any); ok {
		result := make([]string, len(req))
		for i, r := range req {
			if s, ok := r.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	return nil
}
