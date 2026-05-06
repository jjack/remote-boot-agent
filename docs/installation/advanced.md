# Advanced Installation Methods

You can install `remote-boot-agent` using the pre-built packages, binaries, or from source. The pre-built packages are recommended.

## Option A: Pre-built Packages (Recommended)

Download the appropriate package for your OS from the Releases Page.

For Debian/Ubuntu:
```bash
sudo dpkg -i remote-boot-agent_*_amd64.deb
```

## Option B: Pre-built Binaries

Download the binary archive for your architecture from the Releases Page.

```bash
tar -xzf remote-boot-agent_*_Linux_x86_64.tar.gz
sudo mv remote-boot-agent /usr/local/bin/
```

## Option C: From Source

Ensure you have Go installed on your system.

```bash
git clone https://github.com/jjack/remote-boot-agent.git
cd remote-boot-agent
go build -o remote-boot-agent .
sudo mv remote-boot-agent /usr/local/bin/
```

## Next Steps

Once installed, run the automated setup wizard to configure the agent and install the necessary system hooks:
`sudo remote-boot-agent setup`
