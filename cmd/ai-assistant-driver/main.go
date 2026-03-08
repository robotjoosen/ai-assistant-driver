package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/robotjoosen/ai-assistant-driver/internal/config"
	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Debug("no .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.GetLogLevel(),
	}))

	slog.SetDefault(logger)

	logger.Info("starting AI Assistant Driver", "address", cfg.ESPHomeAddress)

	client := esphome.NewClient(cfg.ESPHomeAddress, logger)

	if err := client.Connect(context.Background()); err != nil {
		logger.Error("failed to connect to ESPHome device", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	logger.Info("connected to ESPHome device")

	if err := client.SubscribeVoiceAssistant(context.Background()); err != nil {
		logger.Error("failed to subscribe to voice assistant", "error", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("listening for voice assistant events and audio")

	for {
		select {
		case event := <-client.Events():
			logger.Info("voice assistant event", "phase", event.Phase.String(), "error", event.Error)
		case audio := <-client.AudioEvents():
			logger.Info("received audio", "size", len(audio.Data), "end", audio.End)
		case <-sigChan:
			logger.Info("shutting down")
			return
		}
	}
}
