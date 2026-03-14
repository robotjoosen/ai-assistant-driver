package stt

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/robotjoosen/ai-assistant-driver/internal/config"
	"github.com/robotjoosen/ai-assistant-driver/internal/vad"
	"github.com/robotjoosen/ai-assistant-driver/internal/wyoming"
)

const (
	DefaultSTTPort  = 10300
	AudioSampleRate = 16000
	AudioBitDepth   = 2
	AudioChannels   = 1
)

type STTTranscriber struct {
	client         *wyoming.Client
	host           string
	port           int
	language       string
	vadDetector    *vad.Detector
	mu             sync.Mutex
	closed         bool
	connected      bool
	audioSent      bool
	transcribeSent bool
}

func NewTranscriber(cfg config.ConversationalConfig, vadCfg config.VadConfig) (Transcriber, error) {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Port
	if port == 0 {
		port = DefaultSTTPort
	}

	language := cfg.Language
	if language == "" {
		language = "en"
	}

	vadDetector := vad.NewDetector(
		vad.WithThresholdRatio(vadCfg.ThresholdRatio),
		vad.WithMinSilenceMs(vadCfg.MinSilenceMs),
	)

	return &STTTranscriber{
		host:        host,
		port:        port,
		language:    language,
		vadDetector: vadDetector,
	}, nil
}

func (t *STTTranscriber) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transcriber is closed")
	}

	if t.client != nil {
		t.client.Close()
		t.client = nil
	}

	client, err := wyoming.NewClient(t.host, t.port)
	if err != nil {
		return err
	}

	t.client = client
	t.connected = true
	t.audioSent = false
	t.transcribeSent = false

	slog.Info("connected to STT service", "host", t.host, "port", t.port)

	return nil
}

func (t *STTTranscriber) SendAudio(audioData []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.client == nil || !t.connected {
		return fmt.Errorf("not connected to STT service")
	}

	if !t.transcribeSent {
		slog.Debug("sending transcribe event to STT", "language", t.language)
		event := wyoming.NewTranscribeEvent(t.language)
		if err := t.client.WriteEvent(event, nil); err != nil {
			return fmt.Errorf("failed to send transcribe: %w", err)
		}
		t.transcribeSent = true
		slog.Debug("transcribe event sent to STT")
	}

	if !t.audioSent {
		slog.Debug("sending audio-start to STT", "rate", AudioSampleRate, "width", AudioBitDepth, "channels", AudioChannels)
		event := wyoming.NewAudioStartEvent(AudioSampleRate, AudioBitDepth, AudioChannels)
		if err := t.client.WriteEvent(event, nil); err != nil {
			return fmt.Errorf("failed to send audio-start: %w", err)
		}
		t.audioSent = true
		slog.Debug("audio-start sent to STT")
	}

	slog.Debug("sending audio-chunk to STT", "size", len(audioData))
	event := wyoming.NewAudioChunkEvent(AudioSampleRate, AudioBitDepth, AudioChannels, 0, len(audioData))
	if err := t.client.WriteEvent(event, audioData); err != nil {
		t.connected = false
		return fmt.Errorf("failed to send audio-chunk: %w", err)
	}

	if t.vadDetector != nil {
		_, silenceEnded := t.vadDetector.ProcessAudio(audioData)
		if silenceEnded {
			slog.Debug("VAD detected end of speech")
		}
	}

	slog.Debug("audio-chunk sent to STT", "size", len(audioData))

	return nil
}

func (t *STTTranscriber) Recv() (*Transcript, error) {
	if t.client == nil || !t.connected {
		return nil, fmt.Errorf("not connected to STT service")
	}

	slog.Debug("waiting for transcript from STT...")

	event, payload, err := t.client.ReadEvent()
	if err != nil {
		t.connected = false
		return nil, fmt.Errorf("failed to read event: %w", err)
	}

	slog.Debug("received event from STT", "type", event.Type)

	switch event.Type {
	case wyoming.EventTranscript:
		transcriptData, err := event.GetTranscriptData()
		if err != nil {
			return nil, fmt.Errorf("failed to parse transcript data: %w", err)
		}

		_ = payload

		return &Transcript{
			Text:    transcriptData.Text,
			Start:   0,
			End:     0,
			IsFinal: transcriptData.Final,
		}, nil

	default:
		slog.Debug("received unhandled event", "type", event.Type)
		return nil, nil
	}
}

func (t *STTTranscriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	if t.client != nil && t.audioSent {
		slog.Debug("sending audio-stop event")
		event := wyoming.NewAudioStopEvent(0)
		if err := t.client.WriteEvent(event, nil); err != nil {
			slog.Warn("failed to send audio-stop", "error", err)
		} else {
			slog.Debug("audio-stop sent successfully")
		}
	}

	if t.client != nil {
		slog.Info("closing connection to STT service")
		err := t.client.Close()
		t.client = nil
		return err
	}

	return nil
}

func (t *STTTranscriber) SendAudioStop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.client == nil || !t.connected {
		return fmt.Errorf("not connected to STT service")
	}

	if !t.audioSent {
		return nil
	}

	slog.Debug("sending audio-stop event (standalone)")
	event := wyoming.NewAudioStopEvent(0)
	if err := t.client.WriteEvent(event, nil); err != nil {
		return fmt.Errorf("failed to send audio-stop: %w", err)
	}

	slog.Debug("audio-stop sent successfully")
	return nil
}

func (t *STTTranscriber) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = false
	t.connected = false
	t.audioSent = false
	t.transcribeSent = false
	t.client = nil
	if t.vadDetector != nil {
		t.vadDetector.Reset()
	}
}

func (t *STTTranscriber) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

func (t *STTTranscriber) SilenceDetected() bool {
	if t.vadDetector == nil {
		return false
	}
	return t.vadDetector.SpeechEnded()
}

func (t *STTTranscriber) ResetVAD() {
	if t.vadDetector != nil {
		t.vadDetector.Reset()
	}
}
