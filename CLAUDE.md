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

### BedsHandler Accordion Grouping Logic (`GetBedsByRoom`)
Groups come from `sk_bed` (kamar_key = `kamar` or fallback to `namaruang`), then beds from `beds` table are matched by `kamar`. Fallback rule: if a bed's `kamar` is empty and only one group exists, it's assigned to that group — otherwise a new group with defaults is created.

**Impact:** If `sk_bed.kamar` and `beds.kamar` values mismatch, new accordion groups with defaults (id_tt_siranap, covid, id_kelas) are auto-created.

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
example/                       # Example files (e.g., sk_bed_import_template.xlsx)
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

Enable template: `git config commit.template .gitmessage`

## ABSOLUTE RULE — NO EXCEPTIONS

**NEVER add AI (Claude, Copilot, or any AI) as co-author, committer, or contributor in git commits or PR/MR messages.**
Only the user's registered email may appear in commits. This is company policy — commits and PRs with AI
authorship WILL BE REJECTED. Do not use `--author`, `Co-authored-by`, `Generated by`, `Created with`, or any
other mechanism to attribute work to AI. Never add AI attribution text (e.g., "Generated by Claude",
"Built with AI") in commit messages, PR/MR descriptions, or code comments. This applies to ALL commits
and PRs, including those made by tools and subagents.

## Critical Rules (never forget)

- Always use `task <name>` to run commands — never run raw commands directly.
- Python: always `uv` (never pip, conda, pipenv). No bash scripts.
- Node.js: always `bun`/`bunx` (never node, npm, npx).
- Containers: use `docker`/`docker compose` or `podman`/`podman compose` — whichever is available. Prefer auto-detection in Taskfile.
- Use brainstorming skill when user starts a new topic or plans something.
- Check and update INSTRUCTION.md and README.md when making significant changes.
- Conventional Commits: `<type>(<scope>): <description>`.
- Branch per change, squash merge. Use `gh`/`glab` for PR/MR and CI checks.
- Never automatically commit and push. Wait for explicit user approval.
- Before pushing: `git fetch --tags && git pull --rebase` then `git push`.
