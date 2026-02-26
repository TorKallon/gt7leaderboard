# GT7 Family Leaderboard — Design Document

**Date:** 2026-02-26
**Status:** Approved

---

## 1. Project Overview

A family leaderboard for Gran Turismo 7 that automatically captures lap times from PS5 telemetry, identifies the driver via PSN presence, detects track and car, and publishes results to a hosted web leaderboard.

**Users:** 3 family members + occasional guests
**Domain:** gt7.mcnamara.io

---

## 2. Architecture

### 2.1 Local Collector Service (Go)

Runs on the home LAN (same network as PS5). Dockerized, intended to be always-on.

**Responsibilities:**
- Listen for GT7 UDP telemetry on port 33740
- Send heartbeat packets to PS5 on port 33739
- Decrypt Salsa20-encrypted telemetry packets (custom implementation)
- Detect sessions via idle gap (>=30s no packets)
- At session start: query PSN presence API to identify driver
- Detect track by matching XYZ positions against reference geometry
- Identify car from car ID field + bundled car database
- Record lap times and push to hosted API
- Send Discord webhook notifications for new records
- Serve local web UI for PSN token management and service status
- Auto-refresh car database and track reference data from GitHub
- Report metrics to Datadog

### 2.2 Hosted Web App (Next.js on Vercel)

**Responsibilities:**
- API endpoints to receive lap data from collector
- Neon PostgreSQL for storage
- Google OAuth (@mcnamara.io domain restriction)
- Leaderboard UI with hierarchical views
- Lap/session management UI (reassign driver, tag weather, manage guests)
- Report metrics to Datadog

---

## 3. Key Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Telemetry library | Custom implementation | go-gt7-telemetry is GPL-3.0, no tests, data races, unmaintained. Protocol is simple (Salsa20 + binary struct). |
| Car data source | `ddm999/gt7info` `data-stock-perf.csv` | Single file has car ID, name, manufacturer, group (Gr.1-4/B/N/X), PP values. Updated daily. |
| Track data source | `profittlich/gt7speedboard` `.gt7track` files | 82 track layouts. Files are encrypted telemetry packets — reuse same Salsa20 decryption. |
| Track detection | Brute-force nearest-neighbor elimination | Port of gt7speedboard algorithm. Simple, proven, O(candidates * ref_points). |
| PSN API | Custom Go HTTP client against Sony REST API | No Go PSN library exists. Auth flow well-documented from psn-api (JS) and psnawp (Python). |
| Go config | Viper | Supports YAML config, env vars, hot-reload for token updates. |
| Database | Neon PostgreSQL (direct) | More generous free tier than Vercel Postgres (which is Neon anyway). |
| ORM | Drizzle | Lighter than Prisma, faster Vercel cold starts, SQL-native feel. |
| Auth (web) | NextAuth.js + Google OAuth | Domain restriction to @mcnamara.io. |
| Metrics | Datadog | Covers both local service health and web app observability. |
| PersonalRecord table | Dropped | Premature optimization for 3 users. SQL queries with window functions suffice. |
| Drivetrain field | Dropped from data model | Not available in ddm999/gt7info data. Not critical for leaderboard. |

---

## 4. Data Model

### Driver
```sql
id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
psn_account_id  TEXT (nullable — null for guests. This is the numeric PSN account ID)
psn_online_id   TEXT (nullable — the human-readable PSN gamertag)
display_name    TEXT NOT NULL (e.g., "Rourke", "Jake")
is_guest        BOOLEAN DEFAULT false
created_at      TIMESTAMPTZ DEFAULT now()
updated_at      TIMESTAMPTZ DEFAULT now()
```

### Track
```sql
id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
name            TEXT NOT NULL (e.g., "Trial Mountain")
layout          TEXT NOT NULL (e.g., "Full Course")
slug            TEXT UNIQUE NOT NULL (URL-safe, e.g., "trial-mountain-full")
country         TEXT (nullable)
length_meters   INTEGER (nullable — from course.csv)
num_corners     INTEGER (nullable)
has_weather     BOOLEAN DEFAULT false (whether track supports rain in GT7)
created_at      TIMESTAMPTZ DEFAULT now()
```

### Car
```sql
id              INTEGER PRIMARY KEY (GT7 internal car ID)
name            TEXT NOT NULL (e.g., "BMW M6 GT3 '16")
manufacturer    TEXT NOT NULL
category        TEXT NOT NULL (enum: "Gr.1", "Gr.2", "Gr.3", "Gr.4", "Gr.B", "N", "X")
pp_stock        REAL (stock PP on comfort hard tires, nullable)
created_at      TIMESTAMPTZ DEFAULT now()
updated_at      TIMESTAMPTZ DEFAULT now()
```

### Session
```sql
id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
driver_id       UUID REFERENCES drivers(id) (nullable — null if unknown)
track_id        UUID REFERENCES tracks(id) (nullable — null if unknown track)
car_id          INTEGER REFERENCES cars(id)
started_at      TIMESTAMPTZ NOT NULL
ended_at        TIMESTAMPTZ (nullable — set when session ends)
detection_method TEXT NOT NULL DEFAULT 'unmatched' (enum: "matched", "unmatched", "manual")
created_at      TIMESTAMPTZ DEFAULT now()
updated_at      TIMESTAMPTZ DEFAULT now()
```

### LapRecord
```sql
id                      UUID PRIMARY KEY DEFAULT gen_random_uuid()
driver_id               UUID REFERENCES drivers(id) (nullable — null if unknown)
track_id                UUID REFERENCES tracks(id) (nullable)
car_id                  INTEGER REFERENCES cars(id)
session_id              UUID REFERENCES sessions(id)
lap_time_ms             INTEGER NOT NULL (lap time in milliseconds)
lap_number              INTEGER NOT NULL (lap number within session)
auto_detected_driver_id UUID REFERENCES drivers(id) (immutable — what PSN detected)
weather                 TEXT NOT NULL DEFAULT 'unknown' (enum: "dry", "wet", "unknown")
is_valid                BOOLEAN DEFAULT true (false if discarded/invalidated)
recorded_at             TIMESTAMPTZ NOT NULL (real-world time)
created_at              TIMESTAMPTZ DEFAULT now()
updated_at              TIMESTAMPTZ DEFAULT now()

INDEX idx_lap_records_leaderboard ON lap_records(track_id, car_id, driver_id, lap_time_ms)
INDEX idx_lap_records_driver ON lap_records(driver_id, recorded_at)
INDEX idx_lap_records_session ON lap_records(session_id, lap_number)
```

---

## 5. Telemetry Protocol

### Packet Format (296 bytes after decryption)

Key fields for leaderboard:

| Offset | Type | Field | Usage |
|--------|------|-------|-------|
| 0x00 | int32 | Magic (0x47375330) | Validation |
| 0x04 | float32 | position_x | Track detection |
| 0x08 | float32 | position_y | Track detection |
| 0x0C | float32 | position_z | Track detection |
| 0x10 | float32 | velocity_x | Track detection (direction) |
| 0x14 | float32 | velocity_y | Track detection |
| 0x18 | float32 | velocity_z | Track detection |
| 0x4C | float32 | car_speed (m/s) | Session activity |
| 0x70 | int32 | package_id | Packet sequencing |
| 0x74 | int16 | current_lap | Lap completion detection |
| 0x76 | int16 | total_laps | Context |
| 0x78 | int32 | best_lap_time | Sanity check |
| 0x7C | int32 | last_lap_time | **Primary lap time source** |
| 0x80 | int32 | current_lap_time | Live display |
| 0x124 | int32 | car_id | Car identification |

Flags byte (offset TBD from library analysis):
- `IsPaused` — used for replay detection heuristic
- `IsLoading` — ignore packets during loading
- `InRace` — only record laps when in race

### Salsa20 Decryption
- **Key:** `Simulator Interface Packet GT7 ver 0.0` (first 32 bytes as ASCII)
- **IV:** 4 bytes at packet offset 0x40, XOR'd with `0xDEADBEAF`
- **Library:** `golang.org/x/crypto/salsa20`
- **Validation:** After decryption, check magic == `0x47375330`

### Heartbeat
- Send byte `"A"` to PS5 IP on port 33739 every 10 seconds
- PS5 responds with telemetry stream on port 33740

---

## 6. Track Detection Algorithm

Ported from gt7speedboard (Python → Go).

### Algorithm: Elimination-based nearest-neighbor matching

1. Load all 82 reference tracks on startup (decrypt .gt7track files → extract XYZ + velocity per point)
2. Start with all tracks as candidates
3. For each incoming telemetry point (only when `car_speed > 0`):
   - Wait for minimum 300 points (~5 seconds at 60Hz)
   - For each remaining candidate track:
     - Find closest reference point (3D Euclidean distance)
     - If closest point > `eliminateDistance` (default 30m): **eliminate track**
     - If gap > 10 consecutive unhit reference points between hits: **eliminate track**
     - If close enough, check velocity angle (dot product) — classify as forward or reverse hit
4. Track identified when: 1 candidate remains AND >= 5 hits AND 60 more points received
5. If all candidates share same venue prefix before " - ", report venue early

### Reference Data Auto-Refresh
- On startup + daily: fetch .gt7track files from gt7speedboard GitHub repo
- Compare file list with local cache, download new/updated files
- Decrypt and parse into in-memory reference data
- Fall back to bundled data if fetch fails

---

## 7. Driver Detection (PSN Presence)

### Auth Flow (implemented as Go HTTP client)

1. **NPSSO token** → manual, entered via local web UI at `http://collector:8080/auth`
2. **NPSSO → Authorization Code**: `GET https://ca.account.sony.com/api/authz/v3/oauth/authorize` (don't follow redirect, extract `code` from Location header)
3. **Auth Code → Access + Refresh Tokens**: `POST https://ca.account.sony.com/api/authz/v3/oauth/token`
4. **Refresh flow**: Use refresh token to get new access token when expired (~1 hour)
5. **Token storage**: Persist refresh token to local file (Viper config), encrypt at rest

### Presence Detection

At session start (telemetry resumes after >=30s idle):

1. Call bulk presence endpoint:
   ```
   GET https://m.np.playstation.com/api/userProfile/v2/internal/users//basicPresences
     ?accountIds={id1},{id2},{id3}
     &type=primary
     &platforms=PS5
     &withOwnGameTitleInfo=true
   ```
2. Check which account has `availability: "availableToPlay"` with GT7 in `gameTitleInfoList`
3. **One match** → auto-tag session with that driver (`detection_method: "matched"`)
4. **No match / multiple matches** → tag as Unknown (`detection_method: "unmatched"`)

### GT7 Title Detection
- Check `titleName` contains "Gran Turismo 7" OR `npTitleId` matches known GT7 IDs:
  - PS5: `PPSA01316_00` (check at runtime, may vary by region)
  - PS4: `CUSA24767_00`

### Token Lifecycle & Reminders

**Refresh token (~60 days):**
- Track issue date in local state file
- **14 days before expiry**: Discord notification + local web UI warning banner
- **7 days**: Escalating Discord notification
- **3 days / 1 day**: Critical warnings
- **Expired + session detected**: Immediate Discord alert: "GT7 session detected but PSN auth expired! Laps recording as Unknown Driver."
- Local web UI at `/auth`: paste new NPSSO token, hot-reload via Viper

**Access token (~1 hour):**
- Auto-refresh using refresh token, transparent to user
- Log warning if refresh fails

### Account Setup
- Config file maps PSN online IDs to driver display names
- On first run: resolve online IDs to numeric account IDs via PSN API, cache result
- All configured accounts must be friends of the authenticating account

---

## 8. Session & Lap Recording Flow

1. **Idle detection**: No telemetry packets for >=30 seconds = session boundary
2. **Session start**: New packets arrive →
   - Query PSN presence (identify driver)
   - Begin collecting XYZ samples (track detection)
   - Read car_id from first packet (car identification)
3. **Track detected**: Usually within first partial lap (~5 seconds)
4. **Lap completion**: When `current_lap` increments, read `last_lap_time` → create LapRecord
5. **Car change**: If `car_id` changes mid-session, end current session, start new one
6. **Session end**: No packets for >=30s → close session, set `ended_at`
7. **Push to API**: Each completed lap POSTed immediately. Sessions synced on start/end.

### Lap Validation
- Discard `last_lap_time` == 0
- Discard `last_lap_time` < 10000ms (10 seconds)
- Discard first lap of session (usually incomplete)
- Use `best_lap_time` as sanity check
- Skip packets where `IsPaused` or `IsLoading` flags are set

### Replay Detection
- If `IsPaused` flag toggles in patterns consistent with replay controls, mark session
- If lap times exactly match previously recorded laps, flag as potential replay
- Best-effort — not critical path. Replays are an edge case.

---

## 9. API Design

### Ingest Endpoints (Local Service → Vercel)

Auth: `Authorization: Bearer <INGEST_API_KEY>`

```
POST /api/ingest/sessions
  Body: { driver_id?, track_slug?, car_id, started_at, detection_method }
  Returns: { session_id }

POST /api/ingest/laps
  Body: { session_id, lap_time_ms, lap_number, recorded_at }
  Returns: { lap_id, records: [{ type: "overall"|"category"|"car", previous_time_ms, previous_driver }] }

POST /api/ingest/sessions/:id/end
  Body: { ended_at }
  Returns: { ok: true }

POST /api/ingest/heartbeat
  Body: { status: "online"|"idle", current_session_id?, uptime_seconds }
  Returns: { ok: true }
  (Called every 60s for service health monitoring)
```

### Leaderboard Endpoints (Web UI)

Auth: Google OAuth session (NextAuth.js)

```
GET /api/tracks
  Returns: [{ id, name, layout, slug, lap_count, driver_count, record_holder, record_time_ms }]

GET /api/tracks/:slug/leaderboard
  Query: ?category=Gr.3&car_id=1905&driver_id=xxx
  Returns: { track, entries: [{ rank, driver, lap_time_ms, car, delta_to_leader_ms, achieved_at }] }

GET /api/drivers
  Returns: [{ id, display_name, is_guest, lap_count }]

GET /api/drivers/:id
  Returns: { driver, stats: { total_laps, tracks_driven, favorite_track, favorite_car }, recent_laps, records }

GET /api/sessions
  Query: ?driver_id=xxx&limit=20&offset=0
  Returns: [{ id, driver, track, car, lap_count, started_at, ended_at, detection_method }]

GET /api/sessions/:id
  Returns: { session, laps: [{ id, lap_number, lap_time_ms, weather, is_valid }] }
```

### Management Endpoints (Web UI)

Auth: Google OAuth session

```
PATCH /api/laps/:id
  Body: { driver_id?, weather? }

PATCH /api/sessions/:id/reassign
  Body: { driver_id }
  (Reassigns all laps in session)

POST /api/drivers
  Body: { display_name, psn_online_id? }

PATCH /api/drivers/:id
  Body: { display_name? }

GET /api/status
  Returns: { collector_online, last_heartbeat, current_session?, psn_token_status }
```

---

## 10. Record Detection & Discord Notifications

### Record Detection (in POST /api/ingest/laps handler)

After inserting a new lap, check:
1. **Overall track record**: Is this the fastest lap on this track by any driver, any car?
2. **Category record**: Is this the fastest lap on this track in this car's category?
3. **Car-specific record**: Is this the fastest lap on this track with this specific car?

Each check is a SQL query:
```sql
SELECT MIN(lap_time_ms) FROM lap_records
WHERE track_id = $1 AND is_valid = true AND lap_time_ms < $2
  AND [category filter or car_id filter as appropriate]
```

Return record info in the lap response so the collector can fire Discord webhooks.

### Discord Webhook Format

```json
{
  "embeds": [{
    "title": "New Record!",
    "description": "Rourke just set a new Trial Mountain record!",
    "color": 16766720,
    "fields": [
      { "name": "Car", "value": "BMW M6 GT3 '16 (Gr.3)", "inline": true },
      { "name": "Time", "value": "1:42.387", "inline": true },
      { "name": "Previous", "value": "1:43.102 (Rourke)", "inline": true },
      { "name": "Category", "value": "Gr.3", "inline": true }
    ],
    "url": "https://gt7.mcnamara.io/tracks/trial-mountain-full"
  }]
}
```

### Notification Rules
- **Overall track record**: Always notify
- **Category record**: Always notify
- **Car-specific record**: Configurable (default off — can be noisy)
- **Debouncing**: If multiple records set within same session, batch into single notification at session end

---

## 11. Leaderboard Hierarchy

For each track:

1. **Overall** — fastest lap ever, any car, any driver
2. **By Category** — Gr.1, Gr.2, Gr.3, Gr.4, Gr.B, N (with PP sub-bands), X
3. **By Car** — head-to-head same-car comparison

### Road Car (N) PP Sub-Bands
Based on stock PP (Comfort Hard tires from `data-stock-perf.csv`):
- **N100-300** — kei cars, economy
- **N300-500** — hot hatches, sports cars
- **N500-700** — supercars, muscle
- **N700+** — hypercars, exotics

Known limitation: tuned cars appear in stock PP bucket.

---

## 12. Web UI Pages

### Auth
- Google OAuth via NextAuth.js, @mcnamara.io domain only
- Unauthenticated → login page

### Dashboard (/)
- Recent activity feed (last N laps)
- Quick stats: total laps, tracks driven, per-driver lap counts
- Collector status: online/offline, last heartbeat, PSN token status
- "Unclaimed laps" alert if Unknown Driver sessions exist

### Track List (/tracks)
- Grid of tracks with lap count, driver count, record holder + time
- Search/filter by name

### Track Leaderboard (/tracks/:slug)
- **Overall tab**: Best lap per driver, ranked
- **Category tabs**: Gr.1, Gr.2, Gr.3, Gr.4, Gr.B, N (PP sub-bands), X
- **Car filter**: Select specific car for head-to-head
- Each entry: rank, driver, lap time, car, delta to leader, date

### Driver Profile (/drivers/:id)
- Personal records across tracks
- Recent laps
- Stats: favorite track, favorite car, total laps

### Session Management (/sessions)
- List sessions with driver, track, car, lap count
- Click into session → see all laps
- Reassign driver (preserves auto_detected_driver_id)
- Tag weather (dry/wet)
- Create guest driver inline

### Settings (/settings)
- Collector status and health
- PSN token expiry warning
- Driver management (add/edit guests, configure PSN IDs)

---

## 13. Auto-Refresh Pipeline

### Car Data
- **Source**: `https://ddm999.github.io/gt7info/data-stock-perf.csv`
- **Schedule**: On collector startup + every 24 hours
- **Process**: Fetch CSV → parse → upsert to local cache + push to API for DB sync
- **Fallback**: Bundled CSV in Docker image if fetch fails
- **Fields**: car_id, name, manufacturer, category (group), pp_stock (CH tire value)

### Track Reference Data
- **Source**: `https://github.com/profittlich/gt7speedboard/tree/master/tracks/` (individual .gt7track files)
- **Schedule**: On collector startup + weekly (tracks change less often)
- **Process**: Fetch file list via GitHub API → download new/updated .gt7track files → decrypt → parse to reference points
- **Fallback**: Bundled .gt7track files in Docker image
- **Track metadata**: Additionally fetch `https://ddm999.github.io/gt7info/data/db/course.csv` for track length, corners, etc.

### Track DB Sync
- When collector detects a track, if it doesn't exist in the web DB, push track metadata via ingest API
- Web app has a `POST /api/ingest/tracks` endpoint for this

---

## 14. Observability (Datadog)

### Local Collector Metrics
- `gt7.telemetry.packets_received` (counter)
- `gt7.telemetry.packets_decryption_failed` (counter)
- `gt7.session.active` (gauge, 0 or 1)
- `gt7.session.laps_recorded` (counter per session)
- `gt7.track_detection.time_ms` (histogram)
- `gt7.track_detection.candidates_remaining` (gauge)
- `gt7.psn.token_days_remaining` (gauge)
- `gt7.psn.auth_failures` (counter)
- `gt7.api.push_latency_ms` (histogram)
- `gt7.api.push_failures` (counter)
- `gt7.data_refresh.last_success` (gauge, unix timestamp)

### Web App Metrics
- `gt7.api.ingest_requests` (counter, tagged by endpoint)
- `gt7.api.leaderboard_requests` (counter)
- `gt7.api.latency_ms` (histogram, tagged by endpoint)
- `gt7.records.new_records` (counter, tagged by type)

---

## 15. Configuration

### Local Collector (config.yaml via Viper)

```yaml
playstation:
  ip: "192.168.1.100"
  telemetry_send_port: 33739
  telemetry_listen_port: 33740

psn:
  npsso_token: ""  # Set via web UI at /auth
  accounts:
    - online_id: "dad_psn_name"
      driver_name: "Rourke"
    - online_id: "son_psn_name"
      driver_name: "Son"
    - online_id: "daughter_psn_name"
      driver_name: "Daughter"

api:
  endpoint: "https://gt7.mcnamara.io"
  api_key: "<shared secret>"

discord:
  webhook_url: "https://discord.com/api/webhooks/..."
  notify_overall_records: true
  notify_category_records: true
  notify_car_records: false

datadog:
  enabled: true
  api_key: "<dd api key>"
  site: "datadoghq.com"
  service: "gt7-collector"
  env: "production"

session:
  idle_timeout_seconds: 30
  track_detection_min_points: 300

data_refresh:
  car_data_url: "https://ddm999.github.io/gt7info/data-stock-perf.csv"
  car_refresh_interval_hours: 24
  track_data_repo: "profittlich/gt7speedboard"
  track_refresh_interval_hours: 168  # weekly
```

### Vercel Environment Variables

```
DATABASE_URL=<Neon connection string>
GOOGLE_CLIENT_ID=<OAuth client ID>
GOOGLE_CLIENT_SECRET=<OAuth client secret>
NEXTAUTH_SECRET=<random secret>
NEXTAUTH_URL=https://gt7.mcnamara.io
INGEST_API_KEY=<shared secret>
DISCORD_WEBHOOK_URL=<webhook URL>
ALLOWED_DOMAIN=mcnamara.io
DD_API_KEY=<Datadog API key>
DD_SITE=datadoghq.com
```

---

## 16. Deployment

### Local Collector (Docker)

```dockerfile
FROM golang:1.23-alpine AS builder
# Build for target arch
RUN CGO_ENABLED=0 go build -o /collector ./cmd/collector

FROM alpine:3.19
COPY --from=builder /collector /usr/local/bin/collector
COPY data/ /app/data/  # Bundled fallback car/track data
ENTRYPOINT ["collector"]
```

```yaml
# docker-compose.yml
services:
  collector:
    image: ghcr.io/rourkem/gt7-collector:latest
    ports:
      - "33740:33740/udp"  # Telemetry listen
      - "8080:8080"        # Local web UI (status, auth)
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./data:/app/data   # Persistent cache for car/track data
    environment:
      - DD_API_KEY=${DD_API_KEY}
    restart: unless-stopped
```

Multi-arch build: linux/amd64, linux/arm64. Native `go run` for macOS development.

### Web App (Vercel)

- Next.js deployed to Vercel free tier
- CNAME: gt7.mcnamara.io → cname.vercel-dns.com
- Neon PostgreSQL provisioned separately
- Drizzle migrations run via `npx drizzle-kit push`

---

## 17. Monorepo Structure

```
gt7leaderboard/
  local-service/           # Go collector
    cmd/
      collector/           # Main binary
      update-data/         # Manual data refresh tool
    internal/
      telemetry/           # UDP listener, Salsa20 decrypt, packet parsing
      session/             # Session detection, lap recording
      trackdetect/         # Track detection algorithm
      psn/                 # PSN API client, auth, presence
      cardb/               # Car database loading and lookup
      api/                 # HTTP client for pushing to hosted API
      discord/             # Discord webhook client
      config/              # Viper config management
      metrics/             # Datadog metrics
      webui/               # Local status/auth web UI
    data/                  # Bundled fallback data
      cars/                # Car CSV
      tracks/              # .gt7track files
    Dockerfile
    docker-compose.yml
  web/                     # Next.js app
    src/
      app/                 # App Router pages
        (auth)/            # Login page
        page.tsx           # Dashboard
        tracks/
          page.tsx         # Track list
          [slug]/
            page.tsx       # Track leaderboard
        drivers/
          page.tsx         # Driver list
          [id]/
            page.tsx       # Driver profile
        sessions/
          page.tsx         # Session list
          [id]/
            page.tsx       # Session detail
        settings/
          page.tsx         # Settings/admin
        api/
          ingest/          # Ingest endpoints
          tracks/          # Leaderboard endpoints
          drivers/         # Driver endpoints
          sessions/        # Session endpoints
          status/          # Collector status endpoint
          auth/            # NextAuth.js
      lib/
        db/                # Drizzle schema, queries
        auth/              # NextAuth config
        discord/           # Discord webhook (server-side)
        metrics/           # Datadog RUM + server metrics
      components/          # React components
    drizzle/               # Migration files
    package.json
    vercel.json
  docs/
    plans/                 # Design docs and plans
  CLAUDE.md
  README.md
```

---

## 18. Known Limitations

- **Tuned cars**: Cannot detect modifications. Stock PP used for N-class bucketing.
- **Weather**: Not in telemetry. Defaults to "unknown", manual tagging in UI.
- **Couch handoff**: If someone plays on another's PSN profile, auto-detection attributes wrong. Manual reassignment available.
- **PSN NPSSO renewal**: ~60 day expiry, requires manual browser login. Reminders via Discord + web UI.
- **New tracks**: Unrecognized until gt7speedboard adds reference data. Laps recorded as "Unknown Track."
- **New cars**: Appear as "Car ID 1234" until ddm999/gt7info updates.
- **Replay telemetry**: Best-effort detection via IsPaused flag. May occasionally record replay laps.
- **Single console**: Assumes one PS5 on network.
- **Docker UDP on macOS**: Requires explicit port mapping. Native `go run` recommended for development.
