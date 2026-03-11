package main

import (
	"time"

	"github.com/robotjoosen/ai-assistant-driver/internal/shutdown"
)

const (
	maxWhisperRetries = 3
	retryDelay        = 1 * time.Second
)

type appState struct {
	streaming          bool
	transcriberRetries int
	transcriberFailed  bool
	audioSent          bool
}

func main() {
	logger, cfg := loadConfiguration()

	logger.Info("starting AI Assistant Driver", "address", cfg.ESPHomeAddress)

	shutdownMgr := shutdown.New()

	client, err := connectToESPHome(shutdownMgr.Context(), cfg.ESPHomeAddress, logger)
	if err != nil {
		logger.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { client.Close() })

	transcriber, err := newTranscriber(cfg, logger)
	if err != nil {
		logger.Error("setup failed", "error", err)
		shutdownMgr.Cancel()
		<-shutdownMgr.Done()
		return
	}
	shutdownMgr.Add(func() { transcriber.Close() })

	logger.Info("listening for voice assistant events and audio")

	go func() {
		state := &appState{}
		for {
			select {
			case <-shutdownMgr.Context().Done():
				return
			case event := <-client.Events():
				handleVoiceAssistantEvent(shutdownMgr.Context(), event, state, transcriber, client, logger)
			case audio := <-client.AudioEvents():
				handleAudioEvent(shutdownMgr.Context(), audio, state, transcriber, client, logger)
			}
		}
	}()

	<-shutdownMgr.Done()
}
