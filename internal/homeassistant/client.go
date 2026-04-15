package homeassistant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jjack/remote-boot-agent/internal/config"
)

const HTTP_CLIENT_TIMEOUT = 10 * time.Second

type HAPayload struct {
	MACAddress string   `json:"mac_address"`
	Hostname   string   `json:"hostname"`
	Bootloader string   `json:"bootloader"`
	OSList     []string `json:"os_list"`
}

type Client struct {
	BaseURL    string
	WebhookID  string
	HTTPClient *http.Client
}

func NewClient(cfg config.HAConfig) *Client {
	webhookID := cfg.WebhookID
	if webhookID == "" {
		webhookID = "remote_boot_manager_ingest"
	}
	return &Client{
		BaseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		WebhookID:  webhookID,
		HTTPClient: &http.Client{Timeout: HTTP_CLIENT_TIMEOUT},
	}
}

func (c *Client) GetSelectedOS(mac string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/remote_boot_manager/%s", c.BaseURL, mac)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error communicating with Home Assistant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error reading response: %w", err)
		}
		return string(body), nil
	}
	return "", fmt.Errorf("received HTTP %d from Home Assistant", resp.StatusCode)
}

func (c *Client) PushAvailableOSes(payload HAPayload) error {
	endpoint := fmt.Sprintf("%s/api/webhook/%s", c.BaseURL, c.WebhookID)
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error posting to Home Assistant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("received HTTP %d from Home Assistant", resp.StatusCode)
}

