package vad

import (
	"math"
)

type Detector struct {
	threshold        float64
	thresholdRatio   float64
	minSilenceFrames int
	sampleRate       int
	bitsPerSample    int

	noiseFloor      float64
	learningFrames  int
	framesProcessed int
	isLearning      bool

	silentFrames int
	wasSpeaking  bool
	speechEnded  bool
}

type Option func(*Detector)

func WithThreshold(threshold float64) Option {
	return func(d *Detector) {
		d.threshold = threshold
	}
}

func WithThresholdRatio(ratio float64) Option {
	return func(d *Detector) {
		d.thresholdRatio = ratio
	}
}

func WithMinSilenceMs(ms int) Option {
	return func(d *Detector) {
		chunkDurationMs := 32
		d.minSilenceFrames = ms / chunkDurationMs
		if d.minSilenceFrames < 1 {
			d.minSilenceFrames = 1
		}
	}
}

func WithLearningFrames(frames int) Option {
	return func(d *Detector) {
		d.learningFrames = frames
	}
}

func NewDetector(options ...Option) *Detector {
	d := &Detector{
		threshold:        0,
		thresholdRatio:   2.5,
		minSilenceFrames: 31,
		learningFrames:   16,
		sampleRate:       16000,
		bitsPerSample:    16,
		noiseFloor:       0,
		isLearning:       true,
	}

	chunkDurationMs := 32
	d.minSilenceFrames = 1000 / chunkDurationMs
	if d.minSilenceFrames < 1 {
		d.minSilenceFrames = 1
	}

	for _, opt := range options {
		opt(d)
	}

	return d
}

func (d *Detector) ProcessAudio(audioData []byte) (isSpeaking bool, silenceEnded bool) {
	rms := calculateRMS(audioData)

	d.framesProcessed++

	if d.isLearning {
		if d.framesProcessed == 1 {
			d.noiseFloor = rms
		} else {
			minNoise := d.noiseFloor
			if rms < minNoise {
				minNoise = rms
			}
			d.noiseFloor = d.noiseFloor*0.95 + minNoise*0.05
		}

		if d.framesProcessed >= d.learningFrames {
			d.isLearning = false
			d.threshold = d.noiseFloor * d.thresholdRatio
			if d.threshold < 50 {
				d.threshold = 50
			}
			d.wasSpeaking = true
		}

		return true, false
	}

	graduallyUpdateNoiseFloor := d.framesProcessed % 100
	if graduallyUpdateNoiseFloor == 0 && rms < d.noiseFloor*1.5 {
		minNoise := d.noiseFloor
		if rms < minNoise {
			minNoise = rms
		}
		d.noiseFloor = d.noiseFloor*0.99 + minNoise*0.01
		d.threshold = d.noiseFloor * d.thresholdRatio
		if d.threshold < 50 {
			d.threshold = 50
		}
	}

	isBelowThreshold := rms < d.threshold

	if isBelowThreshold {
		d.silentFrames++
	} else {
		d.silentFrames = 0
		d.wasSpeaking = true
	}

	if d.silentFrames >= d.minSilenceFrames && d.wasSpeaking {
		d.speechEnded = true
		return false, true
	}

	return d.wasSpeaking, false
}

func (d *Detector) Reset() {
	d.silentFrames = 0
	d.wasSpeaking = false
	d.speechEnded = false
	d.framesProcessed = 0
	d.isLearning = true
	d.noiseFloor = 0
	d.threshold = 0
}

func (d *Detector) SpeechEnded() bool {
	if d.speechEnded {
		d.speechEnded = false
		return true
	}
	return false
}

func (d *Detector) Threshold() float64 {
	return d.threshold
}

func (d *Detector) NoiseFloor() float64 {
	return d.noiseFloor
}

func (d *Detector) IsLearning() bool {
	return d.isLearning
}

func calculateRMS(audioData []byte) float64 {
	if len(audioData) == 0 {
		return 0
	}

	samples := make([]int16, len(audioData)/2)
	for i := range samples {
		samples[i] = int16(audioData[2*i]) | int16(audioData[2*i+1])<<8
	}

	if len(samples) == 0 {
		return 0
	}

	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}

	return math.Sqrt(sum / float64(len(samples)))
}
