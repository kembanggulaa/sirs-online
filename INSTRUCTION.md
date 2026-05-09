# INSTRUCTION.md

<!--
This file is the single source of truth for AI assistants working on this project.
AI: Read this file at the start of every session. Update it when conventions,
architecture, or key paths change. Also keep README.md in sync.
If Obsidian MCP is available, check the vault for additional knowledge, best
practices, and reference notes. Ask the user for the vault path if needed.
-->

## Project Overview

SIRS Online Bridging V3 is a background service built with Go for RSUD Sleman (Sleman Regional General Hospital). It acts as a bridge between the internal SIMRS (Hospital Information Management System) database and the Ministry of Health's public API, automatically reporting bed availability at configurable intervals.
The application includes a browser-based web dashboard for real-time monitoring, SK (Surat Keterangan / Official Letter) management, bed mapping, and manual synchronization.

## System Requirements

- **Operating System**: Windows (designed to support Windows Service API and interactive Console App)
- **Database**: Microsoft SQL Server (Mssql)
- **Programming Language**: Golang (v1.23.0 or newer)
- **Key Dependencies**:
  - `github.com/microsoft/go-mssqldb` (Mssql database connector driver)
  - `github.com/go-resty/resty/v2` (HTTP Client for Kemenkes Web Services API)
  - `github.com/spf13/viper` (Configuration data loading management)
  - `github.com/gin-gonic/gin` (HTTP framework - migrated from net/http in v2.0.0)
  - `golang.org/x/sys` (Core OS system library, especially Windows service integration)

## Installation & Configuration

1. **Clone Repository**:
   ```bash
   git clone <url-repo-anda>
   cd sirs-online
   ```

2. **Install Go Dependencies**:
   ```bash
   go mod tidy
   ```

3. **Configure Environment (`.env`)**:
   Copy and rename `.env.example` to `.env`:
   ```bash
   copy .env.example .env
   ```
   Edit `.env` with your actual values:
   ```env
   # SIMRS Database Connection Setup
   DB_HOST=127.0.0.1
   DB_PORT=1433
   DB_USER=sa
   DB_PASS=P@ssw0rdDB
   DB_NAME=db_simrs_utama

   # Kemenkes SIRS Online Credentials
   API_URL=https://sirs.kemkes.go.id/fo/index.php
   API_RS_ID=KODE_RS_ANDA
   API_PASS=PASSWORD_RAHASIA_API_ANDA

   # Internal Ticker / Bridge Server Operational
   APP_PORT=9271
   SYNC_INTERVAL_HOURS=2
   LOG_FILE=logs/sirs.log
   ORG_UNIT_CODE=KODE_RS_ANDA
   DASHBOARD_ORIGIN=http://localhost:9271
   TLS_SKIP_VERIFY=false
   ```

## Running the Application

The Go application uniquely detects and handles two execution modes:

**A. Development / Interactive Mode (Console)**:
```bash
go run main.go
# Or if binary has been built:
./sirs-online.exe
```

**B. Production Mode (Windows Service)**:
```bash
sc create SIRSOnline binPath="C:\path\to\sirs-online.exe" start=auto
sc start SIRSOnline
```
Open `http://localhost:9271` in browser to verify dashboard loads and backend is serving on port `9271`.

The application auto-detects mode: Administrator login or interactive Terminal → console app. Otherwise → Windows Service.

## System Architecture

The application uses modular design pattern inside `internal/` (Go idiomatic struct layout):

- **Handler (`internal/handler`)**:
  - `APIHandler`: Internal monitoring endpoints (beds, logs, sync, worker status)
  - `SKHandler` & `BedsHandler`: SK management and bed ID mapping
  - `ProxyHandler`: Routes/injects credentials server-side so browser JS never sees them
- **Worker (`internal/worker`)**: Background sync worker — ticker-based interval + manual trigger
  - `Dispatcher`: Job queue and ticker scheduling
  - `Worker`: Sync logic (DB → Kemenkes PUT)
  - `Client`: Resty HTTP client with TLS support
- **Repository (`internal/repository`)**: SQL Server queries abstracted into repository interfaces
  - `bed_repository.go`: GetActiveSKNo, GetBedAvailability
  - `sk_repository.go`: SK list, detail, bulk insert
  - `beds_repository.go`: Rooms, kamar, upsert bed mapping
- **Config & Logger**: Boot-time config loading and file-based logging with ReadLast()

### Sync Flow (Worker Pool)

```
[Ticker every N hours] ──or── [Sync Now Button]
        │
        ▼
  Dispatcher.dispatch() → Job channel
        │
        ▼
  Worker.processJob()
   ├── 1. GetActiveSKNo()        → Query sk_bed WHERE tgl_berakhir IS NULL
   ├── 2. GetBedAvailability()   → Temp table #temp_ranap + main query in ONE tx
   ├── 3. GET /Fasyankes         → Fetch id_t_tt mapping from Kemenkes
   └── 4. PUT /Fasyankes/{id_tt} → Send per room with retry (max RETRY_MAX+1 attempts)
```

### Critical DB Rule
Both temp table (`#temp_ranap`) and main bed availability query **must run on the same `*sql.Tx`** (single connection). The temp table is session-bound in SQL Server.

### Repository Interfaces (`internal/repository/interfaces.go`)
- `BedRepositoryInterface` — used by Worker (GetActiveSKNo, GetBedAvailability)
- `SKRepositoryInterface` — used by SKHandler (list, detail, bulk insert)
- `BedsRepositoryInterface` — used by BedsHandler (rooms, kamar, upsert)

### Critical Security Rule
**DO NOT use `fmt.Sprintf` or string interpolation for SQL queries in repository files.**
All queries MUST use parameterized queries (`?` placeholders). String interpolation in SQL = SQL injection vulnerability.

### Graceful Shutdown
`ctx.Done()` → `dispatcher.Stop()` → `srv.Shutdown(15s timeout)`. Dispatcher must stop before server to stop the ticker. If server fails to start (e.g., port already in use), error is sent to `serverErr` channel and logged.

## Dashboard — 6 Tabs

Located at `http://localhost:9271` with **6 tabs**:

| Tab | Name | Function |
|---|---|---|
| **1** | Info Ruang | Real-time bed availability from in-memory state |
| **2** | Master Referensi | Read-only reference data from Kemenkes |
| **3** | Manajemen TT | POST/PUT forms for manual bed management |
| **4** | Operasional & Worker | Logs, Sync Now button, worker status |
| **5** | SK Manajemen | List, detail, preview, import SK (Excel .xlsx upload with 15-column format) |
| **6** | Beds Management | `class_room_id` → kamar → `id_t_tt` mapping (upsert) |

A separate executive dashboard exists at `web/static/eksekutif.html`.

## API Documentation

### Internal Backend Dashboard API
- `GET /api/beds` — Real-time bed availability from in-memory state
- `GET /api/logs` — Last 200 lines from log file
- `POST /api/sync` — Trigger instant sync (ignores interval schedule)
- `GET /api/worker/status` — Worker status: `Running` vs `Idle`
- `GET /api/sk-active` — Current active SK number
- `GET /api/healthz` — Health check for monitoring

### SK Management (Tab 5)
- `GET /api/sk/list` — List all SK numbers
- `GET /api/sk/detail?sk_no=<SK>` — Detail of specific SK
- `POST /api/sk/preview` — Preview data before import
- `POST /api/sk/import` — Import SK data to database

### Bed Management (Tab 6)
- `GET /api/beds/rooms` — List `class_room_id`
- `GET /api/beds/kamar?class_room_id=<ID>` — List rooms
- `GET /api/beds/by-room?class_room_id=<ID>` — Raw beds data per room
- `POST /api/beds/upsert` — Save/update bed mapping

### Bridge Proxy Endpoints (Kemenkes API)
- `GET /api/proxy/referensi` — Forward request to Kemenkes `/Referensi/tempat_tidur`
- `GET /api/proxy/fasyankes` — View fasyankes records from Kemenkes
- `POST /api/kemenkes/tempat-tidur` — Create new TT at Kemenkes
- `PUT /api/kemenkes/tempat-tidur/{id_tt}` — Update TT at Kemenkes
- `GET /api/beds/executive` — Proxy to custom `EXECUTIVE_API_URL`

## Excel Import Format (Tab 5)

Excel upload in Tab 5 expects **15 columns** (header row required, order flexible due to keyword matching):

| clinic_id | class_room_id | kelas | bed | id_tt_siranap | ruang_siranap | kelas_siranap | covid | siranap | jml_ruang_siranap | kodekelas | namakelas | namaruang | kris | kamar |

See `example/sk_bed_import_template.xlsx` for a ready-to-use template.

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

## Conventions

- **Python**: Always use `uv` (never pip, conda, pipenv). All scripts must be Python, no bash scripts.
- **Node.js**: Always use `bun`/`bunx` (never node, npm, npx).
- **Brainstorming**: When the user starts a new topic or plans something, always use the brainstorming skill first. If unsure whether to brainstorm, ask the user.
- **Superpowers**: Ensure superpowers skills are installed and use them. Brainstorm before features, TDD for implementation, systematic-debugging for bugs.
- **README.md**: Human-readable project documentation. Update when user-facing behavior changes.
- **INSTRUCTION.md**: AI-readable project context (this file). Update when project conventions, architecture, or key paths change.
- **Scripts**: For complex automation, create proper UV projects (Python) or Bun projects (TypeScript) with `pyproject.toml`/`package.json`. Always `.gitignore` generated files (`.venv/`, `node_modules/`, `__pycache__/`).
- **Containers**: Use `docker`/`docker compose` or `podman`/`podman compose` — whichever container runtime is available. Prefer auto-detection in Taskfile (check `docker info` then `podman info`). Do not hardcode a single runtime.
- **Commits**: Use [Conventional Commits](https://www.conventionalcommits.org/). Format: `<type>(<scope>): <description>`. Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`.
- **Branching**: Create a new branch for each feature or fix (`feat/...`, `fix/...`). Create PR (GitHub, use `gh`) or MR (GitLab, use `glab`) when ready. Always squash commits on merge. Use `gh`/`glab` to check PR/MR and CI pipeline results before merging.
- **Test Coverage**: Target at least 75% test coverage. Use a combination of unit tests, integration tests (with testcontainers), and e2e tests. Prioritize critical paths and business logic.
- **Git workflow**: Never automatically commit and push. Complete work, show results, wait for explicit approval. Before pushing: `git fetch --tags && git pull --rebase` then `git push`.
- **Semantic Release**: This project uses Semantic Release on `main`. It auto-creates commits, tags, and releases. Always fetch/pull before push.

## Troubleshooting

1. **Dashboard Empty or "404 Not Found"**
   - *Cause*: Wrong root path (run from different parent folder). Application depends on `web/static/` folder being in the working directory. Confirm terminal path when running.
2. **MSSQL Database Connection Fails**
   - *Cause*: `DB_PORT`, `DB_HOST`, or instance name not readable. Ensure `sa` user is not locked out and SQL Browser port is reachable.
3. **SSL Certificate Verification Error When Hitting Kemenkes API**
   - *Solution*: Government API platforms sometimes have SSL certificate rotation latency. Set `TLS_SKIP_VERIFY=true` in `.env`.
4. **Log Not Saved / Cannot Open File Permission Error**
   - *Solution*: Folder path not available. Ensure `logs/` folder exists in root directory, and Windows Service user has Read & Write permissions.

## Absolute Rules

**NEVER add AI (Claude, Copilot, or any AI) as co-author, committer, or contributor in git commits or PR/MR messages.**
Only the user's registered email may appear in commits. This is company policy — commits and PRs with AI authorship WILL BE REJECTED. Do not use `--author`, `Co-authored-by`, `Generated by`, `Created with`, or any other mechanism to attribute work to AI. Never add AI attribution text (e.g., "Generated by Claude", "Built with AI") in commit messages, PR/MR descriptions, or code comments. This applies to ALL commits and PRs, including those made by tools and subagents.
-->
