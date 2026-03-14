package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/robotjoosen/ai-assistant-driver/internal/config"
	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/history"
	"github.com/robotjoosen/ai-assistant-driver/internal/llm"
	"github.com/robotjoosen/ai-assistant-driver/internal/llm/ollama"
	"github.com/robotjoosen/ai-assistant-driver/internal/stt"
	"github.com/robotjoosen/ai-assistant-driver/internal/tools/openwrt"
	"github.com/robotjoosen/ai-assistant-driver/internal/tools/speedtest"
	"github.com/robotjoosen/ai-assistant-driver/internal/tools/weather"
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

func newLLMClient(cfg *config.Config) (llm.Client, error) {
	ollamaConfig := ollama.Config{
		Host:          cfg.LLM.Host,
		Port:          cfg.LLM.Port,
		Model:         cfg.LLM.Model,
		SystemMessage: cfg.LLM.SystemMessage,
	}

	client := ollama.NewClient(ollamaConfig)
	slog.Info("LLM client initialized", "host", cfg.LLM.Host, "port", cfg.LLM.Port, "model", cfg.LLM.Model)

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

func newHistoryManager(cfg *config.Config, llmClient llm.Client) (*history.ConversationManager, error) {
	manager, err := history.NewConversationManager(cfg.Conversational.HistoryStoragePath, llmClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize history manager: %w", err)
	}

	slog.Info("history manager initialized", "storage_path", cfg.Conversational.HistoryStoragePath)

	return manager, nil
}

func newToolExecutor(cfg *config.Config) *llm.ToolExecutor {
	executor := llm.NewToolExecutor()

	openwrtClient := openwrt.NewClient(
		cfg.OpenWrt.Host,
		cfg.OpenWrt.Port,
		cfg.OpenWrt.Username,
		cfg.OpenWrt.Password,
	)
	openwrt.Register(openwrtClient)

	weatherClient := weather.NewClient(
		cfg.Weather.Latitude,
		cfg.Weather.Longitude,
	)
	weather.Register(weatherClient)

	speedtestClient := speedtest.NewClient()
	speedtest.Register(speedtestClient)

	registeredTools := llm.GetTools()
	if len(registeredTools) > 0 {
		executor.Register(registeredTools...)
		slog.Info("tool executor initialized", "tool_count", len(registeredTools))
	} else {
		slog.Info("tool executor initialized", "tool_count", 0)
	}

	return executor
}
