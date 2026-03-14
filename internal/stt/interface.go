package stt

import (
	"time"
)

type Transcript struct {
	Text    string
	Start   time.Duration
	End     time.Duration
	IsFinal bool
}

type Transcriber interface {
	SendAudio(data []byte) error
	SendAudioStop() error
	Recv() (*Transcript, error)
	Close() error
	Reset()
	ResetVAD()
	SilenceDetected() bool
}
