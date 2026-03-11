package main

import (
	"github.com/robotjoosen/ai-assistant-driver/internal/controller"
	"github.com/robotjoosen/ai-assistant-driver/internal/shutdown"
)

func main() {
	logger, cfg := loadConfiguration()

	logger.Info("starting AI Assistant Driver", "address", cfg.ESPHomeAddress)

	shutdownMgr := shutdown.New()

	esphomeClient, err := connectToESPHome(shutdownMgr.Context(), cfg.ESPHomeAddress, logger)
	if err != nil {
		logger.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { esphomeClient.Close() })

	transcriberClient, err := newTranscriber(cfg, logger)
	if err != nil {
		logger.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { transcriberClient.Close() })

	aiClient, err := newAIClient(cfg, logger)
	if err != nil {
		logger.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}

	ctrl := controller.New(
		controller.Config{
			Transcriber: transcriberClient,
			AIClient:    aiClient,
			Logger:      logger,
		},
		esphomeClient.Events(),
		esphomeClient.AudioEvents(),
		esphomeClient.Commands(),
	)

	logger.Info("listening for voice assistant events and audio")

	go ctrl.Run(shutdownMgr.Context())

	<-shutdownMgr.Done()
}
