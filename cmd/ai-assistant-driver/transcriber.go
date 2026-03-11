package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/wyoming"
)

func connectTranscriber(ctx context.Context, transcriber wyoming.StreamTranscriber, logger *slog.Logger) bool {
	for attempt := 1; attempt <= maxWhisperRetries; attempt++ {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		if err := transcriber.Connect(ctx); err != nil {
			logger.Warn("transcriber connection failed", "attempt", attempt, "max", maxWhisperRetries, "error", err)
			if attempt < maxWhisperRetries {
				select {
				case <-ctx.Done():
					return false
				case <-time.After(retryDelay):
				}
			}
			continue
		}

		logger.Info("connected to transcriber")
		return true
	}

	logger.Error("transcriber connection failed after max retries")
	return false
}

func reconnectTranscriber(ctx context.Context, transcriber wyoming.StreamTranscriber, logger *slog.Logger) bool {
	transcriber.Reset()
	return connectTranscriber(ctx, transcriber, logger)
}

func disconnectTranscriberGraceful(ctx context.Context, transcriber wyoming.StreamTranscriber, client *esphome.Client, logger *slog.Logger) {
	logger.Info("sending audio-stop for final transcript")

	if err := transcriber.SendAudioStop(); err != nil {
		logger.Error("failed to send audio-stop", "error", err)
	}

	transcript, err := transcriber.Recv()
	if err != nil {
		if !transcriber.IsConnected() {
			logger.Debug("connection closed while waiting for transcript")
		} else {
			logger.Error("error receiving transcript", "error", err)
		}
	} else if transcript != nil {
		if transcript.IsFinal {
			logger.Info("final transcription", "text", transcript.Text, "start", transcript.Start, "end", transcript.End)

			if err := client.SendSTTEvent(ctx, false); err != nil {
				logger.Error("failed to send STT_END event to ESPHome", "error", err)
			}
		} else {
			logger.Info("partial transcription", "text", transcript.Text, "start", transcript.Start, "end", transcript.End)
		}
	}

	if err := transcriber.Close(); err != nil {
		logger.Error("error closing transcriber", "error", err)
	}

	logger.Info("transcriber disconnected")
}
