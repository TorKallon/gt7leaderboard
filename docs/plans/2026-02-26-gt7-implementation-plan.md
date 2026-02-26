# GT7 Family Leaderboard — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a complete GT7 family leaderboard system: a Go telemetry collector service and a Next.js web app with leaderboards, session management, and Discord notifications.

**Architecture:** Monorepo with `local-service/` (Go) and `web/` (Next.js). The Go collector runs on the home LAN, captures PS5 telemetry, identifies drivers via PSN, detects tracks/cars, and pushes lap data to the hosted Next.js API on Vercel. The web app stores data in Neon PostgreSQL, serves leaderboard UI, and fires Discord webhooks for records.

**Tech Stack:** Go 1.23, Viper, Salsa20, Datadog statsd | Next.js 14 (App Router), Drizzle ORM, Neon PostgreSQL, NextAuth.js, Tailwind CSS, Vercel

**Design Doc:** `docs/plans/2026-02-26-gt7-leaderboard-design.md`

---

## Task Groups (Parallelization Map)

```
Group A: Scaffolding (sequential, do first)
  Task 1: Monorepo + Go module + Next.js app

Group B: Go Core Libraries (parallel after A)
  Task 2: Telemetry - Salsa20 + packet parsing
  Task 3: Car database loader
  Task 4: Track reference loader + detection algorithm
  Task 5: PSN API client (auth + presence)
  Task 6: Config management (Viper)
  Task 7: Discord webhook client
  Task 8: API push client
  Task 9: Datadog metrics

Group C: Web Foundation (parallel with B, after A)
  Task 10: Drizzle schema + migrations
  Task 11: NextAuth.js setup
  Task 12: Ingest API endpoints
  Task 13: Leaderboard + management API endpoints

Group D: Go Integration (after B)
  Task 14: Session manager + lap recorder
  Task 15: Data auto-refresh pipeline
  Task 16: Local web UI (status + auth)
  Task 17: Main collector binary (wire everything)

Group E: Web UI (after C)
  Task 18: UI components + dashboard
  Task 19: Track list + leaderboard pages
  Task 20: Driver profile + session management pages

Group F: Deployment (after D + E)
  Task 21: Dockerfile + docker-compose + Vercel config
```

---

## Task 1: Monorepo Scaffolding

**Files:**
- Create: `local-service/go.mod`
- Create: `local-service/cmd/collector/main.go`
- Create: `web/package.json`
- Create: `web/src/app/layout.tsx`
- Create: `web/src/app/page.tsx`
- Create: `web/tailwind.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/drizzle.config.ts`
- Create: `web/next.config.js`
- Create: `.gitignore`
- Create: `CLAUDE.md`

**Step 1: Initialize Go module**

```bash
cd local-service
go mod init github.com/rourkem/gt7leaderboard/local-service
```

Create `cmd/collector/main.go`:
```go
package main

import "fmt"

func main() {
    fmt.Println("GT7 Collector starting...")
}
```

**Step 2: Initialize Next.js app**

```bash
cd web
npx create-next-app@latest . --typescript --tailwind --eslint --app --src-dir --import-alias "@/*" --no-turbopack
npm install drizzle-orm @neondatabase/serverless
npm install -D drizzle-kit
npm install next-auth@beta @auth/drizzle-adapter
npm install dd-trace
```

**Step 3: Create .gitignore**

```gitignore
# Go
local-service/collector
local-service/update-data
local-service/data/cache/

# Node
web/node_modules/
web/.next/
web/.env.local

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store

# Data
*.env
config.yaml
```

**Step 4: Create CLAUDE.md**

```markdown
# GT7 Leaderboard

## Project Structure
- `local-service/` — Go collector service (UDP telemetry, PSN detection, track detection)
- `web/` — Next.js app (Vercel, Neon PostgreSQL, Drizzle ORM)
- `docs/plans/` — Design docs and implementation plans

## Go Commands
- Build: `cd local-service && go build ./cmd/collector`
- Test: `cd local-service && go test ./...`
- Single package test: `cd local-service && go test ./internal/telemetry/...`

## Web Commands
- Dev: `cd web && npm run dev`
- Build: `cd web && npm run build`
- Test: `cd web && npm test`
- DB push: `cd web && npx drizzle-kit push`
- DB generate: `cd web && npx drizzle-kit generate`

## Conventions
- Go: standard library style, internal packages, table-driven tests
- TypeScript: App Router, server components by default, Drizzle for DB
- All times stored as milliseconds (lap_time_ms) or TIMESTAMPTZ
- UUIDs for all primary keys except Car (uses GT7 integer ID)
```

**Step 5: Verify builds**

```bash
cd local-service && go build ./cmd/collector
cd ../web && npm run build
```

**Step 6: Commit**

```bash
git add -A
git commit -m "feat: initialize monorepo with Go module and Next.js app"
```

---

## Task 2: Telemetry — Salsa20 Decryption + Packet Parsing

**Files:**
- Create: `local-service/internal/telemetry/crypto.go`
- Create: `local-service/internal/telemetry/crypto_test.go`
- Create: `local-service/internal/telemetry/packet.go`
- Create: `local-service/internal/telemetry/packet_test.go`
- Create: `local-service/internal/telemetry/listener.go`
- Create: `local-service/internal/telemetry/listener_test.go`
- Test: `local-service/internal/telemetry/`

**Dependencies:**
```bash
cd local-service
go get golang.org/x/crypto
```

**Step 1: Write crypto tests**

Test vectors: create a known plaintext, encrypt with Salsa20 using the GT7 key, then verify decryption recovers it. Also test with a real captured packet if available, or construct one with the correct magic number.

```go
// crypto_test.go
package telemetry

import (
    "encoding/binary"
    "testing"
)

func TestDecryptPacket_ValidPacket(t *testing.T) {
    // Create a fake 296-byte packet with known structure
    plain := make([]byte, 296)
    // Set magic at offset 0x00
    binary.LittleEndian.PutUint32(plain[0x00:], MagicNumber) // 0x47375330
    // Set a car_id at offset 0x124
    binary.LittleEndian.PutUint32(plain[0x124:], 42)
    // Set position_x at offset 0x04
    // ... (set float32 values)

    encrypted := encryptPacketForTest(plain)
    decrypted, err := DecryptPacket(encrypted)
    if err != nil {
        t.Fatalf("DecryptPacket failed: %v", err)
    }
    magic := binary.LittleEndian.Uint32(decrypted[0x00:])
    if magic != MagicNumber {
        t.Errorf("magic = 0x%X, want 0x%X", magic, MagicNumber)
    }
}

func TestDecryptPacket_TooShort(t *testing.T) {
    _, err := DecryptPacket(make([]byte, 100))
    if err == nil {
        t.Error("expected error for short packet")
    }
}

func TestDecryptPacket_BadMagic(t *testing.T) {
    plain := make([]byte, 296)
    binary.LittleEndian.PutUint32(plain[0x00:], 0xDEADBEEF) // wrong magic
    encrypted := encryptPacketForTest(plain)
    _, err := DecryptPacket(encrypted)
    if err == nil {
        t.Error("expected error for bad magic")
    }
}
```

**Step 2: Implement crypto**

```go
// crypto.go
package telemetry

import (
    "encoding/binary"
    "errors"
    "golang.org/x/crypto/salsa20"
)

const (
    MagicNumber    = 0x47375330
    PacketSize     = 296
    IVOffset       = 0x40
    IVXORMask      = 0xDEADBEAF
)

var salsa20Key = func() [32]byte {
    var key [32]byte
    copy(key[:], []byte("Simulator Interface Packet GT7 ver 0.0")[:32])
    return key
}()

func DecryptPacket(data []byte) ([]byte, error) {
    if len(data) < PacketSize {
        return nil, errors.New("packet too short")
    }

    // Extract IV from offset 0x40 (4 bytes), XOR with mask, pad to 8 bytes
    ivRaw := binary.LittleEndian.Uint32(data[IVOffset : IVOffset+4])
    ivVal := ivRaw ^ IVXORMask
    var nonce [8]byte
    binary.LittleEndian.PutUint32(nonce[:4], ivVal)

    // Decrypt
    out := make([]byte, len(data))
    copy(out, data)
    salsa20.XORKeyStream(out, out, &nonce, &salsa20Key)

    // Validate magic
    magic := binary.LittleEndian.Uint32(out[0x00:])
    if magic != MagicNumber {
        return nil, errors.New("invalid magic after decryption")
    }

    return out, nil
}
```

**Step 3: Write packet parsing tests**

```go
// packet_test.go
package telemetry

import (
    "testing"
    "math"
)

func TestParsePacket(t *testing.T) {
    raw := makeTestPacketBytes(func(p *RawPacketData) {
        p.PositionX = 100.5
        p.PositionY = 50.0
        p.PositionZ = -30.2
        p.CarSpeed = 45.0
        p.CurrentLap = 3
        p.LastLapTime = 92387
        p.CarID = 1905
        p.PackageID = 12345
    })
    pkt, err := ParsePacket(raw)
    if err != nil {
        t.Fatalf("ParsePacket: %v", err)
    }
    if pkt.CarID != 1905 {
        t.Errorf("CarID = %d, want 1905", pkt.CarID)
    }
    if pkt.CurrentLap != 3 {
        t.Errorf("CurrentLap = %d, want 3", pkt.CurrentLap)
    }
    if pkt.LastLapTimeMs != 92387 {
        t.Errorf("LastLapTimeMs = %d, want 92387", pkt.LastLapTimeMs)
    }
    if math.Abs(float64(pkt.Position.X)-100.5) > 0.01 {
        t.Errorf("PositionX = %f, want 100.5", pkt.Position.X)
    }
}

func TestParsePacket_Flags(t *testing.T) {
    raw := makeTestPacketBytes(func(p *RawPacketData) {
        p.Flags = 0x02 // IsPaused
    })
    pkt, _ := ParsePacket(raw)
    if !pkt.IsPaused {
        t.Error("expected IsPaused = true")
    }
    if pkt.InRace {
        t.Error("expected InRace = false")
    }
}
```

**Step 4: Implement packet parsing**

```go
// packet.go
package telemetry

import (
    "encoding/binary"
    "errors"
    "math"
)

type Vec3 struct {
    X, Y, Z float32
}

type Packet struct {
    Position      Vec3
    Velocity      Vec3
    CarSpeed      float32  // m/s
    PackageID     int32
    CurrentLap    int16
    TotalLaps     int16
    BestLapTimeMs int32
    LastLapTimeMs int32
    CurrentTimeMs int32
    CarID         int32
    IsPaused      bool
    IsLoading     bool
    InRace        bool
    Timestamp     int64 // local receive time (unix ms)
}

func ParsePacket(data []byte) (*Packet, error) {
    if len(data) < PacketSize {
        return nil, errors.New("data too short for packet")
    }
    p := &Packet{}

    // Position (offsets 0x04, 0x08, 0x0C)
    p.Position.X = math.Float32frombits(binary.LittleEndian.Uint32(data[0x04:]))
    p.Position.Y = math.Float32frombits(binary.LittleEndian.Uint32(data[0x08:]))
    p.Position.Z = math.Float32frombits(binary.LittleEndian.Uint32(data[0x0C:]))

    // Velocity (offsets 0x10, 0x14, 0x18)
    p.Velocity.X = math.Float32frombits(binary.LittleEndian.Uint32(data[0x10:]))
    p.Velocity.Y = math.Float32frombits(binary.LittleEndian.Uint32(data[0x14:]))
    p.Velocity.Z = math.Float32frombits(binary.LittleEndian.Uint32(data[0x18:]))

    // Car speed (offset 0x4C)
    p.CarSpeed = math.Float32frombits(binary.LittleEndian.Uint32(data[0x4C:]))

    // Package ID (offset 0x70)
    p.PackageID = int32(binary.LittleEndian.Uint32(data[0x70:]))

    // Lap info (offsets 0x74, 0x76)
    p.CurrentLap = int16(binary.LittleEndian.Uint16(data[0x74:]))
    p.TotalLaps = int16(binary.LittleEndian.Uint16(data[0x76:]))

    // Lap times (offsets 0x78, 0x7C, 0x80)
    p.BestLapTimeMs = int32(binary.LittleEndian.Uint32(data[0x78:]))
    p.LastLapTimeMs = int32(binary.LittleEndian.Uint32(data[0x7C:]))
    p.CurrentTimeMs = int32(binary.LittleEndian.Uint32(data[0x80:]))

    // Car ID (offset 0x124)
    p.CarID = int32(binary.LittleEndian.Uint32(data[0x124:]))

    // Flags (offset 0x8E based on go-gt7-telemetry analysis)
    flags := data[0x8E]
    p.InRace = flags&0x01 != 0
    p.IsPaused = flags&0x02 != 0
    p.IsLoading = flags&0x04 != 0

    return p, nil
}
```

**Step 5: Write listener with interface**

```go
// listener.go
package telemetry

import (
    "context"
    "fmt"
    "net"
    "time"
)

type PacketHandler func(*Packet)

type Listener struct {
    psIP         string
    sendPort     int
    listenPort   int
    handler      PacketHandler
    heartbeatSec int
}

func NewListener(psIP string, sendPort, listenPort int, handler PacketHandler) *Listener {
    return &Listener{
        psIP:         psIP,
        sendPort:     sendPort,
        listenPort:   listenPort,
        handler:      handler,
        heartbeatSec: 10,
    }
}

func (l *Listener) Run(ctx context.Context) error {
    listenAddr := &net.UDPAddr{Port: l.listenPort}
    conn, err := net.ListenUDP("udp", listenAddr)
    if err != nil {
        return fmt.Errorf("listen UDP: %w", err)
    }
    defer conn.Close()

    // Start heartbeat goroutine
    go l.heartbeatLoop(ctx, conn)

    buf := make([]byte, 4096)
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        conn.SetReadDeadline(time.Now().Add(time.Duration(l.heartbeatSec) * time.Second))
        n, _, err := conn.ReadFromUDP(buf)
        if err != nil {
            if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                continue // timeout, heartbeat will resend
            }
            continue
        }

        if n < PacketSize {
            continue
        }

        decrypted, err := DecryptPacket(buf[:n])
        if err != nil {
            continue
        }

        pkt, err := ParsePacket(decrypted)
        if err != nil {
            continue
        }
        pkt.Timestamp = time.Now().UnixMilli()

        l.handler(pkt)
    }
}

func (l *Listener) heartbeatLoop(ctx context.Context, conn *net.UDPConn) {
    psAddr := &net.UDPAddr{IP: net.ParseIP(l.psIP), Port: l.sendPort}
    ticker := time.NewTicker(time.Duration(l.heartbeatSec) * time.Second)
    defer ticker.Stop()

    // Send initial heartbeat
    conn.WriteToUDP([]byte("A"), psAddr)

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            conn.WriteToUDP([]byte("A"), psAddr)
        }
    }
}
```

**Step 6: Run tests, commit**

```bash
cd local-service && go test ./internal/telemetry/... -v
git add local-service/internal/telemetry/
git commit -m "feat: implement GT7 telemetry decryption and packet parsing"
```

---

## Task 3: Car Database Loader

**Files:**
- Create: `local-service/internal/cardb/cardb.go`
- Create: `local-service/internal/cardb/cardb_test.go`
- Create: `local-service/internal/cardb/testdata/test-stock-perf.csv`

**Step 1: Write tests**

```go
// cardb_test.go
package cardb

import (
    "os"
    "strings"
    "testing"
)

const testCSV = `carid,manufacturer,name,group,CH,CM,CS,SH,SM,SS,RH,RM,RS,IM,W,D
2108,Dodge,SRT Tomahawk X VGT,X,1195.32,1244.24,1277.29,1295.66,1320.72,1343.01,1361.67,1370.96,1377.64,1343.02,1320.71,0
575,BMW,M6 GT3 Endurance Model '16,3,587.22,601.45,612.55,620.87,631.15,640.23,647.92,652.67,656.32,640.22,631.14,0
1365,Toyota,GR Yaris RZ High Performance '20,N,398.51,410.23,419.87,427.54,436.89,445.12,451.78,455.92,459.01,445.11,436.88,0
`

func TestLoadFromReader(t *testing.T) {
    db, err := LoadFromReader(strings.NewReader(testCSV))
    if err != nil {
        t.Fatalf("LoadFromReader: %v", err)
    }
    if db.Count() != 3 {
        t.Errorf("Count = %d, want 3", db.Count())
    }
}

func TestLookup(t *testing.T) {
    db, _ := LoadFromReader(strings.NewReader(testCSV))

    car, ok := db.Lookup(575)
    if !ok {
        t.Fatal("expected to find car 575")
    }
    if car.Name != "M6 GT3 Endurance Model '16" {
        t.Errorf("Name = %q", car.Name)
    }
    if car.Manufacturer != "BMW" {
        t.Errorf("Manufacturer = %q", car.Manufacturer)
    }
    if car.Category != "Gr.3" {
        t.Errorf("Category = %q, want Gr.3", car.Category)
    }

    _, ok = db.Lookup(99999)
    if ok {
        t.Error("expected not to find car 99999")
    }
}

func TestCategoryNormalization(t *testing.T) {
    db, _ := LoadFromReader(strings.NewReader(testCSV))

    tests := []struct {
        id       int
        wantCat  string
    }{
        {2108, "Gr.X"},
        {575, "Gr.3"},
        {1365, "N"},
    }
    for _, tt := range tests {
        car, _ := db.Lookup(tt.id)
        if car.Category != tt.wantCat {
            t.Errorf("car %d: Category = %q, want %q", tt.id, car.Category, tt.wantCat)
        }
    }
}

func TestPPSubBand(t *testing.T) {
    tests := []struct {
        pp   float64
        want string
    }{
        {250.0, "N100-300"},
        {420.0, "N300-500"},
        {650.0, "N500-700"},
        {800.0, "N700+"},
    }
    for _, tt := range tests {
        got := PPSubBand(tt.pp)
        if got != tt.want {
            t.Errorf("PPSubBand(%f) = %q, want %q", tt.pp, got, tt.want)
        }
    }
}
```

**Step 2: Implement car database**

```go
// cardb.go
package cardb

import (
    "encoding/csv"
    "fmt"
    "io"
    "strconv"
    "strings"
)

type Car struct {
    ID           int
    Name         string
    Manufacturer string
    Category     string  // "Gr.1", "Gr.2", "Gr.3", "Gr.4", "Gr.B", "N", "Gr.X"
    PPStock      float64 // Comfort Hard PP
}

type Database struct {
    cars map[int]*Car
}

func LoadFromReader(r io.Reader) (*Database, error) {
    reader := csv.NewReader(r)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, fmt.Errorf("read csv: %w", err)
    }
    if len(records) < 2 {
        return nil, fmt.Errorf("csv has no data rows")
    }

    db := &Database{cars: make(map[int]*Car)}
    for _, row := range records[1:] { // skip header
        if len(row) < 5 {
            continue
        }
        id, err := strconv.Atoi(row[0])
        if err != nil {
            continue
        }
        pp, _ := strconv.ParseFloat(row[4], 64) // CH column
        cat := normalizeCategory(row[3])
        db.cars[id] = &Car{
            ID:           id,
            Manufacturer: row[1],
            Name:         row[2],
            Category:     cat,
            PPStock:      pp,
        }
    }
    return db, nil
}

func LoadFromFile(path string) (*Database, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    return LoadFromReader(f)
}

func (db *Database) Lookup(carID int) (*Car, bool) {
    car, ok := db.cars[carID]
    return car, ok
}

func (db *Database) Count() int {
    return len(db.cars)
}

func normalizeCategory(raw string) string {
    raw = strings.TrimSpace(raw)
    switch raw {
    case "1":
        return "Gr.1"
    case "2":
        return "Gr.2"
    case "3":
        return "Gr.3"
    case "4":
        return "Gr.4"
    case "B":
        return "Gr.B"
    case "X":
        return "Gr.X"
    case "N":
        return "N"
    default:
        return raw
    }
}

func PPSubBand(pp float64) string {
    switch {
    case pp < 300:
        return "N100-300"
    case pp < 500:
        return "N300-500"
    case pp < 700:
        return "N500-700"
    default:
        return "N700+"
    }
}
```

**Step 3: Run tests, commit**

```bash
cd local-service && go test ./internal/cardb/... -v
git add local-service/internal/cardb/
git commit -m "feat: implement car database loader with CSV parsing"
```

---

## Task 4: Track Reference Loader + Detection Algorithm

**Files:**
- Create: `local-service/internal/trackdetect/reference.go`
- Create: `local-service/internal/trackdetect/detector.go`
- Create: `local-service/internal/trackdetect/detector_test.go`
- Create: `local-service/internal/trackdetect/reference_test.go`

**Step 1: Write reference loader tests**

Test parsing of track name from filename (including `!WIDTH-` and `!PIT-` suffixes). Test decryption and extraction of XYZ points from a fake .gt7track file.

```go
// reference_test.go
package trackdetect

import "testing"

func TestParseTrackFilename(t *testing.T) {
    tests := []struct {
        filename string
        wantName string
        wantLayout string
        wantElimDist float64
    }{
        {"Tsukuba Circuit - Full Course.gt7track", "Tsukuba Circuit", "Full Course", 30.0},
        {"Watkins Glen - Short Course!PIT-18-92-35!WIDTH-64.gt7track", "Watkins Glen", "Short Course", 32.0},
        {"Daytona International Speedway - Tri-Oval!WIDTH-120.gt7track", "Daytona International Speedway", "Tri-Oval", 60.0},
        {"Special Stage Route X.gt7track", "Special Stage Route X", "", 30.0},
    }
    for _, tt := range tests {
        info := ParseTrackFilename(tt.filename)
        if info.Name != tt.wantName {
            t.Errorf("%s: Name = %q, want %q", tt.filename, info.Name, tt.wantName)
        }
        if info.Layout != tt.wantLayout {
            t.Errorf("%s: Layout = %q, want %q", tt.filename, info.Layout, tt.wantLayout)
        }
        if info.EliminateDistance != tt.wantElimDist {
            t.Errorf("%s: EliminateDistance = %f, want %f", tt.filename, info.EliminateDistance, tt.wantElimDist)
        }
    }
}
```

**Step 2: Implement reference loader**

Parse filenames for track name + layout + WIDTH overrides. Read .gt7track files (sequences of 296-byte encrypted packets), decrypt each, extract XYZ + velocity. Store as `TrackReference` with slice of `ReferencePoint`.

```go
// reference.go
package trackdetect

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "strconv"

    "github.com/rourkem/gt7leaderboard/local-service/internal/telemetry"
)

type ReferencePoint struct {
    Position telemetry.Vec3
    Velocity telemetry.Vec3
}

type TrackInfo struct {
    Name              string
    Layout            string
    EliminateDistance  float64
    Slug              string
}

type TrackReference struct {
    Info   TrackInfo
    Points []ReferencePoint
}

func ParseTrackFilename(filename string) TrackInfo {
    name := strings.TrimSuffix(filename, ".gt7track")
    info := TrackInfo{EliminateDistance: 30.0}

    // Parse !-delimited suffixes
    parts := strings.Split(name, "!")
    baseName := parts[0]
    for _, suffix := range parts[1:] {
        if strings.HasPrefix(suffix, "WIDTH-") {
            w, err := strconv.ParseFloat(strings.TrimPrefix(suffix, "WIDTH-"), 64)
            if err == nil {
                info.EliminateDistance = w / 2.0
            }
        }
    }

    // Split "Track Name - Layout"
    if idx := strings.Index(baseName, " - "); idx >= 0 {
        info.Name = baseName[:idx]
        info.Layout = baseName[idx+3:]
    } else {
        info.Name = baseName
        info.Layout = ""
    }

    info.Slug = slugify(info.Name, info.Layout)
    return info
}

func slugify(name, layout string) string {
    s := strings.ToLower(name)
    if layout != "" {
        s += "-" + strings.ToLower(layout)
    }
    s = strings.Map(func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
            return r
        }
        if r == ' ' || r == '-' {
            return '-'
        }
        return -1
    }, s)
    // Collapse multiple dashes
    for strings.Contains(s, "--") {
        s = strings.ReplaceAll(s, "--", "-")
    }
    return strings.Trim(s, "-")
}

func LoadTrackFile(path string) (*TrackReference, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    info := ParseTrackFilename(filepath.Base(path))
    ref := &TrackReference{Info: info}

    for i := 0; i+telemetry.PacketSize <= len(data); i += telemetry.PacketSize {
        chunk := data[i : i+telemetry.PacketSize]
        decrypted, err := telemetry.DecryptPacket(chunk)
        if err != nil {
            continue
        }
        pkt, err := telemetry.ParsePacket(decrypted)
        if err != nil {
            continue
        }
        ref.Points = append(ref.Points, ReferencePoint{
            Position: pkt.Position,
            Velocity: pkt.Velocity,
        })
    }

    if len(ref.Points) == 0 {
        return nil, fmt.Errorf("no valid points in track file %s", path)
    }
    return ref, nil
}

func LoadAllTracks(dir string) ([]*TrackReference, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, err
    }
    var tracks []*TrackReference
    for _, e := range entries {
        if !strings.HasSuffix(e.Name(), ".gt7track") {
            continue
        }
        ref, err := LoadTrackFile(filepath.Join(dir, e.Name()))
        if err != nil {
            continue // skip bad files
        }
        tracks = append(tracks, ref)
    }
    return tracks, nil
}
```

**Step 3: Write detector tests**

```go
// detector_test.go
package trackdetect

import (
    "testing"

    "github.com/rourkem/gt7leaderboard/local-service/internal/telemetry"
)

func makeTrackRef(name, layout string, points []telemetry.Vec3) *TrackReference {
    ref := &TrackReference{
        Info: TrackInfo{Name: name, Layout: layout, EliminateDistance: 30.0, Slug: slugify(name, layout)},
    }
    for _, p := range points {
        ref.Points = append(ref.Points, ReferencePoint{
            Position: p,
            Velocity: telemetry.Vec3{X: 1, Y: 0, Z: 0}, // forward
        })
    }
    return ref
}

func TestDetector_SingleTrack(t *testing.T) {
    // Create a simple circular track reference
    trackA := makeTrackRef("Test Track", "Full", []telemetry.Vec3{
        {X: 0, Y: 0, Z: 0},
        {X: 50, Y: 0, Z: 0},
        {X: 100, Y: 0, Z: 0},
        {X: 100, Y: 0, Z: 50},
        {X: 100, Y: 0, Z: 100},
        {X: 50, Y: 0, Z: 100},
        {X: 0, Y: 0, Z: 100},
        {X: 0, Y: 0, Z: 50},
    })

    d := NewDetector([]*TrackReference{trackA}, DetectorConfig{
        MinPointsBeforeDetection: 5,
        MinHitsForTrack:         3,
        PostDetectionPoints:     2,
    })

    // Feed points along the track
    points := []telemetry.Vec3{
        {X: 1, Y: 0, Z: 1},
        {X: 48, Y: 0, Z: 2},
        {X: 99, Y: 0, Z: 1},
        {X: 101, Y: 0, Z: 49},
        {X: 99, Y: 0, Z: 99},
        {X: 51, Y: 0, Z: 101},
        {X: 2, Y: 0, Z: 99},
        {X: 1, Y: 0, Z: 51},
    }

    var result *DetectionResult
    for _, p := range points {
        pkt := &telemetry.Packet{
            Position: p,
            Velocity: telemetry.Vec3{X: 1, Y: 0, Z: 0},
            CarSpeed: 30.0,
        }
        result = d.AddPoint(pkt)
        if result != nil {
            break
        }
    }

    if result == nil {
        t.Fatal("expected detection result")
    }
    if result.Track.Info.Name != "Test Track" {
        t.Errorf("detected %q, want Test Track", result.Track.Info.Name)
    }
}

func TestDetector_EliminatesFarTrack(t *testing.T) {
    trackA := makeTrackRef("Track A", "", []telemetry.Vec3{
        {X: 0, Y: 0, Z: 0}, {X: 50, Y: 0, Z: 0}, {X: 100, Y: 0, Z: 0},
    })
    trackB := makeTrackRef("Track B", "", []telemetry.Vec3{
        {X: 5000, Y: 0, Z: 5000}, {X: 5050, Y: 0, Z: 5000}, {X: 5100, Y: 0, Z: 5000},
    })

    d := NewDetector([]*TrackReference{trackA, trackB}, DetectorConfig{
        MinPointsBeforeDetection: 2,
        MinHitsForTrack:         2,
        PostDetectionPoints:     1,
    })

    // Feed points near Track A - should eliminate Track B
    points := []telemetry.Vec3{
        {X: 2, Y: 0, Z: 1},
        {X: 51, Y: 0, Z: 0},
        {X: 99, Y: 0, Z: 1},
    }

    var result *DetectionResult
    for _, p := range points {
        pkt := &telemetry.Packet{Position: p, Velocity: telemetry.Vec3{X: 1}, CarSpeed: 30}
        result = d.AddPoint(pkt)
        if result != nil {
            break
        }
    }

    if result == nil {
        t.Fatal("expected detection")
    }
    if result.Track.Info.Name != "Track A" {
        t.Errorf("detected %q, want Track A", result.Track.Info.Name)
    }
}

func TestDetector_IgnoresStationaryPoints(t *testing.T) {
    track := makeTrackRef("T", "", []telemetry.Vec3{{X: 0, Y: 0, Z: 0}})
    d := NewDetector([]*TrackReference{track}, DefaultConfig())

    pkt := &telemetry.Packet{Position: telemetry.Vec3{X: 0}, CarSpeed: 0} // stationary
    result := d.AddPoint(pkt)
    if result != nil {
        t.Error("should not detect with stationary point")
    }
    if d.pointCount != 0 {
        t.Errorf("pointCount = %d, want 0 (stationary ignored)", d.pointCount)
    }
}
```

**Step 4: Implement detector**

```go
// detector.go
package trackdetect

import (
    "math"

    "github.com/rourkem/gt7leaderboard/local-service/internal/telemetry"
)

type DetectorConfig struct {
    MinPointsBeforeDetection int
    MinHitsForTrack          int
    PostDetectionPoints      int
    MaxGapLength             int
    ValidAngleDeg            float64
}

func DefaultConfig() DetectorConfig {
    return DetectorConfig{
        MinPointsBeforeDetection: 300,
        MinHitsForTrack:          5,
        PostDetectionPoints:      60,
        MaxGapLength:             10,
        ValidAngleDeg:            15.0,
    }
}

type candidate struct {
    track           *TrackReference
    hits            []bool
    forwardHits     int
    reverseHits     int
    eliminated      bool
}

type DetectionResult struct {
    Track     *TrackReference
    IsReverse bool
}

type Detector struct {
    candidates           []*candidate
    config               DetectorConfig
    pointCount           int
    detectedTrack        *candidate
    postDetectionCounter int
}

func NewDetector(tracks []*TrackReference, config DetectorConfig) *Detector {
    candidates := make([]*candidate, len(tracks))
    for i, t := range tracks {
        candidates[i] = &candidate{
            track: t,
            hits:  make([]bool, len(t.Points)),
        }
    }
    return &Detector{candidates: candidates, config: config}
}

func (d *Detector) AddPoint(pkt *telemetry.Packet) *DetectionResult {
    if pkt.CarSpeed <= 0 {
        return nil
    }
    d.pointCount++

    if d.pointCount < d.config.MinPointsBeforeDetection {
        return nil
    }

    // If already detected, count post-detection points
    if d.detectedTrack != nil {
        d.postDetectionCounter++
        if d.postDetectionCounter >= d.config.PostDetectionPoints {
            isReverse := d.detectedTrack.reverseHits > d.detectedTrack.forwardHits
            return &DetectionResult{Track: d.detectedTrack.track, IsReverse: isReverse}
        }
        return nil
    }

    remaining := 0
    var lastAlive *candidate

    for _, c := range d.candidates {
        if c.eliminated {
            continue
        }

        closestIdx, closestDist := findClosestPoint(c.track.Points, pkt.Position)

        if closestDist > c.track.Info.EliminateDistance {
            c.eliminated = true
            continue
        }

        // Mark hit and check angle
        c.hits[closestIdx] = true
        angle := angleBetween(pkt.Velocity, c.track.Points[closestIdx].Velocity)
        if angle < d.config.ValidAngleDeg {
            c.forwardHits++
        } else if angle > (180 - d.config.ValidAngleDeg) {
            c.reverseHits++
        }

        // Check for gaps
        if hasLargeGap(c.hits, d.config.MaxGapLength) {
            c.eliminated = true
            continue
        }

        remaining++
        lastAlive = c
    }

    if remaining == 1 && lastAlive != nil {
        totalHits := lastAlive.forwardHits + lastAlive.reverseHits
        if totalHits >= d.config.MinHitsForTrack {
            d.detectedTrack = lastAlive
            d.postDetectionCounter = 0
        }
    }

    return nil
}

func (d *Detector) Reset() {
    for _, c := range d.candidates {
        c.eliminated = false
        c.forwardHits = 0
        c.reverseHits = 0
        c.hits = make([]bool, len(c.track.Points))
    }
    d.pointCount = 0
    d.detectedTrack = nil
    d.postDetectionCounter = 0
}

func findClosestPoint(points []ReferencePoint, pos telemetry.Vec3) (int, float64) {
    minDist := math.MaxFloat64
    minIdx := 0
    for i, p := range points {
        d := distance3D(pos, p.Position)
        if d < minDist {
            minDist = d
            minIdx = i
        }
    }
    return minIdx, minDist
}

func distance3D(a, b telemetry.Vec3) float64 {
    dx := float64(a.X - b.X)
    dy := float64(a.Y - b.Y)
    dz := float64(a.Z - b.Z)
    return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func angleBetween(a, b telemetry.Vec3) float64 {
    dot := float64(a.X*b.X + a.Y*b.Y + a.Z*b.Z)
    magA := math.Sqrt(float64(a.X*a.X + a.Y*a.Y + a.Z*a.Z))
    magB := math.Sqrt(float64(b.X*b.X + b.Y*b.Y + b.Z*b.Z))
    if magA == 0 || magB == 0 {
        return 90.0
    }
    cos := dot / (magA * magB)
    cos = math.Max(-1, math.Min(1, cos))
    return math.Acos(cos) * 180.0 / math.Pi
}

func hasLargeGap(hits []bool, maxGap int) bool {
    firstHit := -1
    lastHit := -1
    for i, h := range hits {
        if h {
            if firstHit == -1 {
                firstHit = i
            }
            lastHit = i
        }
    }
    if firstHit == -1 || firstHit == lastHit {
        return false
    }
    gap := 0
    for i := firstHit; i <= lastHit; i++ {
        if !hits[i] {
            gap++
            if gap > maxGap {
                return true
            }
        } else {
            gap = 0
        }
    }
    return false
}
```

**Step 5: Run tests, commit**

```bash
cd local-service && go test ./internal/trackdetect/... -v
git commit -am "feat: implement track detection algorithm with reference loader"
```

---

## Task 5: PSN API Client

**Files:**
- Create: `local-service/internal/psn/auth.go`
- Create: `local-service/internal/psn/auth_test.go`
- Create: `local-service/internal/psn/presence.go`
- Create: `local-service/internal/psn/presence_test.go`
- Create: `local-service/internal/psn/types.go`

**Step 1: Write types**

```go
// types.go
package psn

import "time"

const (
    AuthorizeURL = "https://ca.account.sony.com/api/authz/v3/oauth/authorize"
    TokenURL     = "https://ca.account.sony.com/api/authz/v3/oauth/token"
    PresenceURL  = "https://m.np.playstation.com/api/userProfile/v2/internal/users//basicPresences"
    ProfileURL   = "https://us-prof.np.community.playstation.net/userProfile/v1/users/%s/profile2"

    ClientID     = "09515159-7237-4370-9b40-3806e67c0891"
    ClientSecret = "ucPjka5tntB2KqsP"
    RedirectURI  = "com.scee.psxandroid.scecompcall://redirect"
    Scopes       = "psn:mobile.v2.core psn:clientapp"
    BasicAuth    = "MDk1MTUxNTktNzIzNy00MzcwLTliNDAtMzgwNmU2N2MwODkxOnVjUGprYTV0bnRCMktxc1A="

    GT7TitlePS5  = "PPSA01316_00"
    GT7TitlePS4  = "CUSA24767_00"
)

type Tokens struct {
    AccessToken          string    `json:"access_token"`
    RefreshToken         string    `json:"refresh_token"`
    AccessTokenExpiresAt time.Time `json:"access_token_expires_at"`
    RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
    NpssoSetAt           time.Time `json:"npsso_set_at"`
}

type PresenceResponse struct {
    BasicPresences []BasicPresence `json:"basicPresences"`
}

type SinglePresenceResponse struct {
    BasicPresence BasicPresence `json:"basicPresence"`
}

type BasicPresence struct {
    AccountID          string            `json:"accountId"`
    Availability       string            `json:"availability"`
    LastAvailableDate  string            `json:"lastAvailableDate,omitempty"`
    PrimaryPlatformInfo *PlatformInfo    `json:"primaryPlatformInfo,omitempty"`
    GameTitleInfoList  []GameTitleInfo   `json:"gameTitleInfoList,omitempty"`
}

type PlatformInfo struct {
    OnlineStatus string `json:"onlineStatus"`
    Platform     string `json:"platform"`
}

type GameTitleInfo struct {
    NpTitleID    string `json:"npTitleId"`
    TitleName    string `json:"titleName"`
    Format       string `json:"format"`
}

func (bp *BasicPresence) IsPlayingGT7() bool {
    for _, g := range bp.GameTitleInfoList {
        if g.NpTitleID == GT7TitlePS5 || g.NpTitleID == GT7TitlePS4 {
            return true
        }
        // Fallback: check title name
        if containsIgnoreCase(g.TitleName, "gran turismo 7") {
            return true
        }
    }
    return false
}
```

**Step 2: Implement auth flow with tests using httptest**

The auth tests use `httptest.NewServer` to mock Sony's endpoints. Test the NPSSO → auth code → tokens flow, and the refresh flow.

**Step 3: Implement presence detection with tests**

Test bulk presence endpoint parsing, GT7 detection logic, and the `IdentifyDriver` function that takes a list of configured accounts and returns which one is playing GT7.

**Step 4: Implement token lifecycle checker**

```go
func (t *Tokens) DaysUntilRefreshExpiry() int {
    return int(time.Until(t.RefreshTokenExpiresAt).Hours() / 24)
}

func (t *Tokens) NeedsReminder() (bool, string) {
    days := t.DaysUntilRefreshExpiry()
    switch {
    case days <= 0:
        return true, "PSN token has EXPIRED. Sessions will record as Unknown Driver."
    case days <= 1:
        return true, "PSN token expires TOMORROW."
    case days <= 3:
        return true, fmt.Sprintf("PSN token expires in %d days. Please renew soon.", days)
    case days <= 7:
        return true, fmt.Sprintf("PSN token expires in %d days.", days)
    case days <= 14:
        return true, fmt.Sprintf("PSN token expires in %d days.", days)
    default:
        return false, ""
    }
}
```

**Step 5: Run tests, commit**

```bash
cd local-service && go test ./internal/psn/... -v
git commit -am "feat: implement PSN API client with auth and presence detection"
```

---

## Task 6: Viper Config Management

**Files:**
- Create: `local-service/internal/config/config.go`
- Create: `local-service/internal/config/config_test.go`

**Step 1: Define config struct and implement Viper loading**

```go
// config.go
package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    PlayStation  PlayStationConfig  `mapstructure:"playstation"`
    PSN          PSNConfig          `mapstructure:"psn"`
    API          APIConfig          `mapstructure:"api"`
    Discord      DiscordConfig      `mapstructure:"discord"`
    Datadog      DatadogConfig      `mapstructure:"datadog"`
    Session      SessionConfig      `mapstructure:"session"`
    DataRefresh  DataRefreshConfig  `mapstructure:"data_refresh"`
}

type PlayStationConfig struct {
    IP             string `mapstructure:"ip"`
    SendPort       int    `mapstructure:"telemetry_send_port"`
    ListenPort     int    `mapstructure:"telemetry_listen_port"`
}

type PSNConfig struct {
    NpssoToken string       `mapstructure:"npsso_token"`
    Accounts   []PSNAccount `mapstructure:"accounts"`
}

type PSNAccount struct {
    OnlineID   string `mapstructure:"online_id"`
    DriverName string `mapstructure:"driver_name"`
}

type APIConfig struct {
    Endpoint string `mapstructure:"endpoint"`
    APIKey   string `mapstructure:"api_key"`
}

type DiscordConfig struct {
    WebhookURL           string `mapstructure:"webhook_url"`
    NotifyOverallRecords bool   `mapstructure:"notify_overall_records"`
    NotifyCategoryRecords bool  `mapstructure:"notify_category_records"`
    NotifyCarRecords     bool   `mapstructure:"notify_car_records"`
}

type DatadogConfig struct {
    Enabled bool   `mapstructure:"enabled"`
    APIKey  string `mapstructure:"api_key"`
    Site    string `mapstructure:"site"`
    Service string `mapstructure:"service"`
    Env     string `mapstructure:"env"`
}

type SessionConfig struct {
    IdleTimeoutSeconds       int `mapstructure:"idle_timeout_seconds"`
    TrackDetectionMinPoints  int `mapstructure:"track_detection_min_points"`
}

type DataRefreshConfig struct {
    CarDataURL               string `mapstructure:"car_data_url"`
    CarRefreshIntervalHours  int    `mapstructure:"car_refresh_interval_hours"`
    TrackDataRepo            string `mapstructure:"track_data_repo"`
    TrackRefreshIntervalHours int   `mapstructure:"track_refresh_interval_hours"`
}

func Load(path string) (*Config, error) {
    viper.SetConfigFile(path)
    viper.SetDefault("playstation.telemetry_send_port", 33739)
    viper.SetDefault("playstation.telemetry_listen_port", 33740)
    viper.SetDefault("session.idle_timeout_seconds", 30)
    viper.SetDefault("session.track_detection_min_points", 300)
    viper.SetDefault("data_refresh.car_data_url", "https://ddm999.github.io/gt7info/data-stock-perf.csv")
    viper.SetDefault("data_refresh.car_refresh_interval_hours", 24)
    viper.SetDefault("data_refresh.track_data_repo", "profittlich/gt7speedboard")
    viper.SetDefault("data_refresh.track_refresh_interval_hours", 168)
    viper.SetDefault("discord.notify_overall_records", true)
    viper.SetDefault("discord.notify_category_records", true)
    viper.SetDefault("discord.notify_car_records", false)

    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

**Step 2: Test with YAML fixtures, verify defaults, test hot-reload of NPSSO token**

**Step 3: Run tests, commit**

```bash
cd local-service && go test ./internal/config/... -v
git commit -am "feat: implement Viper config management"
```

---

## Task 7: Discord Webhook Client

**Files:**
- Create: `local-service/internal/discord/webhook.go`
- Create: `local-service/internal/discord/webhook_test.go`

Implement `SendRecordNotification(webhookURL string, record RecordNotification) error` that formats the embed JSON and POSTs to Discord. Test with httptest mock. Include `FormatLapTime(ms int) string` helper (e.g., 102387 → "1:42.387").

**Commit:** `feat: implement Discord webhook client`

---

## Task 8: API Push Client

**Files:**
- Create: `local-service/internal/api/client.go`
- Create: `local-service/internal/api/client_test.go`

HTTP client for pushing to the hosted API. Methods:
- `CreateSession(req CreateSessionRequest) (*CreateSessionResponse, error)`
- `RecordLap(req RecordLapRequest) (*RecordLapResponse, error)`
- `EndSession(sessionID string, endedAt time.Time) error`
- `SendHeartbeat(status HeartbeatRequest) error`
- `SyncCars(cars []CarSync) error`
- `SyncTrack(track TrackSync) error`

All methods set `Authorization: Bearer <apiKey>`. Test with httptest.

**Commit:** `feat: implement API push client`

---

## Task 9: Datadog Metrics

**Files:**
- Create: `local-service/internal/metrics/metrics.go`
- Create: `local-service/internal/metrics/metrics_test.go`

Thin wrapper around `github.com/DataDog/datadog-go/v5/statsd`. Interface-based so we can use a no-op implementation when Datadog is disabled.

```go
type Metrics interface {
    Incr(name string, tags []string)
    Gauge(name string, value float64, tags []string)
    Histogram(name string, value float64, tags []string)
    Close()
}
```

**Commit:** `feat: implement Datadog metrics wrapper`

---

## Task 10: Drizzle Schema + Migrations

**Files:**
- Create: `web/src/lib/db/schema.ts`
- Create: `web/src/lib/db/index.ts`
- Create: `web/src/lib/db/queries.ts`
- Create: `web/drizzle.config.ts`

**Step 1: Define schema**

```typescript
// schema.ts
import { pgTable, uuid, text, integer, boolean, real, timestamp, index } from 'drizzle-orm/pg-core';

export const drivers = pgTable('drivers', {
  id: uuid('id').defaultRandom().primaryKey(),
  psnAccountId: text('psn_account_id'),
  psnOnlineId: text('psn_online_id'),
  displayName: text('display_name').notNull(),
  isGuest: boolean('is_guest').default(false).notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow().notNull(),
});

export const tracks = pgTable('tracks', {
  id: uuid('id').defaultRandom().primaryKey(),
  name: text('name').notNull(),
  layout: text('layout').notNull().default(''),
  slug: text('slug').unique().notNull(),
  country: text('country'),
  lengthMeters: integer('length_meters'),
  numCorners: integer('num_corners'),
  hasWeather: boolean('has_weather').default(false).notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
});

export const cars = pgTable('cars', {
  id: integer('id').primaryKey(), // GT7 internal car ID
  name: text('name').notNull(),
  manufacturer: text('manufacturer').notNull(),
  category: text('category').notNull(),
  ppStock: real('pp_stock'),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow().notNull(),
});

export const sessions = pgTable('sessions', {
  id: uuid('id').defaultRandom().primaryKey(),
  driverId: uuid('driver_id').references(() => drivers.id),
  trackId: uuid('track_id').references(() => tracks.id),
  carId: integer('car_id').references(() => cars.id),
  startedAt: timestamp('started_at', { withTimezone: true }).notNull(),
  endedAt: timestamp('ended_at', { withTimezone: true }),
  detectionMethod: text('detection_method').notNull().default('unmatched'),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow().notNull(),
});

export const lapRecords = pgTable('lap_records', {
  id: uuid('id').defaultRandom().primaryKey(),
  driverId: uuid('driver_id').references(() => drivers.id),
  trackId: uuid('track_id').references(() => tracks.id),
  carId: integer('car_id').references(() => cars.id),
  sessionId: uuid('session_id').references(() => sessions.id),
  lapTimeMs: integer('lap_time_ms').notNull(),
  lapNumber: integer('lap_number').notNull(),
  autoDetectedDriverId: uuid('auto_detected_driver_id').references(() => drivers.id),
  weather: text('weather').notNull().default('unknown'),
  isValid: boolean('is_valid').default(true).notNull(),
  recordedAt: timestamp('recorded_at', { withTimezone: true }).notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow().notNull(),
}, (table) => [
  index('idx_lap_records_leaderboard').on(table.trackId, table.carId, table.driverId, table.lapTimeMs),
  index('idx_lap_records_driver').on(table.driverId, table.recordedAt),
  index('idx_lap_records_session').on(table.sessionId, table.lapNumber),
]);

export const collectorHeartbeats = pgTable('collector_heartbeats', {
  id: uuid('id').defaultRandom().primaryKey(),
  status: text('status').notNull(),
  currentSessionId: uuid('current_session_id'),
  uptimeSeconds: integer('uptime_seconds'),
  receivedAt: timestamp('received_at', { withTimezone: true }).defaultNow().notNull(),
});
```

**Step 2: Set up DB connection and Drizzle config**

**Step 3: Push schema to Neon**

```bash
cd web && npx drizzle-kit push
```

**Step 4: Implement key queries in queries.ts**

Leaderboard query (best lap per driver for a track, optionally filtered by category/car):
```typescript
export async function getTrackLeaderboard(trackId: string, opts?: { category?: string; carId?: number }) {
  // Use window function: ROW_NUMBER() OVER (PARTITION BY driver_id ORDER BY lap_time_ms ASC)
  // Return rank, driver, lap_time_ms, car, delta_to_leader, achieved_at
}
```

**Step 5: Commit**

```bash
git commit -am "feat: implement Drizzle schema and DB queries"
```

---

## Task 11: NextAuth.js Setup

**Files:**
- Create: `web/src/lib/auth/index.ts`
- Create: `web/src/app/api/auth/[...nextauth]/route.ts`
- Create: `web/src/middleware.ts`

Configure NextAuth.js with Google provider. Restrict to @mcnamara.io domain in the signIn callback. Protect all routes except `/api/ingest/*` (which uses API key auth).

**Commit:** `feat: implement Google OAuth with domain restriction`

---

## Task 12: Ingest API Endpoints

**Files:**
- Create: `web/src/app/api/ingest/sessions/route.ts`
- Create: `web/src/app/api/ingest/laps/route.ts`
- Create: `web/src/app/api/ingest/sessions/[id]/end/route.ts`
- Create: `web/src/app/api/ingest/heartbeat/route.ts`
- Create: `web/src/app/api/ingest/cars/route.ts`
- Create: `web/src/app/api/ingest/tracks/route.ts`
- Create: `web/src/lib/ingest-auth.ts`

All ingest endpoints validate `Authorization: Bearer <INGEST_API_KEY>`.

The `POST /api/ingest/laps` endpoint is the most critical — after inserting the lap, it checks for new records (overall, category, car-specific) and returns them in the response.

Record detection SQL:
```sql
-- Check if this is a new overall track record
SELECT MIN(lap_time_ms) as best FROM lap_records
WHERE track_id = $1 AND is_valid = true AND id != $2;

-- Check category record (join with cars table)
SELECT MIN(lr.lap_time_ms) as best FROM lap_records lr
JOIN cars c ON lr.car_id = c.id
WHERE lr.track_id = $1 AND lr.is_valid = true AND lr.id != $2 AND c.category = $3;

-- Check car-specific record
SELECT MIN(lap_time_ms) as best FROM lap_records
WHERE track_id = $1 AND car_id = $2 AND is_valid = true AND id != $3;
```

**Commit:** `feat: implement ingest API endpoints with record detection`

---

## Task 13: Leaderboard + Management API Endpoints

**Files:**
- Create: `web/src/app/api/tracks/route.ts`
- Create: `web/src/app/api/tracks/[slug]/leaderboard/route.ts`
- Create: `web/src/app/api/drivers/route.ts`
- Create: `web/src/app/api/drivers/[id]/route.ts`
- Create: `web/src/app/api/sessions/route.ts`
- Create: `web/src/app/api/sessions/[id]/route.ts`
- Create: `web/src/app/api/sessions/[id]/reassign/route.ts`
- Create: `web/src/app/api/laps/[id]/route.ts`
- Create: `web/src/app/api/status/route.ts`

All protected by NextAuth session. Implement the leaderboard query with window functions for ranking + delta calculation. The management endpoints (PATCH laps, PATCH sessions/reassign, POST drivers) handle driver reassignment and weather tagging.

**Commit:** `feat: implement leaderboard and management API endpoints`

---

## Task 14: Session Manager + Lap Recorder

**Files:**
- Create: `local-service/internal/session/manager.go`
- Create: `local-service/internal/session/manager_test.go`

This is the core orchestration logic for the local service. The SessionManager:
1. Receives telemetry packets from the listener
2. Detects session boundaries (idle gap >= 30s)
3. On session start: triggers PSN presence check, starts track detection, reads car ID
4. On lap completion (current_lap increments): validates and records the lap
5. On car change: ends current session, starts new one
6. On session end: closes session via API
7. Pushes laps to hosted API immediately

```go
type Manager struct {
    apiClient    *api.Client
    psnClient    *psn.Client
    carDB        *cardb.Database
    detector     *trackdetect.Detector
    discord      *discord.Client
    metrics      metrics.Metrics
    config       *config.Config

    currentSession *ActiveSession
    lastPacketTime time.Time
    idleTimeout    time.Duration
    mu             sync.Mutex
}

type ActiveSession struct {
    ID              string
    DriverID        string
    TrackSlug       string
    CarID           int32
    LastLap         int16
    StartedAt       time.Time
    DetectionMethod string
}
```

Test with mocked dependencies (interfaces for API client, PSN client, etc.).

**Commit:** `feat: implement session manager with lap recording`

---

## Task 15: Data Auto-Refresh Pipeline

**Files:**
- Create: `local-service/internal/refresh/refresh.go`
- Create: `local-service/internal/refresh/refresh_test.go`

Implements periodic fetching of car data CSV and track reference files from GitHub.

- `RefreshCars()`: Fetch `data-stock-perf.csv` from ddm999/gt7info, parse, update local cardb, push to API
- `RefreshTracks()`: Use GitHub API to list .gt7track files, download new/updated ones, save to local cache
- `StartScheduler()`: Run car refresh every 24h, track refresh every 168h, both on startup

**Commit:** `feat: implement auto-refresh pipeline for car and track data`

---

## Task 16: Local Web UI

**Files:**
- Create: `local-service/internal/webui/server.go`
- Create: `local-service/internal/webui/server_test.go`
- Create: `local-service/internal/webui/templates/status.html`
- Create: `local-service/internal/webui/templates/auth.html`

Simple Go HTTP server on port 8080 serving:
- `GET /` — Status page (current session, PSN token status, uptime, last data refresh)
- `GET /auth` — Form to paste new NPSSO token
- `POST /auth` — Accept NPSSO token, trigger auth flow, update Viper config
- `GET /health` — JSON health check for monitoring

Use `html/template` — no frontend framework needed for this simple UI.

**Commit:** `feat: implement local status and auth web UI`

---

## Task 17: Main Collector Binary

**Files:**
- Modify: `local-service/cmd/collector/main.go`

Wire everything together:
1. Load config via Viper
2. Initialize Datadog metrics
3. Load car database (from cache or bundled)
4. Load track references (from cache or bundled)
5. Initialize PSN client
6. Initialize API client
7. Initialize Discord client
8. Create session manager
9. Start telemetry listener
10. Start data refresh scheduler
11. Start local web UI
12. Handle graceful shutdown (SIGINT/SIGTERM)

**Commit:** `feat: wire up main collector binary`

---

## Task 18: UI Components + Dashboard

**Files:**
- Create: `web/src/components/lap-time.tsx` — Format milliseconds to "M:SS.mmm"
- Create: `web/src/components/leaderboard-table.tsx` — Reusable ranked table
- Create: `web/src/components/driver-badge.tsx` — Driver name with color
- Create: `web/src/components/category-tabs.tsx` — Tab switcher for categories
- Create: `web/src/components/activity-feed.tsx` — Recent laps list
- Create: `web/src/components/stat-card.tsx` — Quick stat display
- Create: `web/src/components/collector-status.tsx` — Online/offline indicator
- Create: `web/src/components/nav.tsx` — Navigation bar
- Modify: `web/src/app/layout.tsx` — Add nav, Tailwind setup
- Modify: `web/src/app/page.tsx` — Dashboard with activity feed, stats, collector status

Use server components by default. Client components only where interactivity needed (tabs, filters).

**Commit:** `feat: implement dashboard and shared UI components`

---

## Task 19: Track List + Leaderboard Pages

**Files:**
- Create: `web/src/app/tracks/page.tsx`
- Create: `web/src/app/tracks/[slug]/page.tsx`
- Create: `web/src/components/track-card.tsx`

Track list: grid of cards showing track name, lap count, record holder + time.
Track leaderboard: tabbed view (Overall, Gr.1-4, Gr.B, N sub-bands, Gr.X) with ranked table. Car filter dropdown for head-to-head.

**Commit:** `feat: implement track list and leaderboard pages`

---

## Task 20: Driver Profile + Session Management Pages

**Files:**
- Create: `web/src/app/drivers/page.tsx`
- Create: `web/src/app/drivers/[id]/page.tsx`
- Create: `web/src/app/sessions/page.tsx`
- Create: `web/src/app/sessions/[id]/page.tsx`
- Create: `web/src/app/settings/page.tsx`
- Create: `web/src/components/session-card.tsx`
- Create: `web/src/components/reassign-dialog.tsx`
- Create: `web/src/components/weather-tag.tsx`

Driver profile: records across tracks, recent laps, stats.
Session management: list, detail view, reassign driver, tag weather.
Settings: collector status, PSN token status, driver management.

**Commit:** `feat: implement driver profiles, session management, and settings`

---

## Task 21: Deployment Config

**Files:**
- Create: `local-service/Dockerfile`
- Create: `local-service/docker-compose.yml`
- Create: `local-service/config.example.yaml`
- Create: `web/vercel.json`
- Create: `web/.env.example`

**Dockerfile** (multi-stage, multi-arch):
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o collector ./cmd/collector

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/collector .
COPY data/ ./data/
EXPOSE 33740/udp 8080
ENTRYPOINT ["./collector"]
CMD ["--config", "/app/config.yaml"]
```

**docker-compose.yml** with port mappings, volume mounts, restart policy.

**vercel.json** with rewrites if needed.

**Commit:** `feat: add Docker and Vercel deployment configuration`

---

## Testing Strategy

### Go (local-service)
- **Unit tests**: Every internal package has `_test.go` files with table-driven tests
- **Integration test helpers**: `makeTestPacketBytes()`, `encryptPacketForTest()` for telemetry
- **Mock interfaces**: API client, PSN client, metrics all have interfaces for testing
- **httptest**: PSN API and Discord webhook tests use `httptest.NewServer`
- **Run**: `cd local-service && go test ./... -v -race`

### TypeScript (web)
- **API route tests**: Test ingest endpoints with mock DB, verify record detection logic
- **Component tests**: Verify lap time formatting, leaderboard ranking
- **Run**: `cd web && npm test`

### End-to-end (manual)
- Start collector with test config pointing at a mock PS5 (UDP packet replay)
- Verify laps appear in web UI
- Verify Discord notifications fire for records
