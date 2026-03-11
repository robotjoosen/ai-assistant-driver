package whisper

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/robotjoosen/ai-assistant-driver/internal/config"
)

type WebSocketTranscriber struct {
	logger    *slog.Logger
	conn      *websocket.Conn
	host      string
	port      int
	closed    bool
	connected bool
}

func NewWebSocketTranscriber(cfg config.WhisperConfig, logger *slog.Logger) (*WebSocketTranscriber, error) {
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Port
	if port == 0 {
		port = 8765
	}

	return &WebSocketTranscriber{
		logger: logger,
		host:   host,
		port:   port,
	}, nil
}

func (t *WebSocketTranscriber) Connect(ctx context.Context) error {
	if t.closed {
		return fmt.Errorf("transcriber is closed")
	}

	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}

	address := fmt.Sprintf("ws://%s:%d", t.host, t.port)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, address, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Whisper WebSocket server: %w", err)
	}

	t.conn = conn
	t.connected = true

	t.conn.SetPingHandler(func(appData string) error {
		return t.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	t.logger.Info("connected to Whisper WebSocket server", "address", address)

	return nil
}

func (t *WebSocketTranscriber) SendAudio(audioData []byte) error {
	if t.conn == nil || !t.connected {
		return fmt.Errorf("not connected to Whisper WebSocket server")
	}

	err := t.conn.WriteMessage(websocket.BinaryMessage, audioData)
	if err != nil {
		t.connected = false
		return fmt.Errorf("failed to send audio: %w", err)
	}

	return nil
}

func (t *WebSocketTranscriber) SendAudioStop() error {
	return nil
}

func (t *WebSocketTranscriber) Recv() (*Transcript, error) {
	if t.conn == nil || !t.connected {
		return nil, fmt.Errorf("not connected to Whisper WebSocket server")
	}

	_, message, err := t.conn.ReadMessage()
	if err != nil {
		t.connected = false
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	transcript := &Transcript{
		Text:    string(message),
		Start:   0,
		End:     0,
		IsFinal: true,
	}

	return transcript, nil
}

func (t *WebSocketTranscriber) IsConnected() bool {
	return t.connected
}

func (t *WebSocketTranscriber) Close() error {
	if t.closed {
		return nil
	}

	t.closed = true

	if t.conn != nil {
		t.logger.Info("closing connection to Whisper WebSocket server")
		t.conn.WriteMessage(websocket.CloseMessage, []byte{})
		return t.conn.Close()
	}
	return nil
}

func (t *WebSocketTranscriber) Reset() {
	t.closed = false
	t.connected = false
	t.conn = nil
}
