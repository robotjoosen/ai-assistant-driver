package ai

import "context"

type Client interface {
	Chat(ctx context.Context, prompt string, context string) (string, error)
}
