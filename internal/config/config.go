package config

import (
	"errors"
	"log/slog"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

var (
	ErrESPHomeAddressRequired = errors.New("ESP_HOME_ADDRESS is required")
	ErrAIModelRequired        = errors.New("AI_MODEL is required")
)

type Config struct {
	ESPHomeAddress string `env:"ESP_HOME_ADDRESS"`
	LogLevel       string `env:"LOG_LEVEL"`
	Conversational ConversationalConfig
	VAD            VadConfig
	AI             AIConfig
}

type ConversationalConfig struct {
	Host               string `env:"STT_HOST"`
	Port               int    `env:"STT_PORT"`
	Language           string `env:"STT_LANGUAGE"`
	StoragePath        string `env:"TTS_STORAGE_PATH"`
	HistoryStoragePath string `env:"HISTORY_STORAGE_PATH"`

	SynthesizerHost     string `env:"TTS_HOST"`
	SynthesizerPort     int    `env:"TTS_PORT"`
	SynthesizerLanguage string `env:"TTS_LANGUAGE"`

	HTTPHost string `env:"TTS_HTTP_HOST"`
	HTTPPort int    `env:"TTS_HTTP_PORT"`
}

type VadConfig struct {
	ThresholdRatio float64 `env:"VAD_THRESHOLD_RATIO"`
	MinSilenceMs   int     `env:"VAD_MIN_SILENCE_MS"`
}

type AIConfig struct {
	Host          string `env:"AI_HOST"`
	Port          int    `env:"AI_PORT"`
	Model         string `env:"AI_MODEL"`
	SystemMessage string `env:"AI_SYSTEM_MESSAGE"`
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Debug("error loading .env file", "error", err)
		}
	}

	cfg := &Config{
		LogLevel: "info",
		Conversational: ConversationalConfig{
			Host:                "localhost",
			Port:                10300,
			Language:            "en",
			StoragePath:         "data/tts",
			HistoryStoragePath:  "data/history",
			SynthesizerHost:     "localhost",
			SynthesizerPort:     10200,
			SynthesizerLanguage: "en",
			HTTPHost:            "0.0.0.0",
			HTTPPort:            8080,
		},
		VAD: VadConfig{
			ThresholdRatio: 2.5,
			MinSilenceMs:   1000,
		},
		AI: AIConfig{
			Host: "localhost",
			Port: 11434,
		},
	}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	if cfg.ESPHomeAddress == "" {
		return nil, ErrESPHomeAddressRequired
	}

	if cfg.AI.Model == "" {
		return nil, ErrAIModelRequired
	}

	return cfg, nil
}

func (c *Config) GetLogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
