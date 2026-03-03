#!/bin/bash
# Launcher script for GT7 Collector .app bundle.
# Resolves the config path relative to the bundle location.
DIR="$(cd "$(dirname "$0")/../../.." && pwd)"
exec "$DIR/GT7Collector.app/Contents/MacOS/collector" --config "$DIR/config.dev.yaml" >> /tmp/gt7-collector.log 2>&1
