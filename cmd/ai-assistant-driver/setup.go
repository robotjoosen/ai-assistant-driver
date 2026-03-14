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
	"github.com/robotjoosen/ai-assistant-driver/internal/history"
	"github.com/robotjoosen/ai-assistant-driver/internal/stt"
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

	if err := client.ConnectWithRetry(shutdownCtx); err != nil {
		return nil, fmt.Errorf("failed to connect to ESPHome device: %w", err)
	}

	if err := client.SubscribeVoiceAssistant(shutdownCtx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to subscribe to voice assistant: %w", err)
	}

	client.StartReconnectionHandler(shutdownCtx)

	slog.Info("connected to ESPHome device", "address", address)

	return client, nil
}

func newSTTTranscriber(ctx context.Context, cfg *config.Config) (stt.Transcriber, error) {
	if cfg.Conversational.Host == "" && cfg.Conversational.Port == 0 {
		return nil, fmt.Errorf("STT configuration is required. Please set STT_HOST and STT_PORT")
	}

	sttClient, err := stt.NewTranscriber(cfg.Conversational, cfg.VAD)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize STT: %w", err)
	}

	slog.Info("STT service initialized", "host", cfg.Conversational.Host, "port", cfg.Conversational.Port)

	return sttClient, nil
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

func newTTSSynthesizer(ctx context.Context, cfg *config.Config) (tts.Synthesizer, *tts.Server, error) {
	synthesizer, err := tts.NewSynthesizer(cfg.Conversational)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize TTS synthesizer: %w", err)
	}

	server := tts.NewServer(cfg.Conversational.HTTPHost, cfg.Conversational.HTTPPort)
	if err := server.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start TTS server: %w", err)
	}

	slog.Info("TTS synthesizer initialized", "tts_host", cfg.Conversational.SynthesizerHost, "tts_port", cfg.Conversational.SynthesizerPort)
	slog.Info("TTS server started", "url", server.URL())

	return synthesizer, server, nil
}

func newHistoryManager(cfg *config.Config, aiClient ai.Client) (*history.ConversationManager, error) {
	manager, err := history.NewConversationManager(cfg.Conversational.HistoryStoragePath, aiClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize history manager: %w", err)
	}

	slog.Info("history manager initialized", "storage_path", cfg.Conversational.HistoryStoragePath)

	return manager, nil
}
