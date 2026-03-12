package tts

import (
	"context"
)

type Synthesizer interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
	Close() error
}
