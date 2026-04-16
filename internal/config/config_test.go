package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	configPath := filepath.Join("..", "..", "config.sample.yaml")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Host.MACAddress != "00:11:22:33:44:55" {
		t.Errorf("expected MAC 00:11:22:33:44:55, got %s", cfg.Host.MACAddress)
	}
	if cfg.Host.Hostname != "my-remote-pc" {
		t.Errorf("expected Hostname my-remote-pc, got %s", cfg.Host.Hostname)
	}
	if cfg.Bootloader.Name != "grub" {
		t.Errorf("expected Bootloader grub, got %s", cfg.Bootloader.Name)
	}
	if cfg.Bootloader.ConfigPath != "/boot/grub/grub.cfg" {
		t.Errorf("expected Bootloader config_path /boot/grub/grub.cfg, got %s", cfg.Bootloader.ConfigPath)
	}
	if cfg.HomeAssistant.URL != "https://homeassistant.local:8123" {
		t.Errorf("expected HA URL https://homeassistant.local:8123, got %s", cfg.HomeAssistant.URL)
	}
	if cfg.HomeAssistant.WebhookID != "your-generated-webhook-id" {
		t.Errorf("expected HA Webhook your-generated-webhook-id, got %s", cfg.HomeAssistant.WebhookID)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty.yaml")
	os.WriteFile(configPath, []byte(""), 0644)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig with temp path failed: %v", err)
	}

	if cfg.HomeAssistant.URL != "http://homeassistant.local:8123" {
		t.Errorf("expected default HA URL, got %s", cfg.HomeAssistant.URL)
	}
	if cfg.HomeAssistant.WebhookID != "remote_boot_manager_ingest" {
		t.Errorf("expected default Webhook ID, got %s", cfg.HomeAssistant.WebhookID)
	}
}
