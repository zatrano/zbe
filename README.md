# ZBE — ZATRANO Backend

Production-ready Go Fiber API server for the ZATRANO ecosystem.

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.21+ |
| Web Framework | Fiber v2 |
| Database | PostgreSQL 17 + GORM |
| Auth | JWT (access + refresh) + Google OAuth2 |
| Mail | SMTP via gomail |
| Logging | Uber Zap |
| Validation | go-playground/validator |

---

## Quick Start

### Option A — Docker (recommended)

```bash
# 1. Clone and enter the repo
cd zbe

# 2. Copy and configure environment
cp .env.example .env
# Edit .env — set JWT_SECRET at minimum

# 3. Start the full stack (Postgres + MailHog + ZBE)
docker-compose up -d

# 4. Watch logs
docker-compose logs -f zbe
```

Services available:
- **API**: http://localhost:8080
- **MailHog UI** (dev email): http://localhost:8025

---

### Option B — Local (requires PostgreSQL)

```bash
# 1. Prerequisites
#    - Go 1.21+  (https://go.dev/dl/)
#    - PostgreSQL 17 running locally

# 2. Copy environment file
cp .env.example .env

# 3. Edit .env — required fields:
#    JWT_SECRET=<at-least-32-char-random-string>
#    DB_HOST=localhost
#    DB_PASSWORD=<your-postgres-password>

# 4. Create the database
make db-create

# 5. Download dependencies
make deps

# 6. Build and run
make run
```

The server auto-runs migrations and seeds on every startup.

---

## Environment Variables

See [`.env.example`](.env.example) for the full list with documentation.

**Required in production:**

| Variable | Description |
|---|---|
| `JWT_SECRET` | Secret for signing JWTs — min 32 chars, keep it secret |
| `DATABASE_URL` or `DB_*` | PostgreSQL connection |
| `SMTP_*` | Mail server for verification/reset emails |
| `GOOGLE_CLIENT_ID` | For Google OAuth (optional) |
| `GOOGLE_CLIENT_SECRET` | For Google OAuth (optional) |

---

## API Endpoints

### Health
| Method | Path | Description |
|---|---|---|
| GET | `/health` | Liveness probe |
| GET | `/ready` | Readiness probe (checks DB) |

### Authentication
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/auth/register` | — | Register new user |
| POST | `/api/v1/auth/login` | — | Login, get tokens |
| POST | `/api/v1/auth/logout` | ✓ Bearer | Revoke access token |
| POST | `/api/v1/auth/refresh-token` | — | Issue new access token |
| GET  | `/api/v1/auth/verify-email?token=` | — | Verify email address |
| GET  | `/api/v1/auth/google/login` | — | Redirect to Google OAuth |
| GET  | `/api/v1/auth/google/callback` | — | Google OAuth callback |

### Password Reset
| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/password-reset/request` | Request reset email |
| POST | `/api/v1/password-reset/confirm` | Set new password |

### Profile (authenticated)
| Method | Path | Description |
|---|---|---|
| GET   | `/api/v1/profile` | Get own profile |
| PATCH | `/api/v1/profile` | Update name / avatar |
| POST  | `/api/v1/profile/change-password` | Change password |

### Users (admin only)
| Method | Path | Description |
|---|---|---|
| GET    | `/api/v1/users` | Paginated user list |
| POST   | `/api/v1/users` | Create user |
| GET    | `/api/v1/users/:id` | Get user |
| PUT    | `/api/v1/users/:id` | Update user |
| DELETE | `/api/v1/users/:id` | Soft-delete user |
| PUT    | `/api/v1/users/:id/roles` | Replace user roles |

### Roles (admin only)
| Method | Path | Description |
|---|---|---|
| GET    | `/api/v1/roles` | List all roles |
| POST   | `/api/v1/roles` | Create role |
| GET    | `/api/v1/roles/:id` | Get role |
| PUT    | `/api/v1/roles/:id` | Update role |
| DELETE | `/api/v1/roles/:id` | Delete role |

---

## Authentication Flow

```
POST /api/v1/auth/register
  → 201 { access_token, refresh_token, user }

POST /api/v1/auth/login
  → 200 { access_token, refresh_token, user }

# Use access_token in header:
Authorization: Bearer <access_token>

# Refresh when access token expires:
POST /api/v1/auth/refresh-token
  body: { "refresh_token": "..." }
  → 200 { access_token, refresh_token, user }
```

---

## Security Features

- **Passwords**: bcrypt (cost 12 default)
- **JWT**: HS256, short-lived access tokens (15m), rotating refresh tokens
- **Rate limiting**: 100 req/min general; 5 req/min on auth endpoints; 10 req/min on password reset
- **Token revocation**: revoked JTIs stored until expiry
- **CORS**: configurable allowed origins
- **Security headers**: X-Frame-Options, X-Content-Type-Options, etc.
- **Input validation**: go-playground/validator with HTML sanitization
- **SQL injection**: GORM parameterized queries only; sort field allowlist

---

## Default Seed Data

After first startup:

| Resource | Value |
|---|---|
| Admin email | `admin@zatrano.com` (or `SEED_ADMIN_EMAIL`) |
| Admin password | `Admin@123456` (or `SEED_ADMIN_PASSWORD`) — **change immediately!** |
| Roles | `admin`, `moderator`, `user` |

---

## Development

```bash
# Hot reload (requires air)
go install github.com/air-verse/air@latest
make dev

# Run tests
make test

# Run tests with coverage
make test-cover

# Lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
make lint

# Format
make fmt
```

---

## Project Structure

```
zbe/
├── cmd/server/main.go          # Entrypoint — wires everything together
├── config/config.go            # Env loading, typed config struct
├── internal/
│   ├── app/
│   │   ├── database.go         # GORM + connection pool setup
│   │   └── router.go           # Fiber app, middleware, route registration
│   ├── domain/
│   │   ├── user.go             # User, Role, OAuthProvider, RevokedToken models
│   │   ├── dto.go              # Request/response DTOs + pagination helpers
│   │   └── types.go            # Custom GORM types (Permissions JSONB)
│   ├── repository/             # GORM data access layer
│   ├── service/                # Business logic
│   ├── handler/                # HTTP handlers (thin — call services)
│   └── middleware/             # JWT auth, RBAC, rate limiter, logger
├── pkg/
│   ├── logger/                 # Zap logger singleton
│   ├── mail/                   # SMTP mail service + HTML templates
│   └── utils/                  # JWT, bcrypt, validation, response helpers
├── migrations/migrate.go       # GORM AutoMigrate
├── seed/seed.go                # Default roles + admin user
├── Makefile
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

---

## Production Checklist

- [ ] Set a strong `JWT_SECRET` (32+ random chars)
- [ ] Change `SEED_ADMIN_PASSWORD` before first run, then delete the env var
- [ ] Set `APP_ENV=production`
- [ ] Set `COOKIE_SECURE=true` (requires HTTPS)
- [ ] Configure real SMTP (`SMTP_HOST`, `SMTP_USER`, `SMTP_PASSWORD`)
- [ ] Set `DB_SSLMODE=require` for production Postgres
- [ ] Set `ALLOWED_ORIGINS` to your actual frontend domain only
- [ ] Run behind a reverse proxy (nginx/Caddy) with TLS termination
- [ ] Set up log aggregation (the server emits structured JSON logs)
- [ ] Configure `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` if using OAuth
