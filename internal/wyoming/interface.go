package wyoming

import (
	"context"
)

type StreamTranscriber interface {
	Connect(ctx context.Context) error
	SendAudio(audioData []byte) error
	SendAudioStop() error
	Recv() (*Transcript, error)
	Close() error
	Reset()
	IsConnected() bool
	SilenceDetected() bool
	ResetVAD()
}
