# wacli justfile

# Build wacli with sqlite_fts5 support
build:
    @echo "Building wacli with sqlite_fts5..."
    CGO_ENABLED=1 go build -tags sqlite_fts5 -o wacli ./cmd/wacli
    @echo "✓ Built ./wacli"

# Install binary to ~/.local/bin
install: build
    @echo "Installing wacli to ~/.local/bin..."
    @mkdir -p ~/.local/bin
    cp wacli ~/.local/bin/wacli
    @echo "✓ Installed to ~/.local/bin/wacli"

# Run tests
test:
    go test ./...

# Clean build artifacts
clean:
    rm -f wacli
