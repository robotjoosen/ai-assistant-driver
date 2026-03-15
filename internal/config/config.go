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
	ErrLLMModelRequired       = errors.New("LLM_MODEL is required")
)

type Config struct {
	ESPHomeAddress string `env:"ESP_HOME_ADDRESS"`
	LogLevel       string `env:"LOG_LEVEL"`
	Conversational ConversationalConfig
	VAD            VadConfig
	LLM            LLMConfig
	OpenWrt        OpenWrtConfig
	Weather        WeatherConfig
	API            APIConfig
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

type LLMConfig struct {
	Host          string `env:"LLM_HOST"`
	Port          int    `env:"LLM_PORT"`
	Model         string `env:"LLM_MODEL"`
	SystemMessage string `env:"LLM_SYSTEM_MESSAGE"`
}

type OpenWrtConfig struct {
	Host     string `env:"OPENWRT_HOST"`
	Port     int    `env:"OPENWRT_PORT"`
	Username string `env:"OPENWRT_USERNAME"`
	Password string `env:"OPENWRT_PASSWORD"`
}

type WeatherConfig struct {
	Latitude  float64 `env:"WEATHER_LATITUDE"`
	Longitude float64 `env:"WEATHER_LONGITUDE"`
}

type APIConfig struct {
	Enabled bool   `env:"API_ENABLED"`
	APIKey  string `env:"API_KEY"`
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
		LLM: LLMConfig{
			Host: "localhost",
			Port: 11434,
		},
		OpenWrt: OpenWrtConfig{
			Port:     80,
			Username: "root",
			Password: "password",
		},
		Weather: WeatherConfig{
			Latitude:  0,
			Longitude: 0,
		},
		API: APIConfig{
			Enabled: true,
		},
	}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	if cfg.ESPHomeAddress == "" {
		return nil, ErrESPHomeAddressRequired
	}

	if cfg.LLM.Model == "" {
		return nil, ErrLLMModelRequired
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
