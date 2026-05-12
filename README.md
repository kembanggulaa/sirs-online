# SIRS Online Bridging System V3

> Automated system for synchronizing bed availability data from **SIMRS (SQL Server)** to **API RS Online Kemenkes**.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Key Features](#2-key-features)
3. [System Architecture](#3-system-architecture)
4. [Prerequisites](#4-prerequisites)
5. [Installation & Configuration](#5-installation--configuration)
6. [Running the Application](#6-running-the-application)
7. [Dashboard — 6 Tabs](#7-dashboard--6-tabs)
8. [Managing SK & Excel Import (Tab 5)](#8-managing-sk--excel-import-tab-5)
9. [API Endpoints](#9-api-endpoints)
10. [Testing](#10-testing)
11. [Git Conventions](#11-git-conventions)

---

## 1. Overview

SIRS Online Bridging V3 is a **background service** built with Go for **RSUD Sleman**. It acts as a bridge between the internal SIMRS database and the Ministry of Health's public API.

### How It Works

Every **2 hours** (or manually), the system:
1. Detects the active SK (Surat Keterangan / Official Letter) from the SIMRS database
2. Fetches bed availability data
3. Sends a report to the **Kemenkes API** (`sirs.kemkes.go.id`)

### Who Should Use?

- **SIMRS Operators** — managing SK and mapping beds
- **IT Administrators** — deploying and monitoring the server
- **Yankes Team** — monitoring RS Online Kemenkes data

---

## 2. Key Features

| Feature | Description |
|---|---|
| **Automatic Sync** | Worker pool runs every N hours (default: 2 hours) |
| **Manual Sync** | "Sync Now" button for on-demand triggering |
| **Web Dashboard** | Browser interface for monitoring and control (6 tabs) |
| **SK Management** | Create, edit, and import SK data via Excel |
| **Bed Mapping** | Map `class_room_id` to Kemenkes data |
| **Graceful Shutdown** | Clean shutdown when service is stopped |
| **Windows Service** | Runs 24/7 without operator login |
| **Structured Logging** | Logs stored in file, last 200 lines viewable via API |

---

## 3. System Architecture

> **HTTP Framework**: Since v2.0.0, the application uses **Gin** (`github.com/gin-gonic/gin`) for HTTP routing and middleware. Previously used `net/http` standard library.

```
sirs-online/
├── main.go                     # Entry point (console / Windows Service)
├── config/config.go            # Configuration from .env
├── internal/
│   ├── handler/                # HTTP endpoints (Gin framework)
│   │   ├── api_handler.go      # beds, logs, sync, worker status
│   │   ├── sk_handler.go       # SK management (list, detail, import)
│   │   ├── beds_handler.go      # Bed mapping
│   │   ├── proxy_handler.go    # Proxy to Kemenkes API
│   │   └── middleware.go       # CORS and body size middleware
│   ├── worker/                 # Core sync system
│   │   ├── dispatcher.go       # Ticker scheduler & job queue
│   │   ├── worker.go           # Sync logic (DB → Kemenkes PUT)
│   │   └── client.go           # HTTP client to Kemenkes
│   ├── repository/             # SQL Server queries
│   │   ├── bed_repository.go  # GetActiveSKNo, GetBedAvailability
│   │   ├── sk_repository.go   # SK list, detail, bulk insert
│   │   └── beds_repository.go  # Rooms, kamar, upsert mapping
│   └── logger/                 # File-based logger
└── web/static/                 # Dashboard (Alpine.js + Tailwind CSS)
```

### Sync Flow

```
[Ticker every 2 hours] ──or── [Sync Now Button]
        │
        ▼
  Dispatcher.dispatch() → Job channel
        │
        ▼
  Worker.processJob()
   ├── 1. GetActiveSKNo()        → Query sk_bed WHERE tgl_berakhir IS NULL
   ├── 2. GetBedAvailability()   → Temp table #temp_ranap + main query in ONE tx
   ├── 3. GET /Fasyankes         → Fetch id_t_tt mapping from Kemenkes
   └── 4. PUT /Fasyankes/{id_tt} → Send per room (with retry, max RETRY_MAX+1)
```

### Critical Rules

- **Both `#temp_ranap` and main query must run on the same `*sql.Tx`** — temp table is session-bound in SQL Server
- **All SQL queries MUST use parameterized queries (`?` placeholders)** — string interpolation = SQL injection vulnerability
- **Repository interfaces** (`internal/repository/interfaces.go`) enable mockable testability:
  - `BedRepositoryInterface` — Worker queries
  - `SKRepositoryInterface` — SKHandler queries
  - `BedsRepositoryInterface` — BedsHandler queries

---

## 4. Prerequisites

- **Go 1.23+** — [download at golang.org/dl](https://golang.org/dl/)
- **SQL Server SIMRS** — host, port, user, password, database name
- **Kemenkes API Credentials** — `API_RS_ID` and `API_PASS` from Kemenkes
- **Windows** — for Windows Service mode (optional)
- **`logs/` directory** — create manually before first run
- **SIMRS Tables** — `docs/schema.sql` documents the required database tables. If your SIMRS schema differs, adapt the queries in `internal/repository/` accordingly.

---

## 5. Installation & Configuration

### 5.1 Clone & Install Dependencies

```bash
git clone <repository-url>
cd sirs-online
go mod tidy
```

### 5.2 Create Logs Directory

```bash
mkdir logs
```

Log file will be written to `logs/sirs.log`. If the folder doesn't exist, the application may fail or logs won't be saved.

### 5.3 Create `.env` File

```bash
copy .env.example .env
```

Edit `.env` with actual values:

```env
# ── SIMRS Database (SQL Server) ──────────────────────────────────────
DB_HOST=localhost
DB_PORT=1433
DB_USER=sa
DB_PASS=P@ssw0rdDB
DB_NAME=db_simrs_utama

# ── Kemenkes RS Online API ───────────────────────────────────────────
API_URL=https://sirs.kemkes.go.id/fo/index.php
API_RS_ID=KODE_RS_ANDA
API_PASS=PASSWORD_API_RAHASIA

# ── Operational ─────────────────────────────────────────────────────
APP_PORT=9271
SYNC_INTERVAL_HOURS=2
RETRY_MAX=2
LOG_FILE=logs/sirs.log
ORG_UNIT_CODE=KODE_RS_ANDA

# ── TLS (for government APIs with self-signed certificates) ─────────
TLS_SKIP_VERIFY=false

# ── Security ────────────────────────────────────────────────────────
DASHBOARD_ORIGIN=http://localhost:9271
MAX_BODY_BYTES=1048576
```

### Environment Variable Reference

| Variable | Default | Description |
|---|---|---|
| `DB_HOST` | — | SQL Server host |
| `DB_PORT` | `1433` | SQL Server port |
| `DB_USER` | — | SIMRS database username |
| `DB_PASS` | — | Database password |
| `DB_NAME` | — | SIMRS database name |
| `API_URL` | — | Kemenkes API base URL |
| `API_RS_ID` | — | RS code (X-rs-id header) |
| `API_PASS` | — | Kemenkes API password |
| `EXECUTIVE_API_URL` | — | Executive dashboard API URL (optional) |
| `APP_PORT` | `9271` | Dashboard HTTP port |
| `SYNC_INTERVAL_HOURS` | `2` | Auto-sync interval (hours) |
| `RETRY_MAX` | `2` | Retry attempts if PUT fails |
| `LOG_FILE` | `logs/sirs.log` | Log file location |
| `TLS_SKIP_VERIFY` | `false` | Skip TLS verification |
| `ORG_UNIT_CODE` | — | Hospital organization unit code |
| `DASHBOARD_ORIGIN` | — | CORS origin for dashboard |
| `MAX_BODY_BYTES` | `1048576` | Max POST/PUT body size |

---

## 6. Running the Application

### Development Mode (Console)

```bash
go run main.go
```

Open dashboard at: **`http://localhost:9271`**

### Build Binary

```bash
go build -o sirs-online.exe .
./sirs-online.exe
```

### Windows Service Mode (Production)

```bash
# Register as Windows Service (run as Administrator)
sc create SIRSOnline binPath="C:\path\to\sirs-online.exe" start=auto

# Start service
sc start SIRSOnline

# Stop service
sc stop SIRSOnline

# Delete service
sc delete SIRSOnline
```

> **Note:** The application auto-detects mode. Administrator login or interactive Terminal → console app. If not → Windows Service.

---

## 7. Dashboard — 6 Tabs

Dashboard available at `http://localhost:9271` with **6 tabs**:

| Tab | Name | Function |
|---|---|---|
| **1** | Info Ruang | Real-time bed availability from last sync |
| **2** | Master Referensi | Read-only reference data from Kemenkes |
| **3** | Manajemen TT | POST/PUT forms for manual bed management |
| **4** | Operasional & Worker | Activity logs, Sync Now button, worker status |
| **5** | SK Manajemen | Manage Official Letters — including Excel import |
| **6** | Beds Management | `class_room_id` → kamar → `id_t_tt` mapping (upsert) |

### Executive Dashboard

Additional executive dashboard at **`web/static/eksekutif.html`** for management-level monitoring.

---

## 8. Managing SK & Excel Import (Tab 5)

Tab 5 is used to **create and import SK (Surat Keterangan) data** into the `sk_bed` table in SIMRS database.

### Excel Import Steps

1. **Open Tab 5 — SK Manajemen**
2. **Step 1**: Fill in *New SK Number* and *Effective Date*
3. **Upload Excel File**: Click **"Choose File Excel"** or drag-and-drop `.xlsx` file
4. **Step 2**: Data from Excel appears. Review and edit if needed.
5. **Step 3**: Click **Save** — old SK (if any) will be automatically retired.

### Excel Column Format (15 Columns)

Excel file must have **header row** with the following columns:

| No | Column Name | Type | Description |
|---|---|---|---|
| 1 | `clinic_id` | text | Clinic ID (can be empty) |
| 2 | `class_room_id` | text | Ward/room ID |
| 3 | `kelas` | text | Care class |
| 4 | `bed` | number | Number of beds |
| 5 | `id_tt_siranap` | text | TT ID from Kemenkes |
| 6 | `ruang_siranap` | text | Siranap room name |
| 7 | `kelas_siranap` | text | Siranap class |
| 8 | `covid` | number | COVID flag (0 or 1) |
| 9 | `siranap` | text | Siranap name |
| 10 | `jml_ruang_siranap` | number | Room count (default 1) |
| 11 | `kodekelas` | text | Class code |
| 12 | `namakelas` | text | Class name |
| 13 | `namaruang` | text | Room name (fallback if kamar empty) |
| 14 | `kris` | text | Kris indicator |
| 15 | `kamar` | text | Room number |

### Excel Template

Ready-to-use template available at: **`example/sk_bed_import_template.xlsx`**

Open the file in Excel or Google Sheets, edit the data as needed, then save.

### Important Notes

- **Column order is flexible** — system searches columns by header name (case-insensitive)
- **Empty rows** are automatically skipped
- If **old SK is still active**, the system will automatically retire it (tgl_berakhir = H-1 from new SK's tgl_berlaku)
- This process is **idempotent** — re-importing with the same SK will overwrite old data

---

## 9. API Endpoints

All endpoints run at `http://localhost:{APP_PORT}`.

### Monitoring & Sync

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/beds` | Bed availability data |
| `GET` | `/api/logs` | Last 200 lines from log file |
| `POST` | `/api/sync` | Trigger manual sync |
| `GET` | `/api/worker/status` | Worker status (`Running` / `Idle`) |
| `GET` | `/api/sk-active` | Currently active SK number |
| `GET` | `/api/healthz` | Health check |

### SK Management (Tab 5)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/sk/list` | List all SK numbers |
| `GET` | `/api/sk/detail?sk_no=<SK>` | Detail of specific SK |
| `POST` | `/api/sk/preview` | Preview data before import |
| `POST` | `/api/sk/import` | Import SK data to database |

### Bed Management (Tab 6)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/beds/rooms` | List `class_room_id` |
| `GET` | `/api/beds/kamar?class_room_id=<ID>` | List rooms |
| `GET` | `/api/beds/by-room?class_room_id=<ID>` | Raw beds data per room |
| `POST` | `/api/beds/upsert` | Save/update bed mapping |

### Kemenkes Proxy

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/proxy/referensi` | TT reference from Kemenkes |
| `GET` | `/api/proxy/fasyankes` | Fasyankes data already submitted |
| `POST` | `/api/kemenkes/tempat-tidur` | Create new TT at Kemenkes |
| `PUT` | `/api/kemenkes/tempat-tidur/{id_tt}` | Update TT at Kemenkes |

---

## 10. Testing

### Run All Tests

```bash
go test ./...
```

### With Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

> **Note:** The `-race` flag cannot be used on Windows due to `CGO_ENABLED=0`.

### Integration Tests (Repository)

```bash
TEST_DATABASE_DSN="server=...;port=...;user id=...;password=...;database=..." go test ./internal/repository/... -v -run Integration
```

---

## 11. Git Conventions

This project uses **Conventional Commits**.

Format:
```
<type>(<scope>): <subject>
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`, `revert`

**Scopes:** `config`, `handler`, `worker`, `repo`, `ui`, `logger`, `main`

Commit examples:
```bash
feat(worker): add exponential backoff on API timeout
fix(sk_handler): add tgl_berlaku validation on preview and import
docs(readme): update API endpoint table
```

Enable commit template:
```bash
git config commit.template .gitmessage
```

---

## License

Proprietary — **RSUD Sleman**. All rights reserved.
