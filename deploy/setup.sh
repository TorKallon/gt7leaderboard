#!/usr/bin/env bash
set -euo pipefail

# GT7 Collector first-time setup on a Linux server.
# Usage: ./deploy/setup.sh
#
# Prerequisites:
#   - Go installed (https://go.dev/dl/)
#   - Git installed
#   - Repo cloned to ~/gt7leaderboard
#
# After running this script:
#   1. Copy config.example.yaml to config.yaml and fill in your values
#   2. Run: ./deploy/deploy.sh

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SERVICE_DIR="$REPO_ROOT/local-service"
USER="$(whoami)"
SERVICE_NAME="gt7-collector@${USER}"

echo "==> Setting up GT7 Collector for user: $USER"
echo "    Repo: $REPO_ROOT"

# Create data directories.
echo "==> Creating data directories..."
mkdir -p "$SERVICE_DIR/data/cars/raw"
mkdir -p "$SERVICE_DIR/data/tracks"

# Build the collector.
echo "==> Building collector..."
cd "$SERVICE_DIR"
go build -o collector ./cmd/collector

# Install systemd unit.
echo "==> Installing systemd service..."
sudo cp "$REPO_ROOT/deploy/gt7-collector.service" /etc/systemd/system/gt7-collector@.service
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"

# Check for config.
if [ ! -f "$SERVICE_DIR/config.yaml" ]; then
    echo ""
    echo "!!! config.yaml not found."
    echo "    Copy the example and fill in your values:"
    echo ""
    echo "    cp $SERVICE_DIR/config.example.yaml $SERVICE_DIR/config.yaml"
    echo "    nano $SERVICE_DIR/config.yaml"
    echo ""
    echo "    Then start with: sudo systemctl start $SERVICE_NAME"
else
    echo "==> Starting service..."
    sudo systemctl start "$SERVICE_NAME"
    sudo systemctl status "$SERVICE_NAME" --no-pager -l
fi

echo ""
echo "==> Setup complete!"
echo "    Logs:    journalctl -u $SERVICE_NAME -f"
echo "    Deploy:  ./deploy/deploy.sh"
echo "    Status:  sudo systemctl status $SERVICE_NAME"
