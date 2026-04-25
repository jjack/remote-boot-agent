package config

import (
	"testing"
)

func TestValidateMACAddress(t *testing.T) {
	tests := []struct {
		name    string
		mac     string
		wantErr bool
	}{
		{"valid mac", "00:11:22:33:44:55", false},
		{"empty mac", "", true},
		{"invalid format", "invalid-mac", true},
		{"missing colons", "001122334455", false},
		{"too long", "00:11:22:33:44:55:66:77:88", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMACAddress(tt.mac)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMACAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{"valid hostname", "my-host.name", false},
		{"empty hostname", "", true},
		{"invalid characters", "my_host!name", true},
		{"spaces", "my host", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.host)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostname() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://localhost:8123", false},
		{"valid https", "https://homeassistant.local", false},
		{"empty", "", true},
		{"invalid format", "not-a-url", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateURL(tt.url); (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWebhookID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid id", "my-webhook_123", false},
		{"empty", "", true},
		{"invalid characters", "webhook!", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateWebhookID(tt.id); (err != nil) != tt.wantErr {
				t.Errorf("ValidateWebhookID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := &Config{
		Host: HostConfig{
			MACAddress: "00:11:22:33:44:55",
			Hostname:   "test-host",
		},
		HomeAssistant: HomeAssistantConfig{
			URL:       "http://localhost:8123",
			WebhookID: "test_webhook",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got %v", err)
	}

	cfg.Host.MACAddress = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid config")
	}
}
