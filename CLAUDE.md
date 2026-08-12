# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

多云管理平台 (Cloud Manager) — a multi-cloud management web platform for SMBs to centrally manage
Aliyun / Tencent cloud servers, domains, and DNS records. Full-stack: Go (Gin) + SQLite + React + Ant Design 5.
The codebase and UI are in Chinese; keep user-facing strings Chinese when editing.

## Commands

Backend (port 8002):
```bash
cd backend
# First-time on China networks — configure Go proxy/toolchain:
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=off
go env -w GOTOOLCHAIN=local
go mod tidy
GIN_MODE=debug go run ./cmd/server                                          # dev (release mode is the build default)
go build -trimpath -ldflags="-s -w" -o bin/cloud-manager.exe ./cmd/server   # build (-s -w strips debug symbols, ~30% smaller)
```

The built `cloud-manager.exe` defaults to production mode (`GIN_MODE=release`), binding `0.0.0.0:8002`.
If `data/config.json` exists next to `data/app.db`, it overrides `host` / `port` / `db_path` /
`jwt_secret` / `aes_key` / `gin_mode`; environment variables (`HOST`, `PORT`, `DB_PATH`, `JWT_SECRET`,
`AES_KEY`, `GIN_MODE`) take precedence over both. See `config/config.go`.

Frontend (port 5173, proxies `/api` → `:8002`):
```bash
cd frontend
pnpm install
pnpm dev          # dev
pnpm build        # tsc --noEmit && vite build → dist/
```

Start both at once: `./start.ps1` (PowerShell; launches both minimized, press any key to stop).
Login: `admin` / `admin123`.

There is no test suite or lint config in this repo — do not assume `go test` / `pnpm test` exist.

## Backend architecture

Bootstrap (`cmd/server/main.go`): `config.Load` → `logging.Init` → `repository.Init` (GORM `AutoMigrate`) →
`migrations.Seed` → `service.New` → `api.NewRouter`. Layered as **handler → service → repository → GORM/SQLite**,
with a separate `internal/cloud/` package of provider adapters.

- **Pure-Go SQLite** (`github.com/glebarez/sqlite`) — no CGO/GCC required. DB at `backend/data/app.db` (gitignored).
- **Config** (`config/config.go`), priority low → high: built-in defaults (production mode, `0.0.0.0:8002`) → `data/config.json` (optional, keys `host`/`port`/`db_path`/`jwt_secret`/`aes_key`/`gin_mode`) → env vars (`HOST`/`PORT`/`DB_PATH`/`JWT_SECRET`/`AES_KEY`/`GIN_MODE`). `AES_KEY` must be 16/24/32 bytes; the default is padded/truncated to 32.
- **Logging** (`internal/logging`): `logging.Init("./logs")` redirects the standard `log` package and gin's default/error writers to `logs/app.log` (10MB rolling, 10 backups, 30-day retention, gzip via lumberjack), in addition to stdout. **Stability**: every background goroutine (multi-region SDK fan-out in `internal/cloud/aliyun|tencent`, `CloudAccountService.BatchSync`/`StartAutoSync`, the WebShell SSH↔WebSocket bridge in `internal/ws/ssh.go`, and the startup async sync in `migrations/seed.go`) is guarded with `defer logging.Recover("tag")` so a panic in one goroutine logs a stack trace instead of crashing the whole process. HTTP handler panics are already caught by `gin.Recovery()`. When adding a new `go func(){...}()`, add the same `defer logging.Recover("tag")` as its first statement.
- **Seed** (`migrations/seed.go`) creates only the admin user, then **async** re-syncs all existing cloud accounts on startup (multi-region SDK calls fan out, so sync runs in a goroutine to avoid blocking HTTP start). Cloud accounts are user-entered via the UI — seed no longer inserts mock example accounts.
- **Response envelope**: `model.Response{code, message, data}` with `code==0` = success. Handlers use `helpers.go` (`JSON`/`FailErr`/`FailMsg`). Operation logging is best-effort via `H.recordOp`.
- **Middleware**: JWT (sets `CtxUserID`/`CtxUsername`/`CtxRole`), CORS, Logger. `/api/auth/login` and `/health` are public; everything else under `/api` requires a Bearer JWT.

### CloudProvider — the central abstraction

`internal/cloud/provider.go` defines the `CloudProvider` interface; **the business layer only programs to this interface**. Implementations:
- `mock` — in-memory singleton (`defaultMock`), stateful (power on/off, DNS CRUD persist across calls).
- `aliyun` — real Aliyun SDK (ECS / Domain / Alidns).
- `tencent` — real Tencent SDK (CVM / domain / dnspod).

Each provider registers itself in `init()` via `cloud.Register(code, factory)`. `router.go` **blank-imports** `aliyun` and `tencent` to trigger registration (mock self-registers). Routing is `cloud.Get(acct.Provider)` keyed by the provider string (`"aliyun"` / `"tencent"` / `"mock"`).

> ⚠️ **`mode` field discrepancy**: the README and the frontend `CloudAccountForm` (`frontend/src/api/index.ts`) reference a `mode` field (`mock` / `sdk`) to switch between mock and real adapters. The **backend `CloudAccount` model has no `Mode` field** — routing is purely by `Provider` string. Mock-vs-real selection via `mode` is therefore not currently wired through the backend model; the mock provider distinguishes aliyun/tencent data by `InstanceID` prefix (`i-bp1` / `ins_cvm`) and domain registrar. Keep this in mind before trusting the README's mode description.

### Resource model: synced snapshots, not live

Cloud resources are **cached snapshots** in SQLite with a `synced_at` timestamp, pulled on demand by `CloudAccountService.Sync` (servers, then domains, then per-domain DNS records — all upserted). Server **actions** (start/stop/reboot) call the provider live, then update the local cached status so the UI reflects immediately. Aliyun domain/DNS are global services pinned to `cn-hangzhou`; servers fan out across all regions via `DescribeRegions` + per-region `DescribeInstances`.

### Secret handling

`AccessKeySecret` is AES-GCM encrypted (`internal/auth/crypto.go`, base64-encoded) before storage and tagged `json:"-"`; API responses expose only `access_key_masked`. Services decrypt on demand when building `cloud.Account`.

## Frontend architecture

React 18 + Ant Design 5 + Zustand + react-router 6 + axios + ECharts, bundled with Vite.

- **`api/client.ts`** — axios instance, `baseURL: '/api'`. Request interceptor injects `Authorization: Bearer <token>` from the Zustand store; response interceptor unwraps the `{code,message,data}` envelope (rejects when `code!==0`), and on 401 logs out + redirects to `/login`.
- **`stores/auth.ts`** — Zustand store persisted to `localStorage` (key `cloud-manager-auth`) holding `token` + `user`.
- **`routes/index.tsx`** — `createBrowserRouter` with a `RequireAuth` guard, `MainLayout` shell, and lazy-loaded pages.
- **`api/index.ts`** — typed API wrappers grouped by resource; return types live in `types/index.ts`.
- Vite dev proxy forwards `/api` → `http://localhost:8080`.
