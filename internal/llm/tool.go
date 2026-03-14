package llm

import "context"

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, args map[string]any) (string, error)
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type ToolResult struct {
	CallID string
	Result string
	Error  error
}

var globalTools = []Tool{}

func RegisterTool(t Tool) {
	globalTools = append(globalTools, t)
}

func GetTools() []Tool {
	return globalTools
}

func ClearTools() {
	globalTools = nil
}
