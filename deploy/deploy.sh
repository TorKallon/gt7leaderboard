#!/usr/bin/env bash
set -euo pipefail

# GT7 Collector deploy script
# Usage: ./deploy/deploy.sh
#
# Run from the repo root on the Linux server.
# Pulls latest code, builds the collector, and restarts the service.

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SERVICE_DIR="$REPO_ROOT/local-service"
SERVICE_NAME="gt7-collector@$(whoami)"

echo "==> Pulling latest changes..."
cd "$REPO_ROOT"
git pull --ff-only

echo "==> Building collector..."
cd "$SERVICE_DIR"
go build -o collector ./cmd/collector

echo "==> Restarting service..."
sudo systemctl restart "$SERVICE_NAME"

echo "==> Status:"
sudo systemctl status "$SERVICE_NAME" --no-pager -l

echo ""
echo "==> Done. View logs with: journalctl -u $SERVICE_NAME -f"
