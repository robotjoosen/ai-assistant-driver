package history

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockAIClient struct {
	response string
	err      error
}

func (m *mockAIClient) Chat(ctx context.Context, prompt string, context string) (string, error) {
	return m.response, m.err
}

func TestNewConversationManager_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()

	mgr, err := NewConversationManager(tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mgr.totalCount != 0 {
		t.Errorf("expected totalCount 0, got %d", mgr.totalCount)
	}

	if mgr.summary != "" {
		t.Errorf("expected empty summary, got %q", mgr.summary)
	}

	if len(mgr.buffer) != 0 {
		t.Errorf("expected empty buffer, got %d messages", len(mgr.buffer))
	}
}

func TestConversationManager_AddMessage(t *testing.T) {
	tempDir := t.TempDir()

	mgr, err := NewConversationManager(tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mgr.AddMessage("Hello", "Hi there!")

	if mgr.totalCount != 1 {
		t.Errorf("expected totalCount 1, got %d", mgr.totalCount)
	}

	if len(mgr.buffer) != 1 {
		t.Errorf("expected buffer length 1, got %d", len(mgr.buffer))
	}

	if mgr.buffer[0].Transcript != "Hello" {
		t.Errorf("expected transcript 'Hello', got %q", mgr.buffer[0].Transcript)
	}

	if mgr.buffer[0].Response != "Hi there!" {
		t.Errorf("expected response 'Hi there!', got %q", mgr.buffer[0].Response)
	}
}

func TestConversationManager_BufferOverflow(t *testing.T) {
	tempDir := t.TempDir()

	mgr, err := NewConversationManager(tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 15; i++ {
		mgr.AddMessage(
			strings.Repeat("x", 10),
			strings.Repeat("y", 10),
		)
	}

	if len(mgr.buffer) != 10 {
		t.Errorf("expected buffer length 10, got %d", len(mgr.buffer))
	}

	if mgr.buffer[0].Transcript != strings.Repeat("x", 10) {
		t.Errorf("expected first message to be the 6th (index 5), got %q", mgr.buffer[0].Transcript)
	}

	if mgr.buffer[9].Transcript != strings.Repeat("x", 10) {
		t.Errorf("expected last message to be the 15th (index 14), got %q", mgr.buffer[9].Transcript)
	}
}

func TestConversationManager_GetContext_Empty(t *testing.T) {
	tempDir := t.TempDir()

	mgr, err := NewConversationManager(tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := mgr.GetContext("Current message")
	if ctx != "" {
		t.Errorf("expected empty context, got %q", ctx)
	}
}

func TestConversationManager_GetContext_WithMessages(t *testing.T) {
	tempDir := t.TempDir()

	mgr, err := NewConversationManager(tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mgr.AddMessage("What's the weather?", "It's sunny today.")
	mgr.AddMessage("What about tomorrow?", "Tomorrow will be cloudy.")

	ctx := mgr.GetContext("Do I need an umbrella?")

	if !strings.Contains(ctx, "## Conversation Context") {
		t.Error("expected context to contain header")
	}

	if !strings.Contains(ctx, "What's the weather?") {
		t.Error("expected context to contain first message")
	}

	if !strings.Contains(ctx, "What about tomorrow?") {
		t.Error("expected context to contain second message")
	}

	if !strings.Contains(ctx, "Current request: Do I need an umbrella?") {
		t.Error("expected context to contain current request")
	}
}

func TestConversationManager_GetContext_WithSummary(t *testing.T) {
	tempDir := t.TempDir()

	mgr, err := NewConversationManager(tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mgr.summary = "User prefers Celsius temperature."

	ctx := mgr.GetContext("What's the temperature?")

	if !strings.Contains(ctx, "Previous summary:") {
		t.Error("expected context to contain summary header")
	}

	if !strings.Contains(ctx, "User prefers Celsius temperature.") {
		t.Error("expected context to contain summary content")
	}
}

func TestConversationManager_Persistence(t *testing.T) {
	tempDir := t.TempDir()

	{
		mgr, err := NewConversationManager(tempDir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mgr.AddMessage("Hello", "Hi there!")
		mgr.AddMessage("How are you?", "I'm good, thanks!")

		mgr.mu.Lock()
		mgr.summary = "User is polite."
		mgr.mu.Unlock()

		if err := mgr.save(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}
	}

	{
		mgr2, err := NewConversationManager(tempDir, nil)
		if err != nil {
			t.Fatalf("unexpected error on reload: %v", err)
		}

		if mgr2.totalCount != 2 {
			t.Errorf("expected totalCount 2, got %d", mgr2.totalCount)
		}

		if mgr2.summary != "User is polite." {
			t.Errorf("expected summary 'User is polite.', got %q", mgr2.summary)
		}

		if len(mgr2.buffer) != 2 {
			t.Errorf("expected buffer length 2, got %d", len(mgr2.buffer))
		}
	}
}

func TestConversationManager_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()

	mgr, err := NewConversationManager(tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mgr.AddMessage("Test", "Result")

	tmpPath := filepath.Join(tempDir, "conversation_history.tmp")
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("temp file should not exist after successful write")
	}

	finalPath := filepath.Join(tempDir, "conversation_history.json")
	if _, err := os.Stat(finalPath); err != nil {
		t.Errorf("final file should exist: %v", err)
	}
}

func TestConversationManager_SummarizationTrigger(t *testing.T) {
	tempDir := t.TempDir()

	mockClient := &mockAIClient{
		response: "This is a summary.",
	}

	mgr, err := NewConversationManager(tempDir, mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 10; i++ {
		mgr.AddMessage("Message", "Response")
	}

	time.Sleep(200 * time.Millisecond)

	if mgr.summary != "This is a summary." {
		t.Errorf("expected summary 'This is a summary.', got %q", mgr.summary)
	}
}

func TestConversationManager_SummarizationSkipOnError(t *testing.T) {
	tempDir := t.TempDir()

	mockClient := &mockAIClient{
		err: assertAnError(),
	}

	mgr, err := NewConversationManager(tempDir, mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 10; i++ {
		mgr.AddMessage("Message", "Response")
	}

	if mgr.summary != "" {
		t.Errorf("expected summary to remain empty on error, got %q", mgr.summary)
	}
}

func assertAnError() error {
	return &customError{}
}

type customError struct{}

func (e *customError) Error() string {
	return "mock error"
}

func TestConversationManager_GetSummary(t *testing.T) {
	tempDir := t.TempDir()

	mgr, err := NewConversationManager(tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emptySummary := mgr.GetSummary()
	if emptySummary != "" {
		t.Errorf("expected empty summary, got %q", emptySummary)
	}

	mgr.summary = "Test summary"

	returnedSummary := mgr.GetSummary()
	if returnedSummary != "Test summary" {
		t.Errorf("expected 'Test summary', got %q", returnedSummary)
	}
}
