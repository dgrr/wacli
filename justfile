# wacli justfile - Build and service management

# Default binary install location
BINARY_DIR := env_var_or_default("BINARY_DIR", "~/.local/bin")
BINARY_PATH := BINARY_DIR / "wacli"

# Store paths (can be overridden via environment variables)
PERSONAL_STORE := env_var_or_default("WACLI_PERSONAL_STORE", "~/.wacli")
WORK_STORE := env_var_or_default("WACLI_WORK_STORE", "~/.wacli-uae")

# macOS plist paths
LAUNCH_AGENTS := "~/Library/LaunchAgents"
PLIST_PERSONAL := "com.wacli.sync.personal.plist"
PLIST_WORK := "com.wacli.sync.work.plist"

# Linux systemd paths
SYSTEMD_USER := "~/.config/systemd/user"
SERVICE_PERSONAL := "wacli-sync-personal.service"
SERVICE_WORK := "wacli-sync-work.service"

# Build wacli with sqlite_fts5 support
build:
    @echo "Building wacli with sqlite_fts5..."
    CGO_ENABLED=1 go build -tags sqlite_fts5 -o wacli ./cmd/wacli

# Install binary to ~/.local/bin (or BINARY_DIR)
install: build
    @echo "Installing wacli to {{BINARY_DIR}}..."
    @mkdir -p {{BINARY_DIR}}
    cp wacli {{BINARY_PATH}}
    @echo "✓ Installed to {{BINARY_PATH}}"

# Clean build artifacts
clean:
    rm -f wacli

# Run tests
test:
    go test ./...

# ============ macOS Service Management ============

# Install both macOS launchd services (personal + work)
install-service-macos: _check-macos
    @echo "Installing macOS launchd services..."
    @mkdir -p {{LAUNCH_AGENTS}}
    @mkdir -p {{PERSONAL_STORE}}
    @mkdir -p {{WORK_STORE}}
    @# Personal service
    @sed -e "s|{{{{BINARY_PATH}}}}|$(eval echo {{BINARY_PATH}})|g" \
         -e "s|{{{{STORE_PATH}}}}|$(eval echo {{PERSONAL_STORE}})|g" \
         -e "s|{{{{HOME}}}}|$HOME|g" \
         install/com.wacli.sync.personal.plist.template > "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_PERSONAL}}"
    @echo "✓ Created {{PLIST_PERSONAL}}"
    @# Work service
    @sed -e "s|{{{{BINARY_PATH}}}}|$(eval echo {{BINARY_PATH}})|g" \
         -e "s|{{{{STORE_PATH}}}}|$(eval echo {{WORK_STORE}})|g" \
         -e "s|{{{{HOME}}}}|$HOME|g" \
         install/com.wacli.sync.work.plist.template > "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_WORK}}"
    @echo "✓ Created {{PLIST_WORK}}"
    @echo ""
    @echo "Services installed. Use 'just load-macos' to load them."

# Load macOS services
load-macos: _check-macos
    @echo "Loading macOS services..."
    -launchctl unload "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_PERSONAL}}" 2>/dev/null
    -launchctl unload "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_WORK}}" 2>/dev/null
    launchctl load "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_PERSONAL}}"
    launchctl load "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_WORK}}"
    @echo "✓ Services loaded"

# Unload macOS services
unload-macos: _check-macos
    @echo "Unloading macOS services..."
    -launchctl unload "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_PERSONAL}}" 2>/dev/null
    -launchctl unload "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_WORK}}" 2>/dev/null
    @echo "✓ Services unloaded"

# Start macOS sync services
start-macos: _check-macos
    @echo "Starting macOS sync services..."
    launchctl start com.wacli.sync.personal
    launchctl start com.wacli.sync.work
    @echo "✓ Services started"

# Stop macOS sync services
stop-macos: _check-macos
    @echo "Stopping macOS sync services..."
    -launchctl stop com.wacli.sync.personal 2>/dev/null
    -launchctl stop com.wacli.sync.work 2>/dev/null
    @echo "✓ Services stopped"

# Restart macOS sync services (stop, unload, load, start)
sync-macos: _check-macos
    @echo "Restarting macOS sync services..."
    -launchctl stop com.wacli.sync.personal 2>/dev/null
    -launchctl stop com.wacli.sync.work 2>/dev/null
    -launchctl unload "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_PERSONAL}}" 2>/dev/null
    -launchctl unload "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_WORK}}" 2>/dev/null
    launchctl load "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_PERSONAL}}"
    launchctl load "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_WORK}}"
    launchctl start com.wacli.sync.personal
    launchctl start com.wacli.sync.work
    @echo "✓ Services restarted"

# Show macOS service status
status-macos: _check-macos
    @echo "=== macOS Service Status ==="
    @echo "Personal:"
    @launchctl list | grep com.wacli.sync.personal || echo "  Not loaded"
    @echo "Work:"
    @launchctl list | grep com.wacli.sync.work || echo "  Not loaded"

# Uninstall macOS services
uninstall-service-macos: _check-macos
    @echo "Uninstalling macOS services..."
    -launchctl unload "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_PERSONAL}}" 2>/dev/null
    -launchctl unload "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_WORK}}" 2>/dev/null
    -rm -f "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_PERSONAL}}"
    -rm -f "$(eval echo {{LAUNCH_AGENTS}})/{{PLIST_WORK}}"
    @echo "✓ Services uninstalled"

# ============ Linux Service Management ============

# Install both Linux systemd services (personal + work)
install-service-linux: _check-linux
    @echo "Installing Linux systemd services..."
    @mkdir -p {{SYSTEMD_USER}}
    @mkdir -p {{PERSONAL_STORE}}
    @mkdir -p {{WORK_STORE}}
    @# Personal service
    @sed -e "s|{{{{BINARY_PATH}}}}|$(eval echo {{BINARY_PATH}})|g" \
         -e "s|{{{{STORE_PATH}}}}|$(eval echo {{PERSONAL_STORE}})|g" \
         -e "s|{{{{HOME}}}}|$HOME|g" \
         install/wacli-sync-personal.service.template > "$(eval echo {{SYSTEMD_USER}})/{{SERVICE_PERSONAL}}"
    @echo "✓ Created {{SERVICE_PERSONAL}}"
    @# Work service
    @sed -e "s|{{{{BINARY_PATH}}}}|$(eval echo {{BINARY_PATH}})|g" \
         -e "s|{{{{STORE_PATH}}}}|$(eval echo {{WORK_STORE}})|g" \
         -e "s|{{{{HOME}}}}|$HOME|g" \
         install/wacli-sync-work.service.template > "$(eval echo {{SYSTEMD_USER}})/{{SERVICE_WORK}}"
    @echo "✓ Created {{SERVICE_WORK}}"
    @systemctl --user daemon-reload
    @echo ""
    @echo "Services installed. Use 'just enable-linux' to enable them."

# Enable and start Linux services
enable-linux: _check-linux
    @echo "Enabling Linux services..."
    systemctl --user enable {{SERVICE_PERSONAL}}
    systemctl --user enable {{SERVICE_WORK}}
    systemctl --user start {{SERVICE_PERSONAL}}
    systemctl --user start {{SERVICE_WORK}}
    @echo "✓ Services enabled and started"

# Start Linux sync services
start-linux: _check-linux
    @echo "Starting Linux sync services..."
    systemctl --user start {{SERVICE_PERSONAL}}
    systemctl --user start {{SERVICE_WORK}}
    @echo "✓ Services started"

# Stop Linux sync services
stop-linux: _check-linux
    @echo "Stopping Linux sync services..."
    -systemctl --user stop {{SERVICE_PERSONAL}} 2>/dev/null
    -systemctl --user stop {{SERVICE_WORK}} 2>/dev/null
    @echo "✓ Services stopped"

# Restart Linux sync services
sync-linux: _check-linux
    @echo "Restarting Linux sync services..."
    systemctl --user restart {{SERVICE_PERSONAL}}
    systemctl --user restart {{SERVICE_WORK}}
    @echo "✓ Services restarted"

# Show Linux service status
status-linux: _check-linux
    @echo "=== Linux Service Status ==="
    @echo "Personal:"
    @systemctl --user status {{SERVICE_PERSONAL}} --no-pager || true
    @echo ""
    @echo "Work:"
    @systemctl --user status {{SERVICE_WORK}} --no-pager || true

# Uninstall Linux services
uninstall-service-linux: _check-linux
    @echo "Uninstalling Linux services..."
    -systemctl --user stop {{SERVICE_PERSONAL}} 2>/dev/null
    -systemctl --user stop {{SERVICE_WORK}} 2>/dev/null
    -systemctl --user disable {{SERVICE_PERSONAL}} 2>/dev/null
    -systemctl --user disable {{SERVICE_WORK}} 2>/dev/null
    -rm -f "$(eval echo {{SYSTEMD_USER}})/{{SERVICE_PERSONAL}}"
    -rm -f "$(eval echo {{SYSTEMD_USER}})/{{SERVICE_WORK}}"
    @systemctl --user daemon-reload
    @echo "✓ Services uninstalled"

# ============ Cross-platform Helpers ============

# Uninstall services (auto-detects OS)
uninstall-service:
    #!/usr/bin/env sh
    if [ "$(uname)" = "Darwin" ]; then
        just uninstall-service-macos
    else
        just uninstall-service-linux
    fi

# Show logs (auto-detects OS, shows personal by default)
logs account="personal":
    #!/usr/bin/env sh
    if [ "{{account}}" = "personal" ]; then
        STORE="$(eval echo {{PERSONAL_STORE}})"
    else
        STORE="$(eval echo {{WORK_STORE}})"
    fi
    echo "=== Sync Log ($STORE/sync.log) ==="
    tail -50 "$STORE/sync.log" 2>/dev/null || echo "No log file yet"
    echo ""
    echo "=== Error Log ($STORE/sync.err) ==="
    tail -20 "$STORE/sync.err" 2>/dev/null || echo "No error file yet"

# Follow logs in real-time
logs-follow account="personal":
    #!/usr/bin/env sh
    if [ "{{account}}" = "personal" ]; then
        STORE="$(eval echo {{PERSONAL_STORE}})"
    else
        STORE="$(eval echo {{WORK_STORE}})"
    fi
    tail -f "$STORE/sync.log" "$STORE/sync.err"

# ============ Internal Helpers ============

[private]
_check-macos:
    @if [ "$(uname)" != "Darwin" ]; then echo "Error: This target is macOS only"; exit 1; fi

[private]
_check-linux:
    @if [ "$(uname)" = "Darwin" ]; then echo "Error: This target is Linux only"; exit 1; fi
