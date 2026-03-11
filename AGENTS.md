# GT7 Leaderboard

## Project Structure
- `local-service/` — Go collector service (UDP telemetry, PSN detection, track detection)
- `web/` — Next.js app (Vercel, Neon PostgreSQL, Drizzle ORM)
- `docs/plans/` — Design docs and implementation plans

## Go Commands
- Build: `cd local-service && go build ./cmd/collector`
- Test: `cd local-service && go test ./...`
- Single package test: `cd local-service && go test ./internal/telemetry/... -v`
- Race detection: `cd local-service && go test ./... -race`

## Web Commands
- Dev: `cd web && npm run dev`
- Build: `cd web && npm run build`
- Lint: `cd web && npm run lint`
- DB push: `cd web && npx drizzle-kit push`
- DB generate: `cd web && npx drizzle-kit generate`

## Conventions
- Go: standard library style, internal packages, table-driven tests
- TypeScript: App Router, server components by default, Drizzle for DB
- All times stored as milliseconds (lap_time_ms) or TIMESTAMPTZ
- UUIDs for all primary keys except Car (uses GT7 integer ID)
- Config: Viper (Go), environment variables (web)
- Metrics: Datadog (both services)

## Notion Integration
- Project Tracker Page: 317c085e-9118-8105-9ab7-e87ccadf164d
- After completing significant work (features, bug fixes, refactors), update the project's Notion changelog by appending a dated entry with a summary of what changed.
- Update the "Current Focus" and "Next Steps" properties as appropriate.
