package controller

import (
	"context"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/history"
	"github.com/robotjoosen/ai-assistant-driver/internal/stt"
	"github.com/robotjoosen/ai-assistant-driver/internal/tts"
)

type Phase int

const (
	PhaseIdle Phase = iota
	PhaseListening
	PhaseThinking
	PhaseReply
)

func (p Phase) String() string {
	switch p {
	case PhaseIdle:
		return "idle"
	case PhaseListening:
		return "listening"
	case PhaseThinking:
		return "thinking"
	case PhaseReply:
		return "reply"
	default:
		return "unknown"
	}
}

type ErrorEvent struct {
	Phase       Phase
	Message     string
	Recoverable bool
}

type Config struct {
	STT            stt.Transcriber
	AIClient       ai.Client
	TTSSynthesizer tts.Synthesizer
	TTSServer      *tts.Server
	HistoryManager *history.ConversationManager
	Conversational ConversationalConfig
}

type ConversationalConfig struct {
	StoragePath string
}

type Controller struct {
	config      Config
	phase       Phase
	transcript  string
	llmResponse string
	ttsCleanup  func()

	voiceAssistantEvents <-chan esphome.VoiceAssistantEvent
	audioEvents          <-chan esphome.AudioEvent
	mediaPlayerEvents    <-chan esphome.MediaPlayerEvent
	commands             chan<- esphome.Command
	errors               chan ErrorEvent
}

func New(
	cfg Config,
	voiceAssistantEvents <-chan esphome.VoiceAssistantEvent,
	audioEvents <-chan esphome.AudioEvent,
	mediaPlayerEvents <-chan esphome.MediaPlayerEvent,
	commands chan<- esphome.Command,
) *Controller {
	return &Controller{
		config:               cfg,
		phase:                PhaseIdle,
		voiceAssistantEvents: voiceAssistantEvents,
		audioEvents:          audioEvents,
		mediaPlayerEvents:    mediaPlayerEvents,
		commands:             commands,
		errors:               make(chan ErrorEvent, 10),
	}
}

func (c *Controller) Errors() <-chan ErrorEvent {
	return c.errors
}

func (c *Controller) Run(ctx context.Context) {
	slog.Info("phase controller started")

	for {
		select {
		case <-ctx.Done():
			if c.ttsCleanup != nil {
				c.ttsCleanup()
			}
			slog.Info("phase controller stopped")
			return
		case event := <-c.voiceAssistantEvents:
			c.handleVoiceAssistantEvent(event)
		case audio := <-c.audioEvents:
			c.handleAudioEvent(audio)
		case mediaPlayer := <-c.mediaPlayerEvents:
			c.handleMediaPlayerEvent(mediaPlayer)
		}
	}
}

func (c *Controller) handleMediaPlayerEvent(event esphome.MediaPlayerEvent) {
	if event.State == "MEDIA_PLAYER_STATE_IDLE" && c.ttsCleanup != nil {
		slog.Info("media player idle, cleaning up TTS file")
		c.ttsCleanup()
		c.ttsCleanup = nil

		slog.Info("sending voice assistant end event")
		c.commands <- esphome.Command{Type: esphome.CommandVoiceAssistantEnd}
	}
}
