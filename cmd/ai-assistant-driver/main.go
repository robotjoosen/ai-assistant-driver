package main

import (
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/controller"
	"github.com/robotjoosen/ai-assistant-driver/internal/shutdown"
)

func main() {
	cfg, err := loadConfiguration()
	if err != nil {
		slog.Error("setup failed", "error", err)
		return
	}

	slog.Info("starting AI Assistant Driver", "address", cfg.ESPHomeAddress)

	shutdownMgr := shutdown.New()

	esphomeClient, err := connectToESPHome(shutdownMgr.Context(), cfg.ESPHomeAddress)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { esphomeClient.Close() })

	sttClient, err := newSTTTranscriber(shutdownMgr.Context(), cfg)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { sttClient.Close() })

	llmClient, err := newLLMClient(cfg)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}

	ttsSynthesizer, err := newTTSSynthesizer(shutdownMgr.Context(), cfg)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}

	historyManager, err := newHistoryManager(cfg, llmClient)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}

	toolExecutor := newToolExecutor(cfg)

	ctrl := controller.New(
		controller.Config{
			STT:            sttClient,
			LLMClient:      llmClient,
			TTSSynthesizer: ttsSynthesizer,
			HistoryManager: historyManager,
			ToolExecutor:   toolExecutor,
			Conversational: controller.ConversationalConfig{
				StoragePath: cfg.Conversational.StoragePath,
			},
		},
		esphomeClient.Events(),
		esphomeClient.AudioEvents(),
		esphomeClient.MediaPlayerEvents(),
		esphomeClient.Commands(),
	)

	httpServer, err := newHTTPServer(cfg, ttsSynthesizer, ctrl)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { httpServer.Close() })

	slog.Info("listening for voice assistant events and audio")

	go ctrl.Run(shutdownMgr.Context())

	<-shutdownMgr.Done()
}
