package config

import (
	"errors"
	"os"
	"testing"
)

func TestLoad_Success(t *testing.T) {
	os.Setenv("ESP_HOME_ADDRESS", "192.168.1.100:6053")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LLM_MODEL", "llama3.2")
	defer os.Unsetenv("ESP_HOME_ADDRESS")
	defer os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("LLM_MODEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ESPHomeAddress != "192.168.1.100:6053" {
		t.Errorf("expected address 192.168.1.100:6053, got %s", cfg.ESPHomeAddress)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.LogLevel)
	}

	if cfg.LLM.Model != "llama3.2" {
		t.Errorf("expected LLM model llama3.2, got %s", cfg.LLM.Model)
	}
}

func TestLoad_DefaultLogLevel(t *testing.T) {
	os.Setenv("ESP_HOME_ADDRESS", "192.168.1.100:6053")
	os.Setenv("LLM_MODEL", "llama3.2")
	defer os.Unsetenv("ESP_HOME_ADDRESS")
	defer os.Unsetenv("LLM_MODEL")
	os.Unsetenv("LOG_LEVEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level info, got %s", cfg.LogLevel)
	}
}

func TestLoad_MissingAddress(t *testing.T) {
	os.Unsetenv("ESP_HOME_ADDRESS")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing address, got nil")
	}

	if !errors.Is(err, ErrESPHomeAddressRequired) {
		t.Errorf("expected ErrESPHomeAddressRequired, got %v", err)
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		level string
		want  int
	}{
		{"debug", -4},
		{"info", 0},
		{"warn", 4},
		{"error", 8},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			cfg := &Config{LogLevel: tt.level}
			got := cfg.GetLogLevel()
			if int(got) != tt.want {
				t.Errorf("got level %d, want %d", got, tt.want)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	if ErrESPHomeAddressRequired.Error() != "ESP_HOME_ADDRESS is required" {
		t.Error("ErrESPHomeAddressRequired message mismatch")
	}
}
