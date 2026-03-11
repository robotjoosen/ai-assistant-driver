package main

import (
	"context"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/whisper"
)

func handleVoiceAssistantEvent(
	ctx context.Context,
	event esphome.VoiceAssistantEvent,
	state *appState,
	transcriber whisper.StreamTranscriber,
	client *esphome.Client,
	logger *slog.Logger,
) bool {
	logger.Info("voice assistant event", "phase", event.Phase.String(), "error", event.Error)

	switch event.Phase {
	case esphome.VoiceAssistantPhaseListening:
		if state.streaming || state.whisperFailed {
			return false
		}

		transcriber.ResetVAD()

		if !connectWhisper(ctx, transcriber, logger) {
			state.whisperRetries++
			if state.whisperRetries >= maxWhisperRetries {
				logger.Error("whisper connection failed after max retries, falling back to listening only")
				state.whisperFailed = true
			}
			return false
		}

		state.whisperRetries = 0
		state.streaming = true
		state.audioSent = false

		if err := client.SendSTTEvent(ctx, true); err != nil {
			logger.Error("failed to send STT_START event to ESPHome", "error", err)
		}

		return true
	case esphome.VoiceAssistantPhaseThinking:
		if state.streaming {
			disconnectWhisperGraceful(ctx, transcriber, client, logger)
			state.streaming = false
			transcriber.Reset()
		}

		return false
	case esphome.VoiceAssistantPhaseIdle, esphome.VoiceAssistantPhaseError:
		if state.streaming {
			disconnectWhisperGraceful(ctx, transcriber, client, logger)
			state.streaming = false
			transcriber.Reset()
		}

		return false
	default:
		return false
	}
}

func handleAudioEvent(
	ctx context.Context,
	audio esphome.AudioEvent,
	state *appState,
	transcriber whisper.StreamTranscriber,
	client *esphome.Client,
	logger *slog.Logger,
) bool {
	logger.Debug("received audio", "size", len(audio.Data), "end", audio.End)

	if !state.streaming || len(audio.Data) == 0 {
		return false
	}

	if err := transcriber.SendAudio(audio.Data); err != nil {
		logger.Error("failed to send audio to whisper", "error", err)
		state.streaming = false
		transcriber.Reset()

		state.whisperRetries++
		if state.whisperRetries >= maxWhisperRetries {
			logger.Error("whisper reconnection failed after max retries, falling back to listening only")
			state.whisperFailed = true
			return false
		}

		logger.Warn("whisper send failed, attempting reconnect", "attempt", state.whisperRetries, "max", maxWhisperRetries)

		if reconnectWhisper(ctx, transcriber, logger) {
			state.streaming = true
			return true
		}

		return false
	}

	if !state.audioSent {
		state.audioSent = true
		if err := client.SendVADEvent(ctx, false); err != nil {
			logger.Error("failed to send STT_VAD_START event to ESPHome", "error", err)
		}
	}

	if transcriber.SilenceDetected() {
		logger.Info("VAD detected end of speech, notifying ESPHome to stop listening")

		if err := client.SendVADEvent(ctx, true); err != nil {
			logger.Error("failed to send VAD event to ESPHome", "error", err)
		}

		disconnectWhisperGraceful(ctx, transcriber, client, logger)
		state.streaming = false
		transcriber.Reset()
		return false
	}

	if audio.End {
		logger.Info("audio stream ended")
	}

	return false
}
