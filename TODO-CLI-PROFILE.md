# TODO: Server Init Enhancements

## 1. Create CLI Profile After Init

When `muxi-server init` runs, if CLI is installed on the same machine, create the default CLI profile.

### Detection

```bash
which muxi || [ -f ~/.local/bin/muxi ]
```

### Action

After generating server keys, create `~/.muxi/cli/profiles.yaml`:

```yaml
version: "1.0"
default: localhost
profiles:
    localhost:
        url: http://localhost:7890
        key_id: <generated_key_id>
        secret_key: <generated_secret_key>
        added_at: <timestamp>
```

---

## 2. Service/Daemon Setup

During `muxi-server init`, detect platform and offer to set up as a service.

### Linux (systemd)

```bash
# Create service file
sudo tee /etc/systemd/system/muxi-server.service << EOF
[Unit]
Description=MUXI Server
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=$HOME/.local/bin/muxi-server start
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable muxi-server
sudo systemctl start muxi-server
```

### macOS (launchd)

```bash
# Create plist
cat > ~/Library/LaunchAgents/org.muxi.server.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>org.muxi.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>$HOME/.local/bin/muxi-server</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
EOF

# Load
launchctl load ~/Library/LaunchAgents/org.muxi.server.plist
```

### Windows (Task Scheduler or Service)

Use `sc.exe` or Task Scheduler to run at startup.

### UX Flow

```
muxi-server init

...credentials generated...

→ Would you like to run MUXI Server as a system service?
  This will start the server automatically on boot.
  
  [Y/n]: y

✓ Created systemd service
✓ Enabled muxi-server.service
✓ Started muxi-server.service

Server is running at http://localhost:7890
```
