package tts

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robotjoosen/ai-assistant-driver/internal/config"
	"github.com/robotjoosen/ai-assistant-driver/internal/wyoming"
)

type WyomingSynthesizer struct {
	host     string
	port     int
	language string
}

func NewSynthesizer(cfg config.ConversationalConfig) (Synthesizer, error) {
	host := cfg.SynthesizerHost
	if host == "" {
		host = cfg.Host
		if host == "" {
			host = "localhost"
		}
	}

	port := cfg.SynthesizerPort
	if port == 0 {
		port = 10200
	}

	language := cfg.SynthesizerLanguage
	if language == "" {
		language = cfg.Language
		if language == "" {
			language = "en"
		}
	}

	return &WyomingSynthesizer{
		host:     host,
		port:     port,
		language: language,
	}, nil
}

func (s *WyomingSynthesizer) Synthesize(ctx context.Context, text string) ([]byte, error) {
	slog.Debug("connecting to Piper for synthesis", "host", s.host, "port", s.port)

	client, err := wyoming.NewClient(s.host, s.port)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Piper: %w", err)
	}
	defer client.Close()

	slog.Debug("sending synthesize event to Piper", "text", text, "text_length", len(text))
	event := wyoming.NewSynthesizeEvent(text)
	if err := client.WriteEvent(event, nil); err != nil {
		return nil, fmt.Errorf("failed to send synthesize event: %w", err)
	}

	slog.Debug("synthesize event sent, waiting for audio response from Piper")

	var audioData []byte
	var sampleRate int
	var sampleWidth int
	var channels int

	for {
		evt, payload, err := client.ReadEventWithTimeout(120 * time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed to read event from Piper: %w", err)
		}

		slog.Debug("received event from Piper", "type", evt.Type)

		switch evt.Type {
		case wyoming.EventAudioStart:
			audioStartData, err := evt.GetAudioStartData()
			if err != nil {
				return nil, fmt.Errorf("failed to parse audio-start data: %w", err)
			}
			sampleRate = audioStartData.Rate
			sampleWidth = audioStartData.Width
			channels = audioStartData.Channels
			slog.Info("received audio-start from Piper", "rate", sampleRate, "width", sampleWidth, "channels", channels)

		case wyoming.EventAudioChunk:
			if payload != nil {
				audioData = append(audioData, payload...)
				slog.Debug("received audio chunk from Piper", "chunk_size", len(payload), "total_size", len(audioData))
			}

		case wyoming.EventAudioStop:
			slog.Info("received audio-stop from Piper", "total_size", len(audioData))
			return audioData, nil

		case wyoming.EventSynthesizeStopped:
			slog.Info("received synthesize-stopped from Piper", "total_size", len(audioData))
			return audioData, nil

		default:
			slog.Debug("unhandled event from Piper", "type", evt.Type)
		}
	}
}

func (s *WyomingSynthesizer) Close() error {
	return nil
}
