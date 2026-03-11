package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/robotjoosen/ai-assistant-driver/internal/config"
	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/whisper"
	"github.com/robotjoosen/ai-assistant-driver/internal/wyoming"
)

func loadConfiguration() (*slog.Logger, *config.Config) {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.GetLogLevel(),
	}))

	slog.SetDefault(logger)

	return logger, cfg
}

func connectToESPHome(shutdownCtx context.Context, address string, logger *slog.Logger) (*esphome.Client, error) {
	logger.Info("connecting to ESPHome device", "address", address)

	client := esphome.NewClient(address, logger)

	if err := client.Connect(shutdownCtx); err != nil {
		return nil, fmt.Errorf("failed to connect to ESPHome device: %w", err)
	}

	if err := client.SubscribeVoiceAssistant(shutdownCtx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to subscribe to voice assistant: %w", err)
	}

	logger.Info("connected to ESPHome device", "address", address)

	return client, nil
}

func newWhisperTranscriber(cfg *config.Config, logger *slog.Logger) (whisper.StreamTranscriber, error) {
	if cfg.Wyoming.Host != "" || cfg.Wyoming.Port != 0 {
		transcriber, err := wyoming.NewTranscriber(cfg.Wyoming, cfg.VAD, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Wyoming transcriber: %w", err)
		}
		return transcriber, nil
	}

	transcriber, err := whisper.NewWebSocketTranscriber(cfg.Whisper, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize whisper transcriber: %w", err)
	}

	return transcriber, nil
}
