package openwrt

import (
	"context"
	"encoding/json"
	"log/slog"
)

type clientsTool struct {
	client *Client
}

func (t *clientsTool) Name() string {
	return "get_router_clients"
}

func (t *clientsTool) Description() string {
	return "Get all devices connected to the OpenWrt router. Returns a list of connected clients including their IP address, MAC address, and hostname if available."
}

func (t *clientsTool) Parameters() map[string]any {
	return map[string]any{}
}

func (t *clientsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	devices, err := t.client.GetConnectedClients()
	if err != nil {
		slog.Error("failed to get router clients", "error", err)
		return "", err
	}

	result, err := json.Marshal(devices)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
