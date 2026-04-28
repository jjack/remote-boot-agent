package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"regexp"
	"strconv"
)

func ValidateMACAddress(v string) error {
	if v == "" {
		return fmt.Errorf("mac address cannot be empty")
	}
	_, err := net.ParseMAC(v)
	if err != nil {
		return fmt.Errorf("invalid MAC address format")
	}
	return nil
}

func ValidateURL(v string) error {
	if v == "" {
		return fmt.Errorf("url cannot be empty")
	}
	u, err := url.ParseRequestURI(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL format")
	}
	return nil
}

func ValidateWebhookID(v string) error {
	if v == "" {
		return fmt.Errorf("webhook id cannot be empty")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(v) {
		return fmt.Errorf("webhook id can only contain letters, numbers, hyphens, and underscores")
	}
	return nil
}

func ValidateBroadcastPort(v string) error {
	if v == "" {
		return fmt.Errorf("WOL port cannot be empty")
	}
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Debug("Invalid WOL port", "port", port)
		return fmt.Errorf("invalid WOL port: must be a number between 1 and 65535")
	}
	return nil
}

func (c *Config) Validate() error {
	if err := ValidateMACAddress(c.Host.MACAddress); err != nil {
		return err
	}
	if err := ValidateHostname(c.Host.Hostname); err != nil {
		return err
	}
	if err := ValidateURL(c.HomeAssistant.URL); err != nil {
		return err
	}
	if err := ValidateWebhookID(c.HomeAssistant.WebhookID); err != nil {
		return err
	}
	if err := ValidateBroadcastPort(strconv.Itoa(c.Host.BroadcastPort)); err != nil {
		return err
	}
	return nil
}

func ValidateHostname(v string) error {
	if v == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString(v) {
		return fmt.Errorf("hostname can only contain letters, numbers, hyphens, and periods")
	}
	return nil
}
