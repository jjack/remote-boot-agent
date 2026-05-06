# Init System Configuration

> **Note:** The `sudo remote-boot-agent setup` and `sudo remote-boot-agent apply` commands handle this automatically. You only need to follow these steps if you are manually configuring the system.

## Configure the Init Manager Shutdown Hook

To run the `push` command on every system shutdown, create a systemd service file at `/etc/systemd/system/remote-boot-agent.service`:

```ini
[Unit]
Description=Push remote boot state to Home Assistant on shutdown
DefaultDependencies=no
Before=shutdown.target reboot.target halt.target network-online.target
Requires=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/remote-boot-agent options push --config /etc/remote-boot-agent/config.yaml
TimeoutSec=10

[Install]
WantedBy=halt.target reboot.target poweroff.target
```

Enable and reload the daemon:

```bash
sudo systemctl daemon-reload
sudo systemctl enable remote-boot-agent.service
```