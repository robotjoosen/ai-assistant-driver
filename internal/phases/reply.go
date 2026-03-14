package phases

import (
	"context"
	"log/slog"

	"github.com/robotjoosen/ai-assistant-driver/internal/esphome"
	"github.com/robotjoosen/ai-assistant-driver/internal/tts"
)

type ReplyPhase struct {
	synthesizer tts.Synthesizer
	server      *tts.Server
	commands    chan<- esphome.Command
	storagePath string
}

func NewReplyPhase(synthesizer tts.Synthesizer, server *tts.Server, commands chan<- esphome.Command, storagePath string) *ReplyPhase {
	return &ReplyPhase{
		synthesizer: synthesizer,
		server:      server,
		commands:    commands,
		storagePath: storagePath,
	}
}

func (p *ReplyPhase) Run(ctx context.Context, response string) (func(), error) {
	slog.Info("reply phase: synthesizing text", "text_length", len(response))

	p.commands <- esphome.Command{Type: esphome.CommandTTSStart, Payload: esphome.TTSEndPayload{Text: response}}

	pcmData, err := p.synthesizer.Synthesize(ctx, response)
	if err != nil {
		slog.Error("reply phase: failed to synthesize", "error", err)
		p.commands <- esphome.Command{Type: esphome.CommandTTSEnd, Payload: esphome.TTSEndPayload{Text: response}}
		return nil, err
	}

	slog.Info("reply phase: synthesized audio", "pcm_size", len(pcmData))

	wavPath, err := tts.SaveWAVFile(pcmData, 22050, 16, 1, p.storagePath)
	if err != nil {
		slog.Error("reply phase: failed to save WAV", "error", err)
		p.commands <- esphome.Command{Type: esphome.CommandTTSEnd, Payload: esphome.TTSEndPayload{Text: response}}
		return nil, err
	}

	slog.Info("reply phase: saved WAV file", "path", wavPath)

	audioURL, cleanup, err := p.server.ServeWAV(wavPath)
	if err != nil {
		slog.Error("reply phase: failed to serve WAV", "error", err)
		p.commands <- esphome.Command{Type: esphome.CommandTTSEnd, Payload: esphome.TTSEndPayload{Text: response}}
		return nil, err
	}

	slog.Info("reply phase: audio URL", "url", audioURL)

	p.commands <- esphome.Command{
		Type:    esphome.CommandTTSEnd,
		Payload: esphome.TTSEndPayload{Text: response, AudioURL: audioURL},
	}

	return cleanup, nil
}
