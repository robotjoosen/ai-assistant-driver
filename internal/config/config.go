package config

import (
	"errors"
	"log/slog"
	"os"
)

var (
	ErrESPHomeAddressRequired = errors.New("ESP_HOME_ADDRESS is required")
)

type Config struct {
	ESPHomeAddress string
	LogLevel       string
}

func Load() (*Config, error) {
	_ = os.Getenv("ESP_HOME_ADDRESS")
	_ = os.Getenv("LOG_LEVEL")

	address := os.Getenv("ESP_HOME_ADDRESS")
	if address == "" {
		return nil, ErrESPHomeAddressRequired
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	return &Config{
		ESPHomeAddress: address,
		LogLevel:       logLevel,
	}, nil
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
