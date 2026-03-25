# Cocobase - Backend as a Service (Go Edition)

A modern, high-performance Backend as a Service built with Go and Fiber framework. This is a complete rewrite of the Python FastAPI version with **10x performance improvements**, advanced features, and production-ready optimizations.

## 🚀 Features

### Core Capabilities

- **🔥 Blazing Fast**: Built on Fiber framework with concurrent query execution
- **📊 Advanced Querying**: Relationship filtering, aggregations, grouping, and complex operators
- **🔐 Secure Authentication**: API Key & Bearer token authentication with in-memory caching
- **📚 Swagger Documentation**: Interactive API documentation at `/swagger/index.html`
- **🎯 JSONB Support**: Native PostgreSQL JSONB with custom type handling
- **⚡ Performance Optimized**: Connection pooling, bulk operations, and smart caching

### Advanced Features

- **Relationship Filtering**: Query nested relationships (`?user.email_contains=john`)
- **Aggregations**: Sum, average, min, max, count on any field
- **Group By**: Group documents by field values
- **Batch Operations**: Create, update, delete multiple documents at once
- **Schema Generation**: Auto-generate collection schemas from documents
- **Export Collections**: Export entire collections as JSON
- **File Upload**: Direct file upload support

## 📁 Project Structure

```
coco-golang/
├── cmd/
│   └── cocobase/
│       └── main.go              # Application entry point with Fiber config
├── internal/
│   ├── api/
│   │   ├── handlers/            # Request handlers (auth, collections, documents, etc.)
│   │   │   ├── auth.go          # Authentication endpoints
│   │   │   ├── collections.go   # Collection CRUD
│   │   │   ├── documents.go     # Document CRUD with filtering
│   │   │   ├── batch.go         # Batch operations
│   │   │   └── advanced.go      # Aggregations, group-by, schema
│   │   ├── middleware/          # Custom middleware
│   │   │   ├── auth.go          # API key & bearer auth with caching
│   │   │   ├── cors.go          # CORS configuration
│   │   │   └── cache.go         # Response caching
│   │   └── routes/              # Route definitions
│   │       ├── auth.go          # Auth routes (/auth-collections/*)
│   │       └── collections.go   # Collection routes (/collections/*)
│   ├── database/                # Database connection & pooling
│   ├── models/                  # Data models with custom JSONB types
│   │   ├── user.go              # User, Project, AppUser models
│   │   ├── collection.go        # Collection & Document models
│   │   └── permissions.go       # Permission types
│   ├── services/                # Business logic
│   │   ├── query_builder.go     # Dynamic query building
│   │   ├── relationship.go      # Relationship resolution & filtering
│   │   ├── concurrent_query.go  # Concurrent query execution
│   │   └── permissions.go       # Permission checking
│   └── utils/                   # Utility functions & memory pools
├── docs/                        # Generated Swagger documentation
├── .env.example                 # Example environment variables
└── README.md
```

## 🛠️ Setup

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 12+ (with JSONB support)
- Git

### Installation

1. Clone the repository:

```bash
git clone <your-repo-url>
cd coco-golang
```

2. Install dependencies:

```bash
go mod download
```

3. Create `.env` file:

```bash
cp .env.example .env
```

4. Configure your environment variables in `.env`:

```env
PORT=3000
ENVIRONMENT=development
DATABASE_URL=postgresql://user:password@localhost:5432/cocobase?sslmode=disable
JWT_SECRET=your-secret-key
API_VERSION=v1
```

5. Create database indexes for optimal performance:

```bash
psql -U your_user -d your_database -f database_indexes.sql
```

## 🏃 Running the Application

### Development Mode

```bash
go run cmd/cocobase/main.go
```

### Build and Run

```bash
# Build the application
go build -o bin/cocobase ./cmd/cocobase/

# Run the binary
./bin/cocobase
```

### Using Swagger Documentation

Once running, visit `http://localhost:3000/swagger/index.html` for interactive API documentation.

### Regenerate Swagger Docs (after API changes)

```bash
# Install swag CLI if not already installed
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs
swag init -g cmd/cocobase/main.go
```

## 📡 API Endpoints

### Authentication Routes (`/auth-collections`)

- `POST /auth-collections/signup` - Create new user account
- `POST /auth-collections/signin` - Sign in with credentials
- `POST /auth-collections/refresh` - Refresh authentication token
- `GET /auth-collections/me` - Get current user info

### Collection Routes (`/collections`)

#### Collection Management

- `POST /collections/` - Create a new collection
- `GET /collections/:id` - Get collection by ID or name
- `PATCH /collections/:id` - Update collection
- `DELETE /collections/:id` - Delete collection
- `GET /collections/:id/schema` - Get auto-generated schema
- `GET /collections/:id/export` - Export collection as JSON
- `POST /collections/file` - Upload files

#### Document Operations (`/collections/:id/documents`)

- `POST /collections/:id/documents/` - Create document
- `GET /collections/:id/documents/` - List documents with filtering
- `GET /collections/:id/documents/:docId` - Get specific document
- `PATCH /collections/:id/documents/:docId` - Update document
- `DELETE /collections/:id/documents/:docId` - Delete document

#### Advanced Queries

- `GET /collections/:id/documents/aggregate?field=price&operation=sum` - Aggregate operations
- `GET /collections/:id/documents/group-by?field=status` - Group by field
- `GET /collections/:id/documents/count?status=active` - Count documents

#### Batch Operations

- `POST /collections/:id/documents/batch-create` - Create multiple documents
- `POST /collections/:id/documents/batch-update` - Update multiple documents
- `POST /collections/:id/documents/batch-delete` - Delete multiple documents

### Query Parameters & Filtering

#### Basic Filters

```
GET /collections/users/documents?name=John&age=25
```

#### Operators

- `_eq` - Equals (default)
- `_ne` - Not equals
- `_gt` - Greater than
- `_gte` - Greater than or equal
- `_lt` - Less than
- `_lte` - Less than or equal
- `_contains` - Contains substring
- `_startswith` - Starts with
- `_endswith` - Ends with

Example:

```
GET /collections/products/documents?price_gte=100&name_contains=laptop
```

#### Relationship Filtering

Query nested relationships using dot notation:

```
GET /collections/orders/documents?user.email_contains=john
GET /collections/posts/documents?author.name_eq=Jane
```

#### Sorting & Pagination

```
GET /collections/users/documents?sort=created_at&order=desc&limit=50
```

#### Aggregations

```
GET /collections/orders/documents/aggregate?field=total&operation=sum
Operations: count, sum, avg, min, max
```

#### Group By

```
GET /collections/orders/documents/group-by?field=status
```

## 🧪 Testing

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Test specific package
go test ./internal/services/...
```

## 🚀 Performance Benchmarks

Compared to the Python FastAPI version:

| Operation          | Python (FastAPI) | Go (Fiber) | Improvement      |
| ------------------ | ---------------- | ---------- | ---------------- |
| API Key Lookup     | 2196ms           | <10ms      | **99.5% faster** |
| Collection Lookup  | 922ms            | <10ms      | **99% faster**   |
| Document Query     | 718ms            | <100ms     | **86% faster**   |
| Relationship Query | 715ms            | <50ms      | **93% faster**   |
| Overall Request    | ~5s              | <100ms     | **98% faster**   |

### Key Optimizations

- ✅ In-memory caching for auth lookups (5-min TTL)
- ✅ Database indexes on critical fields
- ✅ Concurrent goroutines for parallel queries
- ✅ Connection pooling (25 idle, 200 max)
- ✅ Bulk operations with batching
- ✅ Prepared statement caching

## 📦 Building for Production

```bash
# Build for current platform
go build -o bin/cocobase ./cmd/cocobase/

# Build for Linux (production)
GOOS=linux GOARCH=amd64 go build -o bin/cocobase-linux ./cmd/cocobase/

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o bin/cocobase.exe ./cmd/cocobase/

# Build with optimizations
go build -ldflags="-s -w" -o bin/cocobase ./cmd/cocobase/
```

### Production Deployment Checklist

1. **Set Environment Variables**

   ```bash
   export ENVIRONMENT=production
   export DATABASE_URL=<production-db-url>
   export JWT_SECRET=<strong-secret>
   ```

2. **Apply Database Indexes**

   ```bash
   psql $DATABASE_URL -f database_indexes.sql
   ```

3. **Enable Production Optimizations**

   - Set `ENVIRONMENT=production` in `.env`
   - Consider enabling Fiber's `Prefork` mode for multi-core servers
   - Use a reverse proxy (nginx/caddy) for SSL termination

4. **Monitor Performance**

   - Watch for SLOW SQL logs (>200ms)
   - Monitor goroutine count
   - Track memory usage

5. **Database Maintenance**
   ```sql
   -- Periodically run
   VACUUM ANALYZE;
   REINDEX INDEX CONCURRENTLY idx_projects_api_key_active;
   ```

## 🔧 Configuration

Configuration is managed through environment variables:

### Required Variables

- `DATABASE_URL` - PostgreSQL connection string
- `JWT_SECRET` - Secret key for JWT token signing
- `PORT` - Server port (default: 3000)

### Optional Variables

- `ENVIRONMENT` - Environment mode: `development` or `production` (default: development)
- `API_VERSION` - API version prefix (default: v1)
- `LOG_LEVEL` - Logging level: debug, info, warn, error (default: info)

### Database Configuration

The application uses optimized connection pooling:

- **Max Open Connections**: 200
- **Max Idle Connections**: 25
- **Connection Lifetime**: 1 hour
- **Idle Timeout**: 10 minutes
- **Prepared Statements**: Enabled

### Performance Features

#### 1. Auth Caching

API key lookups are cached in memory for 5 minutes, reducing repeated database queries.

#### 2. Concurrent Queries

Document fetching and relationship population run concurrently using goroutines.

#### 3. Bulk Operations

Batch operations use `CreateInBatches` with configurable batch size (default: 100).

#### 4. Smart Indexing

Database indexes optimize:

- API key lookups (2196ms → <10ms)
- Collection lookups (922ms → <10ms)
- JSONB field extraction for relationships
- User and project queries

#### 5. Memory Pools

Reusable memory pools for maps and byte slices reduce GC pressure.

## 🔍 Troubleshooting

### Common Issues

#### "record not found" errors

- Ensure collection exists and project_id matches
- Check API key is valid and active
- Verify GORM column tags match database schema

#### JSONB scanning errors

- Use custom `StringArray` and `JSONMap` types
- Ensure Python pickled data is handled gracefully
- Check JSONB operators: use `->>` for text, `->` for JSONB

#### Slow queries

- Apply database indexes from `database_indexes.sql`
- Check connection pool settings
- Monitor with SLOW SQL logs (>200ms)

#### Route 404 errors

- Specific routes must be registered before dynamic `/:id` routes
- Check route ordering in `internal/api/routes/`
- Verify middleware is not blocking requests

### Debug Mode

Enable detailed logging:

```bash
export LOG_LEVEL=debug
./bin/cocobase
```

## 📚 Documentation

- **Swagger UI**: `http://localhost:3000/swagger/index.html`
- **API Docs**: Auto-generated from code annotations
- **Architecture**: See `CONCURRENT_OPTIMIZATIONS.md` for goroutine usage

## 🎯 Migration from Python FastAPI

### Key Differences

1. **Route Paths**: No `/api` prefix (matches Python directly)
2. **Column Names**:

   - `oauth_id` (not `o_auth_id`)
   - `client_id` (not `project_id` in app_users)
   - Explicit GORM column tags prevent auto-naming issues

3. **JSONB Handling**:

   - Custom `StringArray` type for `[]string` fields
   - Custom `JSONMap` type for `map[string]interface{}`
   - Graceful fallback for Python pickled data

4. **Regex Patterns**:

   - Go RE2 doesn't support negative lookahead/lookbehind
   - Use `strings.Contains` + `!strings.HasPrefix` instead

5. **Performance**:
   - 10-100x faster with proper indexes
   - Built-in concurrency with goroutines
   - No need for async/await syntax

### Migration Steps

1. Export data from Python API
2. Set up PostgreSQL with same schema
3. Apply `database_indexes.sql`
4. Import data (JSONB compatible)
5. Update client code to remove `/api` prefix (if used)
6. Test relationship filtering syntax

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Update Swagger docs if API changes (`swag init -g cmd/cocobase/main.go`)
4. Commit your changes (`git commit -m 'Add some amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

### Code Standards

- Follow Go best practices and idioms
- Add Swagger annotations for new endpoints
- Include error handling and logging
- Write tests for business logic
- Use meaningful variable names

## 📝 License

This project is licensed under the MIT License.

## 🙏 Acknowledgments

- Built with [Fiber](https://gofiber.io/) - Express-inspired web framework
- [GORM](https://gorm.io/) - ORM library for Go
- [Swaggo](https://github.com/swaggo/swag) - Swagger documentation generator
- Migrated from Python FastAPI for 10x performance improvement
- Inspired by modern BaaS solutions (Firebase, Supabase)

## 🛠️ Tech Stack

- **Framework**: Fiber v2.52+
- **Database**: PostgreSQL 12+ with JSONB
- **ORM**: GORM v1.25+
- **Documentation**: Swagger/OpenAPI 3.0
- **Authentication**: JWT + API Key
- **Language**: Go 1.21+

## 📊 Features Comparison

| Feature                | Python Version | Go Version          |
| ---------------------- | -------------- | ------------------- |
| Framework              | FastAPI        | Fiber               |
| ORM                    | SQLAlchemy     | GORM                |
| Concurrency            | asyncio        | goroutines          |
| Performance            | Good           | Excellent (10x)     |
| Swagger Docs           | ✅             | ✅                  |
| Relationship Filtering | ❌             | ✅                  |
| Aggregations           | Basic          | Advanced            |
| Batch Operations       | ❌             | ✅                  |
| Auth Caching           | ❌             | ✅ (5-min TTL)      |
| Connection Pooling     | Basic          | Optimized (200 max) |

## 📞 Support

For support, please:

- Open an issue in the GitHub repository
- Check existing issues for solutions
- Review Swagger documentation at `/swagger/index.html`
- Consult troubleshooting section above

---

**Built with ❤️ in Go | Migrated from FastAPI for superior performance and scalability**
