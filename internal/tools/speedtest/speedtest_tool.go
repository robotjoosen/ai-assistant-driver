package speedtest

import (
	"context"
	"encoding/json"
	"log/slog"
)

type speedtestTool struct {
	client *Client
}

func (t *speedtestTool) Name() string {
	return "get_speedtest"
}

func (t *speedtestTool) Description() string {
	return "Run a speed test to measure internet download speed in Mbps."
}

func (t *speedtestTool) Parameters() map[string]any {
	return map[string]any{}
}

func (t *speedtestTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	result, err := t.client.GetSpeedtest()
	if err != nil {
		slog.Error("speedtest failed", "error", err)
		return "", err
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
