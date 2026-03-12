package controller

import (
	"context"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/transcriber"
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
	Transcriber    transcriber.Transcriber
	AIClient       ai.Client
	TTSSynthesizer tts.Synthesizer
	TTSServer      *tts.Server
}

type Controller struct {
	config      Config
	phase       Phase
	transcript  string
	llmResponse string

	voiceAssistantEvents <-chan esphome.VoiceAssistantEvent
	audioEvents          <-chan esphome.AudioEvent
	commands             chan<- esphome.Command
	errors               chan ErrorEvent
}

func New(
	cfg Config,
	voiceAssistantEvents <-chan esphome.VoiceAssistantEvent,
	audioEvents <-chan esphome.AudioEvent,
	commands chan<- esphome.Command,
) *Controller {
	return &Controller{
		config:               cfg,
		phase:                PhaseIdle,
		voiceAssistantEvents: voiceAssistantEvents,
		audioEvents:          audioEvents,
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
			slog.Info("phase controller stopped")
			return
		case event := <-c.voiceAssistantEvents:
			c.handleVoiceAssistantEvent(event)
		case audio := <-c.audioEvents:
			c.handleAudioEvent(audio)
		}
	}
}
