package wyoming

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventAudioStart EventType = "audio-start"
	EventAudioChunk EventType = "audio-chunk"
	EventAudioStop  EventType = "audio-stop"
	EventTranscript EventType = "transcript"
	EventInfo       EventType = "info"
	EventVersion    EventType = "version"
	EventTranscribe EventType = "transcribe"
)

type Event struct {
	Type          EventType      `json:"type"`
	Data          map[string]any `json:"data,omitempty"`
	PayloadLength int            `json:"payload_length,omitempty"`
}

type AudioStartData struct {
	Rate     int `json:"rate"`
	Width    int `json:"width"`
	Channels int `json:"channels"`
}

type AudioChunkData struct {
	Timestamp int `json:"timestamp"`
}

type AudioStopData struct {
	Timestamp int `json:"timestamp"`
}

type TranscriptData struct {
	Text  string `json:"text"`
	Final bool   `json:"final"`
}

type TranscribeData struct {
	Language string      `json:"language,omitempty"`
	Name     string      `json:"name,omitempty"`
	Context  interface{} `json:"context,omitempty"`
}

type Transcript struct {
	Text    string
	Start   time.Duration
	End     time.Duration
	IsFinal bool
}

func ParseEvent(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func (e *Event) GetAudioStartData() (*AudioStartData, error) {
	dataBytes, err := json.Marshal(e.Data)
	if err != nil {
		return nil, err
	}
	var audioData AudioStartData
	if err := json.Unmarshal(dataBytes, &audioData); err != nil {
		return nil, err
	}
	return &audioData, nil
}

func (e *Event) GetTranscriptData() (*TranscriptData, error) {
	dataBytes, err := json.Marshal(e.Data)
	if err != nil {
		return nil, err
	}
	var transcriptData TranscriptData
	if err := json.Unmarshal(dataBytes, &transcriptData); err != nil {
		return nil, err
	}
	return &transcriptData, nil
}

func NewAudioStartEvent(rate, width, channels int) *Event {
	return &Event{
		Type: EventAudioStart,
		Data: map[string]any{
			"rate":     rate,
			"width":    width,
			"channels": channels,
		},
	}
}

func NewAudioChunkEvent(rate, width, channels int, timestamp int, payloadLength int) *Event {
	return &Event{
		Type: EventAudioChunk,
		Data: map[string]any{
			"rate":      rate,
			"width":     width,
			"channels":  channels,
			"timestamp": timestamp,
		},
		PayloadLength: payloadLength,
	}
}

func NewAudioStopEvent(timestamp int) *Event {
	return &Event{
		Type: EventAudioStop,
		Data: map[string]any{
			"timestamp": timestamp,
		},
	}
}

func NewTranscribeEvent(language string) *Event {
	return &Event{
		Type: EventTranscribe,
		Data: map[string]any{
			"language": language,
		},
	}
}

func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}
