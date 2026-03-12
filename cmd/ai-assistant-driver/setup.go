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
	"github.com/robotjoosen/ai-assistant-driver/internal/tts"
)

func loadConfiguration() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.GetLogLevel(),
	}))

	slog.SetDefault(logger)

	return cfg, nil
}

func connectToESPHome(shutdownCtx context.Context, address string) (*esphome.Client, error) {
	slog.Info("connecting to ESPHome device", "address", address)

	client := esphome.NewClient(address)

	if err := client.Connect(shutdownCtx); err != nil {
		return nil, fmt.Errorf("failed to connect to ESPHome device: %w", err)
	}

	if err := client.SubscribeVoiceAssistant(shutdownCtx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to subscribe to voice assistant: %w", err)
	}

	slog.Info("connected to ESPHome device", "address", address)

	return client, nil
}

func newTranscriber(cfg *config.Config) (transcriber.Transcriber, error) {
	if cfg.Wyoming.Host == "" && cfg.Wyoming.Port == 0 {
		return nil, fmt.Errorf("Wyoming configuration is required. Please set WYOMING_HOST and WYOMING_PORT")
	}

	transcriberClient, err := transcriber.NewTranscriber(cfg.Wyoming, cfg.VAD)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Wyoming transcriber: %w", err)
	}

	return transcriberClient, nil
}

func newAIClient(cfg *config.Config) (ai.Client, error) {
	ollamaConfig := ollama.Config{
		Host:          cfg.AI.Host,
		Port:          cfg.AI.Port,
		Model:         cfg.AI.Model,
		SystemMessage: cfg.AI.SystemMessage,
	}

	client := ollama.NewClient(ollamaConfig)
	slog.Info("AI client initialized", "host", cfg.AI.Host, "port", cfg.AI.Port, "model", cfg.AI.Model)

	return client, nil
}

func newTTSSynthesizer(cfg *config.Config) (tts.Synthesizer, *tts.Server, error) {
	synthesizer, err := tts.NewSynthesizer(cfg.Wyoming)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize TTS synthesizer: %w", err)
	}

	server := tts.NewServer(cfg.Wyoming.HTTPHost, cfg.Wyoming.HTTPPort)
	if err := server.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start TTS server: %w", err)
	}

	slog.Info("TTS synthesizer initialized", "piper_host", cfg.Wyoming.PiperHost, "piper_port", cfg.Wyoming.PiperPort)
	slog.Info("TTS server started", "url", server.URL())

	return synthesizer, server, nil
}
