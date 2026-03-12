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
	Wyoming        WyomingConfig
	VAD            VadConfig
	AI             AIConfig
}

type WyomingConfig struct {
	Host     string `env:"WYOMING_HOST"`
	Port     int    `env:"WYOMING_PORT"`
	Language string `env:"WYOMING_LANGUAGE"`

	PiperHost     string `env:"PIPER_HOST"`
	PiperPort     int    `env:"PIPER_PORT"`
	PiperLanguage string `env:"PIPER_LANGUAGE"`

	HTTPHost string `env:"HTTP_HOST"`
	HTTPPort int    `env:"HTTP_PORT"`
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
		Wyoming: WyomingConfig{
			Host:          "localhost",
			Port:          10300,
			Language:      "en",
			PiperHost:     "localhost",
			PiperPort:     10200,
			PiperLanguage: "en",
			HTTPHost:      "0.0.0.0",
			HTTPPort:      8080,
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
