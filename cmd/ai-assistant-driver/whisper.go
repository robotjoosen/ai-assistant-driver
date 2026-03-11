package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/whisper"
)

func connectWhisper(ctx context.Context, transcriber whisper.StreamTranscriber, logger *slog.Logger) bool {
	for attempt := 1; attempt <= maxWhisperRetries; attempt++ {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		if err := transcriber.Connect(ctx); err != nil {
			logger.Warn("whisper connection failed", "attempt", attempt, "max", maxWhisperRetries, "error", err)
			if attempt < maxWhisperRetries {
				select {
				case <-ctx.Done():
					return false
				case <-time.After(retryDelay):
				}
			}
			continue
		}

		logger.Info("connected to whisper transcriber")
		return true
	}

	logger.Error("whisper connection failed after max retries")
	return false
}

func reconnectWhisper(ctx context.Context, transcriber whisper.StreamTranscriber, logger *slog.Logger) bool {
	transcriber.Reset()
	return connectWhisper(ctx, transcriber, logger)
}

func disconnectWhisper(ctx context.Context, transcriber whisper.StreamTranscriber, logger *slog.Logger) {
	transcriber.Close()
	logger.Info("whisper disconnected")
}

func disconnectWhisperGraceful(ctx context.Context, transcriber whisper.StreamTranscriber, client *esphome.Client, logger *slog.Logger) {
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
		logger.Error("error closing whisper", "error", err)
	}

	logger.Info("whisper disconnected")
}

func startWhisperReceiveLoop(ctx context.Context, transcriber whisper.StreamTranscriber, logger *slog.Logger) {
	defer func() {
		logger.Debug("whisper receive loop ended")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			transcript, err := transcriber.Recv()
			if err != nil {
				if !transcriber.IsConnected() {
					return
				}
				logger.Error("whisper receive error", "error", err)
				return
			}

			if transcript == nil {
				continue
			}

			if transcript.IsFinal {
				logger.Info("final transcription", "text", transcript.Text, "start", transcript.Start, "end", transcript.End)
			} else {
				logger.Info("partial transcription", "text", transcript.Text, "start", transcript.Start, "end", transcript.End)
			}
		}
	}
}
