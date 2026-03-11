package phases

import (
	"context"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
)

const ErrorResponse = "Sorry, I couldn't process that request."

type ThinkingPhase struct {
	aiClient ai.Client
	logger   *slog.Logger
}

func NewThinkingPhase(aiClient ai.Client, logger *slog.Logger) *ThinkingPhase {
	return &ThinkingPhase{
		aiClient: aiClient,
		logger:   logger,
	}
}

func (p *ThinkingPhase) Run(ctx context.Context, transcript string) string {
	p.logger.Info("thinking phase started", "transcript", transcript)

	response, err := p.aiClient.Chat(ctx, transcript)
	if err != nil {
		p.logger.Error("failed to get LLM response", "error", err)
		return ErrorResponse
	}

	p.logger.Info("thinking phase completed", "response_length", len(response))
	return response
}
