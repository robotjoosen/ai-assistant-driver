package esphome

import "strings"

type CommandType int

const (
	CommandSTTStart CommandType = iota
	CommandSTTEnd
	CommandVADStart
	CommandVADEnd
	CommandTTSStart
	CommandTTSEnd
	CommandVoiceAssistantStart
	CommandVoiceAssistantEnd
	CommandVolumeUp
	CommandVolumeDown
)

type Command struct {
	Type    CommandType
	Payload interface{}
}

type TTSEndPayload struct {
	Text     string
	AudioURL string
}

type STTEndPayload struct {
	Text string
}

type MediaPlayerCommandPayload struct {
	Key uint32
}

func containsLower(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), substr)
}
