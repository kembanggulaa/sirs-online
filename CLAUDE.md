# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Install dependencies (required after fresh clone)
go mod tidy

# Build
go build -o sirs-online.exe .

# Run (development)
go run main.go

# Run all tests (note: -race flag cannot be used on Windows due to CGO_ENABLED=0)
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run single test file
go test ./internal/worker -v -run TestWorkerProcess

# Run integration tests (requires TEST_DATABASE_DSN env var)
TEST_DATABASE_DSN="server=...;port=...;user id=...;password=...;database=..." go test ./internal/repository/... -v -run Integration
```

## Architecture Overview

### Sync Flow (Worker Pool)
The application bridges SIMRS (SQL Server) to the Kemenkes RS Online API via a scheduled worker:

```
Ticker (SYNC_INTERVAL_HOURS) or Manual Trigger
        │
        ▼
Dispatcher.dispatch() → Job channel
        │
        ▼
Worker.processJob()
 ├── GetActiveSKNo()          → Query sk_bed WHERE tgl_berakhir IS NULL
 ├── GetBedAvailability()     → Temp table #temp_ranap + main query in ONE tx
 ├── GET /Fasyankes           → Fetch id_t_tt mapping from Kemenkes
 └── PUT /Fasyankes/{id_tt}   → Per room, with retry (max RETRY_MAX+1 attempts)
```

### Critical DB Rule
Both temp table (#temp_ranap) and main bed availability query **must run on the same `*sql.Tx`** (single connection). The temp table is session-bound in SQL Server.

**Tables a clone must adapt:** The `GetBedAvailability()` query at `bed_repository.go:68` has hard SIMRS schema dependencies:

**CTE `TempRanap`** (lines 71-85) — determines occupied beds from `pasien_visitation`:
```sql
WITH TempRanap AS (
    SELECT CONCAT(b.class_room_id, b.kamar) AS kamar
    FROM pasien_visitation pv WITH (NOLOCK)
    LEFT JOIN beds b WITH (NOLOCK) ON b.class_room_id = pv.CLASS_ROOM_ID AND b.bed_id = pv.bed_id
    WHERE pv.no_registration <> ''
      AND pv.class_room_id IS NOT NULL
      AND (pv.keluar_id = 0 OR pv.keluar_id = 33)  -- not discharged
      AND pv.class_room_id IN (SELECT DISTINCT class_room_id FROM sk_bed WHERE sk_no = ? AND tgl_berakhir IS NULL AND class_room_id <> 'NI.BX')
)
```
**To adapt:** Replace `pasien_visitation` with your own patient occupancy table. `keluar_id = 0 OR 33` means "still admitted" — your hospital may use different values or a separate "bed_status" column. If you don't have a temp table concept, you can inline the occupancy subquery directly in the main SELECT.

**Main query** (lines 86-114) — joins `sk_bed` + `status_covid` + `TempRanap`:
- `sk_bed`: SK definitions (`class_room_id`, `kamar`, `id_tt_siranap`, `bed`, `ruang_siranap`, `jml_ruang_siranap`, `kelas_siranap`, `covid`, `sk_no`, `tgl_berlaku`, `tgl_berakhir`)
- `status_covid`: per-TT COVID status (`id_tt`, `status`, `konfirmasi`, `antrian`)
- `beds`: Bed master (`class_room_id`, `kamar`, `bed_id`)

If your hospital doesn't have a `status_covid` table or uses different column names, you need to adjust the `INNER JOIN status_covid` section (lines 101 and 113) accordingly.

**This codebase is SQL Server only.** All repository queries use T-SQL syntax that is **not portable** to PostgreSQL or MySQL:
- `WITH (NOLOCK)` — SQL Server table hints (use `FOR UPDATE` in Postgres or remove in MySQL)
- `IIF(cond, true, false)` — SQL Server inline IF (use `CASE WHEN` for cross-DB)
- `CONCAT(a, b)` — SQL Server concat (ANSI but also works in others; safe to keep)
- `ISNULL(expr, default)` — SQL Server null check (use `COALESCE` for cross-DB)
- `NULLIF(expr, '')` — works in both, but the `LTRIM(RTRIM(...))` pattern needs adapters
- CTE `WITH TempRanap AS (...)` — ANSI SQL, works in Postgres/MySQL 8+, but the temp table + JOIN pattern is tuned for SQL Server session scope

If targeting Postgres/MySQL, **all repository queries must be rewritten** — not just the patient occupancy subquery. The `github.com/denisenkom/go-mssqldb` driver is hardcoded in `main.go` and `config/config.go`. Switching databases requires replacing the driver, updating all query syntax, and refactoring the transaction handling (SQL Server temp tables (`#temp`) are session-bound and not available in other databases).

### Repository Layer
Three interfaces in `internal/repository/interfaces.go`:
- `BedRepositoryInterface` — used by Worker (GetActiveSKNo, GetBedAvailability)
- `SKRepositoryInterface` — used by SKHandler (list, detail, bulk insert)
- `BedsRepositoryInterface` — used by BedsHandler (rooms, kamar, upsert mapping)

### Critical Security Rule
**DO NOT use `fmt.Sprintf` or string interpolation for SQL queries in repository files.**
All queries MUST use parameterized queries (`?` placeholders). String interpolation in SQL = SQL injection vulnerability.

### Handler Structure
- `APIHandler` — internal endpoints: `/api/beds`, `/api/logs`, `/api/sync`, `/api/worker/status`, `/api/sk-active`
- `SKHandler` — SK management: list, detail, preview, import
- `BedsHandler` — bed mapping: rooms, kamar, upsert
- `ProxyHandler` — Kemenkes API proxy (injects auth headers server-side so browser JS never sees credentials)

### API Endpoint Summary
```
GET  /api/beds                  — bed availability data
GET  /api/logs                  — last 200 log lines
POST /api/sync                  — trigger manual sync
GET  /api/worker/status         — worker state (Running/Idle)
GET  /api/sk-active             — active SK number
GET  /api/healthz               — health check
GET  /api/sk/list               — list all SK numbers
GET  /api/sk/detail?sk_no=<SK>  — SK detail by number
POST /api/sk/preview            — preview Excel import data
POST /api/sk/import             — execute SK import
GET  /api/beds/rooms            — list class_room_id values
GET  /api/beds/kamar?class_room_id=<ID> — list kamar for a room
GET  /api/beds/by-room?class_room_id=<ID> — beds data per room
POST /api/beds/upsert          — save/update bed mapping
GET  /api/proxy/referensi        — TT reference from Kemenkes
GET  /api/proxy/fasyankes        — submitted fasyankes data
POST /api/kemenkes/tempat-tidur          — create TT at Kemenkes
PUT  /api/kemenkes/tempat-tidur/{id_tt}  — update TT at Kemenkes
```

### BedsHandler Accordion Grouping Logic (`GetBedsByRoom`)
Groups are determined by `kamar_key` computed from `sk_bed` as:
```
ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), namaruang)
```
Beds from `beds` table are matched to groups by their `kamar` field:
- If `beds.kamar` is empty and exactly one group exists → assign to that group
- If `beds.kamar` doesn't match any group → a new group is auto-created with default `id_tt_siranap`, `covid`, `id_kelas`

**Common issue:** If `sk_bed.kamar` and `beds.kamar` values don't match, the accordion shows unexpected groups with defaults. Trailing spaces or casing differences in `kamar` can also cause mismatches.

### Startup Order (from `main.go`)
`config.Load()` → `logger.Init()` → `defer logger.Close()` → DB open/ping → `repository.New()` / `NewSKRepository()` / `NewBedsRepository(db, orgUnitCode)` → `dispatcher.StartWithContext(ctx)` → HTTP handlers → `srv.ListenAndServe()`

### Graceful Shutdown
`ctx.Done()` → `dispatcher.Stop()` → `srv.Shutdown(15s timeout)`. Dispatcher must stop before server to stop the ticker. If server fails to start (e.g., port already in use), error is sent to `serverErr` channel and logged.

### Kemenkes API Integration
- Base URL: `https://sirs.kemkes.go.id/fo/index.php`
- Required headers per request: `X-rs-id`, `X-pass`, `X-Timestamp` (Unix epoch seconds)
- PUT updates sent **per room row** from `[]BedSiranap`
- ProxyHandler injects credentials server-side — frontend JS only calls `/api/proxy/*`

### Config (nested structs in `config/config.go`)
- `Database` — SQL Server connection (Host, Port, User, Pass, Name)
- `API` — Kemenkes credentials and URL (URL, RsID, Pass, ExecutiveURL)
- `Operational` — APP_PORT, SYNC_INTERVAL_HOURS, RETRY_MAX, LOG_FILE, TLSSKIP_VERIFY, ORG_UNIT_CODE
- `Security` — DASHBOARD_ORIGIN, MAX_BODY_BYTES

### Deployment
- Console mode: `go run main.go`
- Windows Service: `sc create SIRSOnline binPath="C:\path\to\sirs-online.exe" start=auto`
- Auto-detects interactive vs service mode via `svc.IsAnInteractiveSession()`

## Environment Configuration

Copy `.env.example` to `.env`. Key variables:
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME` — SQL Server SIMRS
- `API_URL`, `API_RS_ID`, `API_PASS` — Kemenkes credentials
- `EXECUTIVE_API_URL` — Dashboard Eksternal API URL (optional)
- `APP_PORT` — dashboard port (default 9271)
- `SYNC_INTERVAL_HOURS` — sync interval (default 2h)
- `RETRY_MAX` — retry attempts per PUT request
- `TLS_SKIP_VERIFY` — skip TLS certificate verification (default false, set true for govt APIs with self-signed certs)
- `ORG_UNIT_CODE` — hospital organization unit code
- `DASHBOARD_ORIGIN` — CORS origin for dashboard
- `MAX_BODY_BYTES` — max request body size for POST/PUT

# Create logs directory before first run
mkdir logs

## Project Structure

```
main.go                          # Entry point (console/service detection)
config/config.go                 # Viper config with nested struct domains
docs/schema.sql                 # SIMRS table schema (sk_bed, beds, #temp_ranap)
internal/
├── handler/
│   ├── api_handler.go           # Internal monitoring endpoints
│   ├── beds_handler.go          # Bed mapping management
│   ├── sk_handler.go            # SK list/detail/import
│   └── proxy_handler.go         # Kemenkes API proxy (auth injection)
├── worker/
│   ├── dispatcher.go            # Ticker + job queue
│   ├── worker.go               # Sync logic (DB → Kemenkes PUT)
│   └── client.go               # Kemenkes HTTP client (Resty + TLS)
├── repository/
│   ├── interfaces.go           # Repository interfaces (mockable for tests)
│   ├── bed_repository.go       # Worker DB queries
│   ├── beds_repository.go      # Bed mapping queries
│   └── sk_repository.go        # SK queries
└── logger/logger.go            # File logger with ReadLast()
web/static/                      # Alpine.js + Tailwind CSS dashboard (static, no build step)
test/beds_management/           # Tab 6 Beds Management testing resources
example/
└── sk_bed_import_template.xlsx  # Ready-to-use 15-column template for Tab 5 import
```

### Dashboard (web/static/)
The dashboard at `http://localhost:9271` has **6 tabs**:
- **Tab 1** — Info Ruang: Real-time bed availability from in-memory state
- **Tab 2** — Master Referensi: Read-only reference data from Kemenkes
- **Tab 3** — Manajemen TT: POST/PUT forms for manual bed management
- **Tab 4** — Operasional & Worker: Logs, Sync Now button, worker status
- **Tab 5** — SK Manajemen: List, detail, preview, import Surat Keterangan (supports Excel .xlsx upload with 15-column format)
- **Tab 6** — Beds Management: class_room_id → kamar → id_t_tt mapping (upsert)

A separate executive dashboard exists at `web/static/eksekutif.html`. All frontend is pure Alpine.js + Tailwind CSS — no build step required.

### Tab 5 — Excel Import Format (sk_bed table)
Excel upload in Tab 5 expects 15 columns (header row required, order flexible due to keyword matching):

| clinic_id | class_room_id | kelas | bed | id_tt_siranap | ruang_siranap | kelas_siranap | covid | siranap | jml_ruang_siranap | kodekelas | namakelas | namaruang | kris | kamar |

See `example/sk_bed_import_template.xlsx` for a ready-to-use template.

## Git Conventions

Conventional commits via `.gitmessage` template:
```
<type>(<scope>): <subject>
```
Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`, `revert`
Scopes: `config`, `handler`, `worker`, `repo`, `ui`, `logger`, `main`

NOTE: Do NOT add AI as co-author — only the user's registered email may appear in commits.

Enable template: `git config commit.template .gitmessage`

## Critical Rules (never forget)

- Use brainstorming skill when user starts a new topic or plans something.
- Check and update CLAUDE.md and README.md when making significant changes.
- Never automatically commit and push. Wait for explicit user approval.
- Before pushing: `git fetch --tags && git pull --rebase`
