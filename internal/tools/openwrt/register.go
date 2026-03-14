package openwrt

import "github.com/robotjoosen/ai-assistant-driver/internal/llm"

func Register(client *Client) {
	llm.RegisterTool(&clientsTool{client: client})
	llm.RegisterTool(&networkStatusTool{client: client})
}
