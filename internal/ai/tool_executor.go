package ai

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type ToolExecutor struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{
		tools: make(map[string]Tool),
	}
}

func (e *ToolExecutor) Register(tools ...Tool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, tool := range tools {
		e.tools[tool.Name()] = tool
		slog.Debug("registered tool", "name", tool.Name())
	}
}

func (e *ToolExecutor) ExecuteAll(ctx context.Context, calls []ToolCall) []ToolResult {
	if len(calls) == 0 {
		return nil
	}

	results := make([]ToolResult, len(calls))

	for i, call := range calls {
		result := e.Execute(ctx, call)
		results[i] = result
	}

	return results
}

func (e *ToolExecutor) Execute(ctx context.Context, call ToolCall) ToolResult {
	e.mu.RLock()
	tool, exists := e.tools[call.Name]
	e.mu.RUnlock()

	if !exists {
		slog.Warn("tool not found", "name", call.Name)
		return ToolResult{
			CallID: call.ID,
			Result: "",
			Error:  fmt.Errorf("tool not found: %s", call.Name),
		}
	}

	slog.Info("executing tool", "name", call.Name, "id", call.ID)

	result, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		slog.Error("tool execution failed", "name", call.Name, "error", err)
		return ToolResult{
			CallID: call.ID,
			Result: "",
			Error:  err,
		}
	}

	slog.Info("tool executed successfully", "name", call.Name)

	return ToolResult{
		CallID: call.ID,
		Result: result,
		Error:  nil,
	}
}

func (e *ToolExecutor) GetTools() []Tool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tools := make([]Tool, 0, len(e.tools))
	for _, tool := range e.tools {
		tools = append(tools, tool)
	}
	return tools
}
