package tools

import (
	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
)

var globalTools = []ai.Tool{}

func Register(t ...ai.Tool) {
	globalTools = append(globalTools, t...)
}

func All() []ai.Tool {
	return globalTools
}

func Clear() {
	globalTools = nil
}
