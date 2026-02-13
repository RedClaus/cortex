---
project: Cortex
component: Docs
phase: Build
date_created: 2026-02-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-07T20:49:46.592356
---

# Pinky Service Deployment

This directory contains files for running Pinky as a system service on Linux and macOS.

## Quick Start

```bash
# Build Pinky first
go build -o pinky ./cmd/pinky

# Install as user service (recommended for personal use)
./install-service.sh

# Or install as system service (for servers)
./install-service.sh --system
```

## Directory Structure

```
deploy/
├── README.md                    # This file
├── install-service.sh           # Installation script
├── uninstall-service.sh         # Uninstallation script
├── systemd/
│   ├── pinky.service            # User-level systemd service
│   └── pinky-system.service     # System-level systemd service
└── launchd/
    ├── com.pinky.agent.plist    # macOS LaunchAgent (user login)
    └── com.pinky.daemon.plist   # macOS LaunchDaemon (system boot)
```

## Linux (systemd)

### User Service (Recommended)

Runs when you log in. Best for personal machines.

```bash
# Automatic installation
./install-service.sh

# Manual installation
mkdir -p ~/.config/systemd/user
cp systemd/pinky.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable pinky
systemctl --user start pinky
```

**Commands:**
```bash
systemctl --user start pinky     # Start
systemctl --user stop pinky      # Stop
systemctl --user restart pinky   # Restart
systemctl --user status pinky    # Check status
journalctl --user -u pinky -f    # View logs
```

**Start on boot (without login):**
```bash
loginctl enable-linger $USER
```

### System Service

Runs at system boot. Best for servers.

```bash
# Automatic installation
./install-service.sh --system

# Manual installation
sudo useradd -r -s /bin/false pinky
sudo cp pinky /usr/local/bin/
sudo mkdir -p /etc/pinky /var/lib/pinky
sudo cp systemd/pinky-system.service /etc/systemd/system/pinky.service
sudo systemctl daemon-reload
sudo systemctl enable pinky
sudo systemctl start pinky
```

**Commands:**
```bash
sudo systemctl start pinky       # Start
sudo systemctl stop pinky        # Stop
sudo systemctl restart pinky     # Restart
sudo systemctl status pinky      # Check status
sudo journalctl -u pinky -f      # View logs
```

## macOS (launchd)

### User Agent (Recommended)

Runs when you log in. Best for personal machines.

```bash
# Automatic installation
./install-service.sh

# Manual installation
cp launchd/com.pinky.agent.plist ~/Library/LaunchAgents/
# Edit the plist to replace REPLACE_WITH_USERNAME with your username
launchctl load ~/Library/LaunchAgents/com.pinky.agent.plist
```

**Commands:**
```bash
launchctl start com.pinky.agent  # Start
launchctl stop com.pinky.agent   # Stop
launchctl list | grep pinky      # Check status
tail -f ~/.pinky/logs/pinky.log  # View logs
```

### System Daemon

Runs at system boot. Best for servers.

```bash
# Automatic installation
./install-service.sh --system

# Manual installation
sudo cp pinky /usr/local/bin/
sudo mkdir -p /etc/pinky /var/lib/pinky /var/log/pinky
sudo cp launchd/com.pinky.daemon.plist /Library/LaunchDaemons/
sudo chown root:wheel /Library/LaunchDaemons/com.pinky.daemon.plist
sudo launchctl load /Library/LaunchDaemons/com.pinky.daemon.plist
```

**Commands:**
```bash
sudo launchctl start com.pinky.daemon  # Start
sudo launchctl stop com.pinky.daemon   # Stop
sudo launchctl list | grep pinky       # Check status
tail -f /var/log/pinky/pinky.log       # View logs
```

## Configuration

### Environment Variables

API tokens and sensitive configuration should be stored in environment files:

**Linux User Service:** `~/.config/pinky/env`
**Linux System Service:** `/etc/pinky/env`
**macOS:** Edit the plist file or use `launchctl setenv`

Example env file:
```bash
OPENAI_API_KEY=sk-...
TELEGRAM_BOT_TOKEN=123456:ABC...
DISCORD_BOT_TOKEN=...
```

### Pinky Configuration

The main configuration file is at `~/.pinky/config.yaml` (user) or `/etc/pinky/config.yaml` (system).

See the main README for configuration options.

## Uninstallation

```bash
# Remove user service
./uninstall-service.sh

# Remove system service
./uninstall-service.sh --system

# Remove and purge all data
./uninstall-service.sh --purge
```

## Troubleshooting

### Service won't start

1. Check the binary exists and is executable
2. Check logs for errors
3. Verify configuration file is valid

### Permission denied

- User services can only access files owned by your user
- System services run as `pinky` user with restricted permissions

### Port already in use

Check if another instance is running:
```bash
lsof -i :18800
```

### macOS security prompts

On first run, macOS may prompt for permissions:
- Allow "Terminal" or "pinky" to accept incoming connections
- Grant file access permissions if using restricted directories
