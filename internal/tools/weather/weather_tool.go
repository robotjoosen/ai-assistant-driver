package weather

import (
	"context"
	"encoding/json"
	"log/slog"
)

type weatherTool struct {
	client *Client
}

func (t *weatherTool) Name() string {
	return "get_weather"
}

func (t *weatherTool) Description() string {
	return "Get current weather conditions and forecast. Returns current temperature, humidity, wind, and a 7-day forecast including high/low temperatures and weather conditions."
}

func (t *weatherTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{
				"type":        "string",
				"description": "Optional location override (city name)",
			},
		},
	}
}

func (t *weatherTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	weatherData, err := t.client.GetWeather()
	if err != nil {
		slog.Error("failed to get weather", "error", err)
		return "", err
	}

	result, err := json.Marshal(weatherData)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
