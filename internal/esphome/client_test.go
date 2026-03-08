package esphome

import (
	"testing"
)

func TestVoiceAssistantPhase_String(t *testing.T) {
	tests := []struct {
		phase VoiceAssistantPhase
		want  string
	}{
		{VoiceAssistantPhaseIdle, "idle"},
		{VoiceAssistantPhaseListening, "listening"},
		{VoiceAssistantPhaseThinking, "thinking"},
		{VoiceAssistantPhaseReply, "reply"},
		{VoiceAssistantPhaseError, "error"},
		{VoiceAssistantPhaseMuted, "muted"},
		{VoiceAssistantPhaseNotReady, "not_ready"},
		{VoiceAssistantPhase(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.phase.String(); got != tt.want {
				t.Errorf("VoiceAssistantPhase.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVoiceAssistantEvent_Fields(t *testing.T) {
	event := VoiceAssistantEvent{
		Phase: VoiceAssistantPhaseListening,
		Error: "test error",
	}

	if event.Phase != VoiceAssistantPhaseListening {
		t.Errorf("expected phase listening, got %v", event.Phase)
	}

	if event.Error != "test error" {
		t.Errorf("expected error 'test error', got %s", event.Error)
	}
}
