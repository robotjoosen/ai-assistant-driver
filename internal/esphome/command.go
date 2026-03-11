package esphome

type CommandType int

const (
	CommandSTTStart CommandType = iota
	CommandSTTEnd
	CommandVADStart
	CommandVADEnd
	CommandTTSStart
	CommandTTSEnd
)

type Command struct {
	Type    CommandType
	Payload interface{}
}
