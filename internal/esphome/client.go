package esphome

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/mycontroller-org/esphome_api/pkg/api"
	"google.golang.org/protobuf/proto"
)

var (
	ErrNotConnected = errors.New("not connected to ESPHome device")
)

type VoiceAssistantPhase int

const (
	VoiceAssistantPhaseIdle VoiceAssistantPhase = iota
	VoiceAssistantPhaseListening
	VoiceAssistantPhaseThinking
	VoiceAssistantPhaseReply
	VoiceAssistantPhaseError
	VoiceAssistantPhaseMuted
	VoiceAssistantPhaseNotReady
)

func (p VoiceAssistantPhase) String() string {
	switch p {
	case VoiceAssistantPhaseIdle:
		return "idle"
	case VoiceAssistantPhaseListening:
		return "listening"
	case VoiceAssistantPhaseThinking:
		return "thinking"
	case VoiceAssistantPhaseReply:
		return "reply"
	case VoiceAssistantPhaseError:
		return "error"
	case VoiceAssistantPhaseMuted:
		return "muted"
	case VoiceAssistantPhaseNotReady:
		return "not_ready"
	default:
		return "unknown"
	}
}

type VoiceAssistantEvent struct {
	Phase VoiceAssistantPhase
	Error string
}

type AudioEvent struct {
	Data []byte
	End  bool
}

type Client struct {
	mu            sync.Mutex
	address       string
	logger        *slog.Logger
	connected     bool
	esphomeClient *ESPHomeClient
	eventChannel  chan VoiceAssistantEvent
	audioChannel  chan AudioEvent
	stopChannel   chan struct{}
}

func NewClient(address string, logger *slog.Logger) *Client {
	return &Client{
		address:      address,
		logger:       logger,
		eventChannel: make(chan VoiceAssistantEvent, 10),
		audioChannel: make(chan AudioEvent, 10),
		stopChannel:  make(chan struct{}),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	c.logger.Info("connecting to ESPHome device", "address", c.address)

	esphomeClient := NewESPHomeClient(c.address, c.logger)
	esphomeClient.clientID = "ai-assistant-driver"

	if err := esphomeClient.Connect(ctx); err != nil {
		return err
	}

	if err := esphomeClient.Hello(); err != nil {
		esphomeClient.Close()
		return err
	}

	if err := esphomeClient.Login(""); err != nil {
		c.logger.Warn("Login failed (continuing anyway)", "error", err)
	}

	if err := esphomeClient.SubscribeStates(); err != nil {
		esphomeClient.Close()
		return err
	}

	c.esphomeClient = esphomeClient
	c.logger.Info("connected to ESPHome device", "address", c.address)
	c.connected = true

	go c.handleMessages()

	return nil
}

func (c *Client) handleMessages() {
	for {
		select {
		case <-c.stopChannel:
			return
		case msg := <-c.esphomeClient.Messages():
			c.handleMessage(msg)
		}
	}
}

func (c *Client) handleMessage(msg proto.Message) {
	if msg == nil {
		return
	}

	c.logger.Debug("received message", "type", proto.MessageName(msg))

	switch m := msg.(type) {
	case *api.VoiceAssistantEventResponse:
		c.handleVoiceAssistantEvent(m)
	case *api.VoiceAssistantAudio:
		c.handleVoiceAssistantAudio(m)
	default:
		c.logger.Debug("unhandled message", "type", proto.MessageName(msg))
	}
}

func (c *Client) handleVoiceAssistantEvent(event *api.VoiceAssistantEventResponse) {
	phase := VoiceAssistantPhaseIdle
	errorMsg := ""

	switch event.EventType {
	case api.VoiceAssistantEvent_VOICE_ASSISTANT_ERROR:
		phase = VoiceAssistantPhaseError
		for _, d := range event.Data {
			if d.Name == "error_code" {
				errorMsg = d.Value
			}
		}
	case api.VoiceAssistantEvent_VOICE_ASSISTANT_RUN_START:
		phase = VoiceAssistantPhaseListening
	case api.VoiceAssistantEvent_VOICE_ASSISTANT_STT_VAD_START:
		phase = VoiceAssistantPhaseThinking
	case api.VoiceAssistantEvent_VOICE_ASSISTANT_TTS_START:
		phase = VoiceAssistantPhaseReply
	case api.VoiceAssistantEvent_VOICE_ASSISTANT_RUN_END:
		phase = VoiceAssistantPhaseIdle
	}

	c.logger.Info("voice assistant event", "phase", phase.String(), "error", errorMsg)

	select {
	case c.eventChannel <- VoiceAssistantEvent{Phase: phase, Error: errorMsg}:
	default:
		c.logger.Warn("event channel full, dropping event")
	}
}

func (c *Client) handleVoiceAssistantAudio(audio *api.VoiceAssistantAudio) {
	c.logger.Debug("received audio", "size", len(audio.Data), "end", audio.End)

	select {
	case c.audioChannel <- AudioEvent{Data: audio.Data, End: audio.End}:
	default:
		c.logger.Warn("audio channel full, dropping audio")
	}
}

func (c *Client) SubscribeVoiceAssistant(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.esphomeClient == nil {
		return ErrNotConnected
	}

	c.logger.Info("subscribing to voice assistant")

	if err := c.esphomeClient.SubscribeVoiceAssistant(); err != nil {
		c.logger.Error("SubscribeVoiceAssistant failed", "error", err)
		return err
	}

	c.logger.Info("subscribed to voice assistant, starting")

	if err := c.esphomeClient.StartVoiceAssistant(); err != nil {
		c.logger.Error("StartVoiceAssistant failed", "error", err)
		return err
	}

	c.logger.Info("voice assistant started")
	return nil
}

func (c *Client) Events() <-chan VoiceAssistantEvent {
	return c.eventChannel
}

func (c *Client) AudioEvents() <-chan AudioEvent {
	return c.audioChannel
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	close(c.stopChannel)

	if c.esphomeClient != nil {
		c.esphomeClient.Close()
	}

	close(c.eventChannel)
	close(c.audioChannel)

	c.connected = false
	c.logger.Info("disconnected from ESPHome device")
	return nil
}
