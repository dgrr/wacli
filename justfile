# wacli justfile - Build and service management

# Default binary install location
BINARY_DIR := env_var_or_default("WACLI_BINARY_DIR", "~/.local/bin")
BINARY_PATH := BINARY_DIR / "wacli"

# Default store path (override with WACLI_STORE)
DEFAULT_STORE := env_var_or_default("WACLI_STORE", "~/.wacli")

# macOS paths
LAUNCH_AGENTS := "~/Library/LaunchAgents"

# Linux systemd paths
SYSTEMD_USER := "~/.config/systemd/user"

# Build wacli with sqlite_fts5 support
build:
    @echo "Building wacli with sqlite_fts5..."
    CGO_ENABLED=1 go build -tags sqlite_fts5 -o wacli ./cmd/wacli

# Install binary to ~/.local/bin (or WACLI_BINARY_DIR)
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

# Install a macOS launchd service for a specific store
# Usage: just install-service-macos [name] [store_path]
# Example: just install-service-macos default ~/.wacli
#          just install-service-macos work ~/.wacli-work
install-service-macos name="default" store=DEFAULT_STORE: _check-macos
    #!/usr/bin/env bash
    set -euo pipefail
    LABEL="com.wacli.sync.{{name}}"
    PLIST_FILE="$LABEL.plist"
    STORE_PATH="$(eval echo {{store}})"
    BINARY="$(eval echo {{BINARY_PATH}})"
    AGENTS_DIR="$(eval echo {{LAUNCH_AGENTS}})"
    
    echo "Installing macOS service '$LABEL'..."
    mkdir -p "$AGENTS_DIR"
    mkdir -p "$STORE_PATH"
    
    sed -e "s|__SERVICE_LABEL__|$LABEL|g" \
        -e "s|__BINARY_PATH__|$BINARY|g" \
        -e "s|__STORE_PATH__|$STORE_PATH|g" \
        -e "s|__HOME__|$HOME|g" \
        install/com.wacli.sync.plist.template > "$AGENTS_DIR/$PLIST_FILE"
    
    echo "✓ Created $PLIST_FILE"
    echo "  Store: $STORE_PATH"
    echo "  Logs:  $STORE_PATH/sync.log"
    echo ""
    echo "To load: launchctl load $AGENTS_DIR/$PLIST_FILE"
    echo "To start: launchctl start $LABEL"

# Load a macOS service
load-macos name="default": _check-macos
    #!/usr/bin/env bash
    LABEL="com.wacli.sync.{{name}}"
    PLIST="$(eval echo {{LAUNCH_AGENTS}})/$LABEL.plist"
    launchctl unload "$PLIST" 2>/dev/null || true
    launchctl load "$PLIST"
    echo "✓ Loaded $LABEL"

# Start a macOS service
start-macos name="default": _check-macos
    launchctl start "com.wacli.sync.{{name}}"
    @echo "✓ Started com.wacli.sync.{{name}}"

# Stop a macOS service
stop-macos name="default": _check-macos
    -launchctl stop "com.wacli.sync.{{name}}" 2>/dev/null
    @echo "✓ Stopped com.wacli.sync.{{name}}"

# Restart a macOS service (unload + load + start)
restart-macos name="default": _check-macos
    #!/usr/bin/env bash
    LABEL="com.wacli.sync.{{name}}"
    PLIST="$(eval echo {{LAUNCH_AGENTS}})/$LABEL.plist"
    launchctl stop "$LABEL" 2>/dev/null || true
    launchctl unload "$PLIST" 2>/dev/null || true
    launchctl load "$PLIST"
    launchctl start "$LABEL"
    echo "✓ Restarted $LABEL"

# Show macOS service status
status-macos name="default": _check-macos
    @echo "=== Service: com.wacli.sync.{{name}} ==="
    @launchctl list | grep "com.wacli.sync.{{name}}" || echo "Not loaded"

# List all wacli macOS services
list-macos: _check-macos
    @echo "=== wacli Services ==="
    @launchctl list | grep com.wacli.sync || echo "No services found"

# Uninstall a macOS service
uninstall-service-macos name="default": _check-macos
    #!/usr/bin/env bash
    LABEL="com.wacli.sync.{{name}}"
    PLIST="$(eval echo {{LAUNCH_AGENTS}})/$LABEL.plist"
    launchctl stop "$LABEL" 2>/dev/null || true
    launchctl unload "$PLIST" 2>/dev/null || true
    rm -f "$PLIST"
    echo "✓ Uninstalled $LABEL"

# ============ Linux Service Management ============

# Install a Linux systemd service for a specific store
# Usage: just install-service-linux [name] [store_path]
# Example: just install-service-linux default ~/.wacli
#          just install-service-linux work ~/.wacli-work
install-service-linux name="default" store=DEFAULT_STORE: _check-linux
    #!/usr/bin/env bash
    set -euo pipefail
    SERVICE_NAME="wacli-sync-{{name}}.service"
    STORE_PATH="$(eval echo {{store}})"
    BINARY="$(eval echo {{BINARY_PATH}})"
    SYSTEMD_DIR="$(eval echo {{SYSTEMD_USER}})"
    
    echo "Installing Linux service '$SERVICE_NAME'..."
    mkdir -p "$SYSTEMD_DIR"
    mkdir -p "$STORE_PATH"
    
    sed -e "s|__INSTANCE_NAME__|{{name}}|g" \
        -e "s|__BINARY_PATH__|$BINARY|g" \
        -e "s|__STORE_PATH__|$STORE_PATH|g" \
        -e "s|__HOME__|$HOME|g" \
        install/wacli-sync.service.template > "$SYSTEMD_DIR/$SERVICE_NAME"
    
    systemctl --user daemon-reload
    echo "✓ Created $SERVICE_NAME"
    echo "  Store: $STORE_PATH"
    echo "  Logs:  $STORE_PATH/sync.log"
    echo ""
    echo "To enable: systemctl --user enable $SERVICE_NAME"
    echo "To start:  systemctl --user start $SERVICE_NAME"

# Enable and start a Linux service
enable-linux name="default": _check-linux
    systemctl --user enable "wacli-sync-{{name}}.service"
    systemctl --user start "wacli-sync-{{name}}.service"
    @echo "✓ Enabled and started wacli-sync-{{name}}.service"

# Start a Linux service
start-linux name="default": _check-linux
    systemctl --user start "wacli-sync-{{name}}.service"
    @echo "✓ Started wacli-sync-{{name}}.service"

# Stop a Linux service
stop-linux name="default": _check-linux
    -systemctl --user stop "wacli-sync-{{name}}.service" 2>/dev/null
    @echo "✓ Stopped wacli-sync-{{name}}.service"

# Restart a Linux service
restart-linux name="default": _check-linux
    systemctl --user restart "wacli-sync-{{name}}.service"
    @echo "✓ Restarted wacli-sync-{{name}}.service"

# Show Linux service status
status-linux name="default": _check-linux
    @systemctl --user status "wacli-sync-{{name}}.service" --no-pager || true

# List all wacli Linux services
list-linux: _check-linux
    @echo "=== wacli Services ==="
    @systemctl --user list-units 'wacli-sync-*.service' --no-pager || echo "No services found"

# Uninstall a Linux service
uninstall-service-linux name="default": _check-linux
    #!/usr/bin/env bash
    SERVICE_NAME="wacli-sync-{{name}}.service"
    SYSTEMD_DIR="$(eval echo {{SYSTEMD_USER}})"
    systemctl --user stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl --user disable "$SERVICE_NAME" 2>/dev/null || true
    rm -f "$SYSTEMD_DIR/$SERVICE_NAME"
    systemctl --user daemon-reload
    echo "✓ Uninstalled $SERVICE_NAME"

# ============ Cross-platform Helpers ============

# Show logs for a service
logs name="default" store=DEFAULT_STORE:
    #!/usr/bin/env bash
    STORE="$(eval echo {{store}})"
    echo "=== Sync Log ($STORE/sync.log) ==="
    tail -50 "$STORE/sync.log" 2>/dev/null || echo "No log file yet"
    echo ""
    echo "=== Error Log ($STORE/sync.err) ==="
    tail -20 "$STORE/sync.err" 2>/dev/null || echo "No error file yet"

# Follow logs in real-time
logs-follow name="default" store=DEFAULT_STORE:
    #!/usr/bin/env bash
    STORE="$(eval echo {{store}})"
    tail -f "$STORE/sync.log" "$STORE/sync.err"

# ============ Internal Helpers ============

[private]
_check-macos:
    @if [ "$(uname)" != "Darwin" ]; then echo "Error: This target is macOS only"; exit 1; fi

[private]
_check-linux:
    @if [ "$(uname)" = "Darwin" ]; then echo "Error: This target is Linux only"; exit 1; fi
