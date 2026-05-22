package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	HttpClientTimeout = 10 * time.Second
	OKResponse        = "OK"
)

type Client struct {
	BaseURL    string
	WebhookID  string
	HTTPClient *http.Client
}

type Action string

const (
	ActionRegisterAction Action = "register_agent_token"
	ActionUpdateAction   Action = "update_boot_options"
	ActionUnregisterHost Action = "unregister_host"
)

type CommonPayload struct {
	Action     Action `json:"action"`
	MACAddress string `json:"mac"`
	Address    string `json:"address"`
}

type RegistrationPayload struct {
	CommonPayload
	AgentToken string `json:"agent_token,omitempty"`
	AgentPort  int    `json:"agent_port,omitempty"`
}

type UpdatePayload struct {
	CommonPayload
	BootOptions         []string `json:"boot_options"`
	WolBroadcastAddress string   `json:"broadcast_address,omitempty"`
	WolBroadcastPort    int      `json:"broadcast_port,omitempty"`
}

func NewClient(baseURL, webhookID string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: HttpClientTimeout}
	}
	return &Client{
		BaseURL:    baseURL,
		WebhookID:  webhookID,
		HTTPClient: httpClient,
	}
}

func (c *Client) PostWebhook(ctx context.Context, payload any) error {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base url: %w", err)
	}
	targetURL := u.JoinPath("api/webhook", c.WebhookID).String()

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal push payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to home assistant failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code received from home assistant: %d", resp.StatusCode)
	}

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	if string(bodyBytes) != OKResponse {
		return fmt.Errorf("unexpected response from home assistant (do you have the right webhook_id?): %s", string(bodyBytes))
	}

	return nil
}

func (c *Client) RegisterAgent(ctx context.Context, mac, addr, token string, port int) error {
	payload := RegistrationPayload{
		CommonPayload: CommonPayload{
			Action:     ActionRegisterAction,
			MACAddress: mac,
			Address:    addr,
		},
		AgentToken: token,
		AgentPort:  port,
	}
	return c.PostWebhook(ctx, payload)
}

func (c *Client) UpdateBootOptions(ctx context.Context, mac, addr string, options []string, wolAddr string, wolPort int) error {
	payload := UpdatePayload{
		CommonPayload: CommonPayload{
			Action:     ActionUpdateAction,
			MACAddress: mac,
			Address:    addr,
		},
		BootOptions:         options,
		WolBroadcastAddress: wolAddr,
		WolBroadcastPort:    wolPort,
	}
	return c.PostWebhook(ctx, payload)
}

func (c *Client) UnregisterHost(ctx context.Context, mac, addr string) error {
	payload := CommonPayload{
		Action:     ActionUnregisterHost,
		MACAddress: mac,
		Address:    addr,
	}
	return c.PostWebhook(ctx, payload)
}
