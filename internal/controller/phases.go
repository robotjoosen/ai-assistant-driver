package controller

import (
	"context"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/phases"
)

const (
	maxWhisperRetries = 3
)

func (c *Controller) handleVoiceAssistantEvent(event esphome.VoiceAssistantEvent) {
	slog.Info("voice assistant event", "phase", event.Phase.String(), "error", event.Error)

	switch event.Phase {
	case esphome.VoiceAssistantPhaseListening:
		c.handleListeningStart()
	case esphome.VoiceAssistantPhaseIdle, esphome.VoiceAssistantPhaseError:
		c.handleIdleOrError()
	}
}

func (c *Controller) handleAudioEvent(audio esphome.AudioEvent) {
	slog.Debug("received audio", "size", len(audio.Data), "end", audio.End)

	if c.phase != PhaseListening {
		return
	}

	if len(audio.Data) == 0 {
		return
	}

	transcriber := c.config.Transcriber

	if err := transcriber.SendAudio(audio.Data); err != nil {
		slog.Error("failed to send audio to transcriber", "error", err)
		c.sendError(ErrorEvent{
			Phase:       PhaseListening,
			Message:     err.Error(),
			Recoverable: true,
		})
		return
	}

	if transcriber.SilenceDetected() && c.phase == PhaseListening {
		slog.Debug("VAD detected end of speech")
		c.commands <- esphome.Command{Type: esphome.CommandVADEnd}
		c.handleListeningEnd()
		return
	}

	if audio.End {
		slog.Debug("audio stream ended")
	}
}

func (c *Controller) handleListeningStart() {
	if c.phase != PhaseIdle {
		return
	}

	transcriber := c.config.Transcriber
	transcriber.Reset()
	transcriber.ResetVAD()

	if err := transcriber.Connect(context.Background()); err != nil {
		slog.Error("failed to connect to transcriber", "error", err)
		c.sendError(ErrorEvent{
			Phase:       PhaseListening,
			Message:     err.Error(),
			Recoverable: true,
		})
		return
	}

	c.phase = PhaseListening
	slog.Info("listening phase started")

	c.commands <- esphome.Command{Type: esphome.CommandSTTStart}
}

func (c *Controller) handleListeningEnd() {
	if c.phase != PhaseListening {
		return
	}

	transcriber := c.config.Transcriber

	if err := transcriber.SendAudioStop(); err != nil {
		slog.Error("failed to send audio-stop", "error", err)
	}

	transcript, err := transcriber.Recv()
	if err != nil {
		slog.Error("error receiving transcript", "error", err)
		transcriber.Close()
		c.phase = PhaseIdle
		c.sendError(ErrorEvent{
			Phase:       PhaseListening,
			Message:     err.Error(),
			Recoverable: true,
		})
		return
	}

	if transcript != nil && transcript.Text != "" {
		slog.Info("transcription received", "text", transcript.Text, "final", transcript.IsFinal)
		c.transcript = transcript.Text
	} else {
		c.transcript = ""
	}

	if err := transcriber.Close(); err != nil {
		slog.Error("error closing transcriber", "error", err)
	}

	c.commands <- esphome.Command{
		Type:    esphome.CommandSTTEnd,
		Payload: esphome.STTEndPayload{Text: c.transcript},
	}

	if c.transcript == "" {
		slog.Info("no transcript received, staying idle")
		c.phase = PhaseIdle
		return
	}

	c.handleThinkingStart()
}

func (c *Controller) handleThinkingStart() {
	if c.phase == PhaseThinking {
		slog.Info("thinking phase already started")

		return
	}

	c.phase = PhaseThinking
	slog.Info("thinking phase started", "transcript", c.transcript)

	thinkingPhase := phases.NewThinkingPhase(c.config.AIClient)
	response := thinkingPhase.Run(context.Background(), c.transcript)

	if response == "" {
		slog.Error("no response from LLM")
		c.sendError(ErrorEvent{
			Phase:       PhaseThinking,
			Message:     "no response from LLM",
			Recoverable: false,
		})
		c.phase = PhaseIdle
		return
	}

	c.llmResponse = response
	slog.Info("LLM response received",
		slog.Int("response_length", len(response)),
		slog.String("response", response),
	)

	c.handleReplyStart()
}

func (c *Controller) handleReplyStart() {
	if c.phase != PhaseThinking {
		return
	}

	c.phase = PhaseReply
	slog.Info("reply phase started")

	replyPhase := phases.NewReplyPhase(c.config.TTSSynthesizer, c.config.TTSServer, c.commands)
	cleanup, err := replyPhase.Run(context.Background(), c.llmResponse)
	if err != nil {
		slog.Error("reply phase failed", "error", err)
		c.sendError(ErrorEvent{
			Phase:       PhaseReply,
			Message:     err.Error(),
			Recoverable: false,
		})
		c.phase = PhaseIdle
		c.transcript = ""
		c.llmResponse = ""
		slog.Info("reply phase completed")
		return
	}

	c.ttsCleanup = cleanup
	c.phase = PhaseIdle
	c.transcript = ""
	c.llmResponse = ""
	slog.Info("reply phase completed")
}

func (c *Controller) handleIdleOrError() {
	if c.phase == PhaseIdle {
		return
	}

	slog.Info("transitioning to idle", "previous_phase", c.phase.String())

	if c.config.Transcriber.IsConnected() {
		c.config.Transcriber.Close()
	}

	c.phase = PhaseIdle
	c.transcript = ""
	c.llmResponse = ""
}

func (c *Controller) sendError(err ErrorEvent) {
	select {
	case c.errors <- err:
	default:
		slog.Warn("error channel full, dropping error")
	}
}
