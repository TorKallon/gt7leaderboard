# GT7 Collector Deployment

## macOS Firewall (Required for Mac Mini / macOS hosts)

The collector listens for incoming UDP telemetry on port 33740. macOS's application firewall blocks unsigned binaries by default.

After building the collector binary, you must:

1. Ad-hoc sign it:
   ```bash
   codesign -s - -f ./collector
   ```

2. Add it to the firewall allowlist:
   ```bash
   sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /path/to/collector
   sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp /path/to/collector
   ```

3. Verify it was added:
   ```bash
   /usr/libexec/ApplicationFirewall/socketfilterfw --listapps | grep collector
   ```

**Important:** Every time the binary is rebuilt, macOS treats it as a new app and the firewall rule is invalidated. You must re-run the `codesign` and firewall commands after each build. For development, use `go run ./cmd/collector` instead — macOS caches the firewall approval for the stable temp binary path.

## Running the Collector

The collector must run **natively on the host** (not in Docker) because it needs direct LAN access for UDP telemetry. Docker Desktop on macOS uses a Linux VM, so `network_mode: host` does not expose the host's real network.

```bash
# Build
cd local-service
go build -o collector ./cmd/collector
codesign -s - -f ./collector

# Run
./collector --config ./config.yaml
```

PostgreSQL and the web app can still run in Docker:

```bash
docker compose -f docker-compose.dev.yml up -d
```

## Auto-Discovery

When `playstation.ip` is empty in the config, the collector scans all hosts on local /24 subnets every 10 seconds. Once the PS5 responds, it sends targeted heartbeats. If the PS5 stops responding (e.g., DHCP IP change), it rescans automatically.

## Ports

| Port | Protocol | Direction | Purpose |
|------|----------|-----------|---------|
| 33739 | UDP | Outbound | Heartbeat to PS5 |
| 33740 | UDP | Inbound | Telemetry from PS5 |
| 8081 | TCP | Local | Collector web UI |
| 3001 | TCP | Local | Leaderboard web app (dev) |
| 5433 | TCP | Local | PostgreSQL (dev) |
