package phases

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/history"
	"github.com/robotjoosen/ai-assistant-driver/internal/llm"
)

const ErrorResponse = "Sorry, I couldn't process that request."

type ThinkingPhase struct {
	llmClient      llm.Client
	historyManager *history.ConversationManager
	toolExecutor   *llm.ToolExecutor
}

func NewThinkingPhase(llmClient llm.Client, historyManager *history.ConversationManager, toolExecutor *llm.ToolExecutor) *ThinkingPhase {
	return &ThinkingPhase{
		llmClient:      llmClient,
		historyManager: historyManager,
		toolExecutor:   toolExecutor,
	}
}

func (p *ThinkingPhase) Run(ctx context.Context, transcript string) string {
	var conversationContext string
	if p.historyManager != nil {
		conversationContext = p.historyManager.GetContext(transcript)
	}

	tools := p.toolExecutor.GetTools()

	response, toolCalls, err := p.llmClient.Chat(ctx, transcript, conversationContext, tools)
	if err != nil {
		slog.Error("failed to get LLM response", "error", err)
		return ErrorResponse
	}

	for len(toolCalls) > 0 && len(response) == 0 {
		slog.Info("tool calls detected", "count", len(toolCalls))

		results := p.toolExecutor.ExecuteAll(ctx, toolCalls)

		for _, result := range results {
			slog.Debug("tool result", "call_id", result.CallID, "result", result.Result, "error", result.Error)
		}

		conversationContext = p.buildConversationContextWithToolResults(conversationContext, toolCalls, results)

		response, toolCalls, err = p.llmClient.Chat(ctx, "", conversationContext, tools)
		if err != nil {
			slog.Error("failed to get LLM response after tool execution", "error", err)
			return ErrorResponse
		}
	}

	slog.Info("thinking phase completed",
		slog.Int("response_length", len(response)),
	)

	return response
}

func (p *ThinkingPhase) buildConversationContextWithToolResults(
	conversationContext string,
	calls []llm.ToolCall,
	results []llm.ToolResult,
) string {
	var contextWithResults string

	if conversationContext != "" {
		contextWithResults = conversationContext + "\n\n"
	}

	for i, call := range calls {
		var result string
		if i < len(results) {
			if results[i].Error != nil {
				result = "Error: " + results[i].Error.Error()
			} else {
				result = results[i].Result
			}
		}
		contextWithResults += toolResultMessage(call.Name, call.Arguments, result)
	}

	return contextWithResults
}

func toolResultMessage(toolName string, args map[string]any, result string) string {
	argsStr := ""
	for k, v := range args {
		if argsStr != "" {
			argsStr += ", "
		}
		argsStr += fmt.Sprintf("%s=%v", k, v)
	}

	return fmt.Sprintf("Tool %s(%s) returned: %s\n", toolName, argsStr, result)
}
