package speedtest

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	client *http.Client
}

type SpeedtestResult struct {
	DownloadSpeedMbps float64   `json:"download_speed_mbps"`
	LatencyMs         int       `json:"latency_ms"`
	Timestamp         time.Time `json:"timestamp"`
}

func NewClient() *Client {
	return &Client{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetSpeedtest() (*SpeedtestResult, error) {
	slog.Info("starting speedtest")

	latency, err := c.measureLatency()
	if err != nil {
		slog.Warn("failed to measure latency", "error", err)
		latency = 0
	}

	downloadSpeed, err := c.measureDownloadSpeed()
	if err != nil {
		return nil, fmt.Errorf("failed to measure download speed: %w", err)
	}

	slog.Info("speedtest completed", "download_mbps", downloadSpeed, "latency_ms", latency)

	return &SpeedtestResult{
		DownloadSpeedMbps: downloadSpeed,
		LatencyMs:         latency,
		Timestamp:         time.Now().UTC(),
	}, nil
}

func (c *Client) measureLatency() (int, error) {
	req, err := http.NewRequest("GET", "https://fast.com", nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	latency := int(time.Since(start).Milliseconds())
	return latency, nil
}

func (c *Client) measureDownloadSpeed() (float64, error) {
	urls := []string{
		"https://fast.com/speedtest/assets/speedtest.js",
		"https://speed.hetzner.de/1MB.bin",
		"https://proof.ovh.net/files/1Mb.dat",
	}

	for _, url := range urls {
		speed, err := c.testURL(url)
		if err == nil && speed > 0 {
			return speed, nil
		}
		slog.Debug("speedtest URL failed, trying next", "url", url, "error", err)
	}

	return 0, fmt.Errorf("all speedtest URLs failed")
}

func (c *Client) testURL(url string) (float64, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	downloadStart := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	downloadDuration := time.Since(downloadStart)
	bytesDownloaded := len(body)

	bitsPerSecond := float64(bytesDownloaded*8) / downloadDuration.Seconds()
	mbps := bitsPerSecond / 1_000_000

	return mbps, nil
}
