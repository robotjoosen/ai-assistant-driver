package openwrt

import (
	"context"
	"encoding/json"
	"log/slog"
)

type networkStatusTool struct {
	client *Client
}

func (t *networkStatusTool) Name() string {
	return "get_router_network_status"
}

func (t *networkStatusTool) Description() string {
	return "Get comprehensive network status from the OpenWrt router including interface status, traffic statistics, WiFi information, and DHCP leases."
}

func (t *networkStatusTool) Parameters() map[string]any {
	return map[string]any{}
}

func (t *networkStatusTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	status, err := t.client.GetNetworkStatus()
	if err != nil {
		slog.Error("failed to get router network status", "error", err)
		return "", err
	}

	result, err := json.Marshal(status)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
