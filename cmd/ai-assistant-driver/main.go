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

	transcriberClient, err := newTranscriber(cfg)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { transcriberClient.Close() })

	aiClient, err := newAIClient(cfg)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}

	ttsSynthesizer, ttsServer, err := newTTSSynthesizer(cfg)
	if err != nil {
		slog.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { ttsServer.Close() })

	ctrl := controller.New(
		controller.Config{
			Transcriber:    transcriberClient,
			AIClient:       aiClient,
			TTSSynthesizer: ttsSynthesizer,
			TTSServer:      ttsServer,
		},
		esphomeClient.Events(),
		esphomeClient.AudioEvents(),
		esphomeClient.MediaPlayerEvents(),
		esphomeClient.Commands(),
	)

	slog.Info("listening for voice assistant events and audio")

	go ctrl.Run(shutdownMgr.Context())

	<-shutdownMgr.Done()
}
