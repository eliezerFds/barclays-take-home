# Eagle Bank API

A REST API for Eagle Bank built in Go, conforming to the provided OpenAPI specification.

## Tech Stack

- **Language:** Go
- **HTTP framework:** [Huma v2](https://github.com/danielgtaylor/huma) with stdlib `net/http`
- **Database:** SQLite (in-memory)
- **Migrations:** [sql-migrate](https://github.com/rubenv/sql-migrate)
- **Auth:** JWT (HS256) via [golang-jwt](https://github.com/golang-jwt/jwt)

## Project Structure

```
barclays/
├── main.go                          # Entry point — wires DB, migrations, storage, server
├── openapi.yaml                     # OpenAPI specification (updated as endpoints are added)
├── database/
│   └── migrations/
│       ├── migrations.go            # Embeds all *.sql files for sql-migrate
│       ├── 01_create_users_table.sql
│       ├── 02_create_accounts_table.sql
│       └── 03_create_transactions_table.sql
└── internal/
    ├── server/
    │   ├── server.go                # Server setup, route registration, auth middleware
    │   ├── auth.go                  # Login handler, JWT helpers, password helpers
    │   ├── user.go                  # User handlers and request/response types
    │   ├── account.go               # Account handlers (to be implemented)
    │   └── transaction.go           # Transaction handlers (to be implemented)
    └── storage/
        └── storage.go               # Database layer — all SQL queries and types
```

## Running the Server

```bash
go run main.go
```

The server starts on port `8080` by default. Override with the `SERVER_PORT` environment variable:

```bash
SERVER_PORT=3000 go run main.go
```

## Running Tests

```bash
go test ./...
```

Tests use an in-memory SQLite database spun up per test — no external dependencies required.

## Environment Variables

| Variable      | Default        | Description                        |
|---------------|----------------|------------------------------------|
| `SERVER_PORT` | `8080`         | Port the server listens on         |
| `JWT_SECRET`  | `dev-secret`   | Secret used to sign JWT tokens     |

> Set `JWT_SECRET` to a strong random value in any non-local environment.

---

## Authentication

Eagle Bank uses **JWT bearer token** authentication.

### How it works

1. Create a user via `POST /v1/users` (no token required)
2. Login via `POST /v1/auth/login` with your email and password
3. The response contains a token — include it as a header on all subsequent requests:
   ```
   Authorization: Bearer <token>
   ```

Tokens are signed with HS256 and expire after **24 hours**.

### Login

```
POST /v1/auth/login
```

**Request:**
```json
{
  "email": "john@example.com",
  "password": "password123"
}
```

**Response `200`:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Error responses:**
- `400` — missing email or password
- `401` — invalid credentials (wrong email or password)
- `500` — unexpected server error

### Protected vs Public routes

| Route                    | Auth required |
|--------------------------|---------------|
| `POST /v1/users`         | No            |
| `POST /v1/auth/login`    | No            |
| All other routes         | Yes           |

Requests to protected routes without a valid token return `401 Unauthorized`.

### Authorisation

Beyond authentication, each protected endpoint enforces **ownership**. A user can only access or modify their own resources — attempting to access another user's data returns `403 Forbidden`.

---

## Implemented Endpoints

### Users

| Method   | Path                    | Auth | Description            |
|----------|-------------------------|------|------------------------|
| `POST`   | `/v1/users`             | No   | Create a new user      |
| `POST`   | `/v1/auth/login`        | No   | Authenticate a user    |
| `GET`    | `/v1/users/{userId}`    | Yes  | Fetch a user by ID     |

### Decisions and Deviations from the Spec

- **`password` added to `POST /v1/users`** — the original spec did not include a password field, but one is required for authentication to work. The OpenAPI spec has been updated accordingly with a minimum length of 8 characters.
- **Validation errors return `400`** — Huma returns `422` for validation errors by default; this has been overridden to return `400 Bad Request` to match the spec.
- **Fetch user checks existence before ownership** — `GET /v1/users/{userId}` looks up the user in the DB first. A non-existent ID returns `404` regardless of who is asking. Only then is the `403` ownership check applied. This matches the spec scenarios and avoids ambiguity between "doesn't exist" and "you can't access it".
