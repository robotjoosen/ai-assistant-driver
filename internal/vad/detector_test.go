package vad

import (
	"math"
	"testing"
)

func TestDetector_ProcessAudio(t *testing.T) {
	tests := []struct {
		name             string
		audioData        []byte
		thresholdRatio   float64
		wantSpeaking     bool
		wantSilenceEnded bool
		setupSpeaking    bool
	}{
		{
			name:             "loud audio - speaking",
			audioData:        generateSineWave(440, 16000, 1024, 16000),
			thresholdRatio:   2.5,
			wantSpeaking:     true,
			wantSilenceEnded: false,
			setupSpeaking:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDetector(
				WithThresholdRatio(tt.thresholdRatio),
				WithLearningFrames(2),
			)

			isSpeaking, silenceEnded := d.ProcessAudio(tt.audioData)
			isSpeaking, silenceEnded = d.ProcessAudio(tt.audioData)

			if isSpeaking != tt.wantSpeaking {
				t.Errorf("ProcessAudio() isSpeaking = %v, want %v", isSpeaking, tt.wantSpeaking)
			}
			if silenceEnded != tt.wantSilenceEnded {
				t.Errorf("ProcessAudio() silenceEnded = %v, want %v", silenceEnded, tt.wantSilenceEnded)
			}
		})
	}
}

func TestDetector_ConsecutiveSilenceEndsSpeech(t *testing.T) {
	d := NewDetector(
		WithThresholdRatio(2.5),
		WithMinSilenceMs(200),
		WithLearningFrames(1),
	)

	// First learn the noise floor with some audio
	noiseAudio := generateSineWave(440, 16000, 1024, 16000)
	d.ProcessAudio(noiseAudio)

	// Now process silence - threshold should be set
	// minSilenceFrames = 200 / 32 = 6.25 -> 6
	silentAudio := generateSilentAudio(1024)

	// First silence frame
	_, silenceEnded := d.ProcessAudio(silentAudio)
	t.Logf("After 1st silence frame: silenceEnded=%v, silentFrames=%d, threshold=%.2f", silenceEnded, d.silentFrames, d.Threshold())

	// Process more frames
	for i := 1; i < 10; i++ {
		_, silenceEnded = d.ProcessAudio(silentAudio)
		t.Logf("After %d silence frames: silenceEnded=%v, silentFrames=%d", i+1, silenceEnded, d.silentFrames)
		if silenceEnded {
			break
		}
	}

	if !silenceEnded {
		t.Errorf("Expected silenceEnded=true after minimum silence frames, got threshold=%.2f, silentFrames=%d", d.Threshold(), d.silentFrames)
	}
}

func TestDetector_LearningPhase(t *testing.T) {
	d := NewDetector(
		WithThresholdRatio(2.0),
		WithLearningFrames(5),
	)

	audio := generateSineWave(440, 16000, 1024, 16000)

	for i := 0; i < 5; i++ {
		if d.IsLearning() {
			d.ProcessAudio(audio)
		}
	}

	if d.IsLearning() {
		t.Errorf("Expected learning phase to end after 5 frames")
	}

	if d.Threshold() <= 0 {
		t.Errorf("Expected threshold to be set after learning phase")
	}

	if d.NoiseFloor() <= 0 {
		t.Errorf("Expected noise floor to be set after learning phase")
	}
}

func TestDetector_Reset(t *testing.T) {
	d := NewDetector(WithThresholdRatio(2.5))
	d.wasSpeaking = true
	d.silentFrames = 100
	d.speechEnded = true
	d.framesProcessed = 50

	d.Reset()

	if d.wasSpeaking != false {
		t.Errorf("Reset() wasSpeaking = %v, want false", d.wasSpeaking)
	}
	if d.silentFrames != 0 {
		t.Errorf("Reset() silentFrames = %v, want 0", d.silentFrames)
	}
	if d.speechEnded != false {
		t.Errorf("Reset() speechEnded = %v, want false", d.speechEnded)
	}
	if d.framesProcessed != 0 {
		t.Errorf("Reset() framesProcessed = %v, want 0", d.framesProcessed)
	}
	if !d.IsLearning() {
		t.Errorf("Reset() IsLearning = %v, want true", d.IsLearning())
	}
}

func TestDetector_SpeechEndedClearedAfterCheck(t *testing.T) {
	d := NewDetector(
		WithThresholdRatio(2.5),
		WithLearningFrames(2),
		WithMinSilenceMs(64),
	)

	silentAudio := generateSilentAudio(1024)

	d.ProcessAudio(silentAudio)
	d.ProcessAudio(silentAudio)
	d.ProcessAudio(silentAudio)

	_ = d.SpeechEnded()
	if d.SpeechEnded() {
		t.Errorf("Expected SpeechEnded() to return false after first call")
	}
}

func TestCalculateRMS_Silent(t *testing.T) {
	audio := generateSilentAudio(1024)
	rms := calculateRMS(audio)

	if rms != 0 {
		t.Errorf("calculateRMS() for silent audio = %v, want 0", rms)
	}
}

func TestCalculateRMS_Loud(t *testing.T) {
	audio := generateSineWave(440, 16000, 1024, 16000)
	rms := calculateRMS(audio)

	if rms < 1000 {
		t.Errorf("calculateRMS() for loud audio = %v, want > 1000", rms)
	}
}

func generateSilentAudio(numSamples int) []byte {
	audio := make([]byte, numSamples*2)
	for i := range numSamples {
		audio[2*i] = 0
		audio[2*i+1] = 0
	}
	return audio
}

func generateSineWave(frequency int, sampleRate int, numSamples int, amplitude int) []byte {
	audio := make([]byte, numSamples*2)
	for i := 0; i < numSamples; i++ {
		sample := amplitude * int(math.Sin(2*math.Pi*float64(frequency)*float64(i)/float64(sampleRate)))
		audio[2*i] = byte(sample & 0xFF)
		audio[2*i+1] = byte((sample >> 8) & 0xFF)
	}
	return audio
}
