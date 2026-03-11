package wyoming

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/robotjoosen/ai-assistant-driver/internal/config"
	"github.com/robotjoosen/ai-assistant-driver/internal/vad"
)

const (
	DefaultWyomingPort = 10300
	AudioSampleRate    = 16000
	AudioBitDepth      = 2
	AudioChannels      = 1
)

type Transcriber struct {
	logger         *slog.Logger
	client         *Client
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

func NewTranscriber(cfg config.WyomingConfig, vadCfg config.VadConfig, logger *slog.Logger) (*Transcriber, error) {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Port
	if port == 0 {
		port = DefaultWyomingPort
	}

	language := cfg.Language
	if language == "" {
		language = "en"
	}

	vadDetector := vad.NewDetector(
		vad.WithThresholdRatio(vadCfg.ThresholdRatio),
		vad.WithMinSilenceMs(vadCfg.MinSilenceMs),
	)

	return &Transcriber{
		logger:      logger,
		host:        host,
		port:        port,
		language:    language,
		vadDetector: vadDetector,
	}, nil
}

func (t *Transcriber) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transcriber is closed")
	}

	if t.client != nil {
		t.client.Close()
		t.client = nil
	}

	client, err := NewClient(t.host, t.port)
	if err != nil {
		return err
	}

	t.client = client
	t.connected = true
	t.audioSent = false
	t.transcribeSent = false

	t.logger.Info("connected to Wyoming service", "host", t.host, "port", t.port)

	return nil
}

func (t *Transcriber) SendAudio(audioData []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.client == nil || !t.connected {
		return fmt.Errorf("not connected to Wyoming service")
	}

	if !t.transcribeSent {
		t.logger.Debug("sending transcribe event to Wyoming", "language", t.language)
		event := NewTranscribeEvent(t.language)
		if err := t.client.WriteEvent(event, nil); err != nil {
			return fmt.Errorf("failed to send transcribe: %w", err)
		}
		t.transcribeSent = true
		t.logger.Debug("transcribe event sent to Wyoming")
	}

	if !t.audioSent {
		t.logger.Debug("sending audio-start to Wyoming", "rate", AudioSampleRate, "width", AudioBitDepth, "channels", AudioChannels)
		event := NewAudioStartEvent(AudioSampleRate, AudioBitDepth, AudioChannels)
		if err := t.client.WriteEvent(event, nil); err != nil {
			return fmt.Errorf("failed to send audio-start: %w", err)
		}
		t.audioSent = true
		t.logger.Debug("audio-start sent to Wyoming")
	}

	t.logger.Debug("sending audio-chunk to Wyoming", "size", len(audioData))
	event := NewAudioChunkEvent(AudioSampleRate, AudioBitDepth, AudioChannels, 0, len(audioData))
	if err := t.client.WriteEvent(event, audioData); err != nil {
		t.connected = false
		return fmt.Errorf("failed to send audio-chunk: %w", err)
	}

	if t.vadDetector != nil {
		_, silenceEnded := t.vadDetector.ProcessAudio(audioData)
		if silenceEnded {
			t.logger.Info("VAD detected end of speech")
		}
	}

	t.logger.Debug("audio-chunk sent to Wyoming", "size", len(audioData))

	return nil
}

func (t *Transcriber) Recv() (*Transcript, error) {
	if t.client == nil || !t.connected {
		return nil, fmt.Errorf("not connected to Wyoming service")
	}

	t.logger.Debug("waiting for transcript from Wyoming...")

	event, payload, err := t.client.ReadEvent()
	if err != nil {
		t.connected = false
		return nil, fmt.Errorf("failed to read event: %w", err)
	}

	t.logger.Debug("received event from Wyoming", "type", event.Type)

	switch event.Type {
	case EventTranscript:
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
		t.logger.Debug("received unhandled event", "type", event.Type)
		return nil, nil
	}
}

func (t *Transcriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	if t.client != nil && t.audioSent {
		t.logger.Debug("sending audio-stop event")
		event := NewAudioStopEvent(0)
		if err := t.client.WriteEvent(event, nil); err != nil {
			t.logger.Warn("failed to send audio-stop", "error", err)
		} else {
			t.logger.Debug("audio-stop sent successfully")
		}
	}

	if t.client != nil {
		t.logger.Info("closing connection to Wyoming service")
		err := t.client.Close()
		t.client = nil
		return err
	}

	return nil
}

func (t *Transcriber) SendAudioStop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.client == nil || !t.connected {
		return fmt.Errorf("not connected to Wyoming service")
	}

	if !t.audioSent {
		return nil
	}

	t.logger.Debug("sending audio-stop event (standalone)")
	event := NewAudioStopEvent(0)
	if err := t.client.WriteEvent(event, nil); err != nil {
		return fmt.Errorf("failed to send audio-stop: %w", err)
	}

	t.logger.Debug("audio-stop sent successfully")
	return nil
}

func (t *Transcriber) Reset() {
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

func (t *Transcriber) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

func (t *Transcriber) SilenceDetected() bool {
	if t.vadDetector == nil {
		return false
	}
	return t.vadDetector.SpeechEnded()
}

func (t *Transcriber) ResetVAD() {
	if t.vadDetector != nil {
		t.vadDetector.Reset()
	}
}
