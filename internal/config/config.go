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
)

type Config struct {
	ESPHomeAddress string `env:"ESP_HOME_ADDRESS"`
	LogLevel       string `env:"LOG_LEVEL"`
	Whisper        WhisperConfig
	Wyoming        WyomingConfig
}

type WhisperConfig struct {
	Host string `env:"WHISPER_HOST"`
	Port int    `env:"WHISPER_PORT"`
}

type WyomingConfig struct {
	Host     string `env:"WYOMING_HOST"`
	Port     int    `env:"WYOMING_PORT"`
	Language string `env:"WYOMING_LANGUAGE"`
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Debug("error loading .env file", "error", err)
		}
	}

	cfg := &Config{
		LogLevel: "info",
		Whisper: WhisperConfig{
			Host: "localhost",
			Port: 8765,
		},
		Wyoming: WyomingConfig{
			Host:     "localhost",
			Port:     10300,
			Language: "en",
		},
	}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	if cfg.ESPHomeAddress == "" {
		return nil, ErrESPHomeAddressRequired
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
