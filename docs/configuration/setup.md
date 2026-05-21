# Agent Setup and Configuration

The `grubstation` agent needs to be configured to communicate with your Home Assistant instance and identify itself on your local network. You can use the interactive setup wizard (recommended) or configure it manually.

## 1. Interactive Setup Wizard (Recommended)

The easiest way to get started is by running the automated setup wizard. This tool will auto-detect your system settings and guide you through the integration process.

```bash
sudo grubstation setup
```

### What the wizard handles:
- **System Detection:** Identifies your Init System (e.g., `systemd`) and Bootloader (GRUB).
- **Network Identification:** Helps you select the correct network interface and MAC address for Wake-on-LAN.
- **Home Assistant Integration:** Configures the connection URL and secure Webhook ID.
- **Service Installation:** Automatically installs and starts the background daemon.

## 2. Applying Configuration Directly

If you have an existing `config.yaml` (e.g., deployed via Ansible) or want to re-apply the system hooks without going through the survey, use the `apply` flag:

```bash
sudo grubstation setup --apply --config /etc/grubstation/config.yaml
```

## 3. The GrubStation Daemon

`grubstation` runs as a persistent background service (daemon). This allows it to:
1. **Push updates:** Report available boot entries to Home Assistant whenever they change or during system events.
2. **Handle Remote Shutdown:** Provide an API endpoint for Home Assistant to safely power off the machine.
3. **Health Monitoring:** Provide a status endpoint for monitoring the agent's health.

### API Endpoints

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/status` | `GET` | Returns the current agent status and system metadata. |
| `/shutdown` | `POST` | Triggers a system shutdown (requires Authentication). |

### Remote Shutdown Authentication
The `/shutdown` endpoint is secured via an API key.
- **TOFU (Trust On First Use):** If no key is configured, the agent generates a secure random token on startup and pushes it to Home Assistant.
- **Static Key:** You can optionally define a static `api_key` in your configuration file.

## 4. Manual Configuration Reference

You can generate an empty config file with `grubstation config init -o config.yaml`

| OS | Default Path |
| -- | ------------ |
| Linux | /etc/grubstation/config.yaml |
| Windows | C:\ProgramData\GrubStation\config.yaml 

See [config.sample.yaml](config.sample.yaml) for a complete configuration example.

You can also use [config.shutdown.sample.yaml](config.shutdown.sample.yaml) for a shutdown-only agent.

## 5. Command-Line Overrides

Almost all configuration settings can be overridden at runtime using command-line flags. This is useful for testing or temporary adjustments.

| Flag | Config Key | Description |
| :--- | :--- | :--- |
| `--config` | - | Override the path to the configuration file. |
| `--host-address` | `host.address` | Override the reported IP address. |
| `--host-mac` | `host.mac` | Override the reported MAC address. |
| `--homeassistant-url` | `homeassistant.url` | Override the Home Assistant URL. |
| `--homeassistant-webhook-id` | `homeassistant.webhook_id`| Override the Webhook ID. |
| `--daemon-port` | `daemon.port` | Override the daemon listening port. |
| `--daemon-key` | `daemon.api_key` | Override the /shutdown API key. |
| `--grub-config` | `grub.config_path` | Override the path to `grub.cfg`. |
| `--debug` | - | Enable verbose debug logging. |

## 6. Security Architecture

`grubstation` is designed with a "security-by-default" mindset, with homelabs and trusted networks in mind:

- **Unique Webhooks IDs:** Communication with Home Assistant is routed through a unique, non-guessable Webhook ID. This acts as a shared secret between your agent and your HA instance.
- **Secure Shutdowns:** The remote shutdown feature is protected by a 256-bit token (either pre-configured or generated via TOFU).
- **Minimal Surface:** The daemon only exposes a strictly defined API and does not require incoming connections from the public internet.

## 6. BIOS & UEFI Requirements

For remote booting and Wake-on-LAN to function, ensure the following are enabled in your BIOS/UEFI settings:

1. **Wake-on-LAN (WOL):** Often labeled as "Wake on Magic Packet" or "Power On By PCIe".
2. **Network Stack:** Required for GRUB to initialize the NIC and perform HTTP requests during boot.
3. **Disable Fast Boot:** Many "Fast Boot" implementations skip network initialization.
4. **Disable Deep Sleep (ErP):** If ErP is enabled, the NIC may lose power entirely when the PC is off, preventing it from hearing the WOL packet.
