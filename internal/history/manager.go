package history

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/robotjoosen/ai-assistant-driver/internal/ai"
)

const bufferSize = 10

type Message struct {
	Transcript string    `json:"transcript"`
	Response   string    `json:"response"`
	Timestamp  time.Time `json:"timestamp"`
}

type Storage struct {
	Summary           string    `json:"summary"`
	Messages          []Message `json:"messages"`
	TotalMessageCount int64     `json:"totalMessageCount"`
}

type ConversationManager struct {
	mu          sync.RWMutex
	buffer      []Message
	summary     string
	totalCount  int64
	storagePath string
	aiClient    ai.Client
}

var SummarizationPromptTemplate = `Summarize this conversation concisely, preserving key information, preferences, and important details.
Focus on facts the assistant should remember for future interactions. Keep it brief - 2-4 sentences.

Previous summary (if any):
%s

Recent conversation:
%s

Provide a concise summary:`

func NewConversationManager(storagePath string, aiClient ai.Client) (*ConversationManager, error) {
	mgr := &ConversationManager{
		buffer:      make([]Message, 0, bufferSize),
		storagePath: storagePath,
		aiClient:    aiClient,
	}

	if err := mgr.load(); err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("failed to load conversation history, starting fresh", "error", err)
		}
	}

	slog.Info("conversation manager initialized", "total_messages", mgr.totalCount, "has_summary", mgr.summary != "")

	return mgr, nil
}

func (m *ConversationManager) AddMessage(transcript, response string) {
	m.mu.Lock()

	msg := Message{
		Transcript: transcript,
		Response:   response,
		Timestamp:  time.Now().UTC(),
	}

	m.buffer = append(m.buffer, msg)
	if len(m.buffer) > bufferSize {
		m.buffer = m.buffer[1:]
	}

	m.totalCount++
	triggerSummarize := m.totalCount%10 == 0 && m.aiClient != nil

	storage := Storage{
		Summary:           m.summary,
		Messages:          m.buffer,
		TotalMessageCount: m.totalCount,
	}

	m.mu.Unlock()

	if err := m.saveInternal(storage); err != nil {
		slog.Error("failed to save conversation history", "error", err)
	}

	if triggerSummarize {
		go m.summarize()
	}
}

func (m *ConversationManager) GetContext(transcript string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.summary == "" && len(m.buffer) == 0 {
		return ""
	}

	var conversation strings.Builder
	for i, msg := range m.buffer {
		conversation.WriteString(fmt.Sprintf("%d. User: \"%s\"\n   Assistant: \"%s\"\n\n", i+1, msg.Transcript, msg.Response))
	}

	context := "## Conversation Context\n\n"

	if m.summary != "" {
		context += "Previous summary:\n" + m.summary + "\n\n"
	}

	if len(m.buffer) > 0 {
		context += "Recent conversation:\n" + conversation.String()
	}

	context += "---\nCurrent request: " + transcript

	return context
}

func (m *ConversationManager) GetSummary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.summary
}

func (m *ConversationManager) summarize() {
	m.mu.RLock()
	bufferCopy := make([]Message, len(m.buffer))
	copy(bufferCopy, m.buffer)
	currentSummary := m.summary
	m.mu.RUnlock()

	var conversation string
	for i, msg := range bufferCopy {
		conversation += fmt.Sprintf("%d. User: \"%s\"\n   Assistant: \"%s\"\n\n", i+1, msg.Transcript, msg.Response)
	}

	prompt := fmt.Sprintf(SummarizationPromptTemplate, currentSummary, conversation)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	newSummary, _, err := m.aiClient.Chat(ctx, prompt, "", nil)
	if err != nil {
		slog.Warn("failed to generate summary, skipping", "error", err)
		return
	}

	m.mu.Lock()
	m.summary = newSummary
	m.mu.Unlock()

	if err := m.save(); err != nil {
		slog.Error("failed to save summary", "error", err)
	}

	slog.Info("conversation summarized", "summary_length", len(newSummary))
}

func (m *ConversationManager) save() error {
	m.mu.RLock()
	storage := Storage{
		Summary:           m.summary,
		Messages:          m.buffer,
		TotalMessageCount: m.totalCount,
	}
	m.mu.RUnlock()
	return m.saveInternal(storage)
}

func (m *ConversationManager) saveInternal(storage Storage) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := os.MkdirAll(m.storagePath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	tempPath := filepath.Join(m.storagePath, "conversation_history.tmp")
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	finalPath := filepath.Join(m.storagePath, "conversation_history.json")
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("failed to atomic rename: %w", err)
	}

	return nil
}

func (m *ConversationManager) load() error {
	path := filepath.Join(m.storagePath, "conversation_history.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var storage Storage
	if err := json.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	m.summary = storage.Summary
	m.totalCount = storage.TotalMessageCount

	startIdx := 0
	if len(storage.Messages) > bufferSize {
		startIdx = len(storage.Messages) - bufferSize
	}
	m.buffer = storage.Messages[startIdx:]

	return nil
}
