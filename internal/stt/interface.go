package stt

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

type Transcriber interface {
	Connect(ctx context.Context) error
	SendAudio(data []byte) error
	SendAudioStop() error
	Recv() (*Transcript, error)
	Close() error
	Reset()
	ResetVAD()
	SilenceDetected() bool
	IsConnected() bool
}
