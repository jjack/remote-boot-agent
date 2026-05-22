package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func loadHelper(cfgFile string, flags *pflag.FlagSet) (*Config, error) {
	v := NewViper(cfgFile)
	if flags != nil {
		flagMap := map[string]string{
			"grub.config_path":         FlagGrubConfig,
			"host.mac":                 FlagMac,
			"host.address":             FlagAddress,
			"wake_on_lan.address":      FlagWolBroadcastAddress,
			"wake_on_lan.port":         FlagWolBroadcastPort,
			"homeassistant.url":        FlagHassURL,
			"homeassistant.webhook_id": FlagHassWebhook,
			"daemon.port":              FlagAgentPort,
			"daemon.api_key":           FlagDaemonKey,
		}
		for configKey, flagName := range flagMap {
			if flag := flags.Lookup(flagName); flag != nil {
				_ = v.BindPFlag(configKey, flag)
			}
		}
	}
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return Unmarshal(v)
}

func TestConfig_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg := &Config{
		Host: HostConfig{
			MACAddress: "00:11:22:33:44:55",
			Address:    "10.0.0.1",
		},
		WakeOnLan: &WakeOnLanConfig{
			Address: "192.168.1.255",
			Port:    9,
		},
		HomeAssistant: HomeAssistantConfig{
			URL:       "http://ha.local",
			WebhookID: "test-webhook",
		},
		Daemon: DaemonConfig{ReportBootOptions: true},
		Grub: &GrubConfig{
			ConfigPath: "/boot/grub/grub.cfg",
		},
	}

	// Test writing to the filesystem
	err := Save(cfg, cfgPath)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	stat, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("expected config file to exist at %s, but stat failed: %v", cfgPath, err)
	}

	if stat.Mode().Perm() != 0o600 {
		t.Errorf("expected config file permissions to be 0600, got %04o", stat.Mode().Perm())
	}

	// Test loading from the filesystem
	loadedCfg, err := loadHelper(cfgPath, nil)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loadedCfg.Host.MACAddress != cfg.Host.MACAddress {
		t.Errorf("expected MAC %s, got %s", cfg.Host.MACAddress, loadedCfg.Host.MACAddress)
	}
	if loadedCfg.HomeAssistant.WebhookID != cfg.HomeAssistant.WebhookID {
		t.Errorf("expected Webhook ID %s, got %s", cfg.HomeAssistant.WebhookID, loadedCfg.HomeAssistant.WebhookID)
	}
	if loadedCfg.Grub.ConfigPath != cfg.Grub.ConfigPath {
		t.Errorf("expected Grub ConfigPath %s, got %s", cfg.Grub.ConfigPath, loadedCfg.Grub.ConfigPath)
	}
}

func TestConfig_SaveAndLoad_Defaults(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg := &Config{
		Host: HostConfig{
			MACAddress: "00:11:22:33:44:55",
			Address:    "10.0.0.1",
		},
		WakeOnLan: &WakeOnLanConfig{
			Address: DefaultWolBroadcastAddress,
			Port:    DefaultWolBroadcastPort,
		},
		HomeAssistant: HomeAssistantConfig{
			URL:       "http://ha.local",
			WebhookID: "test-webhook",
		},
		Daemon: DaemonConfig{ReportBootOptions: true},
	}

	err := Save(cfg, cfgPath)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if strings.Contains(string(content), "wake_on_lan") {
		t.Errorf("expected wake_on_lan to be omitted from save, but found in file: %s", string(content))
	}

	loadedCfg, err := loadHelper(cfgPath, nil)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loadedCfg.WakeOnLan != nil {
		t.Errorf("expected WakeOnLan to be nil, got %+v", loadedCfg.WakeOnLan)
	}
}

func TestConfig_SaveError(t *testing.T) {
	cfg := &Config{}
	// Passing a directory path should cause WriteConfigAs to fail
	err := Save(cfg, t.TempDir())
	if err == nil {
		t.Fatal("expected error when saving to a directory path, got nil")
	}
}

func TestConfig_LoadDefaults(t *testing.T) {
	originalWD, _ := os.Getwd()
	_ = os.Chdir(t.TempDir()) // Ensure we're in an empty directory without a config file
	defer func() { _ = os.Chdir(originalWD) }()

	cfg, err := loadHelper("", nil)
	if err != nil {
		t.Fatalf("expected no error when config file is absent, got: %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected a valid, empty config object, got nil")
	}
}

func TestLoad_WithFlags(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String(FlagMac, "", "")
	fs.String(FlagAddress, "", "")
	fs.String(FlagWolBroadcastAddress, "", "")
	fs.Int(FlagWolBroadcastPort, 0, "")
	fs.String(FlagHassURL, "", "")
	fs.String(FlagHassWebhook, "", "")
	fs.String(FlagGrubConfig, "", "")

	_ = fs.Set(FlagMac, "aa:bb:cc:dd:ee:ff")
	_ = fs.Set(FlagAddress, "flag-address")
	_ = fs.Set(FlagWolBroadcastAddress, "1.1.1.1")
	_ = fs.Set(FlagWolBroadcastPort, "7")
	_ = fs.Set(FlagHassURL, "http://flag")
	_ = fs.Set(FlagHassWebhook, "flag-webhook")
	_ = fs.Set(FlagGrubConfig, "/flag/grub.cfg")

	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := loadHelper(cfgPath, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host.MACAddress != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected mac aa:bb:cc:dd:ee:ff, got %v", cfg.Host.MACAddress)
	}
	if cfg.Host.Address != "flag-address" {
		t.Errorf("expected address flag-address, got %v", cfg.Host.Address)
	}
	if cfg.WakeOnLan == nil || cfg.WakeOnLan.Address != "1.1.1.1" {
		t.Errorf("expected broadcast address 1.1.1.1, got %v", cfg.WakeOnLan)
	}
	if cfg.WakeOnLan == nil || cfg.WakeOnLan.Port != 7 {
		t.Errorf("expected broadcast port 7, got %v", cfg.WakeOnLan)
	}
	if cfg.HomeAssistant.URL != "http://flag" {
		t.Errorf("expected url http://flag, got %v", cfg.HomeAssistant.URL)
	}
	if cfg.HomeAssistant.WebhookID != "flag-webhook" {
		t.Errorf("expected webhook flag-webhook, got %v", cfg.HomeAssistant.WebhookID)
	}
	if cfg.Grub == nil || cfg.Grub.ConfigPath != "/flag/grub.cfg" {
		t.Errorf("expected grub config /flag/grub.cfg, got %v", cfg.Grub)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Error("expected non-empty default config path")
	}
}

func TestConfig_ToYAML_DefaultGrub(t *testing.T) {
	cfg := Config{
		Grub: &GrubConfig{
			WaitTimeSeconds: DefaultGrubWaitSeconds,
		},
	}
	exporter := &Exporter{Config: cfg}
	yaml, err := exporter.ToYAML()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(yaml, "grub:") {
		t.Errorf("expected grub to be omitted when it only contains default WaitTimeSeconds, got: %s", yaml)
	}

	// Test exhaustive
	exporter.Exhaustive = true
	yaml, err = exporter.ToYAML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yaml, "grub:") {
		t.Errorf("expected grub to be included in exhaustive mode, got: %s", yaml)
	}
}

func TestConfig_ToYAML_NoMutation(t *testing.T) {
	cfg := Config{
		HomeAssistant: HomeAssistantConfig{
			WebhookID: "original-webhook-id",
		},
		Grub: &GrubConfig{
			WaitTimeSeconds: DefaultGrubWaitSeconds,
		},
		WakeOnLan: &WakeOnLanConfig{
			Address: DefaultWolBroadcastAddress,
			Port:    DefaultWolBroadcastPort,
		},
	}

	exporter := &Exporter{Config: cfg, Mask: true}
	_, err := exporter.ToYAML()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HomeAssistant.WebhookID != "original-webhook-id" {
		t.Errorf("expected original WebhookID to remain unchanged, got %s", cfg.HomeAssistant.WebhookID)
	}
	if cfg.Grub.WaitTimeSeconds != DefaultGrubWaitSeconds {
		t.Errorf("expected original WaitTimeSeconds to remain %d, got %d", DefaultGrubWaitSeconds, cfg.Grub.WaitTimeSeconds)
	}
	if cfg.WakeOnLan.Address != DefaultWolBroadcastAddress {
		t.Errorf("expected original WOL address to remain %s, got %s", DefaultWolBroadcastAddress, cfg.WakeOnLan.Address)
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("invalid: : yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadHelper(cfgPath, nil)
	if err == nil {
		t.Error("expected error for malformed YAML, got nil")
	}
}
