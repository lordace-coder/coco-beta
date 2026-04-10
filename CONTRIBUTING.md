# Contributing to Cocobase

Thanks for wanting to help! Here's everything you need to get started.

---

## Project layout

```
coco-golang/
├── cmd/cocobase/main.go          # Entry point — starts Fiber, runs migrations
├── internal/
│   ├── api/
│   │   ├── handlers/             # HTTP handlers (auth, collections, documents)
│   │   │   └── dashboard/        # Admin dashboard API handlers
│   │   ├── middleware/           # Auth, CORS, rate limiting
│   │   └── routes/               # Route registration
│   ├── database/connection.go    # DB connect + versioned auto-migration
│   ├── models/                   # GORM models
│   ├── services/                 # Business logic (email, storage, query builder)
│   └── dashboardfs/              # Embedded React build (dist/)
├── dashboard/                    # React admin dashboard source
│   ├── src/
│   │   ├── api/client.ts         # Axios client + TypeScript types
│   │   ├── components/           # Shared UI components
│   │   └── pages/                # Page components + tab sub-components
│   └── package.json
├── pkg/config/config.go          # Env var loading
└── go.mod
```

---

## Local development setup

### Prerequisites

- Go 1.21+
- Node.js 18+ (for dashboard development)
- PostgreSQL **or** SQLite (SQLite needs CGO — install `gcc`)
- `air` for hot reload (optional): `go install github.com/air-verse/air@latest`

### 1. Clone and configure

```bash
git clone https://github.com/lordace-coder/coco-golang.git
cd coco-golang
cp .env.example .env   # edit with your DB URL and SECRET
```

### 2. Run the backend

```bash
# With hot reload
air

# Or directly
go run ./cmd/cocobase/
```

### 3. Run the dashboard (dev mode with hot module replacement)

```bash
cd dashboard
npm install
npm run dev
```

The dev server proxies `/_/api/*` to the Go backend at `localhost:3000`, so run both at the same time.

### 4. Build the dashboard into the binary

```bash
cd dashboard
npm run build
cp -r dist/* ../internal/dashboardfs/dist/
cd ..
go build ./cmd/cocobase/
```

The dashboard is embedded into the Go binary via `//go:embed`, so you must rebuild the binary after changing the frontend.

---

## Making changes

### Backend (Go)

- Handlers live in `internal/api/handlers/` (app-facing) and `internal/api/handlers/dashboard/` (admin API).
- Add new routes in `internal/api/routes/`.
- Add new models to `internal/models/` **and** register them in `database.Migrate()` in `internal/database/connection.go`.
- **Bump `schemaVersion`** in `connection.go` whenever you add or change a model — this triggers the migration on the next startup without re-running it on every cold start.

### Dashboard (React + TypeScript)

- API calls go through `dashboard/src/api/client.ts` — add new endpoints there first.
- Pages live in `dashboard/src/pages/`. Per-project tabs are in `dashboard/src/pages/project/`.
- Shared components (buttons, forms, etc.) go in `dashboard/src/components/`.
- After editing the dashboard, run `npm run build` and copy `dist/` to `internal/dashboardfs/dist/` before testing with the Go binary.

---

## Commit style

Use short, present-tense commit messages:

```
add file upload support to collections
fix JWT expiry not being checked
update dashboard to show activity logs
```

No ticket numbers required. If a commit closes a GitHub issue, add `Closes #123` at the end of the message body.

---

## Pull requests

1. Fork the repo and create a branch: `git checkout -b my-feature`
2. Make your changes and test locally.
3. If you added a new model, bump `schemaVersion` in `internal/database/connection.go`.
4. If you changed the dashboard, include the rebuilt `internal/dashboardfs/dist/` files in your PR.
5. Open a pull request against `main` with a clear description of what changed and why.

---

## Reporting bugs

Open a GitHub issue with:

- What you were doing
- What you expected to happen
- What actually happened
- Your OS, Go version, and database type (SQLite / PostgreSQL)

---

## Code style

- Follow standard Go formatting (`gofmt` / `goimports`).
- Keep handlers thin — business logic belongs in `internal/services/`.
- Don't add comments that just restate what the code does. Add them only when the *why* isn't obvious.
- No unused imports or variables — the Go compiler will catch these anyway.
