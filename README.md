# Cocobase

**Open-source, self-hosted Backend as a Service** — built with Go, Fiber, and GORM.

Cocobase gives your app a ready-made backend: user auth, schemaless collections, file storage, and an admin dashboard, all served from a single binary you control.

---

## Features

- **User authentication** — email/password signup & login, JWT tokens, OAuth (Google, GitHub, Apple), email verification, 2FA, password reset
- **Schemaless collections** — create any data structure, query with filters, sorting, and pagination
- **File uploads** — store files in Backblaze B2 (S3-compatible)
- **Admin dashboard** — manage projects, users, collections, and settings through a built-in web UI at `/_/`
- **Activity logs** — every admin action is recorded per project
- **SQLite or PostgreSQL** — use SQLite for simple self-hosted setups, PostgreSQL for production
- **Single binary** — the React dashboard is embedded in the Go binary; no Node.js needed in production

---

## Quick start

### 1. Get the binary

```bash
# Build from source (requires Go 1.21+ and CGO for SQLite)
git clone https://github.com/lordace-coder/coco-golang.git
cd coco-golang
go build -o cocobase ./cmd/cocobase/
```

### 2. Create a `.env` file

```env
# Required
DATABASE_URL=./cocobase.db   # SQLite file — or use a PostgreSQL URL
SECRET=change-me-to-a-long-random-string

# Optional
PORT=3000
ENVIRONMENT=production
```

See the full list of environment variables in the dashboard under **Environment** or in [the env reference below](#environment-variables).

### 3. Run

```bash
./cocobase
```

Open [http://localhost:3000/\_/](http://localhost:3000/_/) in your browser. On first run you will be prompted to create an admin account.

---

## Admin dashboard

The dashboard lives at `/_/` (the server's own URL with `/_/` appended). From there you can:

| Section | What you can do |
|---|---|
| **Projects** | Create projects, copy API keys, set allowed origins |
| **Settings** | Configure SMTP, allowed origins, 2FA, email verification |
| **Users** | Browse, create, and delete app users |
| **Collections** | Create collections and manage documents |
| **Files** | Browse and delete uploaded files |
| **Logs** | See a live audit trail of all admin actions |
| **Environment** | Full reference of every `.env` variable with examples |

---

## Using Cocobase from your app

Install the JavaScript SDK:

```bash
npm install cocobase
```

```js
import { Cocobase } from "cocobase";

const client = new Cocobase({
  baseURL: "https://your-cocobase-server.com",
  apiKey: "your_project_api_key",
});

// Authenticate a user
const { token } = await client.auth.login("user@example.com", "password");

// Read documents
const posts = await client.collection("posts").list();

// Create a document
await client.collection("posts").create({
  title: "Hello world",
  published: true,
});
```

---

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | **Yes** | PostgreSQL URL **or** a SQLite path (e.g. `./cocobase.db`) |
| `SECRET` | **Yes** | JWT signing secret — keep this private |
| `PORT` | No | HTTP port. Default: `3000` |
| `ENVIRONMENT` | No | `production` or `development`. Default: `development` |
| `SMTP_HOST` | No | SMTP server for emails |
| `SMTP_PORT` | No | SMTP port. Default: `587` |
| `SMTP_USERNAME` | No | SMTP login |
| `SMTP_PASSWORD` | No | SMTP password |
| `SMTP_FROM` | No | From address for outgoing emails |
| `SMTP_SECURE` | No | `true` for SSL (port 465), otherwise STARTTLS |
| `REDIS_URL` | No | Redis connection URL for real-time features |
| `BACKBLAZE_KEY_ID` | No | Backblaze B2 key ID for file storage |
| `BACKBLAZE_APPLICATION_KEY` | No | Backblaze B2 application key |
| `BUCKET_NAME` | No | B2 bucket name |
| `BUCKET_ENDPOINT` | No | B2 S3-compatible endpoint URL |
| `GOOGLE_CLIENT_ID` | No | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | No | Google OAuth client secret |
| `GITHUB_CLIENT_ID` | No | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | No | GitHub OAuth app client secret |
| `RATE_LIMIT_REQUESTS` | No | Max requests per window. Default: `0` (unlimited) |
| `RATE_LIMIT_WINDOW` | No | Rate limit window in seconds. Default: `60` |

The dashboard's **Environment** tab has a full reference with example values and a one-click copy button.

---

## API reference

Your project's base URL is the Cocobase server URL. All app-facing routes require an `x-api-key` header (your project's API key from the dashboard).

### Auth routes — `/auth-collections/*`

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth-collections/signup` | Register a new user |
| `POST` | `/auth-collections/login` | Login, returns JWT |
| `GET` | `/auth-collections/user` | Get current user (requires JWT) |
| `PATCH` | `/auth-collections/user` | Update current user |
| `POST` | `/auth-collections/forgot-password` | Send password reset email |
| `POST` | `/auth-collections/reset-password` | Reset password with token |
| `POST` | `/auth-collections/verify-email/send` | Send verification email |
| `POST` | `/auth-collections/verify-email/verify` | Verify email with token |
| `POST` | `/auth-collections/google-verify` | Login with Google token |
| `POST` | `/auth-collections/github-verify` | Login with GitHub token |

### Collection routes — `/api/v1/collections/*`

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/collections/:name/documents` | List documents (supports filtering, sorting, pagination) |
| `POST` | `/api/v1/collections/:name/documents` | Create a document (JSON or multipart with files) |
| `GET` | `/api/v1/collections/:name/documents/:id` | Get a document |
| `PATCH` | `/api/v1/collections/:name/documents/:id` | Update a document |
| `DELETE` | `/api/v1/collections/:name/documents/:id` | Delete a document |

#### Query parameters

| Param | Example | Description |
|---|---|---|
| `limit` | `?limit=20` | Number of results per page |
| `offset` | `?offset=20` | Skip N results |
| `sort` | `?sort=created_at` | Sort field |
| `order` | `?order=desc` | `asc` or `desc` |
| Any field | `?status=published` | Filter by field value |

---

## Deployment

### Railway / Render / Fly.io

Set environment variables in the platform's dashboard (no `.env` file needed). Make sure `DATABASE_URL` points to a managed PostgreSQL instance, or use SQLite with a persistent volume.

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY . .
RUN go build -o cocobase ./cmd/cocobase/

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/cocobase .
EXPOSE 3000
CMD ["./cocobase"]
```

### Building for production

```bash
# Linux (with SQLite CGO support)
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cocobase ./cmd/cocobase/

# Disable SQLite if you only use PostgreSQL (removes CGO requirement)
# Remove the gorm.io/driver/sqlite import first, then:
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cocobase ./cmd/cocobase/
```

---

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.21+ |
| HTTP | Fiber v2 |
| ORM | GORM |
| Database | PostgreSQL or SQLite |
| Dashboard | React + Vite + TanStack Query (embedded in binary) |
| Auth | JWT (golang-jwt) + bcrypt |
| File storage | Backblaze B2 (AWS S3-compatible SDK) |
| Cache | Redis (optional) |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
