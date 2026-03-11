package controller

import (
	"context"

	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/phases"
)

const (
	maxWhisperRetries = 3
)

func (c *Controller) handleVoiceAssistantEvent(event esphome.VoiceAssistantEvent) {
	c.config.Logger.Info("voice assistant event", "phase", event.Phase.String(), "error", event.Error)

	switch event.Phase {
	case esphome.VoiceAssistantPhaseListening:
		c.handleListeningStart()
	case esphome.VoiceAssistantPhaseIdle, esphome.VoiceAssistantPhaseError:
		c.handleIdleOrError()
	}
}

func (c *Controller) handleAudioEvent(audio esphome.AudioEvent) {
	c.config.Logger.Debug("received audio", "size", len(audio.Data), "end", audio.End)

	if c.phase != PhaseListening {
		return
	}

	if len(audio.Data) == 0 {
		return
	}

	transcriber := c.config.Transcriber

	if err := transcriber.SendAudio(audio.Data); err != nil {
		c.config.Logger.Error("failed to send audio to transcriber", "error", err)
		c.sendError(ErrorEvent{
			Phase:       PhaseListening,
			Message:     err.Error(),
			Recoverable: true,
		})
		return
	}

	if transcriber.SilenceDetected() {
		c.config.Logger.Info("VAD detected end of speech")
		c.commands <- esphome.Command{Type: esphome.CommandVADEnd}
		c.handleListeningEnd()
		return
	}

	if audio.End {
		c.config.Logger.Info("audio stream ended")
	}
}

func (c *Controller) handleListeningStart() {
	if c.phase != PhaseIdle {
		return
	}

	transcriber := c.config.Transcriber
	transcriber.ResetVAD()

	if err := transcriber.Connect(context.Background()); err != nil {
		c.config.Logger.Error("failed to connect to transcriber", "error", err)
		c.sendError(ErrorEvent{
			Phase:       PhaseListening,
			Message:     err.Error(),
			Recoverable: true,
		})
		return
	}

	c.phase = PhaseListening
	c.config.Logger.Info("listening phase started")

	c.commands <- esphome.Command{Type: esphome.CommandSTTStart}
}

func (c *Controller) handleListeningEnd() {
	transcriber := c.config.Transcriber

	if err := transcriber.SendAudioStop(); err != nil {
		c.config.Logger.Error("failed to send audio-stop", "error", err)
	}

	transcript, err := transcriber.Recv()
	if err != nil {
		c.config.Logger.Error("error receiving transcript", "error", err)
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
		c.config.Logger.Info("transcription received", "text", transcript.Text, "final", transcript.IsFinal)
		c.transcript = transcript.Text
	} else {
		c.transcript = ""
	}

	if err := transcriber.Close(); err != nil {
		c.config.Logger.Error("error closing transcriber", "error", err)
	}

	c.commands <- esphome.Command{Type: esphome.CommandSTTEnd}

	if c.transcript == "" {
		c.config.Logger.Info("no transcript received, staying idle")
		c.phase = PhaseIdle
		return
	}

	c.handleThinkingStart()
}

func (c *Controller) handleThinkingStart() {
	c.phase = PhaseThinking
	c.config.Logger.Info("thinking phase started", "transcript", c.transcript)

	thinkingPhase := phases.NewThinkingPhase(c.config.AIClient, c.config.Logger)
	response := thinkingPhase.Run(context.Background(), c.transcript)

	if response == "" {
		c.config.Logger.Error("no response from LLM")
		c.sendError(ErrorEvent{
			Phase:       PhaseThinking,
			Message:     "no response from LLM",
			Recoverable: false,
		})
		c.phase = PhaseIdle
		return
	}

	c.llmResponse = response
	c.config.Logger.Info("LLM response received", "response_length", len(response))

	c.handleReplyStart()
}

func (c *Controller) handleReplyStart() {
	c.phase = PhaseReply
	c.config.Logger.Info("reply phase started")

	replyPhase := phases.NewReplyPhase(c.config.Logger)
	if err := replyPhase.Run(context.Background(), c.llmResponse); err != nil {
		c.config.Logger.Error("reply phase failed", "error", err)
		c.sendError(ErrorEvent{
			Phase:       PhaseReply,
			Message:     err.Error(),
			Recoverable: false,
		})
	}

	c.phase = PhaseIdle
	c.transcript = ""
	c.llmResponse = ""
	c.config.Logger.Info("reply phase completed")
}

func (c *Controller) handleIdleOrError() {
	if c.phase == PhaseIdle {
		return
	}

	c.config.Logger.Info("transitioning to idle", "previous_phase", c.phase.String())

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
		c.config.Logger.Warn("error channel full, dropping error")
	}
}
