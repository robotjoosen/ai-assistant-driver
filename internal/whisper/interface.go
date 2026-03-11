package whisper

import (
	"context"
	"time"
)

type Transcript struct {
	Text    string
	Start   time.Duration
	End     time.Duration
	IsFinal bool
}

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
