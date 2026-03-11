package phases

type Phase int

const (
	PhaseIdle Phase = iota
	PhaseListening
	PhaseThinking
	PhaseReply
)

func (p Phase) String() string {
	switch p {
	case PhaseIdle:
		return "idle"
	case PhaseListening:
		return "listening"
	case PhaseThinking:
		return "thinking"
	case PhaseReply:
		return "reply"
	default:
		return "unknown"
	}
}
