package esphome

type CommandType int

const (
	CommandSTTStart CommandType = iota
	CommandSTTEnd
	CommandVADStart
	CommandVADEnd
	CommandTTSStart
	CommandTTSEnd
	CommandVoiceAssistantEnd
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
