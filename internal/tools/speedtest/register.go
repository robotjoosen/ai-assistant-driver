package speedtest

import "github.com/robotjoosen/ai-assistant-driver/internal/llm"

func Register(client *Client) {
	llm.RegisterTool(&speedtestTool{client: client})
}
