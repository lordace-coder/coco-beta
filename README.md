# Cocobase

**Open-source, self-hosted Backend as a Service** — built with Go, Fiber, and GORM.

Cocobase gives your app a ready-made backend: user auth, schemaless collections, file storage, and an admin dashboard, all served from a single binary you control.

> **Want managed hosting, cloud functions, real-time sync, and more?**
> Check out the cloud version at [cocobase.buzz](https://cocobase.buzz) — full docs at [docs.cocobase.buzz](https://docs.cocobase.buzz).

---

## Self-hosted vs Cloud

| | Self-hosted (this repo) | Cloud ([cocobase.buzz](https://cocobase.buzz)) |
|---|---|---|
| Hosting | Your server | Managed, 200+ edge nodes |
| Database | PostgreSQL or SQLite | Managed, auto-scaling |
| Auth | Email, OAuth, 2FA | Email, OAuth, 2FA + more providers |
| File storage | Backblaze B2 | Built-in CDN, 145GB+ per project |
| Real-time | — | WebSocket subscriptions |
| Cloud functions | — | Serverless, with email & queue support |
| Admin dashboard | Built-in | Built-in |
| Price | Free (host it yourself) | Free tier + paid plans |

---

## Features (self-hosted)

- **User authentication** — email/password, JWT tokens, OAuth (Google, GitHub, Apple), email verification, 2FA, password reset
- **Schemaless collections** — create any data structure, query with filters, sorting, and pagination
- **File uploads** — store files in Backblaze B2 (S3-compatible)
- **Admin dashboard** — manage projects, users, collections, and settings through a built-in web UI at `/_/`
- **Activity logs** — every admin action is recorded per project
- **SQLite or PostgreSQL** — use SQLite for simple setups, PostgreSQL for production
- **Single binary** — React dashboard embedded in the Go binary; no Node.js needed in production
- **CLI tools** — reset passwords, wipe data, list projects from the terminal

---

## Quick start

### 1. Get the binary

```bash
git clone https://github.com/lordace-coder/coco-golang.git
cd coco-golang
go build -o cocobase ./cmd/cocobase/
```

### 2. Create a `.env` file

```env
# Required
DATABASE_URL=./cocobase.db        # SQLite, or a PostgreSQL URL
SECRET=change-me-to-a-long-random-string

# Optional
PORT=3000
ENVIRONMENT=production
```

> Running the binary with no `.env` automatically creates a `.env.example` with all available variables.

### 3. Run

```bash
./cocobase
```

Open [http://localhost:3000/\_/](http://localhost:3000/_/) — on first run you'll be prompted to create an admin account.

---

## CLI commands

```bash
cocobase                    # start the server
cocobase serve -port 8080   # start with a port override
cocobase list-projects      # print all projects and API keys
cocobase reset-password     # interactively reset the admin password
cocobase wipe-project       # delete all data for a specific project
cocobase wipe-all           # delete all projects and data (keeps admin)
cocobase help               # show usage
```

---

## SDKs

Install the official SDK for your platform — works with both self-hosted and the cloud version.

| Platform | Install | Docs |
|---|---|---|
| JavaScript / TypeScript | `npm install cocobase` | [docs.cocobase.buzz](https://docs.cocobase.buzz) |
| Python | `pip install cocobase` | [Python guide](https://docs.cocobase.buzz/Python-Guide/authentication) |
| Flutter / Dart | `flutter pub add cocobase` | [Dart guide](https://docs.cocobase.buzz/Dart%20Guide/introduction) |
| Go | `go get github.com/cocobase-team/cocobase-go` | [docs.cocobase.buzz](https://docs.cocobase.buzz) |

### JavaScript / TypeScript example

```bash
npm install cocobase
```

```ts
import { Cocobase } from "cocobase";

const client = new Cocobase({
  baseURL: "https://your-cocobase-server.com", // or https://api.cocobase.buzz for cloud
  apiKey: "your_project_api_key",
});

// Sign up a user
await client.auth.signup("user@example.com", "Password123");

// Login
const { token } = await client.auth.login("user@example.com", "Password123");

// Create a document
await client.collection("posts").create({
  title: "Hello world",
  published: true,
});

// List documents
const posts = await client.collection("posts").list({ limit: 20 });
```

### Python example

```bash
pip install cocobase
```

```python
from cocobase import Cocobase

client = Cocobase(
    base_url="https://your-cocobase-server.com",
    api_key="your_project_api_key"
)

# Create a document
client.collection("posts").create({"title": "Hello", "published": True})

# Query with filters
posts = client.collection("posts").list(filters={"published": True}, limit=20)
```

### Flutter / Dart example

```bash
flutter pub add cocobase
```

```dart
import 'package:cocobase/cocobase.dart';

final client = Cocobase(
  baseURL: 'https://your-cocobase-server.com',
  apiKey: 'your_project_api_key',
);

// Login
await client.auth.login('user@example.com', 'Password123');

// Create a document
await client.collection('posts').create({'title': 'Hello', 'published': true});
```

---

## API reference

All app-facing routes require an `x-api-key` header (your project's API key from the dashboard).

### Auth — `/auth-collections/*`

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

### Collections — `/api/v1/collections/*`

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/collections/:name/documents` | List documents |
| `POST` | `/api/v1/collections/:name/documents` | Create a document |
| `GET` | `/api/v1/collections/:name/documents/:id` | Get a document |
| `PATCH` | `/api/v1/collections/:name/documents/:id` | Update a document |
| `DELETE` | `/api/v1/collections/:name/documents/:id` | Delete a document |

#### Query parameters

| Param | Example | Description |
|---|---|---|
| `limit` | `?limit=20` | Results per page (max 100) |
| `offset` | `?offset=20` | Skip N results |
| `sort` | `?sort=created_at` | Sort field |
| `order` | `?order=desc` | `asc` or `desc` |
| Any field | `?status=published` | Filter by exact value |
| `_contains` | `?title_contains=hello` | Substring match |
| `_gt` / `_lt` | `?price_gt=100` | Greater / less than |
| `_in` | `?status_in=draft,review` | Match any of |

---

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | **Yes** | PostgreSQL URL or SQLite path (e.g. `./cocobase.db`) |
| `SECRET` | **Yes** | JWT signing secret |
| `PORT` | No | HTTP port. Default: `3000` |
| `ENVIRONMENT` | No | `production` or `development` |
| `SMTP_HOST` | No | SMTP server for emails |
| `SMTP_PORT` | No | SMTP port. Default: `587` |
| `SMTP_USERNAME` | No | SMTP login |
| `SMTP_PASSWORD` | No | SMTP password |
| `SMTP_FROM` | No | From address for outgoing emails |
| `SMTP_SECURE` | No | `true` for SSL (port 465) |
| `REDIS_URL` | No | Redis URL for real-time features |
| `BACKBLAZE_KEY_ID` | No | Backblaze B2 key ID |
| `BACKBLAZE_APPLICATION_KEY` | No | Backblaze B2 application key |
| `BUCKET_NAME` | No | B2 bucket name |
| `BUCKET_ENDPOINT` | No | B2 S3-compatible endpoint |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | No | Google OAuth |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` | No | GitHub OAuth |
| `RATE_LIMIT_REQUESTS` | No | Max requests per window. Default: `0` (unlimited) |
| `RATE_LIMIT_WINDOW` | No | Window in seconds. Default: `60` |

Full reference with examples is also available in the dashboard under the **Environment** tab.

---

## Deployment

### Docker

No need to clone the repo. Create a `Dockerfile` anywhere with this content and Docker will pull the source directly from GitHub:

```dockerfile
FROM golang:1.21-alpine AS builder

# CGO is required for SQLite support
RUN apk add --no-cache gcc musl-dev git

# Pull source directly from GitHub
RUN git clone https://github.com/lordace-coder/coco-beta.git /app

WORKDIR /app

RUN go mod download

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o cocobase ./cmd/cocobase/

# ── Runtime image ──────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/cocobase .

# Mount a volume here to persist SQLite data across restarts
RUN mkdir -p /data

EXPOSE 3000

CMD ["./cocobase"]
```

Then build and run:

```bash
# Build
docker build -t cocobase .

# Run with SQLite (data persisted in a Docker volume)
docker run -d \
  --name cocobase \
  -p 3000:3000 \
  -v cocobase_data:/data \
  -e DATABASE_URL=/data/cocobase.db \
  -e SECRET=your-long-random-secret \
  -e ENVIRONMENT=production \
  cocobase

# Open the dashboard
# http://localhost:3000/_/
```

To use PostgreSQL instead of SQLite, swap the `DATABASE_URL`:

```bash
docker run -d \
  --name cocobase \
  -p 3000:3000 \
  -e DATABASE_URL=postgresql://user:pass@your-db-host:5432/cocobase \
  -e SECRET=your-long-random-secret \
  -e ENVIRONMENT=production \
  cocobase
```

### Railway / Render / Fly.io

Point the platform at `https://github.com/lordace-coder/coco-beta` and set these environment variables in the platform dashboard:

```
DATABASE_URL=postgresql://...   (use the platform's managed DB)
SECRET=your-long-random-secret
ENVIRONMENT=production
PORT=3000
```

The platform will build and deploy automatically on every push.

### Build from source

```bash
git clone https://github.com/lordace-coder/coco-beta.git
cd coco-beta
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cocobase ./cmd/cocobase/
```

---

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.21+ |
| HTTP | Fiber v2 |
| ORM | GORM |
| Database | PostgreSQL or SQLite |
| Dashboard | React + Vite + TanStack Query (embedded) |
| Auth | JWT + bcrypt |
| File storage | Backblaze B2 (S3 SDK) |
| Cache | Redis (optional) |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT

## Links

- Self-hosted repo: [github.com/lordace-coder/coco-beta](https://github.com/lordace-coder/coco-beta)
- Cloud platform: [cocobase.buzz](https://cocobase.buzz)
- Full documentation: [docs.cocobase.buzz](https://docs.cocobase.buzz)
- JS SDK: [github.com/lordace-coder/coco_base_js](https://github.com/lordace-coder/coco_base_js)
- Python SDK: [github.com/lordace-coder/coco_base_py](https://github.com/lordace-coder/coco_base_py)
- Flutter SDK: [github.com/lordace-coder/coco_base_flutter](https://github.com/lordace-coder/coco_base_flutter)
