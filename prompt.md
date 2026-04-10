# Cocobase Go — Project Brief

## What This Is

Cocobase is an open-source, self-hosted Backend-as-a-Service (BaaS) written in Go.
It is the open-source equivalent of the hosted Python version at `/home/patrick/Documents/COCOBASE/cocobase`.
Users run it on their own servers. There are no payment plans, no usage limits, no SaaS features.

The Go version lives at `/home/patrick/Desktop/coco-golang`.
Module: `github.com/patrick/cocobase`
Stack: Go 1.25 · Fiber v2 · GORM · PostgreSQL · Redis · Backblaze B2 (S3-compatible)

---

## What Has Been Built (Steps 1 & 2)

### Step 1 — Remove SaaS Limitations
- Deleted `internal/models/pricing.go` (Plan, ProjectPlan, Payment, ApiUsageCounter models)
- Deleted `internal/services/pricing.go` (GetCurrentPlan, GetDefaultFreePlan)
- Removed all storage limit checks from `internal/services/storage.go` and `internal/api/handlers/file.go`
- Removed Paystack keys from config
- Cleaned `pkg/config/config.go` — added open-source-relevant config fields:
  - SMTP settings (HOST, PORT, USERNAME, PASSWORD, FROM, FROM_NAME, SECURE)
  - OAuth providers (Google, GitHub, Apple)
  - Admin credentials (ADMIN_EMAIL, ADMIN_PASSWORD)
  - Optional rate limiting (RATE_LIMIT_REQUESTS, RATE_LIMIT_WINDOW)
  - Redis, Mailer, Frontend URL
- Created `.env.example` with all available options and comments

### Step 2 — Match Python API Routes & Behaviour

#### Auth Routes (`/auth-collections/*`)
All routes match the Python hosted version exactly:

| Method | Path | Handler |
|--------|------|---------|
| POST | /auth-collections/login | UserLogin |
| POST | /auth-collections/signup | UserSignup |
| GET  | /auth-collections/users | ListAllUsers |
| GET  | /auth-collections/users/:id | GetUserByID |
| GET  | /auth-collections/user | GetCurrentUser (JWT) |
| PATCH | /auth-collections/user | UpdateCurrentUser (JWT) |
| POST | /auth-collections/google-verify | VerifyGoogleToken |
| POST | /auth-collections/github-verify | VerifyGitHubToken |
| POST | /auth-collections/apple-verify | VerifyAppleToken |
| GET  | /auth-collections/login-google | LoginWithGoogle (redirect URL) |
| POST | /auth-collections/forgot-password | ForgotPassword |
| POST | /auth-collections/reset-password | ResetPassword |
| GET  | /auth-collections/reset-password-page | ResetPasswordPage (HTML form) |
| POST | /auth-collections/verify-email/send | SendVerificationEmail (JWT) |
| POST | /auth-collections/verify-email/verify | VerifyEmail |
| POST | /auth-collections/verify-email/resend | ResendVerificationEmail (JWT) |

#### Response Formats (match Python SDK exactly)
- login/signup/OAuth: `{ access_token, user }` where user = `{ id, email, data, client_id, created_at, roles, email_verified, email_verified_at }`
- GET /users: `{ data: [...], total, limit, offset, has_more }`
- forgot-password: `{ message: "If that email exists, a reset link has been sent" }`
- reset-password: `{ message: "Password successfully reset" }`
- verify-email/send: `{ message, expires_in_hours: 24 }`
- verify-email/verify: `{ message, email_verified: true, verified_at }`

#### OAuth Implementation
- **Google**: verifies `id_token` via `https://oauth2.googleapis.com/tokeninfo` OR `access_token` via userinfo endpoint. Config stored in `project_integrations` table (integration ID: `046deb41-47b3-403d-aee8-b80ccb80a87e`).
- **GitHub**: accepts `access_token` OR `code` (exchanges with GitHub). Falls back to `/user/emails` if email not public. Config in integrations (ID: `cee2caf5-647d-46b9-bd6b-9f0ed80e74fb`).
- **Apple**: verifies `id_token` by fetching Apple's RSA public keys from `https://appleid.apple.com/auth/keys` and validating JWT signature. Handles first-auth name.
- All OAuth: uses `findOrCreateOAuthUser()` — conflicts with password accounts return 409.

#### Document Routes
| Method | Path | Notes |
|--------|------|-------|
| POST | /collections/documents?collection=name | Legacy query-param style |
| POST | /collections/:id/documents | Standard |
| GET  | /collections/:id/documents | List with filtering/sorting/pagination |
| GET  | /collections/:id/documents/:docId | Get single |
| PATCH | /collections/:id/documents/:docId | Update (JSON or multipart) |
| DELETE | /collections/:id/documents/:docId | Delete |

**File upload behaviour** — matches Python exactly:
- `POST /collections/:id/documents` and `PATCH /collections/:id/documents/:docId` auto-detect `Content-Type: multipart/form-data`
- Any form field whose value is a file gets uploaded to S3 and its URL stored under that field name (e.g. `avatar: photo.jpg` → `{"avatar": "https://..."}`)
- Multiple files on same field name → stored as array of URLs
- `data` form field holds the rest of the document as a JSON string
- JSON body requests work unchanged

**Update supports array operations** (matches Python):
- `$append: { field: [items] }` — appends to array, no duplicates
- `$remove: { field: [items] }` — removes from array
- `override: true` — fully replaces data instead of merging

#### Batch Routes (match Python paths)
- `POST /collections/:id/batch/documents/create`
- `POST /collections/:id/batch/documents/update`
- `POST /collections/:id/batch/documents/delete`

#### Realtime Routes
| Method | Path | Notes |
|--------|------|-------|
| WS | /collections/:id/realtime | Document change stream, auth via first message |
| GET | /realtime/rooms | List active broadcast rooms |
| WS | /realtime/broadcast | Global broadcast |
| WS | /realtime/rooms/:room_id | Named room |
| WS | /notifications/global | Global peer-to-peer |
| WS | /notifications/channel/:channel | Channel-specific |
| POST | /notifications/send | HTTP broadcast trigger |

#### SMTP / Mailer (`internal/services/mailer.go`)
- `SendEmail(EmailMessage)` — sends via SMTP config from `.env`
- `SendPasswordResetEmail(to, token, frontendURL)` — called automatically on forgot-password
- `SendVerificationEmail(to, token, frontendURL)` — called automatically on verify-email/send
- Soft-fails (logs only) if SMTP not configured — nothing breaks
- **Dashboard config will override .env SMTP settings** (to be built in Step 3)

#### Other Changes
- `override` param on `PATCH /auth-collections/user` and `PATCH /collections/:id/documents/:docId` — fully replaces `data` instead of merging
- `GET /auth-collections/users` supports `limit` and `offset` query params

---

## Project File Structure

```
cmd/cocobase/main.go              — entry point
pkg/config/config.go              — all env config
internal/
  api/
    handlers/
      auth.go                     — all auth handlers (login, signup, OAuth, password reset, email verify)
      collections.go              — collection CRUD
      documents.go                — document CRUD (detects multipart)
      document_upload.go          — multipart file upload logic for create & update
      advanced.go                 — count, aggregate, group-by, schema, export
      batch.go                    — batch create/update/delete
      file.go                     — raw file upload/list/delete
      notifications.go            — WebSocket notifications + broadcast rooms list
      realtime.go                 — WebSocket collection subscriptions
      health.go, common.go
    middleware/
      auth.go                     — RequireAPIKey, RequireAppUser, GetProject, GetAppUser
      middleware.go               — CORS, logger, recover
      cache.go
    routes/
      auth.go                     — auth route registration
      collections.go              — collection + document route registration
      realtime.go                 — realtime + broadcast room routes
      notifications.go            — notification routes
      routes.go                   — root setup
  models/
    user.go                       — User, Project, AppUser, PasswordResetToken, EmailVerificationToken, TwoFactorCode, TwoFactorSettings
    collection.go                 — Collection, Document
    integration.go                — ProjectIntegration
    function.go, dto.go, methods.go
  services/
    jwt.go                        — CreateAppUserToken, DecodeAppUserToken
    mailer.go                     — SMTP email sending
    storage.go                    — S3/Backblaze file upload/list/delete
    realtime.go                   — Redis pub/sub, ConnectionManager, WebSocket subscriptions
    permissions.go                — collection access control
    query_builder.go              — advanced filtering/sorting
    relationship.go               — document population (foreign key resolution)
    concurrent_query.go
  database/
    connection.go                 — GORM + PostgreSQL setup
  dto/
    app_users.go                  — AppUserLoginRequest, AppUserSignupRequest, TokenResponse, AppUserResponse
```

---

## What Still Needs Doing (Before Dashboard)

- **2FA** — models exist (TwoFactorCode, TwoFactorSettings), routes not yet registered
- **User online status / last seen** — store in `app_user.data` on login and WebSocket connect
- **Message broker** — Redis pub/sub is wired but no persistent queue for missed events

---

## Step 3 — Admin Dashboard (Self-Host UI)

### What the dashboard needs to do
The dashboard is the UI that someone who self-hosts Cocobase uses to manage their instance.
It is NOT a client-facing product — it is the admin panel for the server operator.

Think of it like: Supabase Studio, PocketBase admin, or Appwrite console.

### Recommended approach: embed in the Go binary

Build the dashboard as a **React (or SvelteKit) SPA** and **embed the built assets directly into the Go binary** using `//go:embed`. This means:
- Single binary deployment — no separate frontend server
- Served at `/_/` (matching Python version's `/_/docs` and `/_/admin`)
- Zero extra infrastructure for self-hosters

### Dashboard pages needed

1. **Setup / Onboarding** — first-run wizard (create admin account, test DB connection, configure SMTP)
2. **Overview** — instance health, active connections, storage usage
3. **Projects** — list projects, create/delete, regenerate API keys, view configs
4. **Collections** — per-project collection browser, schema viewer, document explorer
5. **App Users** — per-project user list, view/edit/delete users
6. **File Storage** — browse uploaded files per project
7. **SMTP / Mailer config** — configure email settings (overrides .env), test connection, view email logs
8. **OAuth Integrations** — enable/configure Google, GitHub, Apple per project (writes to project_integrations table)
9. **Realtime Monitor** — live view of active WebSocket connections and broadcast rooms
10. **Settings** — instance-level settings (stored in DB, not .env)

### Dashboard config table (to be created)
Settings configured from dashboard should be stored in a `dashboard_configs` table:
```
id            uuid primary key
key           varchar unique       -- e.g. "smtp.host", "smtp.port", "app.name"
value         text
is_secret     boolean              -- mask in UI
updated_at    timestamp
```
Dashboard config takes priority over `.env` at runtime. This allows zero-downtime reconfiguration.

### Tech recommendation for dashboard UI

**Option A — React + Vite (embedded)**
- Familiar ecosystem, large component library choice (shadcn/ui works well)
- Build output embedded via `//go:embed dist/*`
- API calls to same origin, no CORS issues
- Con: adds a build step to the repo

**Option B — SvelteKit (embedded)**
- Smaller bundle, faster build
- Same embed approach
- Con: smaller ecosystem

**Option C — Templ + HTMX (Go-native)**
- No separate build step, everything in Go
- Templ generates type-safe HTML templates
- HTMX handles interactivity without a JS framework
- Pro: single language, simpler CI, smaller binary
- Con: less familiar, harder to build complex UI like a document explorer

**Recommendation: React + Vite + shadcn/ui**
Most developers know it, shadcn gives production-quality components fast, and the embed approach keeps deployment simple. Put the frontend in a `/dashboard` folder at the repo root.

### Dashboard API routes (to be built in Go)
All under `/_/api/` prefix, protected by admin JWT (separate from app user JWT):

```
POST  /_/api/auth/login           — admin login
GET   /_/api/projects             — list all projects
POST  /_/api/projects             — create project
GET   /_/api/projects/:id         — get project
PATCH /_/api/projects/:id         — update project
DELETE /_/api/projects/:id        — delete project
GET   /_/api/projects/:id/collections
GET   /_/api/projects/:id/users
GET   /_/api/projects/:id/files
GET   /_/api/projects/:id/integrations
POST  /_/api/projects/:id/integrations/:integrationId/enable
PATCH /_/api/projects/:id/integrations/:integrationId/config
GET   /_/api/config               — get dashboard configs
PATCH /_/api/config               — update dashboard configs
POST  /_/api/config/smtp/test     — test SMTP connection
GET   /_/api/realtime/stats       — all connection stats
GET   /_/api/health               — full instance health
```

### Serving the dashboard
In `main.go`, after routes are set up:
```go
//go:embed dashboard/dist/*
var dashboardFS embed.FS

app.Static("/_", "./dashboard/dist", fiber.Static{
    Index: "index.html",
})
// SPA fallback
app.Get("/_/*", func(c *fiber.Ctx) error {
    return c.SendFile("./dashboard/dist/index.html")
})
```

---

## Environment Variables Reference

```env
# Core
PORT=3000
ENVIRONMENT=development
DATABASE_URL=postgres://user:pass@localhost:5432/cocobase
SECRET=change-me-in-production
API_VERSION=v1

# Storage
BACKBLAZE_KEY_ID=
BACKBLAZE_KEY_NAME=
BACKBLAZE_APPLICATION_KEY=
BUCKET_NAME=
BUCKET_ENDPOINT=

# App
FRONTEND_URL=http://localhost:3000
REDIS_URL=redis://localhost:6379

# SMTP
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=
SMTP_FROM_NAME=Cocobase
SMTP_SECURE=false

# OAuth (per-project config preferred via integrations table)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
APPLE_CLIENT_ID=
APPLE_TEAM_ID=
APPLE_KEY_ID=
APPLE_PRIVATE_KEY=

# Admin dashboard
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=change-me

# Rate limiting (0 = unlimited)
RATE_LIMIT_REQUESTS=0
RATE_LIMIT_WINDOW=60
```

---

## Step 3 — Dashboard Build Plan (Decided & In Progress)

### Decision: React + Vite + shadcn/ui, embedded in Go binary

The dashboard lives at `dashboard/` in the repo root.
Built with `npm run build` → outputs to `dashboard/dist/`.
Go embeds `dashboard/dist/` at compile time via `//go:embed`.
Served at `/_/` so it never conflicts with the public API.

### Folder structure

```
dashboard/                        ← React app root
  index.html
  package.json
  vite.config.ts
  tailwind.config.ts
  tsconfig.json
  src/
    main.tsx                      ← React entry
    App.tsx                       ← Router root
    api/
      client.ts                   ← fetch wrapper (base URL /_/api, attaches admin JWT)
      projects.ts
      collections.ts
      users.ts
      config.ts
      health.ts
    components/
      ui/                         ← shadcn/ui primitives (Button, Input, Table, Dialog…)
      layout/
        Sidebar.tsx
        Header.tsx
        Shell.tsx                 ← wraps every authenticated page
    pages/
      Login.tsx                   ← /login — admin login form
      Setup.tsx                   ← /setup — first-run wizard (shown when no admin exists)
      Overview.tsx                ← / — health + stats
      Projects.tsx                ← /projects
      ProjectDetail.tsx           ← /projects/:id (collections, users, files, integrations tabs)
      CollectionDetail.tsx        ← /projects/:id/collections/:collId (document explorer)
      Settings.tsx                ← /settings (SMTP, instance name, etc.)
    store/
      auth.ts                     ← admin JWT storage (localStorage)
    hooks/
      useApi.ts                   ← SWR/tanstack-query wrapper
```

### Go dashboard API — all routes under `/_/api/`

Protected by a separate admin JWT (not the app-user JWT).
Admin credentials come from `ADMIN_EMAIL` + `ADMIN_PASSWORD` env, or from DB after first setup.

**Auth**
```
POST  /_/api/auth/login              body: {email, password} → {access_token}
GET   /_/api/auth/me                 → current admin info
```

**Projects**
```
GET   /_/api/projects                → [{id, name, api_key, created_at, user_id}]
POST  /_/api/projects                body: {name} → project
GET   /_/api/projects/:id            → project
PATCH /_/api/projects/:id            body: {name, allowed_origins, configs}
DELETE /_/api/projects/:id
POST  /_/api/projects/:id/regen-key  → {api_key}
```

**Collections (per project)**
```
GET   /_/api/projects/:id/collections          → [{id, name, created_at}]
POST  /_/api/projects/:id/collections          body: {name}
GET   /_/api/projects/:id/collections/:colId   → collection + schema
DELETE /_/api/projects/:id/collections/:colId
```

**Documents (per collection — for explorer)**
```
GET   /_/api/projects/:id/collections/:colId/documents   ?limit&offset&sort&order
GET   /_/api/projects/:id/collections/:colId/documents/:docId
PATCH /_/api/projects/:id/collections/:colId/documents/:docId
DELETE /_/api/projects/:id/collections/:colId/documents/:docId
```

**App Users (per project)**
```
GET   /_/api/projects/:id/users               ?limit&offset
GET   /_/api/projects/:id/users/:userId
PATCH /_/api/projects/:id/users/:userId
DELETE /_/api/projects/:id/users/:userId
DELETE /_/api/projects/:id/users              (delete all)
```

**Files (per project)**
```
GET   /_/api/projects/:id/files               ?subdirectory
DELETE /_/api/projects/:id/files              body: {filename, subdirectory}
```

**Integrations (OAuth config per project)**
```
GET   /_/api/projects/:id/integrations
POST  /_/api/projects/:id/integrations/:integrationId/enable   body: {config}
PATCH /_/api/projects/:id/integrations/:integrationId          body: {config, is_enabled}
DELETE /_/api/projects/:id/integrations/:integrationId
```

**Dashboard Config (instance-level, stored in DB)**
```
GET   /_/api/config                  → [{key, value (masked if secret), is_secret}]
PATCH /_/api/config                  body: [{key, value}]
POST  /_/api/config/smtp/test        → {success, message}
```

**Health & Realtime**
```
GET   /_/api/health                  → {database, redis, storage, uptime}
GET   /_/api/realtime/stats          → {collections: {id: count}, total_connections}
```

### DB table: `dashboard_configs`
```sql
CREATE TABLE dashboard_configs (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key        VARCHAR(255) UNIQUE NOT NULL,
  value      TEXT,
  is_secret  BOOLEAN NOT NULL DEFAULT false,
  updated_at TIMESTAMPTZ DEFAULT now()
);
```
Keys used:
- `smtp.host`, `smtp.port`, `smtp.username`, `smtp.password`, `smtp.from`, `smtp.from_name`, `smtp.secure`
- `app.name`, `app.logo_url`
- `admin.email` (set on first-run)

Dashboard config is loaded at startup and merged over `.env` values. SMTP from dashboard overrides `.env` SMTP.

### Go files to create for dashboard

```
internal/
  api/
    handlers/
      dashboard/
        auth.go          ← admin login, me
        projects.go      ← project CRUD + regen key
        collections.go   ← collection CRUD + document explorer
        users.go         ← app user management
        files.go         ← file browser
        integrations.go  ← OAuth integration config
        config.go        ← dashboard_configs CRUD + SMTP test
        health.go        ← full health check
    middleware/
      admin.go           ← RequireAdminJWT middleware
    routes/
      dashboard.go       ← registers all /_/api/* routes + serves embedded SPA
  models/
    dashboard.go         ← DashboardConfig model
  services/
    admin_jwt.go         ← CreateAdminToken, DecodeAdminToken (separate secret)
```

### Embed setup in main.go

```go
import "embed"

//go:embed dashboard/dist
var dashboardDist embed.FS

// In main(), after all API routes:
routes.SetupDashboardRoutes(app, dashboardDist)
```

`SetupDashboardRoutes` registers `/_/api/*` routes, then serves the SPA with an HTML fallback for `/_/*`.

### Dashboard UI pages detail

**Login (`/login`)**
- Email + password form
- POST `/_/api/auth/login` → stores JWT in localStorage
- Redirect to `/` on success
- If first run (no admin exists), redirect to `/setup`

**Setup (`/setup`)**
- Step 1: Create admin account (email + password)
- Step 2: Test DB connection (shown as already connected)
- Step 3: SMTP config (optional, can skip)
- On complete: redirect to `/`

**Overview (`/`)**
- Cards: DB status, Redis status, Storage status, uptime
- Active WebSocket connections count
- Recent projects list

**Projects (`/projects`)**
- Table: name, API key (masked, copy button), created date, actions
- Create project button → modal with name input
- Click row → ProjectDetail

**ProjectDetail (`/projects/:id`)**
- Tabs: Collections | Users | Files | Integrations | Settings
- Collections tab: list of collections, click → CollectionDetail
- Users tab: paginated user list, view/delete user, edit data
- Files tab: file list with URLs, delete
- Integrations tab: Google/GitHub/Apple cards, toggle enable, configure client ID/secret
- Settings tab: allowed origins, project name, delete project

**CollectionDetail (`/projects/:id/collections/:colId`)**
- Schema viewer (field names + types inferred from docs)
- Document table with pagination, sort, filter
- Click document → side panel with JSON editor
- Create / delete document

**Settings (`/settings`)**
- SMTP config form → PATCH `/_/api/config` + test button
- Instance name
- (Future: webhook secrets, rate limits)

### shadcn/ui components needed
- Button, Input, Label, Card, Table, Dialog, Sheet (side panel), Tabs, Badge, Switch, Textarea, Toast, Skeleton, Avatar, DropdownMenu, Separator

### State management
- Admin JWT: localStorage + React context
- Server state: TanStack Query (react-query) — simple, no Redux needed
- Forms: react-hook-form

### Build & development

**Dev (hot reload):**
```bash
# Terminal 1 — Go API
go run ./cmd/cocobase

# Terminal 2 — React dev server (proxies /_/api to :3000)
cd dashboard && npm run dev
```

Vite config proxies `/_/api` → `http://localhost:3000` in dev.

**Production build:**
```bash
cd dashboard && npm run build    # outputs to dashboard/dist/
go build ./cmd/cocobase          # embeds dashboard/dist/ into binary
```

### Implementation order
1. Go: `dashboard_configs` model + migration
2. Go: admin JWT service (`internal/services/admin_jwt.go`)
3. Go: all `/_/api/*` route handlers
4. Go: embed + serve SPA in `routes/dashboard.go`
5. React: scaffold Vite project, install shadcn/ui
6. React: Login page + auth context
7. React: Setup wizard
8. React: Projects list + create
9. React: ProjectDetail tabs (Collections, Users, Files, Integrations)
10. React: CollectionDetail (document explorer)
11. React: Settings page (SMTP)

---

## Notes for Next Developer / AI Session

- The Python version at `/home/patrick/Documents/COCOBASE/cocobase` is the reference implementation. When in doubt about a route's behaviour, check `app/api/auth_collection.py` and `app/api/collections/collections.py`.
- All public client-facing routes (`/auth-collections/*`, `/collections/*`, `/realtime/*`, `/notifications/*`) are already built and match the Python version.
- Dashboard routes (`/_/*`) do not exist yet — that is Step 3.
- The `project_integrations` table already exists. OAuth per-project config is read from there. Integration IDs: Google = `046deb41-47b3-403d-aee8-b80ccb80a87e`, GitHub = `cee2caf5-647d-46b9-bd6b-9f0ed80e74fb`.
- SMTP is configured via `.env` but should also be configurable from the dashboard via a `dashboard_configs` table (not yet created).
- Build command: `go build ./cmd/cocobase`
- The binary is self-contained — just needs `DATABASE_URL` and `SECRET` to run.
