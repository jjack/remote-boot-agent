package config

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
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
