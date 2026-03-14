package phases

import (
	"context"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
	"github.com/robotjoosen/ai-assistant-driver/internal/history"
)

const ErrorResponse = "Sorry, I couldn't process that request."

type ThinkingPhase struct {
	aiClient       ai.Client
	historyManager *history.ConversationManager
}

func NewThinkingPhase(aiClient ai.Client, historyManager *history.ConversationManager) *ThinkingPhase {
	return &ThinkingPhase{
		aiClient:       aiClient,
		historyManager: historyManager,
	}
}

func (p *ThinkingPhase) Run(ctx context.Context, transcript string) string {
	var conversationContext string
	if p.historyManager != nil {
		conversationContext = p.historyManager.GetContext(transcript)
	}

	response, err := p.aiClient.Chat(ctx, transcript, conversationContext)
	if err != nil {
		slog.Error("failed to get LLM response", "error", err)
		return ErrorResponse
	}

	slog.Info("thinking phase completed",
		slog.Int("response_length", len(response)),
	)

	return response
}
