# GT7 Leaderboard

Gran Turismo 7 telemetry and leaderboard tooling split between a local Go collector and a Next.js web app backed by Neon Postgres/Drizzle.

## Start Here

- Read `README.md` for the project overview.
- Read `docs/deployment.md` before deploy or service work.
- Read docs under `docs/plans/` for current feature intent.
- For broader project context, use `/Users/aether/obsidian/notes/INDEX.md` only as a routing map and read only relevant GT7 notes.

Current code and live service state are implementation truth. Planning docs and Notion are context unless the user says otherwise.

## Project Structure
- `local-service/` — Go collector service (UDP telemetry, PSN detection, track detection)
- `web/` — Next.js app (Vercel, Neon PostgreSQL, Drizzle ORM)
- `docs/plans/` — Design docs and implementation plans

## Safety

- Do not deploy, change Vercel/Neon settings, or run Drizzle push/generate against a real database unless explicitly authorized.
- Do not print credentials, PSN tokens, database URLs, or Datadog keys.
- Treat collector signing/entitlements and macOS app bundle behavior as sensitive; verify before changing.
- Do not revert unrelated dirty changes.

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

## Coding Patterns

- Keep collector entrypoints thin; put telemetry parsing, track detection, PSN discovery, persistence, and metrics in focused packages under `local-service/internal`.
- Use typed Go structs and table-driven tests for packet parsing, timing math, and detection rules.
- Follow `/Users/aether/obsidian/notes/Projects/Charlemagne/Charlemagne coding standards.md` for Go helper extraction, comments, and error handling style where it fits this repo.
- In the web app, prefer Next.js App Router server components by default. Keep client components only where interactivity requires them.
- Keep database schema and query changes in Drizzle-owned files; document migration intent before touching live data.
- Keep units explicit. Lap times are milliseconds, dates/times are `TIMESTAMPTZ`, and car IDs use GT7 integer IDs.

## Notion Integration
- Project Tracker Page: 317c085e-9118-8105-9ab7-e87ccadf164d
- After completing significant work (features, bug fixes, refactors), update the project's Notion changelog by appending a dated entry with a summary of what changed.
- Update the "Current Focus" and "Next Steps" properties as appropriate.
