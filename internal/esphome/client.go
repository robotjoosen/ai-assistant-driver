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
	mu             sync.Mutex
	address        string
	logger         *slog.Logger
	connected      bool
	closed         bool
	esphomeClient  *ESPHomeClient
	eventChannel   chan VoiceAssistantEvent
	audioChannel   chan AudioEvent
	commandChannel chan Command
	stopChannel    chan struct{}
}

func NewClient(address string, logger *slog.Logger) *Client {
	return &Client{
		address:        address,
		logger:         logger,
		eventChannel:   make(chan VoiceAssistantEvent, 10),
		audioChannel:   make(chan AudioEvent, 100),
		commandChannel: make(chan Command, 10),
		stopChannel:    make(chan struct{}),
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

	// Login is deprecated in ESPHome 2026.1.0+ (password authentication no longer supported)
	// Skip login - ESPHome now uses noise encryption instead

	if err := esphomeClient.SubscribeStates(); err != nil {
		esphomeClient.Close()
		return err
	}

	c.esphomeClient = esphomeClient
	c.logger.Info("connected to ESPHome device", "address", c.address)
	c.connected = true

	go c.handleMessages()
	go c.handleCommands()

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
	case *api.VoiceAssistantRequest:
		c.handleVoiceAssistantRequest(m)
	case *api.VoiceAssistantEventResponse:
		c.handleVoiceAssistantEvent(m)
	case *api.VoiceAssistantAudio:
		c.handleVoiceAssistantAudio(m)
	case *api.BinarySensorStateResponse:
		c.handleBinarySensorState(m)
	case *api.LightStateResponse:
		c.handleLightState(m)
	case *api.SwitchStateResponse:
		c.handleSwitchState(m)
	case *api.SelectStateResponse:
		c.handleSelectState(m)
	case *api.MediaPlayerStateResponse:
		c.handleMediaPlayerState(m)
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
	case api.VoiceAssistantEvent_VOICE_ASSISTANT_STT_VAD_END:
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

func (c *Client) handleBinarySensorState(state *api.BinarySensorStateResponse) {
	c.logger.Info("binary sensor state", "key", state.Key, "state", state.State, "missing", state.MissingState)
}

func (c *Client) handleLightState(state *api.LightStateResponse) {
	c.logger.Info("light state", "key", state.Key, "state", state.State, "brightness", state.Brightness, "effect", state.Effect)
}

func (c *Client) handleSwitchState(state *api.SwitchStateResponse) {
	c.logger.Info("switch state", "key", state.Key, "state", state.State)
}

func (c *Client) handleSelectState(state *api.SelectStateResponse) {
	c.logger.Info("select state", "key", state.Key, "state", state.State, "missing", state.MissingState)
}

func (c *Client) handleMediaPlayerState(state *api.MediaPlayerStateResponse) {
	c.logger.Info("media player state", "key", state.Key, "state", state.State, "volume", state.Volume, "muted", state.Muted)
}

func (c *Client) handleVoiceAssistantRequest(req *api.VoiceAssistantRequest) {
	if req.Start {
		c.logger.Info("voice assistant start request received", "conversation_id", req.ConversationId)

		response := &api.VoiceAssistantResponse{
			Port:  0,
			Error: false,
		}

		if err := c.esphomeClient.SendMessage(msgVoiceAssistantResponse, response); err != nil {
			c.logger.Error("failed to send VoiceAssistantResponse", "error", err)
			return
		}

		c.logger.Info("sent VoiceAssistantResponse", "port", response.Port)

		select {
		case c.eventChannel <- VoiceAssistantEvent{Phase: VoiceAssistantPhaseListening}:
		default:
			c.logger.Warn("event channel full, dropping start event")
		}
	} else {
		c.logger.Info("voice assistant stopped")

		select {
		case c.eventChannel <- VoiceAssistantEvent{Phase: VoiceAssistantPhaseIdle}:
		default:
			c.logger.Warn("event channel full, dropping stop event")
		}
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

func (c *Client) Commands() chan<- Command {
	return c.commandChannel
}

func (c *Client) handleCommands() {
	for {
		select {
		case <-c.stopChannel:
			return
		case cmd := <-c.commandChannel:
			c.processCommand(cmd)
		}
	}
}

func (c *Client) processCommand(cmd Command) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.esphomeClient == nil {
		return
	}

	switch cmd.Type {
	case CommandSTTStart:
		c.sendSTTEvent(true)
	case CommandSTTEnd:
		c.sendSTTEvent(false)
	case CommandVADStart:
		c.sendVADEvent(false)
	case CommandVADEnd:
		c.sendVADEvent(true)
	case CommandTTSStart:
		c.sendTTSEvent(true)
	case CommandTTSEnd:
		c.sendTTSEvent(false)
	}
}

func (c *Client) sendSTTEvent(start bool) {
	eventType := api.VoiceAssistantEvent_VOICE_ASSISTANT_STT_END
	if start {
		eventType = api.VoiceAssistantEvent_VOICE_ASSISTANT_STT_START
	}

	event := &api.VoiceAssistantEventResponse{
		EventType: eventType,
	}

	c.logger.Info("sending STT event to ESPHome", "start", start)

	if err := c.esphomeClient.SendMessage(msgVoiceAssistantEvent, event); err != nil {
		c.logger.Error("failed to send STT event", "error", err)
	}
}

func (c *Client) sendVADEvent(vadEnd bool) {
	eventType := api.VoiceAssistantEvent_VOICE_ASSISTANT_STT_VAD_END
	if !vadEnd {
		eventType = api.VoiceAssistantEvent_VOICE_ASSISTANT_STT_VAD_START
	}

	event := &api.VoiceAssistantEventResponse{
		EventType: eventType,
	}

	c.logger.Info("sending VAD event to ESPHome", "vad_end", vadEnd)

	if err := c.esphomeClient.SendMessage(msgVoiceAssistantEvent, event); err != nil {
		c.logger.Error("failed to send VAD event", "error", err)
	}
}

func (c *Client) sendTTSEvent(start bool) {
	eventType := api.VoiceAssistantEvent_VOICE_ASSISTANT_TTS_END
	if start {
		eventType = api.VoiceAssistantEvent_VOICE_ASSISTANT_TTS_START
	}

	event := &api.VoiceAssistantEventResponse{
		EventType: eventType,
	}

	c.logger.Info("sending TTS event to ESPHome", "start", start)

	if err := c.esphomeClient.SendMessage(msgVoiceAssistantEvent, event); err != nil {
		c.logger.Error("failed to send TTS event", "error", err)
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
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
