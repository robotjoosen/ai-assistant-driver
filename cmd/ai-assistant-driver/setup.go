package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
	"github.com/robotjoosen/ai-assistant-driver/internal/ai/ollama"
	"github.com/robotjoosen/ai-assistant-driver/internal/config"
	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/transcriber"
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

func newTranscriber(cfg *config.Config, logger *slog.Logger) (transcriber.Transcriber, error) {
	if cfg.Wyoming.Host == "" && cfg.Wyoming.Port == 0 {
		return nil, fmt.Errorf("Wyoming configuration is required. Please set WYOMING_HOST and WYOMING_PORT")
	}

	transcriberClient, err := transcriber.NewTranscriber(cfg.Wyoming, cfg.VAD, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Wyoming transcriber: %w", err)
	}

	return transcriberClient, nil
}

func newAIClient(cfg *config.Config, logger *slog.Logger) (ai.Client, error) {
	ollamaConfig := ollama.Config{
		Host:          cfg.AI.Host,
		Port:          cfg.AI.Port,
		Model:         cfg.AI.Model,
		SystemMessage: cfg.AI.SystemMessage,
	}

	client := ollama.NewClient(ollamaConfig)
	logger.Info("AI client initialized", "host", cfg.AI.Host, "port", cfg.AI.Port, "model", cfg.AI.Model)

	return client, nil
}
