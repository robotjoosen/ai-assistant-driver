package phases

import (
	"context"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
)

const ErrorResponse = "Sorry, I couldn't process that request."

type ThinkingPhase struct {
	aiClient ai.Client
}

func NewThinkingPhase(aiClient ai.Client) *ThinkingPhase {
	return &ThinkingPhase{
		aiClient: aiClient,
	}
}

func (p *ThinkingPhase) Run(ctx context.Context, transcript string) string {
	response, err := p.aiClient.Chat(ctx, transcript)
	if err != nil {
		slog.Error("failed to get LLM response", "error", err)
		return ErrorResponse
	}

	slog.Info("thinking phase completed", "response_length", len(response))
	return response
}
