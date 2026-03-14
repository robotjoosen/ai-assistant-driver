package weather

import "github.com/robotjoosen/ai-assistant-driver/internal/llm"

func Register(client *Client) {
	llm.RegisterTool(&weatherTool{client: client})
}
