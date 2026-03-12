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

type MediaPlayerEvent struct {
	Key    uint32
	State  string
	Volume float32
	Muted  bool
}

type Client struct {
	mu                 sync.Mutex
	address            string
	connected          bool
	closed             bool
	esphomeClient      *ESPHomeClient
	eventChannel       chan VoiceAssistantEvent
	audioChannel       chan AudioEvent
	mediaPlayerChannel chan MediaPlayerEvent
	commandChannel     chan Command
	stopChannel        chan struct{}
}

func NewClient(address string) *Client {
	return &Client{
		address:            address,
		eventChannel:       make(chan VoiceAssistantEvent, 10),
		audioChannel:       make(chan AudioEvent, 100),
		mediaPlayerChannel: make(chan MediaPlayerEvent, 10),
		commandChannel:     make(chan Command, 10),
		stopChannel:        make(chan struct{}),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	slog.Info("connecting to ESPHome device", "address", c.address)

	esphomeClient := NewESPHomeClient(c.address)
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
	slog.Info("connected to ESPHome device", "address", c.address)
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

	slog.Debug("received message", "type", proto.MessageName(msg))

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
		slog.Debug("unhandled message", "type", proto.MessageName(msg))
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

	slog.Info("voice assistant event", "phase", phase.String(), "error", errorMsg)

	select {
	case c.eventChannel <- VoiceAssistantEvent{Phase: phase, Error: errorMsg}:
	default:
		slog.Warn("event channel full, dropping event")
	}
}

func (c *Client) handleVoiceAssistantAudio(audio *api.VoiceAssistantAudio) {
	slog.Debug("received audio", "size", len(audio.Data), "end", audio.End)

	select {
	case c.audioChannel <- AudioEvent{Data: audio.Data, End: audio.End}:
	default:
		slog.Warn("audio channel full, dropping audio")
	}
}

func (c *Client) handleBinarySensorState(state *api.BinarySensorStateResponse) {
	slog.Debug("binary sensor state", "key", state.Key, "state", state.State, "missing", state.MissingState)
}

func (c *Client) handleLightState(state *api.LightStateResponse) {
	slog.Debug("light state", "key", state.Key, "state", state.State, "brightness", state.Brightness, "effect", state.Effect)
}

func (c *Client) handleSwitchState(state *api.SwitchStateResponse) {
	slog.Debug("switch state", "key", state.Key, "state", state.State)
}

func (c *Client) handleSelectState(state *api.SelectStateResponse) {
	slog.Debug("select state", "key", state.Key, "state", state.State, "missing", state.MissingState)
}

func (c *Client) handleMediaPlayerState(state *api.MediaPlayerStateResponse) {
	slog.Debug("media player state", "key", state.Key, "state", state.State, "volume", state.Volume, "muted", state.Muted)

	select {
	case c.mediaPlayerChannel <- MediaPlayerEvent{
		Key:    state.Key,
		State:  state.State.String(),
		Volume: state.Volume,
		Muted:  state.Muted,
	}:
	default:
		slog.Warn("media player channel full, dropping event")
	}
}

func (c *Client) handleVoiceAssistantRequest(req *api.VoiceAssistantRequest) {
	if req.Start {
		slog.Info("voice assistant start request received", "conversation_id", req.ConversationId)

		response := &api.VoiceAssistantResponse{
			Port:  0,
			Error: false,
		}

		if err := c.esphomeClient.SendMessage(msgVoiceAssistantResponse, response); err != nil {
			slog.Error("failed to send VoiceAssistantResponse", "error", err)
			return
		}

		slog.Info("sent VoiceAssistantResponse", "port", response.Port)

		select {
		case c.eventChannel <- VoiceAssistantEvent{Phase: VoiceAssistantPhaseListening}:
		default:
			slog.Warn("event channel full, dropping start event")
		}
	} else {
		slog.Info("voice assistant stopped")

		select {
		case c.eventChannel <- VoiceAssistantEvent{Phase: VoiceAssistantPhaseIdle}:
		default:
			slog.Warn("event channel full, dropping stop event")
		}
	}
}

func (c *Client) SubscribeVoiceAssistant(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.esphomeClient == nil {
		return ErrNotConnected
	}

	slog.Info("subscribing to voice assistant")

	if err := c.esphomeClient.SubscribeVoiceAssistant(); err != nil {
		slog.Error("SubscribeVoiceAssistant failed", "error", err)
		return err
	}

	slog.Info("subscribed to voice assistant, starting")

	if err := c.esphomeClient.StartVoiceAssistant(); err != nil {
		slog.Error("StartVoiceAssistant failed", "error", err)
		return err
	}

	slog.Info("voice assistant started")
	return nil
}

func (c *Client) Events() <-chan VoiceAssistantEvent {
	return c.eventChannel
}

func (c *Client) AudioEvents() <-chan AudioEvent {
	return c.audioChannel
}

func (c *Client) MediaPlayerEvents() <-chan MediaPlayerEvent {
	return c.mediaPlayerChannel
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
		var payload STTEndPayload
		if p, ok := cmd.Payload.(STTEndPayload); ok {
			payload = p
		}
		c.sendSTTEvent(false, payload)
	case CommandVADStart:
		c.sendVADEvent(false)
	case CommandVADEnd:
		c.sendVADEvent(true)
	case CommandTTSStart:
		var payload TTSEndPayload
		if p, ok := cmd.Payload.(TTSEndPayload); ok {
			payload = p
		}
		c.sendTTSEvent(true, payload)
	case CommandTTSEnd:
		var payload TTSEndPayload
		if p, ok := cmd.Payload.(TTSEndPayload); ok {
			payload = p
		}
		c.sendTTSEvent(false, payload)
	case CommandVoiceAssistantEnd:
		c.sendVoiceAssistantEnd()
	}
}

func (c *Client) sendSTTEvent(start bool, payload ...STTEndPayload) {
	eventType := api.VoiceAssistantEvent_VOICE_ASSISTANT_STT_END
	if start {
		eventType = api.VoiceAssistantEvent_VOICE_ASSISTANT_STT_START
	}

	event := &api.VoiceAssistantEventResponse{
		EventType: eventType,
	}

	if len(payload) > 0 && payload[0].Text != "" {
		event.Data = append(event.Data, &api.VoiceAssistantEventData{
			Name:  "text",
			Value: payload[0].Text,
		})
	}

	slog.Info("sending STT event to ESPHome", "start", start, "has_text", len(payload) > 0 && payload[0].Text != "")

	if err := c.esphomeClient.SendMessage(msgVoiceAssistantEvent, event); err != nil {
		slog.Error("failed to send STT event", "error", err)
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

	slog.Info("sending VAD event to ESPHome", "vad_end", vadEnd)

	if err := c.esphomeClient.SendMessage(msgVoiceAssistantEvent, event); err != nil {
		slog.Error("failed to send VAD event", "error", err)
	}
}

func (c *Client) sendTTSEvent(start bool, payload ...TTSEndPayload) {
	eventType := api.VoiceAssistantEvent_VOICE_ASSISTANT_TTS_END
	if start {
		eventType = api.VoiceAssistantEvent_VOICE_ASSISTANT_TTS_START
	}

	event := &api.VoiceAssistantEventResponse{
		EventType: eventType,
	}

	if len(payload) > 0 {
		p := payload[0]
		if p.Text != "" {
			event.Data = append(event.Data, &api.VoiceAssistantEventData{
				Name:  "text",
				Value: p.Text,
			})
		}
		if !start && p.AudioURL != "" {
			event.Data = append(event.Data, &api.VoiceAssistantEventData{
				Name:  "url",
				Value: p.AudioURL,
			})
		}
	}

	slog.Info("sending TTS event to ESPHome", "start", start)

	if err := c.esphomeClient.SendMessage(msgVoiceAssistantEvent, event); err != nil {
		slog.Error("failed to send TTS event", "error", err)
	}
}

func (c *Client) sendVoiceAssistantEnd() {
	event := &api.VoiceAssistantEventResponse{
		EventType: api.VoiceAssistantEvent_VOICE_ASSISTANT_RUN_END,
	}

	slog.Info("sending voice assistant end event to ESPHome")

	if err := c.esphomeClient.SendMessage(msgVoiceAssistantEvent, event); err != nil {
		slog.Error("failed to send voice assistant end event", "error", err)
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
	slog.Info("disconnected from ESPHome device")
	return nil
}
