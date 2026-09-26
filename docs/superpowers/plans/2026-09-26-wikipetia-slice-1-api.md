# WikiPetia Slice 1 — API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the WikiPetia slice 1 API — course authoring, self-study enrolment and consumption, and per-topic mastery — as a tested Go service over Postgres, with an OpenAPI spec the web plan consumes.

**Architecture:** One Go binary. HTTP handlers in `cmd/api` translate requests into calls on domain packages under `internal/`, which never import `net/http`. All SQL lives in `db/queries` as sqlc sources. State changes and the events that describe them are written in one transaction (transactional outbox), so the slice-4 engagement consumer can never disagree with the learning data.

**Tech Stack:** Go 1.25.1, huma v2.39.1, pgx v5.11.0, sqlc v1.31.1, golang-migrate v4.20.1, testify v1.12.1, google/uuid v1.6.0, PostgreSQL 18 in Docker.

**Spec:** `docs/superpowers/specs/2026-09-26-wikipetia-slice-1-design.md`

**Companion plan:** the web front end is a separate plan, written after this one. This plan's deliverable is a complete, tested API plus a generated `api/openapi.yaml`.

## Global Constraints

- Go module path: `github.com/Fikhri02/wikipetia`, Go **1.25.1**.
- Pinned versions, all verified to exist: huma **v2.39.1**, pgx **v5.11.0**, golang-migrate **v4.20.1**, sqlc **v1.31.1**, testify **v1.12.1**, google/uuid **v1.6.0**, Docker image **postgres:18**.
- **No domain package under `internal/` may import `net/http`.** Handlers own HTTP.
- **No learning package (`course`, `enrolment`, `progress`, `srs`) may import `internal/engagement`.** Enforced by CI in Task 19.
- **`block_progress.score` is an integer 0–100.** `blocks.completion.threshold` is compared in the same units. **`topic_mastery.strength` is a float 0–1.** Never mix.
- **Nothing is hard-deleted.** Every read filters `archived_at IS NULL`.
- **Quiz answer keys never appear in any learner-facing response.** Task 18 asserts this.
- Errors are `application/problem+json`. Verified huma shape: `{$schema, title, status, detail, errors:[{message, location, value}]}` — `location`, not `field`. **Shape-validation failures return 422**, not 400.
- sqlc type mapping, **verified by generating and compiling, not assumed**:
  - `uuid → uuid.UUID`, `jsonb → json.RawMessage`, `double precision → float64`, `uuid[] → []uuid.UUID`
  - NOT NULL `timestamptz → time.Time`
  - **nullable `timestamptz → pgtype.Timestamptz`**, **nullable `uuid → pgtype.UUID`** — the string-form override covers only the non-null case, so nullable uuids do NOT become `*uuid.UUID`. Convert at the repository boundary with the `pgconv` helpers from Task 3.
  - nullable `text → *string`, nullable `integer → *int32`
  - a `COALESCE(x, 0)` column is inferred NOT NULL, so it maps to `int32`, not `*int32`
  - `LEFT JOIN`ed NOT NULL columns become pointers (`text → *string`), because the join can produce no row
- Every endpoint gets an authorization test in which a second user is rejected.
- Commit after every task. Branch: `feature/slice-1-api`. Never commit to `main`.

### Toolchain notes (verified on this machine, 2026-09-26)

- `sqlc` and `migrate` are **not** on PATH by default; Task 1 installs them to `$(go env GOPATH)/bin`.
- Local `psql` is version 16 against a server of 18. Always use `docker compose exec db psql` so the client matches the server.
- **Do not put `emit_pointers_for_null_types` together with an object-form (`pointer: true`) `go_type` override, and do not mix string-form and object-form overrides for the same type.** Both produce duplicate imports or `**T` double pointers, and the generated package then fails to compile. The exact working `sqlc.yaml` is given verbatim in Task 3.

---

## File Structure

| Path | Responsibility |
|---|---|
| `api/cmd/api/main.go` | Wiring: config, pool, router, route registration, server |
| `api/cmd/api/handlers_*.go` | One file per resource group; HTTP ↔ domain translation only |
| `api/cmd/api/errors.go` | The single place sentinel errors become status codes |
| `api/internal/platform/config/config.go` | Env parsing |
| `api/internal/platform/db/db.go` | Pool construction, `WithTx` helper |
| `api/internal/platform/db/pgtest/pgtest.go` | Per-test transaction harness |
| `api/internal/platform/sqlcgen/` | Generated. Never hand-edited |
| `api/internal/platform/auth/auth.go` | `Identity` interface + dev stub |
| `api/internal/platform/apperr/apperr.go` | Sentinel error types |
| `api/internal/platform/rank/rank.go` | Lexicographic `sort_key` generation |
| `api/internal/course/` | Courses, topics, modules, blocks, questions, outline |
| `api/internal/enrolment/` | Enrolments |
| `api/internal/progress/` | Completion evaluators, quiz grading, mastery, player read |
| `api/internal/events/` | Append-only event log writer |
| `api/db/migrations/` | golang-migrate SQL |
| `api/db/queries/` | sqlc sources |

---

### Task 1: Repo skeleton, Docker Postgres, health endpoint

**Files:**
- Create: `api/go.mod`, `api/cmd/api/main.go`, `api/cmd/api/main_test.go`
- Create: `api/internal/platform/config/config.go`
- Create: `docker-compose.yml`, `Makefile`, `.gitignore`

**Interfaces:**
- Consumes: nothing.
- Produces: `config.Load() (config.Config, error)` with fields `DatabaseURL string`, `Port int`; `newAPI(cfg config.Config) (http.Handler, huma.API)` used by every later handler test.

- [ ] **Step 1: Create the branch**

```bash
cd ~/Projects/wikipetia && git checkout -b feature/slice-1-api
```

- [ ] **Step 2: Install the CLI tools**

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1
export PATH="$PATH:$(go env GOPATH)/bin"
sqlc version && migrate -version
```
Expected: `sqlc version` prints `v1.31.1`. **`migrate -version` prints `dev`** — `go install` builds carry no release ldflags, so this is correct, not a failure.

Note: golang-migrate v4.20.1 requires Go >= 1.25.11, so the toolchain auto-switches (downloading a newer Go) just to build this CLI. That is expected and does not change the `api` module, which stays on Go 1.25.1. CI needs `GOTOOLCHAIN=auto`, which is the default.

- [ ] **Step 3: Write `.gitignore`**

```gitignore
api/bin/
node_modules/
.env
*.log
```

- [ ] **Step 4: Write `docker-compose.yml`**

```yaml
services:
  db:
    image: postgres:18
    environment:
      POSTGRES_PASSWORD: wikipetia
      POSTGRES_USER: wikipetia
      POSTGRES_DB: wikipetia
    ports:
      - "5433:5432"
    volumes:
      # postgres:18+ expects the mount at /var/lib/postgresql, NOT at
      # /var/lib/postgresql/data. The old path makes the container exit(1) with
      # "PostgreSQL data in ... (unused mount/volume)".
      - wikipetia_pgdata:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U wikipetia"]
      interval: 2s
      timeout: 3s
      retries: 20
volumes:
  wikipetia_pgdata:
```

Port 5433 is deliberate: it avoids colliding with any local Postgres on 5432.

- [ ] **Step 5: Write the `Makefile`**

```makefile
GOBIN := $(shell go env GOPATH)/bin
DB_URL ?= postgres://wikipetia:wikipetia@localhost:5433/wikipetia?sslmode=disable
TEST_DB_URL ?= postgres://wikipetia:wikipetia@localhost:5433/wikipetia_test?sslmode=disable

.PHONY: up down migrate-up migrate-down generate test run psql testdb

up:
	docker compose up -d --wait

down:
	docker compose down

testdb: up
	docker compose exec -T db psql -U wikipetia -d wikipetia -c "SELECT 1 FROM pg_database WHERE datname='wikipetia_test'" -tA | grep -q 1 || \
		docker compose exec -T db createdb -U wikipetia wikipetia_test
	$(GOBIN)/migrate -path api/db/migrations -database "$(TEST_DB_URL)" up

migrate-up: up
	$(GOBIN)/migrate -path api/db/migrations -database "$(DB_URL)" up

migrate-down:
	$(GOBIN)/migrate -path api/db/migrations -database "$(DB_URL)" down 1

generate:
	cd api && $(GOBIN)/sqlc generate
	cd api && go run ./cmd/api -dump-openapi > openapi.yaml

test: testdb
	cd api && TEST_DATABASE_URL="$(TEST_DB_URL)" go test ./... -count=1

run: migrate-up
	cd api && DATABASE_URL="$(DB_URL)" go run ./cmd/api

psql: up
	docker compose exec db psql -U wikipetia -d wikipetia
```

- [ ] **Step 6: Initialise the Go module**

```bash
mkdir -p api/cmd/api api/internal/platform/config api/db/migrations api/db/queries
cd api && go mod init github.com/Fikhri02/wikipetia
go get github.com/danielgtaylor/huma/v2@v2.39.1
go get github.com/jackc/pgx/v5@v5.11.0
go get github.com/google/uuid@v1.6.0
go get github.com/stretchr/testify@v1.12.1
go mod tidy
```

`go mod tidy` is required, not optional: testify v1.12.1 imports `go.yaml.in/yaml/v3` transitively, and without tidy every test fails at setup with `missing go.sum entry`.

- [ ] **Step 7: Write the failing test** — `api/cmd/api/main_test.go`

```go
package main

import (
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("WikiPetia", "0.1.0"))
	registerHealth(api)

	resp := api.Get("/health")
	require.Equal(t, 200, resp.Code)
	assert.Contains(t, resp.Body.String(), `"status":"ok"`)
}
```

- [ ] **Step 8: Run it and confirm it fails**

Run: `cd api && go test ./cmd/api/ -run TestHealth -v`
Expected: FAIL — `undefined: registerHealth`.

- [ ] **Step 9: Write `api/internal/platform/config/config.go`**

```go
package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string
	Port        int
	DevUserName string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        8080,
		DevUserName: envOr("DEV_USER_NAME", "dev"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if p := os.Getenv("PORT"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Config{}, errors.New("PORT must be an integer")
		}
		cfg.Port = n
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 10: Write `api/cmd/api/main.go`**

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/Fikhri02/wikipetia/internal/platform/config"
)

type healthOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

func registerHealth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Liveness probe",
	}, func(ctx context.Context, _ *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}

func newAPI() (http.Handler, huma.API) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("WikiPetia", "0.1.0"))
	registerHealth(api)
	return mux, api
}

func main() {
	dumpSpec := flag.Bool("dump-openapi", false, "print the OpenAPI spec and exit")
	flag.Parse()

	handler, api := newAPI()

	if *dumpSpec {
		spec, err := api.OpenAPI().YAML()
		if err != nil {
			slog.Error("dump openapi", "err", err)
			os.Exit(1)
		}
		fmt.Print(string(spec))
		return
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
```

`-dump-openapi` runs before `config.Load()` on purpose, so `make generate` needs no database.

- [ ] **Step 11: Run the test and confirm it passes**

Run: `cd api && go test ./cmd/api/ -run TestHealth -v`
Expected: PASS.

- [ ] **Step 12: Verify the container and the binary both come up**

```bash
make up
cd api && go run ./cmd/api -dump-openapi > openapi.yaml && head -3 openapi.yaml
```
Expected: compose reports healthy, and `grep '^openapi:' openapi.yaml` prints `openapi: 3.1.0`. The file does **not** begin with that line — huma emits YAML keys alphabetically, so `components:` comes first.

Do **not** run `make generate` yet — it invokes sqlc, whose `sqlc.yaml` does not exist until Task 3. Dump the spec directly here.

- [ ] **Step 13: Commit**

```bash
git add -A && git commit -m "feat(api): repo skeleton, Docker Postgres, health endpoint"
```

---

### Task 2: Slice 1 schema migration

**Files:**
- Create: `api/db/migrations/000001_slice1.up.sql`, `api/db/migrations/000001_slice1.down.sql`
- Create: `api/db/migrations/migrations_test.go`

**Interfaces:**
- Consumes: Task 1's `make up`.
- Produces: the eleven tables every later task queries, with the exact column names used throughout this plan.

- [ ] **Step 1: Write the failing test** — `api/db/migrations/migrations_test.go`

```go
package migrations_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaHasSlice1Tables(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	conn, err := pgx.Connect(context.Background(), url)
	require.NoError(t, err)
	defer conn.Close(context.Background())

	want := []string{
		"users", "courses", "topics", "modules", "blocks", "block_topics",
		"questions", "enrolments", "block_progress", "topic_mastery", "events",
	}
	for _, table := range want {
		var exists bool
		err := conn.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			 WHERE table_schema='public' AND table_name=$1)`, table).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "table %s should exist", table)
	}
}

func TestSelfStudyEnrolmentIsUniquePerCourse(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	require.NoError(t, err)
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	var userID, courseID string
	require.NoError(t, tx.QueryRow(ctx,
		`INSERT INTO users (name) VALUES ('u') RETURNING id`).Scan(&userID))
	require.NoError(t, tx.QueryRow(ctx,
		`INSERT INTO courses (owner_id, title) VALUES ($1,'c') RETURNING id`, userID).Scan(&courseID))

	_, err = tx.Exec(ctx,
		`INSERT INTO enrolments (user_id, course_id, mode) VALUES ($1,$2,'self_study')`, userID, courseID)
	require.NoError(t, err)

	_, err = tx.Exec(ctx,
		`INSERT INTO enrolments (user_id, course_id, mode) VALUES ($1,$2,'self_study')`, userID, courseID)
	require.Error(t, err, "a second self-study enrolment in the same course must be rejected")
}
```

The second test is the point of this task: a plain `UNIQUE (user_id, course_id, class_id)` would **not** catch this, because NULLs never collide in Postgres.

- [ ] **Step 2: Run it and confirm it fails**

Run: `make testdb && cd api && TEST_DATABASE_URL="postgres://wikipetia:wikipetia@localhost:5433/wikipetia_test?sslmode=disable" go test ./db/migrations/ -v`
Expected: FAIL — relation "users" does not exist.

- [ ] **Step 3: Write `api/db/migrations/000001_slice1.up.sql`**

```sql
CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_subject text UNIQUE,
    name         text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE courses (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid NOT NULL REFERENCES users(id),
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    visibility  text NOT NULL DEFAULT 'private'
                CHECK (visibility IN ('public','unlisted','private')),
    pet_enabled boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE INDEX courses_owner_idx ON courses (owner_id) WHERE archived_at IS NULL;

CREATE TABLE topics (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id   uuid NOT NULL REFERENCES courses(id),
    title       text NOT NULL,
    sort_key    text NOT NULL,
    archived_at timestamptz
);
CREATE INDEX topics_course_idx ON topics (course_id, sort_key) WHERE archived_at IS NULL;

CREATE TABLE modules (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id   uuid NOT NULL REFERENCES courses(id),
    title       text NOT NULL,
    sort_key    text NOT NULL,
    archived_at timestamptz
);
CREATE INDEX modules_course_idx ON modules (course_id, sort_key) WHERE archived_at IS NULL;

CREATE TABLE blocks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id   uuid NOT NULL REFERENCES modules(id),
    title       text NOT NULL,
    type        text NOT NULL CHECK (type IN ('reading','video_youtube','quiz')),
    sort_key    text NOT NULL,
    content     jsonb NOT NULL DEFAULT '{}'::jsonb,
    completion  jsonb NOT NULL DEFAULT '{}'::jsonb,
    archived_at timestamptz
);
CREATE INDEX blocks_module_idx ON blocks (module_id, sort_key) WHERE archived_at IS NULL;

CREATE TABLE block_topics (
    block_id uuid NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    PRIMARY KEY (block_id, topic_id)
);

CREATE TABLE questions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    block_id    uuid NOT NULL REFERENCES blocks(id),
    topic_id    uuid REFERENCES topics(id),
    kind        text NOT NULL CHECK (kind IN ('mcq_single','true_false','typed')),
    prompt      jsonb NOT NULL,
    answer      jsonb NOT NULL,
    sort_key    text NOT NULL,
    archived_at timestamptz
);
CREATE INDEX questions_block_idx ON questions (block_id, sort_key) WHERE archived_at IS NULL;

CREATE TABLE enrolments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id),
    course_id    uuid NOT NULL REFERENCES courses(id),
    mode         text NOT NULL CHECK (mode IN ('self_study','classroom')),
    class_id     uuid,
    started_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE UNIQUE INDEX enrolments_self_study_uniq
    ON enrolments (user_id, course_id) WHERE class_id IS NULL;
CREATE UNIQUE INDEX enrolments_class_uniq
    ON enrolments (user_id, course_id, class_id) WHERE class_id IS NOT NULL;
CREATE INDEX enrolments_user_idx ON enrolments (user_id);

CREATE TABLE block_progress (
    enrolment_id uuid NOT NULL REFERENCES enrolments(id),
    block_id     uuid NOT NULL REFERENCES blocks(id),
    status       text NOT NULL CHECK (status IN ('in_progress','completed')),
    score        integer CHECK (score BETWEEN 0 AND 100),
    attempts     integer NOT NULL DEFAULT 0,
    completed_at timestamptz,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (enrolment_id, block_id)
);

CREATE TABLE topic_mastery (
    enrolment_id      uuid NOT NULL REFERENCES enrolments(id),
    topic_id          uuid NOT NULL REFERENCES topics(id),
    strength          double precision NOT NULL DEFAULT 0
                      CHECK (strength >= 0 AND strength <= 1),
    evidence_count    integer NOT NULL DEFAULT 0,
    last_evidence_at  timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (enrolment_id, topic_id)
);

CREATE TABLE events (
    id           bigserial PRIMARY KEY,
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    user_id      uuid NOT NULL REFERENCES users(id),
    enrolment_id uuid REFERENCES enrolments(id),
    kind         text NOT NULL,
    payload      jsonb NOT NULL DEFAULT '{}'::jsonb,
    consumed_at  timestamptz
);
CREATE INDEX events_unconsumed_idx ON events (id) WHERE consumed_at IS NULL;
```

`gen_random_uuid()` needs no extension — it has been core since PostgreSQL 13.

- [ ] **Step 4: Write `api/db/migrations/000001_slice1.down.sql`**

```sql
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS topic_mastery;
DROP TABLE IF EXISTS block_progress;
DROP TABLE IF EXISTS enrolments;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS block_topics;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS modules;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS courses;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 5: Apply and run the tests**

Run: `make test`
Expected: both tests PASS.

- [ ] **Step 6: Verify the migration reverses cleanly**

```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: no errors. A down migration that fails is a migration you cannot iterate on.

- [ ] **Step 7: Commit**

```bash
git add -A && git commit -m "feat(api): slice 1 schema with partial unique enrolment indexes"
```

---

### Task 3: sqlc wiring, connection pool, test harness, users

**Files:**
- Create: `api/sqlc.yaml`, `api/db/queries/users.sql`
- Create: `api/internal/platform/db/db.go`, `api/internal/platform/db/pgtest/pgtest.go`
- Create: `api/internal/user/user.go`, `api/internal/user/user_test.go`
- Generated: `api/internal/platform/sqlcgen/` (committed, never hand-edited)

**Interfaces:**
- Consumes: Task 2's schema.
- Produces:
  - `db.NewPool(ctx context.Context, url string) (*pgxpool.Pool, error)`
  - `db.WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *sqlcgen.Queries) error) error`
  - `pgtest.Begin(t *testing.T) (context.Context, *sqlcgen.Queries)` — every repository test uses this
  - `user.Store.EnsureBySubject(ctx, q *sqlcgen.Queries, subject, name string) (uuid.UUID, error)`

- [ ] **Step 1: Write `api/sqlc.yaml` exactly as given**

This configuration was verified by generating and compiling. Do not add `pointer: true` object-form overrides or mix them with string forms — both break the build.

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "db/migrations"
    queries: "db/queries"
    gen:
      go:
        package: "sqlcgen"
        out: "internal/platform/sqlcgen"
        sql_package: "pgx/v5"
        emit_pointers_for_null_types: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "jsonb"
            go_type: "encoding/json.RawMessage"
          - db_type: "timestamptz"
            go_type: "time.Time"
```

Resulting mapping, verified: NOT NULL `timestamptz → time.Time`; nullable `timestamptz → pgtype.Timestamptz`; `uuid → uuid.UUID`; `jsonb → json.RawMessage`; nullable `text → *string`; nullable `integer → *int32`.

- [ ] **Step 2: Write `api/db/queries/users.sql`**

```sql
-- name: GetUserBySubject :one
SELECT * FROM users WHERE auth_subject = $1;

-- name: CreateUser :one
INSERT INTO users (auth_subject, name) VALUES ($1, $2) RETURNING *;
```

- [ ] **Step 3: Generate and confirm it compiles**

```bash
make generate
cd api && go build ./... && go vet ./...
```
Expected: no output. If you see `redeclared in this block`, the overrides were edited — restore Step 1 verbatim.

- [ ] **Step 4: Write the failing test** — `api/internal/user/user_test.go`

```go
package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/user"
)

func TestEnsureBySubjectCreatesThenReuses(t *testing.T) {
	ctx, q := pgtest.Begin(t)
	store := user.Store{}

	first, err := store.EnsureBySubject(ctx, q, "dev|1", "Irfan")
	require.NoError(t, err)

	second, err := store.EnsureBySubject(ctx, q, "dev|1", "Irfan")
	require.NoError(t, err)

	assert.Equal(t, first, second, "the same subject must map to the same user")
}

func TestEnsureBySubjectSeparatesSubjects(t *testing.T) {
	ctx, q := pgtest.Begin(t)
	store := user.Store{}

	a, err := store.EnsureBySubject(ctx, q, "dev|a", "A")
	require.NoError(t, err)
	b, err := store.EnsureBySubject(ctx, q, "dev|b", "B")
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
}

func TestHarnessIsolatesTests(t *testing.T) {
	ctx, q := pgtest.Begin(t)
	_, err := user.Store{}.EnsureBySubject(ctx, q, "dev|isolated", "X")
	require.NoError(t, err)
	// TestHarnessIsolationProof below must not see this row.
}

func TestHarnessIsolationProof(t *testing.T) {
	ctx, q := pgtest.Begin(t)
	_, err := q.GetUserBySubject(ctx, ptr("dev|isolated"))
	require.Error(t, err, "the previous test's row must have been rolled back")
}

func ptr(s string) *string { return &s }
```

- [ ] **Step 5: Run it and confirm it fails**

Run: `make test`
Expected: FAIL — `undefined: pgtest.Begin`, `undefined: user.Store`.

- [ ] **Step 6: Write `api/internal/platform/db/db.go`**

```go
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// WithTx runs fn inside a single transaction. Every write that must be atomic
// with the events it emits goes through here.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *sqlcgen.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(sqlcgen.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Write `api/internal/platform/db/pgtest/pgtest.go`**

```go
// Package pgtest gives each test its own transaction, rolled back when the
// test ends. Tests therefore share one database without sharing state, and no
// test needs to clean up after itself.
package pgtest

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

var (
	once sync.Once
	pool *pgxpool.Pool
)

// Begin returns a context and a *sqlcgen.Queries bound to a transaction that
// is rolled back during test cleanup.
func Begin(t *testing.T) (context.Context, *sqlcgen.Queries) {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; run via `make test`")
	}

	ctx := context.Background()
	once.Do(func() {
		p, err := pgxpool.New(ctx, url)
		if err != nil {
			t.Fatalf("test pool: %v", err)
		}
		pool = p
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	return ctx, sqlcgen.New(tx)
}
```

- [ ] **Step 8: Write `api/internal/user/user.go`**

```go
package user

import (
	"errors"
	"fmt"

	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Store struct{}

// EnsureBySubject maps an identity-provider subject to our own user row,
// creating it on first sight. Keeping our own users table is what makes the
// auth provider swappable.
func (Store) EnsureBySubject(ctx context.Context, q *sqlcgen.Queries, subject, name string) (uuid.UUID, error) {
	existing, err := q.GetUserBySubject(ctx, &subject)
	switch {
	case err == nil:
		return existing.ID, nil
	case errors.Is(err, pgx.ErrNoRows):
		created, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
			AuthSubject: &subject,
			Name:        name,
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("create user: %w", err)
		}
		return created.ID, nil
	default:
		return uuid.Nil, fmt.Errorf("get user: %w", err)
	}
}
```

- [ ] **Step 9: Write `api/internal/platform/pgconv/pgconv.go`**

Nullable `uuid` and `timestamptz` columns surface as `pgtype.UUID` and `pgtype.Timestamptz`. These two helpers are the only sanctioned place those types appear outside generated code — every repository converts at its boundary so no domain type carries a pgx type.

```go
// Package pgconv converts between pgx's nullable column types and ordinary Go
// pointers. Nullable uuid and timestamptz reach us as pgtype values because
// sqlc's non-null overrides do not cover the nullable case.
package pgconv

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func UUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: [16]byte(*id), Valid: true}
}

func UUIDPtr(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	id := uuid.UUID(v.Bytes)
	return &id
}

func Timestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func TimePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}
```

- [ ] **Step 10: Write its test** — `api/internal/platform/pgconv/pgconv_test.go`

```go
package pgconv_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
)

func TestUUIDRoundTrip(t *testing.T) {
	id := uuid.New()
	back := pgconv.UUIDPtr(pgconv.UUID(&id))
	require.NotNil(t, back)
	assert.Equal(t, id, *back)
}

func TestUUIDNilRoundTrip(t *testing.T) {
	assert.Nil(t, pgconv.UUIDPtr(pgconv.UUID(nil)))
}

func TestTimeRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	back := pgconv.TimePtr(pgconv.Timestamptz(&now))
	require.NotNil(t, back)
	assert.True(t, now.Equal(*back))
}

func TestTimeNilRoundTrip(t *testing.T) {
	assert.Nil(t, pgconv.TimePtr(pgconv.Timestamptz(nil)))
}
```

- [ ] **Step 11: Run the tests and confirm they pass**

Run: `make test`
Expected: all eight tests PASS. `TestHarnessIsolationProof` passing is the proof the harness rolls back.

- [ ] **Step 12: Commit**

```bash
git add -A && git commit -m "feat(api): sqlc wiring, pool, per-test transaction harness, users, pgconv"
```

---

### Task 4: Sentinel errors and the single HTTP mapping

**Files:**
- Create: `api/internal/platform/apperr/apperr.go`, `api/internal/platform/apperr/apperr_test.go`
- Create: `api/cmd/api/errors.go`, `api/cmd/api/errors_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `apperr.NotFound(format string, args ...any) error`
  - `apperr.Forbidden(format string, args ...any) error`
  - `apperr.Conflict(format string, args ...any) error`
  - `apperr.Invalid(field, format string, args ...any) error`
  - sentinels `apperr.ErrNotFound`, `ErrForbidden`, `ErrConflict`, `ErrValidation`
  - `toHTTP(err error) error` in `cmd/api` — every handler returns `toHTTP(err)`

- [ ] **Step 1: Write the failing test** — `api/internal/platform/apperr/apperr_test.go`

```go
package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func TestSentinelsMatchViaErrorsIs(t *testing.T) {
	assert.True(t, errors.Is(apperr.NotFound("course %s", "abc"), apperr.ErrNotFound))
	assert.True(t, errors.Is(apperr.Forbidden("not yours"), apperr.ErrForbidden))
	assert.True(t, errors.Is(apperr.Conflict("already enrolled"), apperr.ErrConflict))
	assert.True(t, errors.Is(apperr.Invalid("title", "must not be empty"), apperr.ErrValidation))
}

func TestSentinelsDoNotCrossMatch(t *testing.T) {
	assert.False(t, errors.Is(apperr.NotFound("x"), apperr.ErrForbidden))
}

func TestWrappingSurvives(t *testing.T) {
	wrapped := fmt.Errorf("loading course: %w", apperr.NotFound("course abc"))
	assert.True(t, errors.Is(wrapped, apperr.ErrNotFound))
}

func TestInvalidCarriesField(t *testing.T) {
	var ve *apperr.ValidationError
	assert.True(t, errors.As(apperr.Invalid("title", "must not be empty"), &ve))
	assert.Equal(t, "title", ve.Field)
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd api && go test ./internal/platform/apperr/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write `api/internal/platform/apperr/apperr.go`**

```go
// Package apperr defines the errors domain packages return. No domain package
// knows about HTTP; cmd/api maps these to status codes in exactly one place.
package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
	ErrValidation = errors.New("validation failed")
)

type kindError struct {
	kind error
	msg  string
}

func (e *kindError) Error() string { return e.msg }
func (e *kindError) Unwrap() error { return e.kind }

func NotFound(format string, args ...any) error {
	return &kindError{kind: ErrNotFound, msg: fmt.Sprintf(format, args...)}
}

func Forbidden(format string, args ...any) error {
	return &kindError{kind: ErrForbidden, msg: fmt.Sprintf(format, args...)}
}

func Conflict(format string, args ...any) error {
	return &kindError{kind: ErrConflict, msg: fmt.Sprintf(format, args...)}
}

// ValidationError carries the offending field so the handler can report it in
// the problem+json errors[] array with a location the UI can map to an input.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Msg) }
func (e *ValidationError) Unwrap() error { return ErrValidation }

func Invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Msg: fmt.Sprintf(format, args...)}
}
```

- [ ] **Step 4: Run and confirm the apperr tests pass**

Run: `cd api && go test ./internal/platform/apperr/ -v`
Expected: PASS.

- [ ] **Step 5: Write the failing mapping test** — `api/cmd/api/errors_test.go`

```go
package main

import (
	"errors"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func statusOf(t *testing.T, err error) int {
	t.Helper()
	var se huma.StatusError
	require.True(t, errors.As(err, &se), "mapped error must be a huma.StatusError")
	return se.GetStatus()
}

func TestToHTTPMapsSentinels(t *testing.T) {
	assert.Equal(t, 404, statusOf(t, toHTTP(apperr.NotFound("course abc"))))
	assert.Equal(t, 403, statusOf(t, toHTTP(apperr.Forbidden("not yours"))))
	assert.Equal(t, 409, statusOf(t, toHTTP(apperr.Conflict("already enrolled"))))
	assert.Equal(t, 422, statusOf(t, toHTTP(apperr.Invalid("title", "required"))))
}

func TestToHTTPHidesUnexpectedErrors(t *testing.T) {
	err := toHTTP(errors.New("pq: connection reset by peer"))
	assert.Equal(t, 500, statusOf(t, err))
	assert.NotContains(t, err.Error(), "connection reset",
		"internal error text must never reach the client")
}

func TestToHTTPPassesNilThrough(t *testing.T) {
	assert.NoError(t, toHTTP(nil))
}
```

- [ ] **Step 6: Run it and confirm it fails**

Run: `cd api && go test ./cmd/api/ -run TestToHTTP -v`
Expected: FAIL — `undefined: toHTTP`.

- [ ] **Step 7: Write `api/cmd/api/errors.go`**

```go
package main

import (
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

// toHTTP is the ONLY place in the codebase that turns a domain error into a
// status code. Handlers return toHTTP(err); domain packages never do.
func toHTTP(err error) error {
	if err == nil {
		return nil
	}

	var ve *apperr.ValidationError
	if errors.As(err, &ve) {
		return &huma.ErrorModel{
			Status: 422,
			Title:  "Unprocessable Entity",
			Detail: "validation failed",
			Errors: []*huma.ErrorDetail{{
				Message:  ve.Msg,
				Location: "body." + ve.Field,
			}},
		}
	}

	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, apperr.ErrForbidden):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, apperr.ErrConflict):
		return huma.Error409Conflict(err.Error())
	}

	ref := uuid.NewString()
	slog.Error("unhandled error", "ref", ref, "err", err)
	return huma.Error500InternalServerError("internal error (ref " + ref + ")")
}
```

`huma.ErrorModel` implements `huma.StatusError`, so returning it directly gives full control of the `errors[]` array while keeping the `application/problem+json` content type.

- [ ] **Step 8: Run and confirm all tests pass**

Run: `cd api && go test ./cmd/api/ ./internal/platform/apperr/ -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -A && git commit -m "feat(api): sentinel errors and single HTTP status mapping"
```

---

### Task 5: Dev-stub auth middleware

**Files:**
- Create: `api/internal/platform/auth/auth.go`, `api/internal/platform/auth/auth_test.go`
- Modify: `api/cmd/api/main.go` (wire the middleware into `newAPI`)

**Interfaces:**
- Consumes: `user.Store.EnsureBySubject`, `db.WithTx`.
- Produces:
  - `auth.Identity` interface: `Subject(r *http.Request) (subject string, name string, err error)`
  - `auth.DevStub{DefaultName string}` implementing it, reading the `X-Dev-User` header and falling back to the configured default
  - `auth.Middleware(next http.Handler, ident Identity, resolve func(ctx context.Context, subject, name string) (uuid.UUID, error)) http.Handler`
  - `auth.UserFromContext(ctx context.Context) (uuid.UUID, bool)`

`auth` is the one non-`cmd` package permitted to import `net/http`, because middleware is by definition HTTP. Domain packages still may not.

- [ ] **Step 1: Write the failing test** — `api/internal/platform/auth/auth_test.go`

```go
package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

func TestDevStubUsesHeaderThenDefault(t *testing.T) {
	stub := auth.DevStub{DefaultName: "dev"}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Dev-User", "irfan")
	subject, name, err := stub.Subject(r)
	require.NoError(t, err)
	assert.Equal(t, "dev|irfan", subject)
	assert.Equal(t, "irfan", name)

	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	subject, name, err = stub.Subject(plain)
	require.NoError(t, err)
	assert.Equal(t, "dev|dev", subject)
	assert.Equal(t, "dev", name)
}

func TestMiddlewarePutsUserOnContext(t *testing.T) {
	want := uuid.New()
	resolve := func(ctx context.Context, subject, name string) (uuid.UUID, error) {
		assert.Equal(t, "dev|irfan", subject)
		return want, nil
	}

	var got uuid.UUID
	var ok bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = auth.UserFromContext(r.Context())
	})

	h := auth.Middleware(inner, auth.DevStub{DefaultName: "dev"}, resolve)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Dev-User", "irfan")
	h.ServeHTTP(httptest.NewRecorder(), r)

	require.True(t, ok, "user id must be on the context")
	assert.Equal(t, want, got)
}

func TestMiddlewareReturns401WhenResolveFails(t *testing.T) {
	resolve := func(ctx context.Context, subject, name string) (uuid.UUID, error) {
		return uuid.Nil, assert.AnError
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler must not run")
	})

	rec := httptest.NewRecorder()
	auth.Middleware(inner, auth.DevStub{DefaultName: "dev"}, resolve).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUserFromContextEmptyWhenAbsent(t *testing.T) {
	_, ok := auth.UserFromContext(context.Background())
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd api && go test ./internal/platform/auth/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write `api/internal/platform/auth/auth.go`**

```go
// Package auth resolves a request to one of our own user ids.
//
// Slice 1 ships a development stub. A real provider (JWT verification) replaces
// DevStub behind the Identity interface without any other package changing,
// because everything downstream keys off our own users row, never the provider's
// subject. Per the spec, the real provider must land before slice 3.
package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type Identity interface {
	Subject(r *http.Request) (subject string, name string, err error)
}

type DevStub struct {
	DefaultName string
}

func (d DevStub) Subject(r *http.Request) (string, string, error) {
	name := r.Header.Get("X-Dev-User")
	if name == "" {
		name = d.DefaultName
	}
	return "dev|" + name, name, nil
}

type ctxKey struct{}

type ResolveFunc func(ctx context.Context, subject, name string) (uuid.UUID, error)

func Middleware(next http.Handler, ident Identity, resolve ResolveFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		subject, name, err := ident.Subject(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userID, err := resolve(r.Context(), subject, name)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, userID)))
	})
}

func UserFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

// MustUser is for handlers, which only run behind Middleware.
func MustUser(ctx context.Context) uuid.UUID {
	id, _ := UserFromContext(ctx)
	return id
}
```

- [ ] **Step 4: Run and confirm the auth tests pass**

Run: `cd api && go test ./internal/platform/auth/ -v`
Expected: PASS.

- [ ] **Step 5: Wire it into `main.go`**

Replace `newAPI()` and `main()` in `api/cmd/api/main.go` with:

```go
type server struct {
	pool *pgxpool.Pool
}

func newAPI(pool *pgxpool.Pool, devUserName string) (http.Handler, huma.API) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("WikiPetia", "0.1.0"))

	registerHealth(api)

	if pool == nil {
		// Spec-dump and unit-test path: no database, no auth.
		return mux, api
	}

	srv := &server{pool: pool}
	srv.registerRoutes(api)

	resolve := func(ctx context.Context, subject, name string) (uuid.UUID, error) {
		var id uuid.UUID
		err := db.WithTx(ctx, pool, func(q *sqlcgen.Queries) error {
			var err error
			id, err = user.Store{}.EnsureBySubject(ctx, q, subject, name)
			return err
		})
		return id, err
	}

	handler := auth.Middleware(mux, auth.DevStub{DefaultName: devUserName}, resolve)
	return handler, api
}

func (s *server) registerRoutes(api huma.API) {
	// Filled in by later tasks.
}

func main() {
	dumpSpec := flag.Bool("dump-openapi", false, "print the OpenAPI spec and exit")
	flag.Parse()

	if *dumpSpec {
		_, api := newAPI(nil, "dev")
		spec, err := api.OpenAPI().YAML()
		if err != nil {
			slog.Error("dump openapi", "err", err)
			os.Exit(1)
		}
		fmt.Print(string(spec))
		return
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	handler, _ := newAPI(pool, cfg.DevUserName)
	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
```

Add the imports `github.com/jackc/pgx/v5/pgxpool`, `github.com/google/uuid`, and the project's `internal/platform/auth`, `internal/platform/db`, `internal/platform/sqlcgen`, `internal/user`.

Note the `pool == nil` branch: `-dump-openapi` must work with no database so `make generate` runs in CI.

- [ ] **Step 6: Fix the health test for the new signature**

In `api/cmd/api/main_test.go`, `TestHealth` still builds its own huma API and calls `registerHealth(api)` — unchanged and still valid. Run it:

Run: `cd api && go test ./... -count=1` (with `make test` for DB-backed packages)
Expected: PASS.

- [ ] **Step 7: Verify the spec still dumps without a database**

```bash
cd api && go run ./cmd/api -dump-openapi | head -3
```
Expected: `openapi: 3.1.0` — and no database error.

- [ ] **Step 8: Commit**

```bash
git add -A && git commit -m "feat(api): dev-stub auth middleware behind a swappable interface"
```

---

### Task 6: Lexicographic sort_key ranking

**Files:**
- Create: `api/internal/platform/rank/rank.go`, `api/internal/platform/rank/rank_test.go`

**Interfaces:**
- Consumes: nothing. Pure functions, no database.
- Produces:
  - `rank.Between(prev, next string) (string, error)` — a key strictly between two keys; `""` for `prev` means "before everything", `""` for `next` means "after everything"
  - `rank.First() string` — the key for the first item in an empty list

Reordering writes one row instead of renumbering a whole course, which is why keys are strings rather than integers.

- [ ] **Step 1: Write the failing test** — `api/internal/platform/rank/rank_test.go`

```go
package rank_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/rank"
)

func TestBetweenEmptyBoundsGivesMiddle(t *testing.T) {
	k, err := rank.Between("", "")
	require.NoError(t, err)
	assert.NotEmpty(t, k)
	assert.Equal(t, rank.First(), k)
}

func TestBetweenOrdersCorrectly(t *testing.T) {
	cases := []struct{ prev, next string }{
		{"", ""},
		{"V", ""},
		{"", "V"},
		{"A", "B"},
		{"AV", "AW"},
		{"0", "1"},
	}
	for _, c := range cases {
		got, err := rank.Between(c.prev, c.next)
		require.NoError(t, err, "Between(%q,%q)", c.prev, c.next)
		if c.prev != "" {
			assert.Greater(t, got, c.prev, "Between(%q,%q)=%q must sort after prev", c.prev, c.next, got)
		}
		if c.next != "" {
			assert.Less(t, got, c.next, "Between(%q,%q)=%q must sort before next", c.prev, c.next, got)
		}
	}
}

func TestBetweenRejectsOutOfOrderBounds(t *testing.T) {
	_, err := rank.Between("B", "A")
	assert.Error(t, err)
}

func TestRepeatedInsertsAtHeadStayOrdered(t *testing.T) {
	// Dragging items to the top 50 times must never collide or invert.
	keys := []string{}
	head := ""
	for i := 0; i < 50; i++ {
		k, err := rank.Between("", head)
		require.NoError(t, err)
		require.NotEqual(t, head, k)
		keys = append(keys, k)
		head = k
	}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	// keys were generated descending, so sorted order is the reverse
	for i := range keys {
		assert.Equal(t, keys[len(keys)-1-i], sorted[i])
	}
}

func TestRepeatedInsertsBetweenTwoStayOrdered(t *testing.T) {
	lo, hi := "A", "B"
	for i := 0; i < 50; i++ {
		mid, err := rank.Between(lo, hi)
		require.NoError(t, err)
		require.Greater(t, mid, lo)
		require.Less(t, mid, hi)
		hi = mid
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd api && go test ./internal/platform/rank/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write `api/internal/platform/rank/rank.go`**

```go
// Package rank generates lexicographically ordered sort keys, so inserting or
// moving one item writes one row instead of renumbering its siblings.
package rank

import (
	"errors"
	"strings"
)

// Ordered by byte value, so plain string comparison matches this order.
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz"

// First is the key for the only item in an empty list.
func First() string {
	return string(alphabet[len(alphabet)/2])
}

// Between returns a key strictly between prev and next.
// prev == "" means "before every existing key"; next == "" means "after every
// existing key".
func Between(prev, next string) (string, error) {
	if prev != "" && next != "" && prev >= next {
		return "", errors.New("rank: prev must sort before next")
	}

	var out strings.Builder
	for i := 0; ; i++ {
		lo := 0
		if i < len(prev) {
			i0 := indexOf(prev[i])
			if i0 < 0 {
				return "", errors.New("rank: prev contains a character outside the alphabet")
			}
			lo = i0
		}

		hi := len(alphabet) // exclusive sentinel: "above every character"
		if next != "" {
			if i < len(next) {
				i1 := indexOf(next[i])
				if i1 < 0 {
					return "", errors.New("rank: next contains a character outside the alphabet")
				}
				hi = i1
			} else {
				// next ran out, e.g. prev="AB", next="A" — impossible given the
				// ordering check above, but guard rather than loop forever.
				return "", errors.New("rank: next is a prefix of prev")
			}
		}

		if hi-lo > 1 {
			out.WriteByte(alphabet[lo+(hi-lo)/2])
			return out.String(), nil
		}

		// lo and hi are adjacent (or equal): keep prev's character and descend.
		out.WriteByte(alphabet[lo])
		// Past this point any suffix sorts after prev and before next, so the
		// upper bound stops constraining us.
		if hi == lo+1 {
			next = ""
		}
	}
}

func indexOf(c byte) int {
	return strings.IndexByte(alphabet, c)
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `cd api && go test ./internal/platform/rank/ -v`
Expected: all five tests PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(api): lexicographic sort key ranking"
```

---

### Task 7: Courses — the canonical resource pattern

Every later resource follows the shape established here: a `Runner` for transactions, a service holding authorization, sqlc queries, handlers that only translate.

**Files:**
- Modify: `api/internal/platform/db/db.go` (add `Runner` + `PoolRunner`)
- Modify: `api/internal/platform/db/pgtest/pgtest.go` (add `NewRunner`)
- Create: `api/db/queries/courses.sql`
- Create: `api/internal/course/service.go`, `api/internal/course/course_test.go`
- Create: `api/cmd/api/handlers_courses.go`, `api/cmd/api/handlers_courses_test.go`
- Modify: `api/cmd/api/main.go` (`registerRoutes`)

**Interfaces:**
- Consumes: `apperr`, `auth.MustUser`, `sqlcgen`, `toHTTP`.
- Produces:
  - `db.Runner` interface: `Tx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error`
  - `db.PoolRunner{Pool *pgxpool.Pool}` implementing it
  - `pgtest.NewRunner(t *testing.T) (context.Context, db.Runner)`
  - `course.Service` with `New(runner db.Runner) *Service`
  - `(*Service).Create(ctx, ownerID uuid.UUID, in CreateInput) (Course, error)`
  - `(*Service).ListOwned(ctx, ownerID uuid.UUID) ([]Course, error)`
  - `(*Service).Get(ctx, userID, courseID uuid.UUID) (Course, error)`
  - `(*Service).Update(ctx, userID, courseID uuid.UUID, in UpdateInput) (Course, error)`
  - `(*Service).Archive(ctx, userID, courseID uuid.UUID) error`
  - `(*Service).RequireOwner(ctx, q *sqlcgen.Queries, userID, courseID uuid.UUID) error` — reused by Tasks 8–12

- [ ] **Step 1: Add `Runner` to `api/internal/platform/db/db.go`**

```go
// Runner runs a function inside a transaction. Production uses PoolRunner;
// tests use pgtest.NewRunner, which reuses the test's rolled-back transaction.
// Services depend on this interface so domain tests need no live pool.
type Runner interface {
	Tx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error
}

type PoolRunner struct {
	Pool *pgxpool.Pool
}

func (r PoolRunner) Tx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error {
	return WithTx(ctx, r.Pool, fn)
}
```

- [ ] **Step 2: Add `NewRunner` to `api/internal/platform/db/pgtest/pgtest.go`**

```go
type txRunner struct {
	q *sqlcgen.Queries
}

func (r txRunner) Tx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error {
	// Already inside the test's transaction; nesting is unnecessary and the
	// rollback in Begin's cleanup undoes everything.
	return fn(r.q)
}

// NewRunner gives a db.Runner backed by the test's transaction.
func NewRunner(t *testing.T) (context.Context, txRunner) {
	t.Helper()
	ctx, q := Begin(t)
	return ctx, txRunner{q: q}
}
```

- [ ] **Step 3: Write `api/db/queries/courses.sql`**

```sql
-- name: CreateCourse :one
INSERT INTO courses (owner_id, title, description, visibility, pet_enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCourseByID :one
SELECT * FROM courses WHERE id = $1 AND archived_at IS NULL;

-- name: ListCoursesByOwner :many
SELECT * FROM courses
WHERE owner_id = $1 AND archived_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateCourse :one
UPDATE courses SET
    title       = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    visibility  = COALESCE(sqlc.narg('visibility'), visibility),
    pet_enabled = COALESCE(sqlc.narg('pet_enabled'), pet_enabled)
WHERE id = sqlc.arg('id') AND archived_at IS NULL
RETURNING *;

-- name: ArchiveCourse :execrows
UPDATE courses SET archived_at = now()
WHERE id = $1 AND archived_at IS NULL;
```

`sqlc.narg` produces a nullable parameter, so a PATCH that omits a field leaves it unchanged.

- [ ] **Step 4: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 5: Write the failing test** — `api/internal/course/course_test.go`

```go
package course_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

func TestCreateAndGet(t *testing.T) {
	ctx, runner := pgtest.NewRunner(t)
	svc := course.New(runner)

	var owner uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		owner, err = user.Store{}.EnsureBySubject(ctx, q, "dev|owner", "owner")
		return err
	}))

	created, err := svc.Create(ctx, owner, course.CreateInput{Title: "Form 4 Chemistry"})
	require.NoError(t, err)
	assert.Equal(t, "Form 4 Chemistry", created.Title)
	assert.Equal(t, "private", created.Visibility, "courses default to private")
	assert.False(t, created.PetEnabled, "pets are opt-in, so default off")

	got, err := svc.Get(ctx, owner, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestCreateRejectsEmptyTitle(t *testing.T) {
	ctx, runner := pgtest.NewRunner(t)
	svc := course.New(runner)

	var owner uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		owner, err = user.Store{}.EnsureBySubject(ctx, q, "dev|owner", "owner")
		return err
	}))

	_, err := svc.Create(ctx, owner, course.CreateInput{Title: "  "})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestGetRejectsNonOwner(t *testing.T) {
	ctx, runner := pgtest.NewRunner(t)
	svc := course.New(runner)

	var owner, stranger uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		// A composite literal in an `if` header is ambiguous to the Go parser
		// ("expected boolean expression, found assignment"), so `store` is
		// built outside the closure rather than written inline here.
		if owner, err = store.EnsureBySubject(ctx, q, "dev|owner", "owner"); err != nil {
			return err
		}
		stranger, err = store.EnsureBySubject(ctx, q, "dev|stranger", "stranger")
		return err
	}))

	created, err := svc.Create(ctx, owner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)

	_, err = svc.Get(ctx, stranger, created.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrForbidden), "a stranger must not read another user's course")
}

func TestGetMissingCourseIsNotFound(t *testing.T) {
	ctx, runner := pgtest.NewRunner(t)
	svc := course.New(runner)
	_, err := svc.Get(ctx, uuid.New(), uuid.New())
	assert.True(t, errors.Is(err, apperr.ErrNotFound))
}

func TestUpdateChangesOnlyProvidedFields(t *testing.T) {
	ctx, runner := pgtest.NewRunner(t)
	svc := course.New(runner)

	var owner uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		owner, err = user.Store{}.EnsureBySubject(ctx, q, "dev|owner", "owner")
		return err
	}))

	created, err := svc.Create(ctx, owner, course.CreateInput{
		Title:       "Chemistry",
		Description: "original",
	})
	require.NoError(t, err)

	newTitle := "Chemistry SPM"
	updated, err := svc.Update(ctx, owner, created.ID, course.UpdateInput{Title: &newTitle})
	require.NoError(t, err)
	assert.Equal(t, "Chemistry SPM", updated.Title)
	assert.Equal(t, "original", updated.Description, "omitted fields must be untouched")
}

func TestArchiveHidesFromListAndGet(t *testing.T) {
	ctx, runner := pgtest.NewRunner(t)
	svc := course.New(runner)

	var owner uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		owner, err = user.Store{}.EnsureBySubject(ctx, q, "dev|owner", "owner")
		return err
	}))

	created, err := svc.Create(ctx, owner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)

	require.NoError(t, svc.Archive(ctx, owner, created.ID))

	_, err = svc.Get(ctx, owner, created.ID)
	assert.True(t, errors.Is(err, apperr.ErrNotFound), "archived courses read as absent")

	list, err := svc.ListOwned(ctx, owner)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestArchiveRejectsNonOwner(t *testing.T) {
	ctx, runner := pgtest.NewRunner(t)
	svc := course.New(runner)

	var owner, stranger uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		// A composite literal in an `if` header is ambiguous to the Go parser
		// ("expected boolean expression, found assignment"), so `store` is
		// built outside the closure rather than written inline here.
		if owner, err = store.EnsureBySubject(ctx, q, "dev|owner", "owner"); err != nil {
			return err
		}
		stranger, err = store.EnsureBySubject(ctx, q, "dev|stranger", "stranger")
		return err
	}))

	created, err := svc.Create(ctx, owner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)

	err = svc.Archive(ctx, stranger, created.ID)
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}
```

Each test creates its own users inline rather than sharing a helper, so a test reads top to bottom without lookups.

- [ ] **Step 6: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: course.New`.

- [ ] **Step 7: Write `api/internal/course/service.go`**

```go
// Package course owns authoring: courses, topics, modules, blocks and questions.
// It never imports net/http — cmd/api translates HTTP into these calls.
package course

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Course struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Title       string
	Description string
	Visibility  string
	PetEnabled  bool
	CreatedAt   time.Time
}

type Service struct {
	runner db.Runner
}

func New(runner db.Runner) *Service {
	return &Service{runner: runner}
}

type CreateInput struct {
	Title       string
	Description string
	Visibility  string
	PetEnabled  bool
}

type UpdateInput struct {
	Title       *string
	Description *string
	Visibility  *string
	PetEnabled  *bool
}

var validVisibility = map[string]bool{"public": true, "unlisted": true, "private": true}

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateInput) (Course, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Course{}, apperr.Invalid("title", "must not be empty")
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = "private"
	}
	if !validVisibility[visibility] {
		return Course{}, apperr.Invalid("visibility", "must be public, unlisted or private")
	}

	var out Course
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		row, err := q.CreateCourse(ctx, sqlcgen.CreateCourseParams{
			OwnerID:     ownerID,
			Title:       title,
			Description: strings.TrimSpace(in.Description),
			Visibility:  visibility,
			PetEnabled:  in.PetEnabled,
		})
		if err != nil {
			return fmt.Errorf("create course: %w", err)
		}
		out = toCourse(row)
		return nil
	})
	return out, err
}

func (s *Service) ListOwned(ctx context.Context, ownerID uuid.UUID) ([]Course, error) {
	out := []Course{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		rows, err := q.ListCoursesByOwner(ctx, ownerID)
		if err != nil {
			return fmt.Errorf("list courses: %w", err)
		}
		for _, row := range rows {
			out = append(out, toCourse(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, userID, courseID uuid.UUID) (Course, error) {
	var out Course
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		row, err := s.load(ctx, q, userID, courseID)
		if err != nil {
			return err
		}
		out = toCourse(row)
		return nil
	})
	return out, err
}

func (s *Service) Update(ctx context.Context, userID, courseID uuid.UUID, in UpdateInput) (Course, error) {
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return Course{}, apperr.Invalid("title", "must not be empty")
	}
	if in.Visibility != nil && !validVisibility[*in.Visibility] {
		return Course{}, apperr.Invalid("visibility", "must be public, unlisted or private")
	}

	var out Course
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.load(ctx, q, userID, courseID); err != nil {
			return err
		}
		row, err := q.UpdateCourse(ctx, sqlcgen.UpdateCourseParams{
			ID:          courseID,
			Title:       trimPtr(in.Title),
			Description: trimPtr(in.Description),
			Visibility:  in.Visibility,
			PetEnabled:  in.PetEnabled,
		})
		if err != nil {
			return fmt.Errorf("update course: %w", err)
		}
		out = toCourse(row)
		return nil
	})
	return out, err
}

func (s *Service) Archive(ctx context.Context, userID, courseID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.load(ctx, q, userID, courseID); err != nil {
			return err
		}
		n, err := q.ArchiveCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("archive course: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("course %s", courseID)
		}
		return nil
	})
}

// RequireOwner is the authorization check every child resource reuses.
func (s *Service) RequireOwner(ctx context.Context, q *sqlcgen.Queries, userID, courseID uuid.UUID) error {
	_, err := s.load(ctx, q, userID, courseID)
	return err
}

func (s *Service) load(ctx context.Context, q *sqlcgen.Queries, userID, courseID uuid.UUID) (sqlcgen.Course, error) {
	row, err := q.GetCourseByID(ctx, courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Course{}, apperr.NotFound("course %s", courseID)
	}
	if err != nil {
		return sqlcgen.Course{}, fmt.Errorf("get course: %w", err)
	}
	if row.OwnerID != userID {
		// Slice 1 has no sharing: only the owner may touch a course.
		return sqlcgen.Course{}, apperr.Forbidden("course %s is not yours", courseID)
	}
	return row, nil
}

func toCourse(row sqlcgen.Course) Course {
	return Course{
		ID:          row.ID,
		OwnerID:     row.OwnerID,
		Title:       row.Title,
		Description: row.Description,
		Visibility:  row.Visibility,
		PetEnabled:  row.PetEnabled,
		CreatedAt:   row.CreatedAt,
	}
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}
```

- [ ] **Step 8: Run the domain tests and confirm they pass**

Run: `make test`
Expected: all seven course tests PASS.

- [ ] **Step 9: Write `api/cmd/api/handlers_courses.go`**

```go
package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type courseBody struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	PetEnabled  bool      `json:"pet_enabled"`
}

func courseToBody(c course.Course) courseBody {
	return courseBody{
		ID:          c.ID,
		Title:       c.Title,
		Description: c.Description,
		Visibility:  c.Visibility,
		PetEnabled:  c.PetEnabled,
	}
}

type createCourseInput struct {
	Body struct {
		Title       string `json:"title" maxLength:"200" doc:"Course title"`
		Description string `json:"description,omitempty" maxLength:"2000"`
		Visibility  string `json:"visibility,omitempty" enum:"public,unlisted,private"`
		PetEnabled  bool   `json:"pet_enabled,omitempty"`
	}
}

type courseOutput struct {
	Body courseBody
}

type courseListOutput struct {
	Body struct {
		Courses []courseBody `json:"courses"`
	}
}

type courseIDInput struct {
	ID uuid.UUID `path:"id"`
}

type updateCourseInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		Title       *string `json:"title,omitempty" maxLength:"200"`
		Description *string `json:"description,omitempty" maxLength:"2000"`
		Visibility  *string `json:"visibility,omitempty" enum:"public,unlisted,private"`
		PetEnabled  *bool   `json:"pet_enabled,omitempty"`
	}
}

func (s *server) registerCourseRoutes(api huma.API) {
	svc := course.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "createCourse", Method: http.MethodPost, Path: "/courses",
		Summary: "Create a course", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createCourseInput) (*courseOutput, error) {
		c, err := svc.Create(ctx, auth.MustUser(ctx), course.CreateInput{
			Title:       in.Body.Title,
			Description: in.Body.Description,
			Visibility:  in.Body.Visibility,
			PetEnabled:  in.Body.PetEnabled,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &courseOutput{Body: courseToBody(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listCourses", Method: http.MethodGet, Path: "/courses",
		Summary: "List courses I own",
	}, func(ctx context.Context, _ *struct{}) (*courseListOutput, error) {
		list, err := svc.ListOwned(ctx, auth.MustUser(ctx))
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &courseListOutput{}
		out.Body.Courses = make([]courseBody, 0, len(list))
		for _, c := range list {
			out.Body.Courses = append(out.Body.Courses, courseToBody(c))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getCourse", Method: http.MethodGet, Path: "/courses/{id}",
		Summary: "Get a course",
	}, func(ctx context.Context, in *courseIDInput) (*courseOutput, error) {
		c, err := svc.Get(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &courseOutput{Body: courseToBody(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateCourse", Method: http.MethodPatch, Path: "/courses/{id}",
		Summary: "Update a course",
	}, func(ctx context.Context, in *updateCourseInput) (*courseOutput, error) {
		c, err := svc.Update(ctx, auth.MustUser(ctx), in.ID, course.UpdateInput{
			Title:       in.Body.Title,
			Description: in.Body.Description,
			Visibility:  in.Body.Visibility,
			PetEnabled:  in.Body.PetEnabled,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &courseOutput{Body: courseToBody(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveCourse", Method: http.MethodPost, Path: "/courses/{id}/archive",
		Summary: "Archive a course", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *courseIDInput) (*struct{}, error) {
		if err := svc.Archive(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})
}
```

- [ ] **Step 10: Change `server` to hold a `db.Runner` and register the routes**

In `api/cmd/api/main.go`:

```go
type server struct {
	runner db.Runner
}

func (s *server) registerRoutes(api huma.API) {
	s.registerCourseRoutes(api)
}
```

and in `newAPI`, replace `srv := &server{pool: pool}` with `srv := &server{runner: db.PoolRunner{Pool: pool}}`.

- [ ] **Step 11: Export the context writer in `auth`**

In `api/internal/platform/auth/auth.go`, add this and make `Middleware` use it, so tests and production write the user onto the context by the same path:

```go
// ContextWithUser attaches a resolved user id. Middleware uses it in production;
// tests use it to run a request as a given user.
func ContextWithUser(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}
```

Then in `Middleware`, replace `context.WithValue(r.Context(), ctxKey{}, userID)` with `ContextWithUser(r.Context(), userID)`.

- [ ] **Step 12: Write the shared handler-test helpers** — `api/cmd/api/testhelpers_test.go`

The mechanism below is verified against huma v2.39.1: `api.UseMiddleware` plus `huma.WithContext` is how a test injects an authenticated user, and it must be registered **before** the routes.

```go
package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/auth"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

// newTestAPI returns a huma test API whose every request runs as `subject`,
// inside the test's transaction (rolled back on cleanup).
func newTestAPI(t *testing.T, subject string) (humatest.TestAPI, uuid.UUID) {
	t.Helper()
	ctx, runner := pgtest.NewRunner(t)

	var userID uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		userID, err = user.Store{}.EnsureBySubject(ctx, q, "dev|"+subject, subject)
		return err
	}))

	_, api := humatest.New(t, huma.DefaultConfig("test", "0.1.0"))
	api.UseMiddleware(func(hctx huma.Context, next func(huma.Context)) {
		next(huma.WithContext(hctx, auth.ContextWithUser(hctx.Context(), userID)))
	})

	srv := &server{runner: runner}
	srv.registerRoutes(api)

	return api, userID
}

// newCourse creates a course over HTTP and returns its id.
func newCourse(t *testing.T, api humatest.TestAPI, title string) string {
	t.Helper()
	resp := api.Post("/courses", map[string]any{"title": title})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	return idFromBody(t, resp.Body.Bytes())
}

// indexOfTitle reports where a title appears in a JSON body, for order assertions.
func indexOfTitle(body, title string) int {
	return strings.Index(body, `"title":"`+title+`"`)
}

func idFromBody(t *testing.T, body []byte) string {
	t.Helper()
	var parsed struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.ID)
	return parsed.ID
}
```

Cross-user rejection is tested at the **service** layer, not here — `TestGetRejectsNonOwner` and `TestArchiveRejectsNonOwner` in Step 5 — because that is where authorization lives. A handler test would only re-test huma's plumbing.

- [ ] **Step 12b: Write the handler test** — `api/cmd/api/handlers_courses_test.go`

```go
package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCourseReturns201(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	resp := api.Post("/courses", map[string]any{"title": "Form 4 Chemistry"})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"visibility":"private"`)
	assert.Contains(t, resp.Body.String(), `"pet_enabled":false`)
}

func TestCreateCourseEmptyTitleReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	resp := api.Post("/courses", map[string]any{"title": "   "})
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), `"location":"body.title"`)
}

func TestCreateCourseRejectsBadVisibility(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	resp := api.Post("/courses", map[string]any{"title": "C", "visibility": "secret"})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
}

func TestListCoursesReturnsOnlyMine(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	require.Equal(t, http.StatusCreated, api.Post("/courses", map[string]any{"title": "A"}).Code)
	require.Equal(t, http.StatusCreated, api.Post("/courses", map[string]any{"title": "B"}).Code)

	resp := api.Get("/courses")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), `"title":"A"`)
	assert.Contains(t, resp.Body.String(), `"title":"B"`)
}

func TestGetCourseNotFoundReturns404(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	assert.Equal(t, http.StatusNotFound, api.Get("/courses/"+uuid.New().String()).Code)
}

func TestGetCourseMalformedIDReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	assert.Equal(t, http.StatusUnprocessableEntity, api.Get("/courses/not-a-uuid").Code)
}

func TestPatchCourseLeavesOmittedFields(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	created := api.Post("/courses", map[string]any{"title": "Chemistry", "description": "original"})
	require.Equal(t, http.StatusCreated, created.Code)
	id := idFromBody(t, created.Body.Bytes())

	resp := api.Patch("/courses/"+id, map[string]any{"pet_enabled": true})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"pet_enabled":true`)
	assert.Contains(t, resp.Body.String(), `"description":"original"`)
	assert.Contains(t, resp.Body.String(), `"title":"Chemistry"`)
}

func TestArchiveCourseReturns204ThenGone(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	created := api.Post("/courses", map[string]any{"title": "Chemistry"})
	require.Equal(t, http.StatusCreated, created.Code)
	id := idFromBody(t, created.Body.Bytes())

	assert.Equal(t, http.StatusNoContent, api.Post("/courses/"+id+"/archive").Code)
	assert.Equal(t, http.StatusNotFound, api.Get("/courses/"+id).Code)
}
```

- [ ] **Step 13: Run the handler tests and confirm they pass**

Run: `make test`
Expected: all course handler tests PASS.

- [ ] **Step 14: Regenerate the OpenAPI spec and commit**

```bash
make generate
git add -A && git commit -m "feat(api): course CRUD with owner authorization"
```

---

### Task 8: Topics and modules — ordered children of a course

Topics and modules are the same shape: a titled, ordered child of a course. They share one placement helper and one authorization path, so they belong in one task and one review.

**Files:**
- Create: `api/db/queries/topics.sql`, `api/db/queries/modules.sql`
- Create: `api/internal/course/place.go`, `api/internal/course/place_test.go`
- Create: `api/internal/course/topics.go`, `api/internal/course/modules.go`
- Create: `api/internal/course/topics_test.go`, `api/internal/course/modules_test.go`
- Create: `api/cmd/api/handlers_structure.go`, `api/cmd/api/handlers_structure_test.go`
- Modify: `api/cmd/api/main.go` (`registerRoutes` gains `s.registerStructureRoutes(api)`)

**Interfaces:**
- Consumes: `rank.Between`, `rank.First`, `(*course.Service).RequireOwner`.
- Produces:
  - `course.Ordered{ID uuid.UUID, SortKey string}`
  - `course.PlaceKey(sibs []Ordered, before, after *uuid.UUID) (string, error)` — pure
  - `(*Service).CreateTopic(ctx, userID, courseID uuid.UUID, title string) (Topic, error)`
  - `(*Service).ListTopics(ctx, userID, courseID uuid.UUID) ([]Topic, error)`
  - `(*Service).UpdateTopic(ctx, userID, topicID uuid.UUID, title string) (Topic, error)`
  - `(*Service).ArchiveTopic(ctx, userID, topicID uuid.UUID) error`
  - `(*Service).CreateModule(ctx, userID, courseID uuid.UUID, title string) (Module, error)`
  - `(*Service).ListModules(ctx, userID, courseID uuid.UUID) ([]Module, error)`
  - `(*Service).UpdateModule(ctx, userID, moduleID uuid.UUID, title string) (Module, error)`
  - `(*Service).ArchiveModule(ctx, userID, moduleID uuid.UUID) error`
  - `(*Service).MoveModule(ctx, userID, moduleID uuid.UUID, before, after *uuid.UUID) error`
  - `Topic{ID, CourseID uuid.UUID, Title, SortKey string}`, `Module{...same fields}`

- [ ] **Step 1: Write the failing placement test** — `api/internal/course/place_test.go`

```go
package course_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
)

func TestPlaceKeyAppendsToEmptyList(t *testing.T) {
	key, err := course.PlaceKey(nil, nil, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, key)
}

func TestPlaceKeyAppendsToEnd(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	key, err := course.PlaceKey(sibs, nil, nil)
	require.NoError(t, err)
	assert.Greater(t, key, "B", "appending must sort after the last sibling")
}

func TestPlaceKeyBeforeFirst(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	key, err := course.PlaceKey(sibs, &a, nil)
	require.NoError(t, err)
	assert.Less(t, key, "A")
}

func TestPlaceKeyAfterLast(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	key, err := course.PlaceKey(sibs, nil, &b)
	require.NoError(t, err)
	assert.Greater(t, key, "B")
}

func TestPlaceKeyBetweenTwo(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	key, err := course.PlaceKey(sibs, &b, nil)
	require.NoError(t, err)
	assert.Greater(t, key, "A")
	assert.Less(t, key, "B")
}

func TestPlaceKeyRejectsUnknownAnchor(t *testing.T) {
	a := uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}}
	stranger := uuid.New()

	_, err := course.PlaceKey(sibs, &stranger, nil)
	assert.Error(t, err)
}

func TestPlaceKeyRejectsBothAnchors(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	_, err := course.PlaceKey(sibs, &a, &b)
	assert.Error(t, err, "before and after are mutually exclusive")
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd api && go test ./internal/course/ -run TestPlaceKey -v`
Expected: FAIL — `undefined: course.PlaceKey`.

- [ ] **Step 3: Write `api/internal/course/place.go`**

```go
package course

import (
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/rank"
)

// Ordered is a sibling in a sorted list.
type Ordered struct {
	ID      uuid.UUID
	SortKey string
}

// PlaceKey computes the sort key for an item placed relative to its siblings.
// sibs must already be sorted by SortKey ascending.
//   - before != nil: immediately before that sibling
//   - after  != nil: immediately after that sibling
//   - both nil:      at the end
//
// The client names a neighbour and the server does the arithmetic, so ranking
// logic lives in one place and two tabs cannot mint colliding keys.
func PlaceKey(sibs []Ordered, before, after *uuid.UUID) (string, error) {
	if before != nil && after != nil {
		return "", apperr.Invalid("before", "before and after are mutually exclusive")
	}

	if before == nil && after == nil {
		prev := ""
		if len(sibs) > 0 {
			prev = sibs[len(sibs)-1].SortKey
		}
		if prev == "" {
			return rank.First(), nil
		}
		return rank.Between(prev, "")
	}

	anchor := before
	field := "before"
	if anchor == nil {
		anchor = after
		field = "after"
	}

	idx := -1
	for i, s := range sibs {
		if s.ID == *anchor {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", apperr.Invalid(field, "no sibling with id %s", *anchor)
	}

	if before != nil {
		prev := ""
		if idx > 0 {
			prev = sibs[idx-1].SortKey
		}
		return rank.Between(prev, sibs[idx].SortKey)
	}

	next := ""
	if idx+1 < len(sibs) {
		next = sibs[idx+1].SortKey
	}
	return rank.Between(sibs[idx].SortKey, next)
}
```

- [ ] **Step 4: Run and confirm the placement tests pass**

Run: `cd api && go test ./internal/course/ -run TestPlaceKey -v`
Expected: all seven PASS.

- [ ] **Step 5: Write `api/db/queries/topics.sql`**

```sql
-- name: CreateTopic :one
INSERT INTO topics (course_id, title, sort_key) VALUES ($1, $2, $3) RETURNING *;

-- name: ListTopicsByCourse :many
SELECT * FROM topics
WHERE course_id = $1 AND archived_at IS NULL
ORDER BY sort_key;

-- name: GetTopicByID :one
SELECT * FROM topics WHERE id = $1 AND archived_at IS NULL;

-- name: UpdateTopicTitle :one
UPDATE topics SET title = $2 WHERE id = $1 AND archived_at IS NULL RETURNING *;

-- name: ArchiveTopic :execrows
UPDATE topics SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;
```

- [ ] **Step 6: Write `api/db/queries/modules.sql`**

```sql
-- name: CreateModule :one
INSERT INTO modules (course_id, title, sort_key) VALUES ($1, $2, $3) RETURNING *;

-- name: ListModulesByCourse :many
SELECT * FROM modules
WHERE course_id = $1 AND archived_at IS NULL
ORDER BY sort_key;

-- name: GetModuleByID :one
SELECT * FROM modules WHERE id = $1 AND archived_at IS NULL;

-- name: UpdateModuleTitle :one
UPDATE modules SET title = $2 WHERE id = $1 AND archived_at IS NULL RETURNING *;

-- name: UpdateModuleSortKey :execrows
UPDATE modules SET sort_key = $2 WHERE id = $1 AND archived_at IS NULL;

-- name: ArchiveModule :execrows
UPDATE modules SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;
```

- [ ] **Step 7: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 8: Write the failing topics test** — `api/internal/course/topics_test.go`

```go
package course_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

// fixture creates a service, an owner, a stranger and one course.
func fixture(t *testing.T) (svc *course.Service, ctx context.Context, owner, stranger, courseID uuid.UUID) {
	t.Helper()
	c, runner := pgtest.NewRunner(t)
	svc = course.New(runner)

	store := user.Store{}
	require.NoError(t, runner.Tx(c, func(q *sqlcgen.Queries) error {
		var err error
		// Built outside the closure: a composite literal in an `if` header is
		// ambiguous to the Go parser.
		if owner, err = store.EnsureBySubject(c, q, "dev|owner", "owner"); err != nil {
			return err
		}
		stranger, err = store.EnsureBySubject(c, q, "dev|stranger", "stranger")
		return err
	}))

	created, err := svc.Create(c, owner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)
	return svc, c, owner, stranger, created.ID
}

func TestCreateTopicsKeepInsertionOrder(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	for _, title := range []string{"Atoms", "Bonding", "Stoichiometry"} {
		_, err := svc.CreateTopic(ctx, owner, courseID, title)
		require.NoError(t, err)
	}

	list, err := svc.ListTopics(ctx, owner, courseID)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "Atoms", list[0].Title)
	assert.Equal(t, "Bonding", list[1].Title)
	assert.Equal(t, "Stoichiometry", list[2].Title)
}

func TestCreateTopicRejectsEmptyTitle(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)
	_, err := svc.CreateTopic(ctx, owner, courseID, "  ")
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestCreateTopicRejectsStranger(t *testing.T) {
	svc, ctx, _, stranger, courseID := fixture(t)
	_, err := svc.CreateTopic(ctx, stranger, courseID, "Atoms")
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}

func TestArchiveTopicRemovesItFromList(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	topic, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)
	require.NoError(t, svc.ArchiveTopic(ctx, owner, topic.ID))

	list, err := svc.ListTopics(ctx, owner, courseID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestArchiveTopicRejectsStranger(t *testing.T) {
	svc, ctx, owner, stranger, courseID := fixture(t)

	topic, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)

	assert.True(t, errors.Is(svc.ArchiveTopic(ctx, stranger, topic.ID), apperr.ErrForbidden))
}

func TestUpdateTopicTitle(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	topic, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)

	updated, err := svc.UpdateTopic(ctx, owner, topic.ID, "Atomic structure")
	require.NoError(t, err)
	assert.Equal(t, "Atomic structure", updated.Title)
}
```

`fixture` is shared by every later test in this package — topics, modules, blocks, questions and the outline all build on it.

- [ ] **Step 9: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: svc.CreateTopic`.

- [ ] **Step 10: Write `api/internal/course/topics.go`**

```go
package course

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Topic struct {
	ID       uuid.UUID
	CourseID uuid.UUID
	Title    string
	SortKey  string
}

func (s *Service) CreateTopic(ctx context.Context, userID, courseID uuid.UUID, title string) (Topic, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Topic{}, apperr.Invalid("title", "must not be empty")
	}

	var out Topic
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
			return err
		}
		sibs, err := s.topicSiblings(ctx, q, courseID)
		if err != nil {
			return err
		}
		key, err := PlaceKey(sibs, nil, nil)
		if err != nil {
			return err
		}
		row, err := q.CreateTopic(ctx, sqlcgen.CreateTopicParams{
			CourseID: courseID,
			Title:    title,
			SortKey:  key,
		})
		if err != nil {
			return fmt.Errorf("create topic: %w", err)
		}
		out = toTopic(row)
		return nil
	})
	return out, err
}

func (s *Service) ListTopics(ctx context.Context, userID, courseID uuid.UUID) ([]Topic, error) {
	out := []Topic{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
			return err
		}
		rows, err := q.ListTopicsByCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("list topics: %w", err)
		}
		for _, row := range rows {
			out = append(out, toTopic(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) UpdateTopic(ctx context.Context, userID, topicID uuid.UUID, title string) (Topic, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Topic{}, apperr.Invalid("title", "must not be empty")
	}

	var out Topic
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		existing, err := s.loadTopic(ctx, q, userID, topicID)
		if err != nil {
			return err
		}
		row, err := q.UpdateTopicTitle(ctx, sqlcgen.UpdateTopicTitleParams{
			ID:    existing.ID,
			Title: title,
		})
		if err != nil {
			return fmt.Errorf("update topic: %w", err)
		}
		out = toTopic(row)
		return nil
	})
	return out, err
}

func (s *Service) ArchiveTopic(ctx context.Context, userID, topicID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadTopic(ctx, q, userID, topicID); err != nil {
			return err
		}
		n, err := q.ArchiveTopic(ctx, topicID)
		if err != nil {
			return fmt.Errorf("archive topic: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("topic %s", topicID)
		}
		return nil
	})
}

// loadTopic resolves a topic and checks the caller owns its course.
func (s *Service) loadTopic(ctx context.Context, q *sqlcgen.Queries, userID, topicID uuid.UUID) (sqlcgen.Topic, error) {
	row, err := q.GetTopicByID(ctx, topicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Topic{}, apperr.NotFound("topic %s", topicID)
	}
	if err != nil {
		return sqlcgen.Topic{}, fmt.Errorf("get topic: %w", err)
	}
	if err := s.RequireOwner(ctx, q, userID, row.CourseID); err != nil {
		return sqlcgen.Topic{}, err
	}
	return row, nil
}

func (s *Service) topicSiblings(ctx context.Context, q *sqlcgen.Queries, courseID uuid.UUID) ([]Ordered, error) {
	rows, err := q.ListTopicsByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	sibs := make([]Ordered, 0, len(rows))
	for _, row := range rows {
		sibs = append(sibs, Ordered{ID: row.ID, SortKey: row.SortKey})
	}
	return sibs, nil
}

func toTopic(row sqlcgen.Topic) Topic {
	return Topic{ID: row.ID, CourseID: row.CourseID, Title: row.Title, SortKey: row.SortKey}
}
```

- [ ] **Step 11: Write `api/internal/course/modules.go`**

Identical structure, plus `MoveModule`.

```go
package course

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Module struct {
	ID       uuid.UUID
	CourseID uuid.UUID
	Title    string
	SortKey  string
}

func (s *Service) CreateModule(ctx context.Context, userID, courseID uuid.UUID, title string) (Module, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Module{}, apperr.Invalid("title", "must not be empty")
	}

	var out Module
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
			return err
		}
		sibs, err := s.moduleSiblings(ctx, q, courseID)
		if err != nil {
			return err
		}
		key, err := PlaceKey(sibs, nil, nil)
		if err != nil {
			return err
		}
		row, err := q.CreateModule(ctx, sqlcgen.CreateModuleParams{
			CourseID: courseID, Title: title, SortKey: key,
		})
		if err != nil {
			return fmt.Errorf("create module: %w", err)
		}
		out = toModule(row)
		return nil
	})
	return out, err
}

func (s *Service) ListModules(ctx context.Context, userID, courseID uuid.UUID) ([]Module, error) {
	out := []Module{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
			return err
		}
		rows, err := q.ListModulesByCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("list modules: %w", err)
		}
		for _, row := range rows {
			out = append(out, toModule(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) UpdateModule(ctx context.Context, userID, moduleID uuid.UUID, title string) (Module, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Module{}, apperr.Invalid("title", "must not be empty")
	}

	var out Module
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		existing, err := s.loadModule(ctx, q, userID, moduleID)
		if err != nil {
			return err
		}
		row, err := q.UpdateModuleTitle(ctx, sqlcgen.UpdateModuleTitleParams{
			ID: existing.ID, Title: title,
		})
		if err != nil {
			return fmt.Errorf("update module: %w", err)
		}
		out = toModule(row)
		return nil
	})
	return out, err
}

func (s *Service) ArchiveModule(ctx context.Context, userID, moduleID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadModule(ctx, q, userID, moduleID); err != nil {
			return err
		}
		n, err := q.ArchiveModule(ctx, moduleID)
		if err != nil {
			return fmt.Errorf("archive module: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("module %s", moduleID)
		}
		return nil
	})
}

// MoveModule repositions a module relative to a sibling. Exactly one of before
// or after must be set; both nil moves it to the end.
func (s *Service) MoveModule(ctx context.Context, userID, moduleID uuid.UUID, before, after *uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		row, err := s.loadModule(ctx, q, userID, moduleID)
		if err != nil {
			return err
		}
		sibs, err := s.moduleSiblings(ctx, q, row.CourseID)
		if err != nil {
			return err
		}
		// Exclude the module being moved: it must not anchor against itself.
		filtered := make([]Ordered, 0, len(sibs))
		for _, sib := range sibs {
			if sib.ID != moduleID {
				filtered = append(filtered, sib)
			}
		}
		if before != nil && *before == moduleID {
			return apperr.Invalid("before", "cannot move a module relative to itself")
		}
		if after != nil && *after == moduleID {
			return apperr.Invalid("after", "cannot move a module relative to itself")
		}

		key, err := PlaceKey(filtered, before, after)
		if err != nil {
			return err
		}
		n, err := q.UpdateModuleSortKey(ctx, sqlcgen.UpdateModuleSortKeyParams{
			ID: moduleID, SortKey: key,
		})
		if err != nil {
			return fmt.Errorf("move module: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("module %s", moduleID)
		}
		return nil
	})
}

func (s *Service) loadModule(ctx context.Context, q *sqlcgen.Queries, userID, moduleID uuid.UUID) (sqlcgen.Module, error) {
	row, err := q.GetModuleByID(ctx, moduleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Module{}, apperr.NotFound("module %s", moduleID)
	}
	if err != nil {
		return sqlcgen.Module{}, fmt.Errorf("get module: %w", err)
	}
	if err := s.RequireOwner(ctx, q, userID, row.CourseID); err != nil {
		return sqlcgen.Module{}, err
	}
	return row, nil
}

func (s *Service) moduleSiblings(ctx context.Context, q *sqlcgen.Queries, courseID uuid.UUID) ([]Ordered, error) {
	rows, err := q.ListModulesByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}
	sibs := make([]Ordered, 0, len(rows))
	for _, row := range rows {
		sibs = append(sibs, Ordered{ID: row.ID, SortKey: row.SortKey})
	}
	return sibs, nil
}

func toModule(row sqlcgen.Module) Module {
	return Module{ID: row.ID, CourseID: row.CourseID, Title: row.Title, SortKey: row.SortKey}
}
```

- [ ] **Step 12: Write the module reorder test** — `api/internal/course/modules_test.go`

```go
package course_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
)

func titles(mods []course.Module) []string {
	out := make([]string, 0, len(mods))
	for _, m := range mods {
		out = append(out, m.Title)
	}
	return out
}

func TestMoveModuleToFront(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	var created []course.Module
	for _, title := range []string{"One", "Two", "Three"} {
		m, err := svc.CreateModule(ctx, owner, courseID, title)
		require.NoError(t, err)
		created = append(created, m)
	}

	first := created[0].ID
	require.NoError(t, svc.MoveModule(ctx, owner, created[2].ID, &first, nil))

	list, err := svc.ListModules(ctx, owner, courseID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Three", "One", "Two"}, titles(list))
}

func TestMoveModuleToEnd(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	var created []course.Module
	for _, title := range []string{"One", "Two", "Three"} {
		m, err := svc.CreateModule(ctx, owner, courseID, title)
		require.NoError(t, err)
		created = append(created, m)
	}

	require.NoError(t, svc.MoveModule(ctx, owner, created[0].ID, nil, nil))

	list, err := svc.ListModules(ctx, owner, courseID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Two", "Three", "One"}, titles(list))
}

func TestMoveModuleRejectsSelfAnchor(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	m, err := svc.CreateModule(ctx, owner, courseID, "One")
	require.NoError(t, err)

	assert.Error(t, svc.MoveModule(ctx, owner, m.ID, &m.ID, nil))
}

func TestModuleReorderWritesOneRow(t *testing.T) {
	// Repeated reordering must stay correct; sort keys grow, order does not break.
	svc, ctx, owner, _, courseID := fixture(t)

	var created []course.Module
	for _, title := range []string{"A", "B", "C", "D"} {
		m, err := svc.CreateModule(ctx, owner, courseID, title)
		require.NoError(t, err)
		created = append(created, m)
	}

	head := created[0].ID
	for i := 0; i < 20; i++ {
		require.NoError(t, svc.MoveModule(ctx, owner, created[3].ID, &head, nil))
		list, err := svc.ListModules(ctx, owner, courseID)
		require.NoError(t, err)
		require.Equal(t, "D", list[0].Title, "iteration %d", i)
		head = list[1].ID
	}
}
```

- [ ] **Step 13: Run and confirm all topic and module tests pass**

Run: `make test`
Expected: PASS.

- [ ] **Step 14: Write `api/cmd/api/handlers_structure.go`**

```go
package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type namedBody struct {
	ID      uuid.UUID `json:"id"`
	Title   string    `json:"title"`
	SortKey string    `json:"sort_key"`
}

type namedListOutput struct {
	Body struct {
		Items []namedBody `json:"items"`
	}
}

type namedOutput struct {
	Body namedBody
}

type createNamedInput struct {
	ID   uuid.UUID `path:"id" doc:"Course id"`
	Body struct {
		Title string `json:"title" maxLength:"200"`
	}
}

type renameInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		Title string `json:"title" maxLength:"200"`
	}
}

type idOnlyInput struct {
	ID uuid.UUID `path:"id"`
}

type moveInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		Before *uuid.UUID `json:"before,omitempty" doc:"Place immediately before this sibling"`
		After  *uuid.UUID `json:"after,omitempty" doc:"Place immediately after this sibling"`
	}
}

func (s *server) registerStructureRoutes(api huma.API) {
	svc := course.New(s.runner)

	// ---- Topics ----

	huma.Register(api, huma.Operation{
		OperationID: "createTopic", Method: http.MethodPost, Path: "/courses/{id}/topics",
		Summary: "Add a topic to a course", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createNamedInput) (*namedOutput, error) {
		topic, err := svc.CreateTopic(ctx, auth.MustUser(ctx), in.ID, in.Body.Title)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &namedOutput{Body: namedBody{ID: topic.ID, Title: topic.Title, SortKey: topic.SortKey}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listTopics", Method: http.MethodGet, Path: "/courses/{id}/topics",
		Summary: "List a course's topics",
	}, func(ctx context.Context, in *idOnlyInput) (*namedListOutput, error) {
		list, err := svc.ListTopics(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &namedListOutput{}
		out.Body.Items = make([]namedBody, 0, len(list))
		for _, topic := range list {
			out.Body.Items = append(out.Body.Items, namedBody{ID: topic.ID, Title: topic.Title, SortKey: topic.SortKey})
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateTopic", Method: http.MethodPatch, Path: "/topics/{id}",
		Summary: "Rename a topic",
	}, func(ctx context.Context, in *renameInput) (*namedOutput, error) {
		topic, err := svc.UpdateTopic(ctx, auth.MustUser(ctx), in.ID, in.Body.Title)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &namedOutput{Body: namedBody{ID: topic.ID, Title: topic.Title, SortKey: topic.SortKey}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveTopic", Method: http.MethodPost, Path: "/topics/{id}/archive",
		Summary: "Archive a topic", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idOnlyInput) (*struct{}, error) {
		if err := svc.ArchiveTopic(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})

	// ---- Modules ----

	huma.Register(api, huma.Operation{
		OperationID: "createModule", Method: http.MethodPost, Path: "/courses/{id}/modules",
		Summary: "Add a module to a course", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createNamedInput) (*namedOutput, error) {
		mod, err := svc.CreateModule(ctx, auth.MustUser(ctx), in.ID, in.Body.Title)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &namedOutput{Body: namedBody{ID: mod.ID, Title: mod.Title, SortKey: mod.SortKey}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listModules", Method: http.MethodGet, Path: "/courses/{id}/modules",
		Summary: "List a course's modules",
	}, func(ctx context.Context, in *idOnlyInput) (*namedListOutput, error) {
		list, err := svc.ListModules(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &namedListOutput{}
		out.Body.Items = make([]namedBody, 0, len(list))
		for _, mod := range list {
			out.Body.Items = append(out.Body.Items, namedBody{ID: mod.ID, Title: mod.Title, SortKey: mod.SortKey})
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateModule", Method: http.MethodPatch, Path: "/modules/{id}",
		Summary: "Rename a module",
	}, func(ctx context.Context, in *renameInput) (*namedOutput, error) {
		mod, err := svc.UpdateModule(ctx, auth.MustUser(ctx), in.ID, in.Body.Title)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &namedOutput{Body: namedBody{ID: mod.ID, Title: mod.Title, SortKey: mod.SortKey}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveModule", Method: http.MethodPost, Path: "/modules/{id}/archive",
		Summary: "Archive a module", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idOnlyInput) (*struct{}, error) {
		if err := svc.ArchiveModule(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "moveModule", Method: http.MethodPost, Path: "/modules/{id}/move",
		Summary: "Reorder a module", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *moveInput) (*struct{}, error) {
		if err := svc.MoveModule(ctx, auth.MustUser(ctx), in.ID, in.Body.Before, in.Body.After); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})
}
```

- [ ] **Step 15: Register the routes**

In `registerRoutes`, add `s.registerStructureRoutes(api)` after `s.registerCourseRoutes(api)`.

- [ ] **Step 16: Write the handler test** — `api/cmd/api/handlers_structure_test.go`

```go
package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicLifecycleOverHTTP(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	created := api.Post("/courses/"+courseID+"/topics", map[string]any{"title": "Atoms"})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	topicID := idFromBody(t, created.Body.Bytes())

	list := api.Get("/courses/" + courseID + "/topics")
	require.Equal(t, http.StatusOK, list.Code)
	assert.Contains(t, list.Body.String(), `"title":"Atoms"`)

	renamed := api.Patch("/topics/"+topicID, map[string]any{"title": "Atomic structure"})
	require.Equal(t, http.StatusOK, renamed.Code)
	assert.Contains(t, renamed.Body.String(), "Atomic structure")

	assert.Equal(t, http.StatusNoContent, api.Post("/topics/"+topicID+"/archive").Code)

	after := api.Get("/courses/" + courseID + "/topics")
	assert.NotContains(t, after.Body.String(), "Atomic structure")
}

func TestModuleMoveOverHTTP(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	ids := []string{}
	for _, title := range []string{"One", "Two", "Three"} {
		resp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": title})
		require.Equal(t, http.StatusCreated, resp.Code)
		ids = append(ids, idFromBody(t, resp.Body.Bytes()))
	}

	moved := api.Post("/modules/"+ids[2]+"/move", map[string]any{"before": ids[0]})
	require.Equal(t, http.StatusNoContent, moved.Code, moved.Body.String())

	list := api.Get("/courses/" + courseID + "/modules")
	require.Equal(t, http.StatusOK, list.Code)
	body := list.Body.String()
	assert.Less(t, indexOfTitle(body, "Three"), indexOfTitle(body, "One"))
}

func TestTopicOnMissingCourseReturns404(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	resp := api.Post("/courses/"+uuid.New().String()+"/topics", map[string]any{"title": "X"})
	assert.Equal(t, http.StatusNotFound, resp.Code)
}
```

`newCourse` and `indexOfTitle` already exist in `testhelpers_test.go` from Task 7. Add the imports this file needs: `"net/http"`, `"testing"`, `"github.com/google/uuid"`, and testify's `assert` and `require`.

- [ ] **Step 17: Run the tests, regenerate, commit**

```bash
make test && make generate
git add -A && git commit -m "feat(api): topics and modules with sort-key reordering"
```

---

### Task 9: Blocks and the per-type validation registry

This is the extensibility claim of the whole design. A block type is defined in exactly one place — a registry entry carrying its content schema, its default completion rule, and its validator. Adding type 18 must mean adding one entry, not editing a switch.

**Files:**
- Create: `api/db/queries/blocks.sql`
- Create: `api/internal/course/blockspec.go`, `api/internal/course/blockspec_test.go`
- Create: `api/internal/course/blocks.go`, `api/internal/course/blocks_test.go`
- Create: `api/cmd/api/handlers_blocks.go`, `api/cmd/api/handlers_blocks_test.go`
- Modify: `api/cmd/api/main.go` (`registerRoutes`)

**Interfaces:**
- Consumes: `PlaceKey`, `RequireOwner`, `loadModule`.
- Produces:
  - `course.BlockType` string constants: `BlockReading = "reading"`, `BlockVideoYouTube = "video_youtube"`, `BlockQuiz = "quiz"`
  - `course.ValidateBlock(blockType string, content, completion json.RawMessage) (json.RawMessage, json.RawMessage, error)` — returns normalised content and completion
  - `course.ReadingContent{BodyMD string}`, `course.VideoContent{VideoID string}`, `course.QuizCompletion{Kind string, Threshold int}`
  - `(*Service).CreateBlock(ctx, userID, moduleID uuid.UUID, in CreateBlockInput) (Block, error)`
  - `(*Service).UpdateBlock(ctx, userID, blockID uuid.UUID, in UpdateBlockInput) (Block, error)`
  - `(*Service).ArchiveBlock(ctx, userID, blockID uuid.UUID) error`
  - `(*Service).MoveBlock(ctx, userID, blockID uuid.UUID, before, after *uuid.UUID) error`
  - `(*Service).loadBlock(ctx, q, userID, blockID) (sqlcgen.Block, error)` — reused by Tasks 10 and 11
  - `Block{ID, ModuleID uuid.UUID, Title, Type, SortKey string, Content, Completion json.RawMessage}`

- [ ] **Step 1: Write the failing registry test** — `api/internal/course/blockspec_test.go`

```go
package course_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func TestValidateBlockRejectsUnknownType(t *testing.T) {
	_, _, err := course.ValidateBlock("hologram", json.RawMessage(`{}`), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestReadingBlockRequiresBody(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockReading, json.RawMessage(`{"body_md":""}`), nil)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestReadingBlockGetsDefaultCompletion(t *testing.T) {
	_, completion, err := course.ValidateBlock(
		course.BlockReading, json.RawMessage(`{"body_md":"# Atoms"}`), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"mark_done"}`, string(completion))
}

func TestVideoBlockRequiresVideoID(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockVideoYouTube, json.RawMessage(`{}`), nil)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestVideoBlockDefaultsToWatchedToEnd(t *testing.T) {
	_, completion, err := course.ValidateBlock(
		course.BlockVideoYouTube, json.RawMessage(`{"video_id":"dQw4w9WgXcQ"}`), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"watched_to_end"}`, string(completion))
}

func TestQuizBlockDefaultsToSeventyPercent(t *testing.T) {
	_, completion, err := course.ValidateBlock(course.BlockQuiz, json.RawMessage(`{}`), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"score_threshold","threshold":70}`, string(completion))
}

func TestQuizThresholdMustBeAPercentage(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockQuiz, json.RawMessage(`{}`),
		json.RawMessage(`{"kind":"score_threshold","threshold":150}`))
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"threshold is a 0-100 percentage, in the same units as block_progress.score")
}

func TestQuizThresholdAcceptsValidValue(t *testing.T) {
	_, completion, err := course.ValidateBlock(course.BlockQuiz, json.RawMessage(`{}`),
		json.RawMessage(`{"kind":"score_threshold","threshold":80}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"score_threshold","threshold":80}`, string(completion))
}

func TestCompletionKindMustMatchBlockType(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockReading,
		json.RawMessage(`{"body_md":"x"}`),
		json.RawMessage(`{"kind":"score_threshold","threshold":70}`))
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestValidateBlockRejectsUnknownContentFields(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockReading,
		json.RawMessage(`{"body_md":"x","sneaky":true}`), nil)
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"unknown fields are rejected so typos do not silently vanish into jsonb")
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd api && go test ./internal/course/ -run TestValidateBlock -v`
Expected: FAIL — `undefined: course.ValidateBlock`.

- [ ] **Step 3: Write `api/internal/course/blockspec.go`**

```go
package course

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

// Block types. Adding a type means adding one registry entry below and one
// React component in the web app — nothing else in the codebase changes.
const (
	BlockReading      = "reading"
	BlockVideoYouTube = "video_youtube"
	BlockQuiz         = "quiz"
)

type ReadingContent struct {
	BodyMD string `json:"body_md"`
}

type VideoContent struct {
	VideoID string `json:"video_id"`
}

// MarkDoneCompletion covers reading blocks: the learner says they are done.
type MarkDoneCompletion struct {
	Kind string `json:"kind"`
}

// WatchedCompletion covers YouTube blocks: the player reports the end event.
type WatchedCompletion struct {
	Kind string `json:"kind"`
}

// QuizCompletion is satisfied when the graded score reaches Threshold.
// Threshold is a percentage 0-100, the same units as block_progress.score.
type QuizCompletion struct {
	Kind      string `json:"kind"`
	Threshold int    `json:"threshold"`
}

type blockSpec struct {
	// validateContent parses and normalises the block's content.
	validateContent func(raw json.RawMessage) (json.RawMessage, error)
	// defaultCompletion is used when the caller supplies none.
	defaultCompletion func() json.RawMessage
	// validateCompletion parses and normalises a supplied completion rule.
	validateCompletion func(raw json.RawMessage) (json.RawMessage, error)
}

var blockSpecs = map[string]blockSpec{
	BlockReading: {
		validateContent: func(raw json.RawMessage) (json.RawMessage, error) {
			var c ReadingContent
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("content", "%s", err)
			}
			if c.BodyMD == "" {
				return nil, apperr.Invalid("content.body_md", "must not be empty")
			}
			return remarshal(c)
		},
		defaultCompletion: func() json.RawMessage {
			return json.RawMessage(`{"kind":"mark_done"}`)
		},
		validateCompletion: func(raw json.RawMessage) (json.RawMessage, error) {
			var c MarkDoneCompletion
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("completion", "%s", err)
			}
			if c.Kind != "mark_done" {
				return nil, apperr.Invalid("completion.kind", "a reading block completes with mark_done")
			}
			return remarshal(c)
		},
	},

	BlockVideoYouTube: {
		validateContent: func(raw json.RawMessage) (json.RawMessage, error) {
			var c VideoContent
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("content", "%s", err)
			}
			if c.VideoID == "" {
				return nil, apperr.Invalid("content.video_id", "must not be empty")
			}
			return remarshal(c)
		},
		defaultCompletion: func() json.RawMessage {
			return json.RawMessage(`{"kind":"watched_to_end"}`)
		},
		validateCompletion: func(raw json.RawMessage) (json.RawMessage, error) {
			var c WatchedCompletion
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("completion", "%s", err)
			}
			if c.Kind != "watched_to_end" {
				return nil, apperr.Invalid("completion.kind", "a video block completes with watched_to_end")
			}
			return remarshal(c)
		},
	},

	BlockQuiz: {
		validateContent: func(raw json.RawMessage) (json.RawMessage, error) {
			// Quiz content is empty: the questions live in their own table.
			var c struct{}
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("content", "a quiz block carries no content fields")
			}
			return json.RawMessage(`{}`), nil
		},
		defaultCompletion: func() json.RawMessage {
			return json.RawMessage(`{"kind":"score_threshold","threshold":70}`)
		},
		validateCompletion: func(raw json.RawMessage) (json.RawMessage, error) {
			var c QuizCompletion
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("completion", "%s", err)
			}
			if c.Kind != "score_threshold" {
				return nil, apperr.Invalid("completion.kind", "a quiz block completes with score_threshold")
			}
			if c.Threshold < 0 || c.Threshold > 100 {
				return nil, apperr.Invalid("completion.threshold", "must be between 0 and 100")
			}
			return remarshal(c)
		},
	},
}

// ValidateBlock checks a block's content and completion rule against its type,
// returning both normalised. A nil or empty completion gets the type's default.
func ValidateBlock(blockType string, content, completion json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	spec, ok := blockSpecs[blockType]
	if !ok {
		return nil, nil, apperr.Invalid("type", "unknown block type %q", blockType)
	}

	if len(content) == 0 {
		content = json.RawMessage(`{}`)
	}
	normalisedContent, err := spec.validateContent(content)
	if err != nil {
		return nil, nil, err
	}

	if len(completion) == 0 || bytes.Equal(bytes.TrimSpace(completion), []byte("null")) {
		return normalisedContent, spec.defaultCompletion(), nil
	}
	normalisedCompletion, err := spec.validateCompletion(completion)
	if err != nil {
		return nil, nil, err
	}
	return normalisedContent, normalisedCompletion, nil
}

// KnownBlockType reports whether a type exists, for use by handlers.
func KnownBlockType(blockType string) bool {
	_, ok := blockSpecs[blockType]
	return ok
}

// strictUnmarshal rejects unknown fields, so a typo in a content payload is an
// error rather than a value silently dropped into jsonb.
func strictUnmarshal(raw json.RawMessage, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return err
	}
	return nil
}

func remarshal(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return b, nil
}
```

- [ ] **Step 4: Run and confirm the registry tests pass**

Run: `cd api && go test ./internal/course/ -run TestValidateBlock -v` and the other spec tests.
Expected: all ten PASS.

- [ ] **Step 5: Write `api/db/queries/blocks.sql`**

```sql
-- name: CreateBlock :one
INSERT INTO blocks (module_id, title, type, sort_key, content, completion)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetBlockByID :one
SELECT * FROM blocks WHERE id = $1 AND archived_at IS NULL;

-- name: ListBlocksByModule :many
SELECT * FROM blocks
WHERE module_id = $1 AND archived_at IS NULL
ORDER BY sort_key;

-- name: UpdateBlock :one
UPDATE blocks SET
    title      = COALESCE(sqlc.narg('title'), title),
    content    = COALESCE(sqlc.narg('content'), content),
    completion = COALESCE(sqlc.narg('completion'), completion)
WHERE id = sqlc.arg('id') AND archived_at IS NULL
RETURNING *;

-- name: UpdateBlockSortKey :execrows
UPDATE blocks SET sort_key = $2 WHERE id = $1 AND archived_at IS NULL;

-- name: ArchiveBlock :execrows
UPDATE blocks SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;

-- name: GetBlockCourseID :one
SELECT m.course_id
FROM blocks b
JOIN modules m ON m.id = b.module_id
WHERE b.id = $1 AND b.archived_at IS NULL AND m.archived_at IS NULL;
```

- [ ] **Step 6: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 7: Write the failing blocks test** — `api/internal/course/blocks_test.go`

```go
package course_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

// moduleFixture extends fixture with one module.
func moduleFixture(t *testing.T) (*course.Service, context.Context, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	svc, ctx, owner, stranger, courseID := fixture(t)
	mod, err := svc.CreateModule(ctx, owner, courseID, "Module 1")
	require.NoError(t, err)
	return svc, ctx, owner, stranger, mod.ID
}

func TestCreateReadingBlock(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	block, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title:   "What is an atom",
		Type:    course.BlockReading,
		Content: json.RawMessage(`{"body_md":"# Atoms"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, course.BlockReading, block.Type)
	assert.JSONEq(t, `{"kind":"mark_done"}`, string(block.Completion))
}

func TestCreateBlockRejectsInvalidContent(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	_, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title:   "Broken",
		Type:    course.BlockVideoYouTube,
		Content: json.RawMessage(`{}`),
	})
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestCreateBlockRejectsStranger(t *testing.T) {
	svc, ctx, _, stranger, moduleID := moduleFixture(t)

	_, err := svc.CreateBlock(ctx, stranger, moduleID, course.CreateBlockInput{
		Title:   "Sneaky",
		Type:    course.BlockReading,
		Content: json.RawMessage(`{"body_md":"x"}`),
	})
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}

func TestUpdateBlockRevalidatesContent(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	block, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title:   "Video",
		Type:    course.BlockVideoYouTube,
		Content: json.RawMessage(`{"video_id":"abc"}`),
	})
	require.NoError(t, err)

	empty := json.RawMessage(`{"video_id":""}`)
	_, err = svc.UpdateBlock(ctx, owner, block.ID, course.UpdateBlockInput{Content: empty})
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"an update must re-run the type's validator, not just write jsonb")
}

func TestUpdateBlockChangesThreshold(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	block, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title: "Quiz", Type: course.BlockQuiz,
	})
	require.NoError(t, err)

	completion := json.RawMessage(`{"kind":"score_threshold","threshold":90}`)
	updated, err := svc.UpdateBlock(ctx, owner, block.ID, course.UpdateBlockInput{Completion: completion})
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"score_threshold","threshold":90}`, string(updated.Completion))
}

func TestMoveBlockReorders(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	var ids []uuid.UUID
	for _, title := range []string{"A", "B", "C"} {
		b, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
			Title: title, Type: course.BlockReading,
			Content: json.RawMessage(`{"body_md":"x"}`),
		})
		require.NoError(t, err)
		ids = append(ids, b.ID)
	}

	require.NoError(t, svc.MoveBlock(ctx, owner, ids[2], &ids[0], nil))

	blocks, err := svc.ListBlocks(ctx, owner, moduleID)
	require.NoError(t, err)
	assert.Equal(t, "C", blocks[0].Title)
}

func TestArchiveBlockHidesIt(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	block, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title: "A", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	require.NoError(t, svc.ArchiveBlock(ctx, owner, block.ID))

	blocks, err := svc.ListBlocks(ctx, owner, moduleID)
	require.NoError(t, err)
	assert.Empty(t, blocks)
}
```

This file needs `"context"`, `"encoding/json"`, `"errors"`, `"testing"`, `"github.com/google/uuid"`, testify's `assert` and `require`, plus the `course` and `apperr` packages.

- [ ] **Step 8: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: svc.CreateBlock`.

- [ ] **Step 9: Write `api/internal/course/blocks.go`**

```go
package course

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Block struct {
	ID         uuid.UUID
	ModuleID   uuid.UUID
	Title      string
	Type       string
	SortKey    string
	Content    json.RawMessage
	Completion json.RawMessage
}

type CreateBlockInput struct {
	Title      string
	Type       string
	Content    json.RawMessage
	Completion json.RawMessage
}

type UpdateBlockInput struct {
	Title      *string
	Content    json.RawMessage
	Completion json.RawMessage
}

func (s *Service) CreateBlock(ctx context.Context, userID, moduleID uuid.UUID, in CreateBlockInput) (Block, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Block{}, apperr.Invalid("title", "must not be empty")
	}
	content, completion, err := ValidateBlock(in.Type, in.Content, in.Completion)
	if err != nil {
		return Block{}, err
	}

	var out Block
	err = s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadModule(ctx, q, userID, moduleID); err != nil {
			return err
		}
		sibs, err := s.blockSiblings(ctx, q, moduleID)
		if err != nil {
			return err
		}
		key, err := PlaceKey(sibs, nil, nil)
		if err != nil {
			return err
		}
		row, err := q.CreateBlock(ctx, sqlcgen.CreateBlockParams{
			ModuleID:   moduleID,
			Title:      title,
			Type:       in.Type,
			SortKey:    key,
			Content:    content,
			Completion: completion,
		})
		if err != nil {
			return fmt.Errorf("create block: %w", err)
		}
		out = toBlock(row)
		return nil
	})
	return out, err
}

func (s *Service) ListBlocks(ctx context.Context, userID, moduleID uuid.UUID) ([]Block, error) {
	out := []Block{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadModule(ctx, q, userID, moduleID); err != nil {
			return err
		}
		rows, err := q.ListBlocksByModule(ctx, moduleID)
		if err != nil {
			return fmt.Errorf("list blocks: %w", err)
		}
		for _, row := range rows {
			out = append(out, toBlock(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) UpdateBlock(ctx context.Context, userID, blockID uuid.UUID, in UpdateBlockInput) (Block, error) {
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return Block{}, apperr.Invalid("title", "must not be empty")
	}

	var out Block
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		existing, err := s.loadBlock(ctx, q, userID, blockID)
		if err != nil {
			return err
		}

		// Re-validate against the block's own type. Whichever of content and
		// completion is absent falls back to what is stored, so a partial
		// update can never leave an invalid pair behind.
		content := in.Content
		if len(content) == 0 {
			content = existing.Content
		}
		completion := in.Completion
		if len(completion) == 0 {
			completion = existing.Completion
		}
		validContent, validCompletion, err := ValidateBlock(existing.Type, content, completion)
		if err != nil {
			return err
		}

		row, err := q.UpdateBlock(ctx, sqlcgen.UpdateBlockParams{
			ID:         blockID,
			Title:      trimPtr(in.Title),
			Content:    validContent,
			Completion: validCompletion,
		})
		if err != nil {
			return fmt.Errorf("update block: %w", err)
		}
		out = toBlock(row)
		return nil
	})
	return out, err
}

func (s *Service) ArchiveBlock(ctx context.Context, userID, blockID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadBlock(ctx, q, userID, blockID); err != nil {
			return err
		}
		n, err := q.ArchiveBlock(ctx, blockID)
		if err != nil {
			return fmt.Errorf("archive block: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("block %s", blockID)
		}
		return nil
	})
}

func (s *Service) MoveBlock(ctx context.Context, userID, blockID uuid.UUID, before, after *uuid.UUID) error {
	if before != nil && *before == blockID {
		return apperr.Invalid("before", "cannot move a block relative to itself")
	}
	if after != nil && *after == blockID {
		return apperr.Invalid("after", "cannot move a block relative to itself")
	}

	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		row, err := s.loadBlock(ctx, q, userID, blockID)
		if err != nil {
			return err
		}
		sibs, err := s.blockSiblings(ctx, q, row.ModuleID)
		if err != nil {
			return err
		}
		filtered := make([]Ordered, 0, len(sibs))
		for _, sib := range sibs {
			if sib.ID != blockID {
				filtered = append(filtered, sib)
			}
		}
		key, err := PlaceKey(filtered, before, after)
		if err != nil {
			return err
		}
		n, err := q.UpdateBlockSortKey(ctx, sqlcgen.UpdateBlockSortKeyParams{ID: blockID, SortKey: key})
		if err != nil {
			return fmt.Errorf("move block: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("block %s", blockID)
		}
		return nil
	})
}

// loadBlock resolves a block and checks the caller owns the course it sits in.
func (s *Service) loadBlock(ctx context.Context, q *sqlcgen.Queries, userID, blockID uuid.UUID) (sqlcgen.Block, error) {
	row, err := q.GetBlockByID(ctx, blockID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Block{}, apperr.NotFound("block %s", blockID)
	}
	if err != nil {
		return sqlcgen.Block{}, fmt.Errorf("get block: %w", err)
	}
	courseID, err := q.GetBlockCourseID(ctx, blockID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Block{}, apperr.NotFound("block %s", blockID)
	}
	if err != nil {
		return sqlcgen.Block{}, fmt.Errorf("get block course: %w", err)
	}
	if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
		return sqlcgen.Block{}, err
	}
	return row, nil
}

func (s *Service) blockSiblings(ctx context.Context, q *sqlcgen.Queries, moduleID uuid.UUID) ([]Ordered, error) {
	rows, err := q.ListBlocksByModule(ctx, moduleID)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	sibs := make([]Ordered, 0, len(rows))
	for _, row := range rows {
		sibs = append(sibs, Ordered{ID: row.ID, SortKey: row.SortKey})
	}
	return sibs, nil
}

func toBlock(row sqlcgen.Block) Block {
	return Block{
		ID:         row.ID,
		ModuleID:   row.ModuleID,
		Title:      row.Title,
		Type:       row.Type,
		SortKey:    row.SortKey,
		Content:    row.Content,
		Completion: row.Completion,
	}
}
```

- [ ] **Step 10: Run the domain tests and confirm they pass**

Run: `make test`
Expected: PASS.

- [ ] **Step 11: Write `api/cmd/api/handlers_blocks.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type blockBody struct {
	ID         uuid.UUID       `json:"id"`
	ModuleID   uuid.UUID       `json:"module_id"`
	Title      string          `json:"title"`
	Type       string          `json:"type"`
	SortKey    string          `json:"sort_key"`
	Content    json.RawMessage `json:"content"`
	Completion json.RawMessage `json:"completion"`
}

func blockToBody(b course.Block) blockBody {
	return blockBody{
		ID: b.ID, ModuleID: b.ModuleID, Title: b.Title, Type: b.Type,
		SortKey: b.SortKey, Content: b.Content, Completion: b.Completion,
	}
}

type blockOutput struct {
	Body blockBody
}

type blockListOutput struct {
	Body struct {
		Blocks []blockBody `json:"blocks"`
	}
}

type createBlockInput struct {
	ID   uuid.UUID `path:"id" doc:"Module id"`
	Body struct {
		Title      string          `json:"title" maxLength:"200"`
		Type       string          `json:"type" enum:"reading,video_youtube,quiz"`
		Content    json.RawMessage `json:"content,omitempty"`
		Completion json.RawMessage `json:"completion,omitempty"`
	}
}

type updateBlockInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		Title      *string         `json:"title,omitempty" maxLength:"200"`
		Content    json.RawMessage `json:"content,omitempty"`
		Completion json.RawMessage `json:"completion,omitempty"`
	}
}

func (s *server) registerBlockRoutes(api huma.API) {
	svc := course.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "createBlock", Method: http.MethodPost, Path: "/modules/{id}/blocks",
		Summary: "Add a block to a module", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createBlockInput) (*blockOutput, error) {
		block, err := svc.CreateBlock(ctx, auth.MustUser(ctx), in.ID, course.CreateBlockInput{
			Title:      in.Body.Title,
			Type:       in.Body.Type,
			Content:    in.Body.Content,
			Completion: in.Body.Completion,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &blockOutput{Body: blockToBody(block)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listBlocks", Method: http.MethodGet, Path: "/modules/{id}/blocks",
		Summary: "List a module's blocks",
	}, func(ctx context.Context, in *idOnlyInput) (*blockListOutput, error) {
		list, err := svc.ListBlocks(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &blockListOutput{}
		out.Body.Blocks = make([]blockBody, 0, len(list))
		for _, b := range list {
			out.Body.Blocks = append(out.Body.Blocks, blockToBody(b))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateBlock", Method: http.MethodPatch, Path: "/blocks/{id}",
		Summary: "Update a block",
	}, func(ctx context.Context, in *updateBlockInput) (*blockOutput, error) {
		block, err := svc.UpdateBlock(ctx, auth.MustUser(ctx), in.ID, course.UpdateBlockInput{
			Title:      in.Body.Title,
			Content:    in.Body.Content,
			Completion: in.Body.Completion,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &blockOutput{Body: blockToBody(block)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveBlock", Method: http.MethodPost, Path: "/blocks/{id}/archive",
		Summary: "Archive a block", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idOnlyInput) (*struct{}, error) {
		if err := svc.ArchiveBlock(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "moveBlock", Method: http.MethodPost, Path: "/blocks/{id}/move",
		Summary: "Reorder a block", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *moveInput) (*struct{}, error) {
		if err := svc.MoveBlock(ctx, auth.MustUser(ctx), in.ID, in.Body.Before, in.Body.After); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})
}
```

- [ ] **Step 12: Register the routes and write the handler test**

Add `s.registerBlockRoutes(api)` to `registerRoutes`, then `api/cmd/api/handlers_blocks_test.go`:

```go
package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBlockOverHTTP(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	modResp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": "Module 1"})
	require.Equal(t, http.StatusCreated, modResp.Code)
	moduleID := idFromBody(t, modResp.Body.Bytes())

	resp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title":   "What is an atom",
		"type":    "reading",
		"content": map[string]any{"body_md": "# Atoms"},
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"kind":"mark_done"`)
}

func TestCreateBlockRejectsUnknownTypeAt422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	modResp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": "Module 1"})
	moduleID := idFromBody(t, modResp.Body.Bytes())

	resp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "X", "type": "hologram",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code,
		"huma's enum tag rejects it before the domain does")
}

func TestCreateVideoBlockWithoutVideoIDReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	modResp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": "Module 1"})
	moduleID := idFromBody(t, modResp.Body.Bytes())

	resp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Video", "type": "video_youtube", "content": map[string]any{},
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "content.video_id")
}
```

- [ ] **Step 13: Run, regenerate, commit**

```bash
make test && make generate
git add -A && git commit -m "feat(api): blocks with per-type content and completion validation"
```

---

### Task 10: Topic tagging and quiz questions

Tagging is what makes mastery per-topic possible; questions are where answer keys live and must never leak.

**Files:**
- Create: `api/db/queries/tagging.sql`, `api/db/queries/questions.sql`
- Create: `api/internal/course/tagging.go`, `api/internal/course/questions.go`
- Create: `api/internal/course/questions_test.go`
- Create: `api/cmd/api/handlers_questions.go`, `api/cmd/api/handlers_questions_test.go`
- Modify: `api/cmd/api/main.go` (`registerRoutes`)

**Interfaces:**
- Produces:
  - `(*Service).SetBlockTopics(ctx, userID, blockID uuid.UUID, topicIDs []uuid.UUID) error`
  - `(*Service).GetBlockTopics(ctx, userID, blockID uuid.UUID) ([]uuid.UUID, error)`
  - `(*Service).CreateQuestion(ctx, userID, blockID uuid.UUID, in QuestionInput) (Question, error)`
  - `(*Service).UpdateQuestion(ctx, userID, questionID uuid.UUID, in QuestionInput) (Question, error)`
  - `(*Service).ArchiveQuestion(ctx, userID, questionID uuid.UUID) error`
  - `(*Service).ListQuestionsForAuthor(ctx, userID, blockID uuid.UUID) ([]Question, error)` — includes answers
  - `course.Question{ID, BlockID uuid.UUID, TopicID *uuid.UUID, Kind string, Prompt, Answer json.RawMessage, SortKey string}`
  - `course.ValidateQuestion(kind string, prompt, answer json.RawMessage) (json.RawMessage, json.RawMessage, error)`
  - Question kinds: `QuestionMCQSingle = "mcq_single"`, `QuestionTrueFalse = "true_false"`, `QuestionTyped = "typed"`

- [ ] **Step 1: Write `api/db/queries/tagging.sql`**

```sql
-- name: DeleteBlockTopics :exec
DELETE FROM block_topics WHERE block_id = $1;

-- name: AddBlockTopic :exec
INSERT INTO block_topics (block_id, topic_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListBlockTopicIDs :many
SELECT bt.topic_id
FROM block_topics bt
JOIN topics t ON t.id = bt.topic_id AND t.archived_at IS NULL
WHERE bt.block_id = $1
ORDER BY t.sort_key;

-- name: CountTopicsInCourse :one
SELECT count(*) FROM topics
WHERE course_id = $1 AND id = ANY(sqlc.arg('topic_ids')::uuid[]) AND archived_at IS NULL;
```

- [ ] **Step 2: Write `api/db/queries/questions.sql`**

```sql
-- name: CreateQuestion :one
INSERT INTO questions (block_id, topic_id, kind, prompt, answer, sort_key)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetQuestionByID :one
SELECT * FROM questions WHERE id = $1 AND archived_at IS NULL;

-- name: ListQuestionsByBlock :many
SELECT * FROM questions
WHERE block_id = $1 AND archived_at IS NULL
ORDER BY sort_key;

-- name: UpdateQuestion :one
UPDATE questions SET
    topic_id = sqlc.narg('topic_id'),
    prompt   = COALESCE(sqlc.narg('prompt'), prompt),
    answer   = COALESCE(sqlc.narg('answer'), answer)
WHERE id = sqlc.arg('id') AND archived_at IS NULL
RETURNING *;

-- name: ArchiveQuestion :execrows
UPDATE questions SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;
```

`topic_id` uses `sqlc.narg` without `COALESCE` deliberately: clearing a question's topic must be possible, so passing null means "untag".

- [ ] **Step 3: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 4: Write the failing test** — `api/internal/course/questions_test.go`

```go
package course_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func TestValidateQuestionMCQRequiresOptionsAndIndex(t *testing.T) {
	_, _, err := course.ValidateQuestion(course.QuestionMCQSingle,
		json.RawMessage(`{"text":"Pick one","options":[]}`),
		json.RawMessage(`{"correct_index":0}`))
	assert.True(t, errors.Is(err, apperr.ErrValidation), "a question needs at least two options")

	_, _, err = course.ValidateQuestion(course.QuestionMCQSingle,
		json.RawMessage(`{"text":"Pick one","options":["a","b"]}`),
		json.RawMessage(`{"correct_index":5}`))
	assert.True(t, errors.Is(err, apperr.ErrValidation), "correct_index must exist in options")
}

func TestValidateQuestionMCQAccepts(t *testing.T) {
	prompt, answer, err := course.ValidateQuestion(course.QuestionMCQSingle,
		json.RawMessage(`{"text":"Which is an element?","options":["Water","Iron"]}`),
		json.RawMessage(`{"correct_index":1}`))
	require.NoError(t, err)
	assert.Contains(t, string(prompt), "Iron")
	assert.JSONEq(t, `{"correct_index":1}`, string(answer))
}

func TestValidateQuestionTypedRequiresAccepted(t *testing.T) {
	_, _, err := course.ValidateQuestion(course.QuestionTyped,
		json.RawMessage(`{"text":"Symbol for iron?"}`),
		json.RawMessage(`{"accepted":[]}`))
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestCreateQuestionRequiresQuizBlock(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	reading, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title: "Reading", Type: course.BlockReading,
		Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	_, err = svc.CreateQuestion(ctx, owner, reading.ID, course.QuestionInput{
		Kind:   course.QuestionTrueFalse,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	})
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"questions belong to quiz blocks only")
}

func TestCreateQuestionOnQuizBlock(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	quiz, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title: "Quiz", Type: course.BlockQuiz,
	})
	require.NoError(t, err)

	q, err := svc.CreateQuestion(ctx, owner, quiz.ID, course.QuestionInput{
		Kind:   course.QuestionTrueFalse,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	})
	require.NoError(t, err)
	assert.Equal(t, course.QuestionTrueFalse, q.Kind)
	assert.Nil(t, q.TopicID, "a question may be untagged")
}

func TestSetBlockTopicsReplacesTheWholeSet(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	mod, err := svc.CreateModule(ctx, owner, courseID, "M1")
	require.NoError(t, err)
	block, err := svc.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "R", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	atoms, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)
	bonding, err := svc.CreateTopic(ctx, owner, courseID, "Bonding")
	require.NoError(t, err)

	require.NoError(t, svc.SetBlockTopics(ctx, owner, block.ID, []uuid.UUID{atoms.ID, bonding.ID}))
	got, err := svc.GetBlockTopics(ctx, owner, block.ID)
	require.NoError(t, err)
	assert.Len(t, got, 2)

	// PUT semantics: the new list replaces, it does not merge.
	require.NoError(t, svc.SetBlockTopics(ctx, owner, block.ID, []uuid.UUID{bonding.ID}))
	got, err = svc.GetBlockTopics(ctx, owner, block.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, bonding.ID, got[0])
}

func TestSetBlockTopicsRejectsForeignTopic(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	mod, err := svc.CreateModule(ctx, owner, courseID, "M1")
	require.NoError(t, err)
	block, err := svc.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "R", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	otherCourse, err := svc.Create(ctx, owner, course.CreateInput{Title: "Physics"})
	require.NoError(t, err)
	foreign, err := svc.CreateTopic(ctx, owner, otherCourse.ID, "Forces")
	require.NoError(t, err)

	err = svc.SetBlockTopics(ctx, owner, block.ID, []uuid.UUID{foreign.ID})
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"a block may only be tagged with topics from its own course")
}
```

- [ ] **Step 5: Run and confirm failure**

Run: `make test`
Expected: FAIL — undefined `course.ValidateQuestion`, `svc.SetBlockTopics`.

- [ ] **Step 6: Write `api/internal/course/questions.go`**

```go
package course

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

const (
	QuestionMCQSingle = "mcq_single"
	QuestionTrueFalse = "true_false"
	QuestionTyped     = "typed"
)

type MCQPrompt struct {
	Text    string   `json:"text"`
	Options []string `json:"options"`
}

type MCQAnswer struct {
	CorrectIndex int `json:"correct_index"`
}

type TextPrompt struct {
	Text string `json:"text"`
}

type BoolAnswer struct {
	Correct bool `json:"correct"`
}

type TypedAnswer struct {
	Accepted      []string `json:"accepted"`
	CaseSensitive bool     `json:"case_sensitive"`
}

type Question struct {
	ID      uuid.UUID
	BlockID uuid.UUID
	TopicID *uuid.UUID
	Kind    string
	Prompt  json.RawMessage
	Answer  json.RawMessage
	SortKey string
}

type QuestionInput struct {
	Kind    string
	TopicID *uuid.UUID
	Prompt  json.RawMessage
	Answer  json.RawMessage
}

// ValidateQuestion checks a prompt and answer against the question kind,
// returning both normalised. Grading in Task 15 relies on these shapes.
func ValidateQuestion(kind string, prompt, answer json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	switch kind {
	case QuestionMCQSingle:
		var p MCQPrompt
		if err := strictUnmarshal(prompt, &p); err != nil {
			return nil, nil, apperr.Invalid("prompt", "%s", err)
		}
		if strings.TrimSpace(p.Text) == "" {
			return nil, nil, apperr.Invalid("prompt.text", "must not be empty")
		}
		if len(p.Options) < 2 {
			return nil, nil, apperr.Invalid("prompt.options", "needs at least two options")
		}
		var a MCQAnswer
		if err := strictUnmarshal(answer, &a); err != nil {
			return nil, nil, apperr.Invalid("answer", "%s", err)
		}
		if a.CorrectIndex < 0 || a.CorrectIndex >= len(p.Options) {
			return nil, nil, apperr.Invalid("answer.correct_index", "must index one of the options")
		}
		return mustPair(p, a)

	case QuestionTrueFalse:
		var p TextPrompt
		if err := strictUnmarshal(prompt, &p); err != nil {
			return nil, nil, apperr.Invalid("prompt", "%s", err)
		}
		if strings.TrimSpace(p.Text) == "" {
			return nil, nil, apperr.Invalid("prompt.text", "must not be empty")
		}
		var a BoolAnswer
		if err := strictUnmarshal(answer, &a); err != nil {
			return nil, nil, apperr.Invalid("answer", "%s", err)
		}
		return mustPair(p, a)

	case QuestionTyped:
		var p TextPrompt
		if err := strictUnmarshal(prompt, &p); err != nil {
			return nil, nil, apperr.Invalid("prompt", "%s", err)
		}
		if strings.TrimSpace(p.Text) == "" {
			return nil, nil, apperr.Invalid("prompt.text", "must not be empty")
		}
		var a TypedAnswer
		if err := strictUnmarshal(answer, &a); err != nil {
			return nil, nil, apperr.Invalid("answer", "%s", err)
		}
		if len(a.Accepted) == 0 {
			return nil, nil, apperr.Invalid("answer.accepted", "needs at least one accepted answer")
		}
		return mustPair(p, a)

	default:
		return nil, nil, apperr.Invalid("kind", "unknown question kind %q", kind)
	}
}

func mustPair(prompt, answer any) (json.RawMessage, json.RawMessage, error) {
	p, err := remarshal(prompt)
	if err != nil {
		return nil, nil, err
	}
	a, err := remarshal(answer)
	if err != nil {
		return nil, nil, err
	}
	return p, a, nil
}

func (s *Service) CreateQuestion(ctx context.Context, userID, blockID uuid.UUID, in QuestionInput) (Question, error) {
	prompt, answer, err := ValidateQuestion(in.Kind, in.Prompt, in.Answer)
	if err != nil {
		return Question{}, err
	}

	var out Question
	err = s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		block, err := s.loadBlock(ctx, q, userID, blockID)
		if err != nil {
			return err
		}
		if block.Type != BlockQuiz {
			return apperr.Invalid("block", "questions may only be added to quiz blocks")
		}
		if err := s.checkTopicBelongsToBlockCourse(ctx, q, blockID, in.TopicID); err != nil {
			return err
		}

		rows, err := q.ListQuestionsByBlock(ctx, blockID)
		if err != nil {
			return fmt.Errorf("list questions: %w", err)
		}
		sibs := make([]Ordered, 0, len(rows))
		for _, row := range rows {
			sibs = append(sibs, Ordered{ID: row.ID, SortKey: row.SortKey})
		}
		key, err := PlaceKey(sibs, nil, nil)
		if err != nil {
			return err
		}

		row, err := q.CreateQuestion(ctx, sqlcgen.CreateQuestionParams{
			BlockID: blockID,
			TopicID: pgconv.UUID(in.TopicID), // nullable uuid -> pgtype.UUID
			Kind:    in.Kind,
			Prompt:  prompt,
			Answer:  answer,
			SortKey: key,
		})
		if err != nil {
			return fmt.Errorf("create question: %w", err)
		}
		out = toQuestion(row)
		return nil
	})
	return out, err
}

func (s *Service) UpdateQuestion(ctx context.Context, userID, questionID uuid.UUID, in QuestionInput) (Question, error) {
	var out Question
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		existing, err := s.loadQuestion(ctx, q, userID, questionID)
		if err != nil {
			return err
		}

		prompt := in.Prompt
		if len(prompt) == 0 {
			prompt = existing.Prompt
		}
		answer := in.Answer
		if len(answer) == 0 {
			answer = existing.Answer
		}
		validPrompt, validAnswer, err := ValidateQuestion(existing.Kind, prompt, answer)
		if err != nil {
			return err
		}
		if err := s.checkTopicBelongsToBlockCourse(ctx, q, existing.BlockID, in.TopicID); err != nil {
			return err
		}

		row, err := q.UpdateQuestion(ctx, sqlcgen.UpdateQuestionParams{
			ID:      questionID,
			TopicID: pgconv.UUID(in.TopicID),
			Prompt:  validPrompt,
			Answer:  validAnswer,
		})
		if err != nil {
			return fmt.Errorf("update question: %w", err)
		}
		out = toQuestion(row)
		return nil
	})
	return out, err
}

func (s *Service) ArchiveQuestion(ctx context.Context, userID, questionID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadQuestion(ctx, q, userID, questionID); err != nil {
			return err
		}
		n, err := q.ArchiveQuestion(ctx, questionID)
		if err != nil {
			return fmt.Errorf("archive question: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("question %s", questionID)
		}
		return nil
	})
}

// ListQuestionsForAuthor includes answer keys. It is ONLY reachable by the
// course owner; the learner-facing read in Task 16 strips answers.
func (s *Service) ListQuestionsForAuthor(ctx context.Context, userID, blockID uuid.UUID) ([]Question, error) {
	out := []Question{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadBlock(ctx, q, userID, blockID); err != nil {
			return err
		}
		rows, err := q.ListQuestionsByBlock(ctx, blockID)
		if err != nil {
			return fmt.Errorf("list questions: %w", err)
		}
		for _, row := range rows {
			out = append(out, toQuestion(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) loadQuestion(ctx context.Context, q *sqlcgen.Queries, userID, questionID uuid.UUID) (sqlcgen.Question, error) {
	row, err := q.GetQuestionByID(ctx, questionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Question{}, apperr.NotFound("question %s", questionID)
	}
	if err != nil {
		return sqlcgen.Question{}, fmt.Errorf("get question: %w", err)
	}
	if _, err := s.loadBlock(ctx, q, userID, row.BlockID); err != nil {
		return sqlcgen.Question{}, err
	}
	return row, nil
}

func toQuestion(row sqlcgen.Question) Question {
	return Question{
		ID: row.ID, BlockID: row.BlockID, TopicID: pgconv.UUIDPtr(row.TopicID),
		Kind: row.Kind, Prompt: row.Prompt, Answer: row.Answer, SortKey: row.SortKey,
	}
}
```

- [ ] **Step 7: Write `api/internal/course/tagging.go`**

```go
package course

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

// SetBlockTopics replaces a block's whole tag set. PUT semantics, because the
// UI is a multi-select and set replacement avoids add/remove races.
func (s *Service) SetBlockTopics(ctx context.Context, userID, blockID uuid.UUID, topicIDs []uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadBlock(ctx, q, userID, blockID); err != nil {
			return err
		}
		if err := s.checkTopicsBelongToBlockCourse(ctx, q, blockID, topicIDs); err != nil {
			return err
		}
		if err := q.DeleteBlockTopics(ctx, blockID); err != nil {
			return fmt.Errorf("clear block topics: %w", err)
		}
		for _, topicID := range topicIDs {
			if err := q.AddBlockTopic(ctx, sqlcgen.AddBlockTopicParams{
				BlockID: blockID, TopicID: topicID,
			}); err != nil {
				return fmt.Errorf("add block topic: %w", err)
			}
		}
		return nil
	})
}

func (s *Service) GetBlockTopics(ctx context.Context, userID, blockID uuid.UUID) ([]uuid.UUID, error) {
	out := []uuid.UUID{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadBlock(ctx, q, userID, blockID); err != nil {
			return err
		}
		ids, err := q.ListBlockTopicIDs(ctx, blockID)
		if err != nil {
			return fmt.Errorf("list block topics: %w", err)
		}
		out = ids
		return nil
	})
	return out, err
}

// checkTopicsBelongToBlockCourse rejects tagging a block with a topic from
// another course, which would make mastery meaningless.
func (s *Service) checkTopicsBelongToBlockCourse(ctx context.Context, q *sqlcgen.Queries, blockID uuid.UUID, topicIDs []uuid.UUID) error {
	if len(topicIDs) == 0 {
		return nil
	}
	courseID, err := q.GetBlockCourseID(ctx, blockID)
	if err != nil {
		return fmt.Errorf("get block course: %w", err)
	}
	count, err := q.CountTopicsInCourse(ctx, sqlcgen.CountTopicsInCourseParams{
		CourseID: courseID, TopicIds: topicIDs,
	})
	if err != nil {
		return fmt.Errorf("count topics: %w", err)
	}
	if int(count) != len(topicIDs) {
		return apperr.Invalid("topic_ids", "every topic must belong to this block's course")
	}
	return nil
}

func (s *Service) checkTopicBelongsToBlockCourse(ctx context.Context, q *sqlcgen.Queries, blockID uuid.UUID, topicID *uuid.UUID) error {
	if topicID == nil {
		return nil
	}
	return s.checkTopicsBelongToBlockCourse(ctx, q, blockID, []uuid.UUID{*topicID})
}
```

- [ ] **Step 8: Run the tests**

Run: `make test`
Expected: PASS. If `CountTopicsInCourse` fails to generate, confirm the query uses `= ANY(sqlc.arg('topic_ids')::uuid[])` exactly — sqlc needs the cast to infer a slice parameter.

- [ ] **Step 9: Write `api/cmd/api/handlers_questions.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type questionBody struct {
	ID      uuid.UUID       `json:"id"`
	BlockID uuid.UUID       `json:"block_id"`
	TopicID *uuid.UUID      `json:"topic_id,omitempty"`
	Kind    string          `json:"kind"`
	Prompt  json.RawMessage `json:"prompt"`
	Answer  json.RawMessage `json:"answer"`
	SortKey string          `json:"sort_key"`
}

type questionOutput struct {
	Body questionBody
}

type questionListOutput struct {
	Body struct {
		Questions []questionBody `json:"questions"`
	}
}

type createQuestionInput struct {
	ID   uuid.UUID `path:"id" doc:"Block id"`
	Body struct {
		Kind    string          `json:"kind" enum:"mcq_single,true_false,typed"`
		TopicID *uuid.UUID      `json:"topic_id,omitempty"`
		Prompt  json.RawMessage `json:"prompt"`
		Answer  json.RawMessage `json:"answer"`
	}
}

type updateQuestionInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		TopicID *uuid.UUID      `json:"topic_id,omitempty"`
		Prompt  json.RawMessage `json:"prompt,omitempty"`
		Answer  json.RawMessage `json:"answer,omitempty"`
	}
}

type setTopicsInput struct {
	ID   uuid.UUID `path:"id" doc:"Block id"`
	Body struct {
		TopicIDs []uuid.UUID `json:"topic_ids"`
	}
}

type topicIDsOutput struct {
	Body struct {
		TopicIDs []uuid.UUID `json:"topic_ids"`
	}
}

func questionToBody(q course.Question) questionBody {
	return questionBody{
		ID: q.ID, BlockID: q.BlockID, TopicID: q.TopicID, Kind: q.Kind,
		Prompt: q.Prompt, Answer: q.Answer, SortKey: q.SortKey,
	}
}

func (s *server) registerQuestionRoutes(api huma.API) {
	svc := course.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "setBlockTopics", Method: http.MethodPut, Path: "/blocks/{id}/topics",
		Summary: "Replace a block's topic tags",
	}, func(ctx context.Context, in *setTopicsInput) (*topicIDsOutput, error) {
		if err := svc.SetBlockTopics(ctx, auth.MustUser(ctx), in.ID, in.Body.TopicIDs); err != nil {
			return nil, toHTTP(err)
		}
		ids, err := svc.GetBlockTopics(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &topicIDsOutput{}
		out.Body.TopicIDs = ids
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createQuestion", Method: http.MethodPost, Path: "/blocks/{id}/questions",
		Summary: "Add a question to a quiz block", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createQuestionInput) (*questionOutput, error) {
		q, err := svc.CreateQuestion(ctx, auth.MustUser(ctx), in.ID, course.QuestionInput{
			Kind: in.Body.Kind, TopicID: in.Body.TopicID,
			Prompt: in.Body.Prompt, Answer: in.Body.Answer,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &questionOutput{Body: questionToBody(q)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listQuestions", Method: http.MethodGet, Path: "/blocks/{id}/questions",
		Summary: "List a quiz block's questions, with answers (author only)",
	}, func(ctx context.Context, in *idOnlyInput) (*questionListOutput, error) {
		list, err := svc.ListQuestionsForAuthor(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &questionListOutput{}
		out.Body.Questions = make([]questionBody, 0, len(list))
		for _, q := range list {
			out.Body.Questions = append(out.Body.Questions, questionToBody(q))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateQuestion", Method: http.MethodPatch, Path: "/questions/{id}",
		Summary: "Update a question",
	}, func(ctx context.Context, in *updateQuestionInput) (*questionOutput, error) {
		q, err := svc.UpdateQuestion(ctx, auth.MustUser(ctx), in.ID, course.QuestionInput{
			TopicID: in.Body.TopicID, Prompt: in.Body.Prompt, Answer: in.Body.Answer,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &questionOutput{Body: questionToBody(q)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveQuestion", Method: http.MethodPost, Path: "/questions/{id}/archive",
		Summary: "Archive a question", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idOnlyInput) (*struct{}, error) {
		if err := svc.ArchiveQuestion(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})
}
```

- [ ] **Step 10: Register the routes**

Add `s.registerQuestionRoutes(api)` to `registerRoutes`.

- [ ] **Step 11: Write the handler test** — `api/cmd/api/handlers_questions_test.go`

```go
package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newQuizBlock creates a course, a module and a quiz block, returning both ids.
func newQuizBlock(t *testing.T, api humatest.TestAPI) (courseID, blockID string) {
	t.Helper()
	courseID = newCourse(t, api, "Chemistry")

	modResp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": "M1"})
	require.Equal(t, http.StatusCreated, modResp.Code, modResp.Body.String())
	moduleID := idFromBody(t, modResp.Body.Bytes())

	blockResp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Quiz", "type": "quiz",
	})
	require.Equal(t, http.StatusCreated, blockResp.Code, blockResp.Body.String())
	return courseID, idFromBody(t, blockResp.Body.Bytes())
}

func TestCreateQuestionOnQuizBlockReturns201(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, quizID := newQuizBlock(t, api)

	resp := api.Post("/blocks/"+quizID+"/questions", map[string]any{
		"kind":   "mcq_single",
		"prompt": map[string]any{"text": "Which is an element?", "options": []string{"Water", "Iron"}},
		"answer": map[string]any{"correct_index": 1},
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"kind":"mcq_single"`)
}

func TestCreateQuestionOnReadingBlockReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	modResp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": "M1"})
	require.Equal(t, http.StatusCreated, modResp.Code)
	moduleID := idFromBody(t, modResp.Body.Bytes())

	readingResp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Reading", "type": "reading",
		"content": map[string]any{"body_md": "# Atoms"},
	})
	require.Equal(t, http.StatusCreated, readingResp.Code)
	readingID := idFromBody(t, readingResp.Body.Bytes())

	resp := api.Post("/blocks/"+readingID+"/questions", map[string]any{
		"kind":   "true_false",
		"prompt": map[string]any{"text": "Iron is an element"},
		"answer": map[string]any{"correct": true},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
}

func TestCreateQuestionWithBadAnswerIndexReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, quizID := newQuizBlock(t, api)

	resp := api.Post("/blocks/"+quizID+"/questions", map[string]any{
		"kind":   "mcq_single",
		"prompt": map[string]any{"text": "Pick one", "options": []string{"a", "b"}},
		"answer": map[string]any{"correct_index": 7},
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "answer.correct_index")
}

func TestSetBlockTopicsReturnsTheNewSet(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID, quizID := newQuizBlock(t, api)

	topicResp := api.Post("/courses/"+courseID+"/topics", map[string]any{"title": "Atoms"})
	require.Equal(t, http.StatusCreated, topicResp.Code)
	topicID := idFromBody(t, topicResp.Body.Bytes())

	resp := api.Put("/blocks/"+quizID+"/topics", map[string]any{
		"topic_ids": []string{topicID},
	})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), topicID)
}

func TestSetBlockTopicsRejectsForeignTopicAt422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, quizID := newQuizBlock(t, api)

	// A topic belonging to a different course.
	otherCourse := newCourse(t, api, "Physics")
	foreignResp := api.Post("/courses/"+otherCourse+"/topics", map[string]any{"title": "Forces"})
	require.Equal(t, http.StatusCreated, foreignResp.Code)
	foreignTopic := idFromBody(t, foreignResp.Body.Bytes())

	resp := api.Put("/blocks/"+quizID+"/topics", map[string]any{
		"topic_ids": []string{foreignTopic},
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), "topic_ids")
}
```

Add `"github.com/danielgtaylor/huma/v2/humatest"` to this file's imports for the helper signature.

- [ ] **Step 12: Run, regenerate, commit**

```bash
make test && make generate
git add -A && git commit -m "feat(api): topic tagging and quiz questions"
```

---

### Task 11: Course outline — one aggregate read

The builder loads a whole course in one request. A version that fetches modules, then blocks per module, then tags per block is a waterfall that feels broken on a phone.

**Files:**
- Create: `api/db/queries/outline.sql`
- Create: `api/internal/course/outline.go`, `api/internal/course/outline_test.go`
- Create: `api/cmd/api/handlers_outline.go`
- Modify: `api/cmd/api/main.go`

**Interfaces:**
- Produces:
  - `(*Service).Outline(ctx, userID, courseID uuid.UUID) (Outline, error)`
  - `course.Outline{Course Course, Topics []Topic, Modules []OutlineModule}`
  - `course.OutlineModule{Module, Blocks []OutlineBlock}`
  - `course.OutlineBlock{Block, TopicIDs []uuid.UUID, QuestionCount int}`

- [ ] **Step 1: Write `api/db/queries/outline.sql`**

```sql
-- name: OutlineBlocks :many
SELECT
    b.id,
    b.module_id,
    b.title,
    b.type,
    b.sort_key,
    b.content,
    b.completion,
    COALESCE(
        (SELECT array_agg(bt.topic_id ORDER BY t.sort_key)
         FROM block_topics bt
         JOIN topics t ON t.id = bt.topic_id AND t.archived_at IS NULL
         WHERE bt.block_id = b.id),
        '{}'
    )::uuid[] AS topic_ids,
    (SELECT count(*) FROM questions qq
     WHERE qq.block_id = b.id AND qq.archived_at IS NULL) AS question_count
FROM blocks b
JOIN modules m ON m.id = b.module_id AND m.archived_at IS NULL
WHERE m.course_id = $1 AND b.archived_at IS NULL
ORDER BY m.sort_key, b.sort_key;
```

One query returns every block in the course with its tags and question count. `array_agg` in a correlated subquery avoids the row multiplication a plain join to `block_topics` would cause.

- [ ] **Step 2: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 3: Write the failing test** — `api/internal/course/outline_test.go`

```go
package course_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func TestOutlineReturnsTheWholeCourse(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	atoms, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)

	m1, err := svc.CreateModule(ctx, owner, courseID, "Module 1")
	require.NoError(t, err)
	m2, err := svc.CreateModule(ctx, owner, courseID, "Module 2")
	require.NoError(t, err)

	reading, err := svc.CreateBlock(ctx, owner, m1.ID, course.CreateBlockInput{
		Title: "Reading", Type: course.BlockReading,
		Content: json.RawMessage(`{"body_md":"# Atoms"}`),
	})
	require.NoError(t, err)
	require.NoError(t, svc.SetBlockTopics(ctx, owner, reading.ID, []uuid.UUID{atoms.ID}))

	quiz, err := svc.CreateBlock(ctx, owner, m2.ID, course.CreateBlockInput{
		Title: "Quiz", Type: course.BlockQuiz,
	})
	require.NoError(t, err)
	_, err = svc.CreateQuestion(ctx, owner, quiz.ID, course.QuestionInput{
		Kind:   course.QuestionTrueFalse,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	})
	require.NoError(t, err)

	outline, err := svc.Outline(ctx, owner, courseID)
	require.NoError(t, err)

	assert.Equal(t, "Chemistry", outline.Course.Title)
	require.Len(t, outline.Topics, 1)
	require.Len(t, outline.Modules, 2)

	assert.Equal(t, "Module 1", outline.Modules[0].Module.Title)
	require.Len(t, outline.Modules[0].Blocks, 1)
	assert.Equal(t, []uuid.UUID{atoms.ID}, outline.Modules[0].Blocks[0].TopicIDs)
	assert.Equal(t, 0, outline.Modules[0].Blocks[0].QuestionCount)

	require.Len(t, outline.Modules[1].Blocks, 1)
	assert.Equal(t, 1, outline.Modules[1].Blocks[0].QuestionCount)
}

func TestOutlineIncludesEmptyModules(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	_, err := svc.CreateModule(ctx, owner, courseID, "Empty")
	require.NoError(t, err)

	outline, err := svc.Outline(ctx, owner, courseID)
	require.NoError(t, err)
	require.Len(t, outline.Modules, 1)
	assert.Empty(t, outline.Modules[0].Blocks,
		"a module with no blocks must still appear, or the builder cannot show it")
}

func TestOutlineRejectsStranger(t *testing.T) {
	svc, ctx, _, stranger, courseID := fixture(t)
	_, err := svc.Outline(ctx, stranger, courseID)
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}

func TestOutlineExcludesArchived(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	mod, err := svc.CreateModule(ctx, owner, courseID, "M1")
	require.NoError(t, err)
	block, err := svc.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "Gone", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)
	require.NoError(t, svc.ArchiveBlock(ctx, owner, block.ID))

	outline, err := svc.Outline(ctx, owner, courseID)
	require.NoError(t, err)
	require.Len(t, outline.Modules, 1)
	assert.Empty(t, outline.Modules[0].Blocks)
}
```

- [ ] **Step 4: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: svc.Outline`.

- [ ] **Step 5: Write `api/internal/course/outline.go`**

```go
package course

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type OutlineBlock struct {
	Block
	TopicIDs      []uuid.UUID
	QuestionCount int
}

type OutlineModule struct {
	Module Module
	Blocks []OutlineBlock
}

type Outline struct {
	Course  Course
	Topics  []Topic
	Modules []OutlineModule
}

// Outline returns a whole course in one round trip: the course, its topics, its
// modules in order, and every block with its tags and question count.
func (s *Service) Outline(ctx context.Context, userID, courseID uuid.UUID) (Outline, error) {
	var out Outline
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		courseRow, err := s.load(ctx, q, userID, courseID)
		if err != nil {
			return err
		}
		out.Course = toCourse(courseRow)

		topicRows, err := q.ListTopicsByCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("list topics: %w", err)
		}
		out.Topics = make([]Topic, 0, len(topicRows))
		for _, row := range topicRows {
			out.Topics = append(out.Topics, toTopic(row))
		}

		moduleRows, err := q.ListModulesByCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("list modules: %w", err)
		}

		blockRows, err := q.OutlineBlocks(ctx, courseID)
		if err != nil {
			return fmt.Errorf("outline blocks: %w", err)
		}

		// Group blocks by module. The query already orders by module then block
		// sort key, so appending preserves order without another sort.
		byModule := make(map[uuid.UUID][]OutlineBlock, len(moduleRows))
		for _, row := range blockRows {
			topicIDs := row.TopicIds
			if topicIDs == nil {
				topicIDs = []uuid.UUID{}
			}
			byModule[row.ModuleID] = append(byModule[row.ModuleID], OutlineBlock{
				Block: Block{
					ID:         row.ID,
					ModuleID:   row.ModuleID,
					Title:      row.Title,
					Type:       row.Type,
					SortKey:    row.SortKey,
					Content:    row.Content,
					Completion: row.Completion,
				},
				TopicIDs:      topicIDs,
				QuestionCount: int(row.QuestionCount),
			})
		}

		out.Modules = make([]OutlineModule, 0, len(moduleRows))
		for _, row := range moduleRows {
			blocks := byModule[row.ID]
			if blocks == nil {
				blocks = []OutlineBlock{}
			}
			out.Modules = append(out.Modules, OutlineModule{
				Module: toModule(row),
				Blocks: blocks,
			})
		}
		return nil
	})
	return out, err
}
```

- [ ] **Step 6: Run and confirm the outline tests pass**

Run: `make test`
Expected: PASS. If `row.TopicIds` does not compile, check the generated field name — sqlc derives it from the SQL alias `topic_ids`.

- [ ] **Step 7: Write `api/cmd/api/handlers_outline.go`**

```go
package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type outlineBlockBody struct {
	blockBody
	TopicIDs      []uuid.UUID `json:"topic_ids"`
	QuestionCount int         `json:"question_count"`
}

type outlineModuleBody struct {
	ID      uuid.UUID          `json:"id"`
	Title   string             `json:"title"`
	SortKey string             `json:"sort_key"`
	Blocks  []outlineBlockBody `json:"blocks"`
}

type outlineOutput struct {
	Body struct {
		Course  courseBody          `json:"course"`
		Topics  []namedBody         `json:"topics"`
		Modules []outlineModuleBody `json:"modules"`
	}
}

func (s *server) registerOutlineRoutes(api huma.API) {
	svc := course.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "getCourseOutline", Method: http.MethodGet, Path: "/courses/{id}/outline",
		Summary: "Get a whole course in one request",
	}, func(ctx context.Context, in *idOnlyInput) (*outlineOutput, error) {
		outline, err := svc.Outline(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}

		out := &outlineOutput{}
		out.Body.Course = courseToBody(outline.Course)

		out.Body.Topics = make([]namedBody, 0, len(outline.Topics))
		for _, topic := range outline.Topics {
			out.Body.Topics = append(out.Body.Topics, namedBody{
				ID: topic.ID, Title: topic.Title, SortKey: topic.SortKey,
			})
		}

		out.Body.Modules = make([]outlineModuleBody, 0, len(outline.Modules))
		for _, mod := range outline.Modules {
			blocks := make([]outlineBlockBody, 0, len(mod.Blocks))
			for _, b := range mod.Blocks {
				blocks = append(blocks, outlineBlockBody{
					blockBody:     blockToBody(b.Block),
					TopicIDs:      b.TopicIDs,
					QuestionCount: b.QuestionCount,
				})
			}
			out.Body.Modules = append(out.Body.Modules, outlineModuleBody{
				ID: mod.Module.ID, Title: mod.Module.Title,
				SortKey: mod.Module.SortKey, Blocks: blocks,
			})
		}
		return out, nil
	})
}
```

- [ ] **Step 8: Register the route**

Add `s.registerOutlineRoutes(api)` to `registerRoutes`.

- [ ] **Step 9: Write the handler test** — `api/cmd/api/handlers_outline_test.go`

```go
package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutlineReturnsCourseTopicsAndModulesInOneRequest(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	topicResp := api.Post("/courses/"+courseID+"/topics", map[string]any{"title": "Atoms"})
	require.Equal(t, http.StatusCreated, topicResp.Code)

	modResp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": "M1"})
	require.Equal(t, http.StatusCreated, modResp.Code)
	moduleID := idFromBody(t, modResp.Body.Bytes())

	blockResp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Reading", "type": "reading",
		"content": map[string]any{"body_md": "# Atoms"},
	})
	require.Equal(t, http.StatusCreated, blockResp.Code)

	resp := api.Get("/courses/" + courseID + "/outline")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var parsed struct {
		Course  struct{ Title string } `json:"course"`
		Topics  []struct{ Title string } `json:"topics"`
		Modules []struct {
			Title  string `json:"title"`
			Blocks []struct {
				Title         string   `json:"title"`
				TopicIDs      []string `json:"topic_ids"`
				QuestionCount int      `json:"question_count"`
			} `json:"blocks"`
		} `json:"modules"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &parsed))

	assert.Equal(t, "Chemistry", parsed.Course.Title)
	require.Len(t, parsed.Topics, 1)
	require.Len(t, parsed.Modules, 1)
	require.Len(t, parsed.Modules[0].Blocks, 1)
	assert.Equal(t, "Reading", parsed.Modules[0].Blocks[0].Title)
	assert.Empty(t, parsed.Modules[0].Blocks[0].TopicIDs)
	assert.Equal(t, 0, parsed.Modules[0].Blocks[0].QuestionCount)
}

func TestOutlineOnMissingCourseReturns404(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	assert.Equal(t, http.StatusNotFound,
		api.Get("/courses/"+uuid.New().String()+"/outline").Code)
}
```

- [ ] **Step 10: Run, regenerate, commit**

```bash
make test && make generate
git add -A && git commit -m "feat(api): course outline aggregate read"
```

---

### Task 12: Enrolments

The enrolment is the seam that lets the classroom arrive last: every progress row hangs off it, and a class enrolment is just one with `class_id` set.

**Files:**
- Create: `api/db/queries/enrolments.sql`
- Create: `api/internal/enrolment/service.go`, `api/internal/enrolment/service_test.go`
- Create: `api/cmd/api/handlers_enrolments.go`
- Modify: `api/cmd/api/main.go`

**Interfaces:**
- Produces:
  - `enrolment.Service` with `New(runner db.Runner) *Service`
  - `(*Service).Enrol(ctx, userID, courseID uuid.UUID, mode string) (Enrolment, error)`
  - `(*Service).ListMine(ctx, userID uuid.UUID) ([]LibraryEntry, error)`
  - `(*Service).Load(ctx, q *sqlcgen.Queries, userID, enrolmentID uuid.UUID) (sqlcgen.Enrolment, error)` — reused by Tasks 14–16
  - `enrolment.ModeSelfStudy = "self_study"`, `enrolment.ModeClassroom = "classroom"`
  - `Enrolment{ID, UserID, CourseID uuid.UUID, Mode string, StartedAt time.Time}`
  - `LibraryEntry{Enrolment, CourseTitle string}`

- [ ] **Step 1: Write `api/db/queries/enrolments.sql`**

```sql
-- name: CreateEnrolment :one
INSERT INTO enrolments (user_id, course_id, mode) VALUES ($1, $2, $3) RETURNING *;

-- name: GetEnrolmentByID :one
SELECT * FROM enrolments WHERE id = $1;

-- name: ListEnrolmentsByUser :many
SELECT e.*, c.title AS course_title
FROM enrolments e
JOIN courses c ON c.id = e.course_id AND c.archived_at IS NULL
WHERE e.user_id = $1
ORDER BY e.started_at DESC;
```

- [ ] **Step 2: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 3: Write the failing test** — `api/internal/enrolment/service_test.go`

```go
package enrolment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

func setup(t *testing.T) (*enrolment.Service, context.Context, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx, runner := pgtest.NewRunner(t)

	var learner uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		learner, err = user.Store{}.EnsureBySubject(ctx, q, "dev|learner", "learner")
		return err
	}))

	created, err := course.New(runner).Create(ctx, learner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)

	return enrolment.New(runner), ctx, learner, created.ID
}

func TestEnrolSelfStudy(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)

	e, err := svc.Enrol(ctx, learner, courseID, enrolment.ModeSelfStudy)
	require.NoError(t, err)
	assert.Equal(t, enrolment.ModeSelfStudy, e.Mode)
	assert.Equal(t, courseID, e.CourseID)
	assert.False(t, e.StartedAt.IsZero(), "started_at is fixed at enrolment")
}

func TestEnrolTwiceIsAConflict(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)

	_, err := svc.Enrol(ctx, learner, courseID, enrolment.ModeSelfStudy)
	require.NoError(t, err)

	_, err = svc.Enrol(ctx, learner, courseID, enrolment.ModeSelfStudy)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrConflict),
		"the partial unique index must surface as a 409, not a 500")
}

func TestEnrolRejectsClassroomModeInSlice1(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)

	_, err := svc.Enrol(ctx, learner, courseID, enrolment.ModeClassroom)
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"classroom mode needs a class, which does not exist until slice 6")
}

func TestEnrolRejectsUnknownMode(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)
	_, err := svc.Enrol(ctx, learner, courseID, "telepathy")
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestEnrolRejectsMissingCourse(t *testing.T) {
	svc, ctx, learner, _ := setup(t)
	_, err := svc.Enrol(ctx, learner, uuid.New(), enrolment.ModeSelfStudy)
	assert.True(t, errors.Is(err, apperr.ErrNotFound))
}

func TestListMineCarriesCourseTitle(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)

	_, err := svc.Enrol(ctx, learner, courseID, enrolment.ModeSelfStudy)
	require.NoError(t, err)

	list, err := svc.ListMine(ctx, learner)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Chemistry", list[0].CourseTitle)
}
```

- [ ] **Step 4: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: enrolment.New`.

- [ ] **Step 5: Write `api/internal/enrolment/service.go`**

```go
// Package enrolment owns the link between a learner and a course. Every
// progress row keys off an enrolment, never a user, which is what lets classes
// arrive in a later slice without touching progress.
package enrolment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

const (
	ModeSelfStudy = "self_study"
	ModeClassroom = "classroom"
)

type Enrolment struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CourseID  uuid.UUID
	Mode      string
	StartedAt time.Time
}

type LibraryEntry struct {
	Enrolment
	CourseTitle string
}

type Service struct {
	runner db.Runner
}

func New(runner db.Runner) *Service {
	return &Service{runner: runner}
}

func (s *Service) Enrol(ctx context.Context, userID, courseID uuid.UUID, mode string) (Enrolment, error) {
	switch mode {
	case ModeSelfStudy:
	case ModeClassroom:
		return Enrolment{}, apperr.Invalid("mode",
			"classroom enrolment requires a class, which arrives in a later slice")
	default:
		return Enrolment{}, apperr.Invalid("mode", "must be %s", ModeSelfStudy)
	}

	var out Enrolment
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		// Slice 1 does not enforce visibility: any existing course may be
		// enrolled in. Access control arrives with slice 5.
		if _, err := q.GetCourseByID(ctx, courseID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.NotFound("course %s", courseID)
			}
			return fmt.Errorf("get course: %w", err)
		}

		row, err := q.CreateEnrolment(ctx, sqlcgen.CreateEnrolmentParams{
			UserID:   userID,
			CourseID: courseID,
			Mode:     mode,
		})
		if err != nil {
			if isUniqueViolation(err) {
				return apperr.Conflict("already enrolled in course %s", courseID)
			}
			return fmt.Errorf("create enrolment: %w", err)
		}
		out = toEnrolment(row)
		return nil
	})
	return out, err
}

func (s *Service) ListMine(ctx context.Context, userID uuid.UUID) ([]LibraryEntry, error) {
	out := []LibraryEntry{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		rows, err := q.ListEnrolmentsByUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("list enrolments: %w", err)
		}
		for _, row := range rows {
			out = append(out, LibraryEntry{
				Enrolment: Enrolment{
					ID:        row.ID,
					UserID:    row.UserID,
					CourseID:  row.CourseID,
					Mode:      row.Mode,
					StartedAt: row.StartedAt,
				},
				CourseTitle: row.CourseTitle,
			})
		}
		return nil
	})
	return out, err
}

// Load resolves an enrolment and checks it belongs to the caller. Progress and
// player reads authorize through this.
func (s *Service) Load(ctx context.Context, q *sqlcgen.Queries, userID, enrolmentID uuid.UUID) (sqlcgen.Enrolment, error) {
	row, err := q.GetEnrolmentByID(ctx, enrolmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Enrolment{}, apperr.NotFound("enrolment %s", enrolmentID)
	}
	if err != nil {
		return sqlcgen.Enrolment{}, fmt.Errorf("get enrolment: %w", err)
	}
	if row.UserID != userID {
		return sqlcgen.Enrolment{}, apperr.Forbidden("enrolment %s is not yours", enrolmentID)
	}
	return row, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func toEnrolment(row sqlcgen.Enrolment) Enrolment {
	return Enrolment{
		ID: row.ID, UserID: row.UserID, CourseID: row.CourseID,
		Mode: row.Mode, StartedAt: row.StartedAt,
	}
}
```

- [ ] **Step 6: Run the tests**

Run: `make test`
Expected: PASS. If `TestEnrolTwiceIsAConflict` returns a 500-shaped error instead, the unique-violation code is wrong — Postgres uses `23505` for `unique_violation`.

- [ ] **Step 7: Write `api/cmd/api/handlers_enrolments.go`**

```go
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type enrolmentBody struct {
	ID          uuid.UUID `json:"id"`
	CourseID    uuid.UUID `json:"course_id"`
	CourseTitle string    `json:"course_title,omitempty"`
	Mode        string    `json:"mode"`
	StartedAt   time.Time `json:"started_at"`
}

type enrolmentOutput struct {
	Body enrolmentBody
}

type enrolmentListOutput struct {
	Body struct {
		Enrolments []enrolmentBody `json:"enrolments"`
	}
}

type createEnrolmentInput struct {
	Body struct {
		CourseID uuid.UUID `json:"course_id"`
		Mode     string    `json:"mode" enum:"self_study,classroom"`
	}
}

func (s *server) registerEnrolmentRoutes(api huma.API) {
	svc := enrolment.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "createEnrolment", Method: http.MethodPost, Path: "/enrolments",
		Summary: "Enrol in a course", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createEnrolmentInput) (*enrolmentOutput, error) {
		e, err := svc.Enrol(ctx, auth.MustUser(ctx), in.Body.CourseID, in.Body.Mode)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &enrolmentOutput{Body: enrolmentBody{
			ID: e.ID, CourseID: e.CourseID, Mode: e.Mode, StartedAt: e.StartedAt,
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listEnrolments", Method: http.MethodGet, Path: "/enrolments",
		Summary: "My library",
	}, func(ctx context.Context, _ *struct{}) (*enrolmentListOutput, error) {
		list, err := svc.ListMine(ctx, auth.MustUser(ctx))
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &enrolmentListOutput{}
		out.Body.Enrolments = make([]enrolmentBody, 0, len(list))
		for _, e := range list {
			out.Body.Enrolments = append(out.Body.Enrolments, enrolmentBody{
				ID: e.ID, CourseID: e.CourseID, CourseTitle: e.CourseTitle,
				Mode: e.Mode, StartedAt: e.StartedAt,
			})
		}
		return out, nil
	})
}
```

- [ ] **Step 8: Register the routes**

Add `s.registerEnrolmentRoutes(api)` to `registerRoutes`.

- [ ] **Step 9: Write the handler test** — `api/cmd/api/handlers_enrolments_test.go`

```go
package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrolReturns201AndAppearsInLibrary(t *testing.T) {
	api, _ := newTestAPI(t, "learner")
	courseID := newCourse(t, api, "Chemistry")

	resp := api.Post("/enrolments", map[string]any{
		"course_id": courseID, "mode": "self_study",
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	library := api.Get("/enrolments")
	require.Equal(t, http.StatusOK, library.Code)
	assert.Contains(t, library.Body.String(), `"course_title":"Chemistry"`)
}

func TestEnrolTwiceReturns409(t *testing.T) {
	api, _ := newTestAPI(t, "learner")
	courseID := newCourse(t, api, "Chemistry")

	first := api.Post("/enrolments", map[string]any{"course_id": courseID, "mode": "self_study"})
	require.Equal(t, http.StatusCreated, first.Code)

	second := api.Post("/enrolments", map[string]any{"course_id": courseID, "mode": "self_study"})
	assert.Equal(t, http.StatusConflict, second.Code, second.Body.String())
}

func TestEnrolInClassroomModeReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "learner")
	courseID := newCourse(t, api, "Chemistry")

	resp := api.Post("/enrolments", map[string]any{"course_id": courseID, "mode": "classroom"})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
}

func TestEnrolInMissingCourseReturns404(t *testing.T) {
	api, _ := newTestAPI(t, "learner")

	resp := api.Post("/enrolments", map[string]any{
		"course_id": uuid.New().String(), "mode": "self_study",
	})
	assert.Equal(t, http.StatusNotFound, resp.Code)
}
```

- [ ] **Step 10: Run, regenerate, commit**

```bash
make test && make generate
git add -A && git commit -m "feat(api): self-study enrolments"
```

---

### Task 13: Event log — the transactional outbox

Events are written in the same transaction as the state they describe. That is what makes the slice-4 engagement consumer trustworthy: a pet can never react to a completion that rolled back, and a completion can never fail to emit its event.

**Files:**
- Create: `api/db/queries/events.sql`
- Create: `api/internal/events/events.go`, `api/internal/events/events_test.go`

**Interfaces:**
- Produces:
  - `events.Kind` constants: `KindBlockCompleted = "block.completed"`, `KindTopicMastered = "topic.mastered"`
  - `events.Append(ctx context.Context, q *sqlcgen.Queries, e Event) error`
  - `events.Event{UserID uuid.UUID, EnrolmentID *uuid.UUID, Kind string, Payload any}`
  - `events.ListUnconsumed(ctx, q *sqlcgen.Queries, limit int32) ([]Record, error)`
  - `events.Record{ID int64, Kind string, Payload json.RawMessage, EnrolmentID *uuid.UUID}`

- [ ] **Step 1: Write `api/db/queries/events.sql`**

```sql
-- name: InsertEvent :one
INSERT INTO events (user_id, enrolment_id, kind, payload)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ListUnconsumedEvents :many
SELECT id, occurred_at, user_id, enrolment_id, kind, payload
FROM events
WHERE consumed_at IS NULL
ORDER BY id
LIMIT $1;

-- name: MarkEventConsumed :execrows
UPDATE events SET consumed_at = now() WHERE id = $1 AND consumed_at IS NULL;

-- name: CountEventsByKind :one
SELECT count(*) FROM events WHERE enrolment_id = $1 AND kind = $2;
```

`MarkEventConsumed` is unused in slice 1 but belongs with its siblings — slice 4's consumer needs it, and a query file split across slices is worse than one unused query.

- [ ] **Step 2: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 3: Write the failing test** — `api/internal/events/events_test.go`

```go
package events_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/user"
)

func TestAppendAndReadBack(t *testing.T) {
	ctx, q := pgtest.Begin(t)

	userID, err := user.Store{}.EnsureBySubject(ctx, q, "dev|learner", "learner")
	require.NoError(t, err)

	require.NoError(t, events.Append(ctx, q, events.Event{
		UserID: userID,
		Kind:   events.KindBlockCompleted,
		Payload: map[string]any{
			"block_id": uuid.New().String(),
		},
	}))

	records, err := events.ListUnconsumed(ctx, q, 10)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, events.KindBlockCompleted, records[0].Kind)
	assert.Contains(t, string(records[0].Payload), "block_id")
}

func TestAppendRejectsUnknownKind(t *testing.T) {
	ctx, q := pgtest.Begin(t)

	userID, err := user.Store{}.EnsureBySubject(ctx, q, "dev|learner", "learner")
	require.NoError(t, err)

	err = events.Append(ctx, q, events.Event{
		UserID: userID, Kind: "pet.fed", Payload: map[string]any{},
	})
	require.Error(t, err, "only kinds the learning side emits are allowed")
}

func TestEventsAreRolledBackWithTheirTransaction(t *testing.T) {
	// The harness rolls back, so this event must not survive to the next test.
	ctx, q := pgtest.Begin(t)

	userID, err := user.Store{}.EnsureBySubject(ctx, q, "dev|rollback", "rollback")
	require.NoError(t, err)

	require.NoError(t, events.Append(ctx, q, events.Event{
		UserID: userID, Kind: events.KindTopicMastered,
		Payload: map[string]any{"topic_id": uuid.New().String()},
	}))
}

func TestPreviousTestsEventIsGone(t *testing.T) {
	ctx, q := pgtest.Begin(t)

	records, err := events.ListUnconsumed(ctx, q, 100)
	require.NoError(t, err)
	assert.Empty(t, records,
		"an event written in a rolled-back transaction must not persist — this is the outbox guarantee")
}
```

- [ ] **Step 4: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: events.Append`.

- [ ] **Step 5: Write `api/internal/events/events.go`**

```go
// Package events is the append-only log the learning side writes facts to and
// the engagement side (slice 4) reads. Writes happen inside the same
// transaction as the state change they describe — the transactional outbox
// pattern — so the two can never disagree.
//
// Delivery to consumers is AT-LEAST-ONCE: a consumer may see the same event
// twice. Every handler that reacts to an event MUST therefore be idempotent.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

// Kinds the learning side emits. Engagement adds no kinds: it only consumes.
const (
	KindBlockCompleted = "block.completed"
	KindTopicMastered  = "topic.mastered"
)

var knownKinds = map[string]bool{
	KindBlockCompleted: true,
	KindTopicMastered:  true,
}

type Event struct {
	UserID      uuid.UUID
	EnrolmentID *uuid.UUID
	Kind        string
	Payload     any
}

type Record struct {
	ID          int64
	OccurredAt  time.Time
	UserID      uuid.UUID
	EnrolmentID *uuid.UUID
	Kind        string
	Payload     json.RawMessage
}

// Append writes one event using the caller's transaction. It takes a
// *sqlcgen.Queries rather than a Runner precisely so it cannot open a
// transaction of its own — the event must share the caller's.
func Append(ctx context.Context, q *sqlcgen.Queries, e Event) error {
	if !knownKinds[e.Kind] {
		return fmt.Errorf("events: unknown kind %q", e.Kind)
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("events: marshal payload: %w", err)
	}
	if _, err := q.InsertEvent(ctx, sqlcgen.InsertEventParams{
		UserID:      e.UserID,
		EnrolmentID: pgconv.UUID(e.EnrolmentID), // nullable uuid -> pgtype.UUID
		Kind:        e.Kind,
		Payload:     payload,
	}); err != nil {
		return fmt.Errorf("events: insert: %w", err)
	}
	return nil
}

func ListUnconsumed(ctx context.Context, q *sqlcgen.Queries, limit int32) ([]Record, error) {
	rows, err := q.ListUnconsumedEvents(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("events: list: %w", err)
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, Record{
			ID:          row.ID,
			OccurredAt:  row.OccurredAt,
			UserID:      row.UserID,
			EnrolmentID: pgconv.UUIDPtr(row.EnrolmentID),
			Kind:        row.Kind,
			Payload:     row.Payload,
		})
	}
	return out, nil
}
```

- [ ] **Step 6: Run the tests and commit**

Run: `make test`
Expected: PASS — including `TestPreviousTestsEventIsGone`, which is the rollback guarantee.

```bash
git add -A && git commit -m "feat(api): append-only event log as a transactional outbox"
```

---

### Task 14: Completion evaluators and mark-complete

One evaluator per block type behind one interface. A `switch` on block type in the handler, the player and the progress query is how the eighteenth block type becomes a three-day change.

**Files:**
- Create: `api/db/queries/progress.sql`
- Create: `api/internal/progress/evaluator.go`, `api/internal/progress/evaluator_test.go`
- Create: `api/internal/progress/service.go`, `api/internal/progress/complete_test.go`
- Create: `api/cmd/api/handlers_progress.go`
- Modify: `api/cmd/api/main.go`

**Interfaces:**
- Produces:
  - `progress.Signal{MarkedDone bool, WatchedToEnd bool, Score *int}`
  - `progress.Evaluate(blockType string, completion json.RawMessage, sig Signal) (bool, error)`
  - `progress.Service` with `New(runner db.Runner, enrolments *enrolment.Service) *Service`
  - `(*Service).MarkComplete(ctx, userID, enrolmentID, blockID uuid.UUID, sig Signal) (BlockProgress, error)`
  - `progress.BlockProgress{BlockID uuid.UUID, Status string, Score *int, Attempts int, CompletedAt *time.Time}`
  - `progress.StatusInProgress = "in_progress"`, `progress.StatusCompleted = "completed"`

- [ ] **Step 1: Write `api/db/queries/progress.sql`**

```sql
-- name: GetBlockProgress :one
SELECT * FROM block_progress WHERE enrolment_id = $1 AND block_id = $2;

-- name: UpsertBlockProgress :one
INSERT INTO block_progress (enrolment_id, block_id, status, score, attempts, completed_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (enrolment_id, block_id) DO UPDATE SET
    status       = EXCLUDED.status,
    score        = EXCLUDED.score,
    attempts     = EXCLUDED.attempts,
    completed_at = COALESCE(block_progress.completed_at, EXCLUDED.completed_at),
    updated_at   = now()
RETURNING *;

-- name: GetBlockForProgress :one
SELECT b.id, b.type, b.completion, m.course_id
FROM blocks b
JOIN modules m ON m.id = b.module_id AND m.archived_at IS NULL
WHERE b.id = $1 AND b.archived_at IS NULL;
```

`completed_at` uses `COALESCE(existing, new)` so the first completion timestamp is never overwritten by a later re-submission.

- [ ] **Step 2: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 3: Write the failing evaluator test** — `api/internal/progress/evaluator_test.go`

```go
package progress_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

func TestReadingCompletesOnMarkDone(t *testing.T) {
	completion := json.RawMessage(`{"kind":"mark_done"}`)

	done, err := progress.Evaluate(course.BlockReading, completion, progress.Signal{MarkedDone: true})
	require.NoError(t, err)
	assert.True(t, done)

	done, err = progress.Evaluate(course.BlockReading, completion, progress.Signal{})
	require.NoError(t, err)
	assert.False(t, done)
}

func TestVideoCompletesOnWatchedToEnd(t *testing.T) {
	completion := json.RawMessage(`{"kind":"watched_to_end"}`)

	done, err := progress.Evaluate(course.BlockVideoYouTube, completion, progress.Signal{WatchedToEnd: true})
	require.NoError(t, err)
	assert.True(t, done)

	done, err = progress.Evaluate(course.BlockVideoYouTube, completion, progress.Signal{MarkedDone: true})
	require.NoError(t, err)
	assert.False(t, done, "marking a video done must not substitute for watching it")
}

func TestQuizCompletesAtOrAboveThreshold(t *testing.T) {
	completion := json.RawMessage(`{"kind":"score_threshold","threshold":70}`)

	for _, tc := range []struct {
		score int
		want  bool
	}{{69, false}, {70, true}, {100, true}, {0, false}} {
		score := tc.score
		done, err := progress.Evaluate(course.BlockQuiz, completion, progress.Signal{Score: &score})
		require.NoError(t, err)
		assert.Equal(t, tc.want, done, "score %d against threshold 70", tc.score)
	}
}

func TestQuizWithoutScoreIsNotComplete(t *testing.T) {
	completion := json.RawMessage(`{"kind":"score_threshold","threshold":70}`)
	done, err := progress.Evaluate(course.BlockQuiz, completion, progress.Signal{})
	require.NoError(t, err)
	assert.False(t, done)
}

func TestEvaluateRejectsUnknownBlockType(t *testing.T) {
	_, err := progress.Evaluate("hologram", json.RawMessage(`{}`), progress.Signal{})
	assert.Error(t, err)
}

func TestEvaluateRejectsMalformedCompletion(t *testing.T) {
	_, err := progress.Evaluate(course.BlockQuiz, json.RawMessage(`{"kind":"score_threshold"`), progress.Signal{})
	assert.Error(t, err)
}
```

- [ ] **Step 4: Run and confirm failure**

Run: `cd api && go test ./internal/progress/ -run TestEvaluate -v`
Expected: FAIL — package does not exist.

- [ ] **Step 5: Write `api/internal/progress/evaluator.go`**

```go
// Package progress owns block completion, quiz grading and per-topic mastery.
// It emits events; it never reads engagement state and must never import
// internal/engagement.
package progress

import (
	"encoding/json"
	"fmt"

	"github.com/Fikhri02/wikipetia/internal/course"
)

const (
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
)

// Signal is what the learner did. Each evaluator reads only the fields its
// block type cares about, so adding a block type adds a field at most.
type Signal struct {
	MarkedDone   bool
	WatchedToEnd bool
	Score        *int // 0-100, set by grading
}

type evaluator func(completion json.RawMessage, sig Signal) (bool, error)

var evaluators = map[string]evaluator{
	course.BlockReading: func(completion json.RawMessage, sig Signal) (bool, error) {
		var rule course.MarkDoneCompletion
		if err := json.Unmarshal(completion, &rule); err != nil {
			return false, fmt.Errorf("progress: reading completion rule: %w", err)
		}
		return sig.MarkedDone, nil
	},

	course.BlockVideoYouTube: func(completion json.RawMessage, sig Signal) (bool, error) {
		var rule course.WatchedCompletion
		if err := json.Unmarshal(completion, &rule); err != nil {
			return false, fmt.Errorf("progress: video completion rule: %w", err)
		}
		// Deliberately does not accept MarkedDone: the YouTube player reports
		// the end event, which is the whole reason this block type is verifiable.
		return sig.WatchedToEnd, nil
	},

	course.BlockQuiz: func(completion json.RawMessage, sig Signal) (bool, error) {
		var rule course.QuizCompletion
		if err := json.Unmarshal(completion, &rule); err != nil {
			return false, fmt.Errorf("progress: quiz completion rule: %w", err)
		}
		if sig.Score == nil {
			return false, nil
		}
		return *sig.Score >= rule.Threshold, nil
	},
}

// Evaluate reports whether a block is complete given its rule and what the
// learner did.
func Evaluate(blockType string, completion json.RawMessage, sig Signal) (bool, error) {
	eval, ok := evaluators[blockType]
	if !ok {
		return false, fmt.Errorf("progress: no evaluator for block type %q", blockType)
	}
	return eval(completion, sig)
}
```

- [ ] **Step 6: Run and confirm the evaluator tests pass**

Run: `cd api && go test ./internal/progress/ -run "TestReading|TestVideo|TestQuiz|TestEvaluate" -v`
Expected: PASS.

- [ ] **Step 7: Write the failing mark-complete test** — `api/internal/progress/complete_test.go`

```go
package progress_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/progress"
	"github.com/Fikhri02/wikipetia/internal/user"
)

type scene struct {
	ctx         context.Context
	runner      db.Runner
	progress    *progress.Service
	learner     uuid.UUID
	stranger    uuid.UUID
	enrolmentID uuid.UUID
	reading     uuid.UUID
	video       uuid.UUID
}

func newScene(t *testing.T) scene {
	t.Helper()
	ctx, runner := pgtest.NewRunner(t)

	store := user.Store{}
	var learner, stranger uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		// Built outside the closure: a composite literal in an `if` header is
		// ambiguous to the Go parser.
		if learner, err = store.EnsureBySubject(ctx, q, "dev|learner", "learner"); err != nil {
			return err
		}
		stranger, err = store.EnsureBySubject(ctx, q, "dev|stranger", "stranger")
		return err
	}))

	courses := course.New(runner)
	c, err := courses.Create(ctx, learner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)
	mod, err := courses.CreateModule(ctx, learner, c.ID, "M1")
	require.NoError(t, err)

	reading, err := courses.CreateBlock(ctx, learner, mod.ID, course.CreateBlockInput{
		Title: "Reading", Type: course.BlockReading,
		Content: json.RawMessage(`{"body_md":"# Atoms"}`),
	})
	require.NoError(t, err)

	video, err := courses.CreateBlock(ctx, learner, mod.ID, course.CreateBlockInput{
		Title: "Video", Type: course.BlockVideoYouTube,
		Content: json.RawMessage(`{"video_id":"abc123"}`),
	})
	require.NoError(t, err)

	enrolments := enrolment.New(runner)
	e, err := enrolments.Enrol(ctx, learner, c.ID, enrolment.ModeSelfStudy)
	require.NoError(t, err)

	return scene{
		ctx:         ctx,
		runner:      runner,
		progress:    progress.New(runner, enrolments),
		learner:     learner,
		stranger:    stranger,
		enrolmentID: e.ID,
		reading:     reading.ID,
		video:       video.ID,
	}
}

func TestMarkReadingCompleteWritesProgressAndEvent(t *testing.T) {
	s := newScene(t)

	got, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	require.NoError(t, err)
	assert.Equal(t, progress.StatusCompleted, got.Status)
	require.NotNil(t, got.CompletedAt)

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 10)
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, events.KindBlockCompleted, records[0].Kind)
		return nil
	}))
}

func TestMarkCompleteIsIdempotent(t *testing.T) {
	s := newScene(t)

	first, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	require.NoError(t, err)

	second, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	require.NoError(t, err)

	assert.Equal(t, progress.StatusCompleted, second.Status)
	assert.Equal(t, first.CompletedAt.Unix(), second.CompletedAt.Unix(),
		"the first completion time must not be overwritten")

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 10)
		require.NoError(t, err)
		assert.Len(t, records, 1,
			"an event fires only on a status transition, never on a repeat")
		return nil
	}))
}

func TestIncompleteSignalStoresInProgressAndEmitsNothing(t *testing.T) {
	s := newScene(t)

	got, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.video,
		progress.Signal{MarkedDone: true}) // wrong signal for a video
	require.NoError(t, err)
	assert.Equal(t, progress.StatusInProgress, got.Status)
	assert.Nil(t, got.CompletedAt)

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 10)
		require.NoError(t, err)
		assert.Empty(t, records)
		return nil
	}))
}

func TestVideoCompletesOnPlayerEndEvent(t *testing.T) {
	s := newScene(t)

	got, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.video,
		progress.Signal{WatchedToEnd: true})
	require.NoError(t, err)
	assert.Equal(t, progress.StatusCompleted, got.Status)
}

func TestMarkCompleteRejectsForeignEnrolment(t *testing.T) {
	s := newScene(t)

	_, err := s.progress.MarkComplete(s.ctx, s.stranger, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	assert.True(t, errors.Is(err, apperr.ErrForbidden),
		"progress writes must be authorized through the enrolment's owner")
}

func TestMarkCompleteRejectsBlockFromAnotherCourse(t *testing.T) {
	s := newScene(t)

	courses := course.New(s.runner)
	other, err := courses.Create(s.ctx, s.learner, course.CreateInput{Title: "Physics"})
	require.NoError(t, err)
	mod, err := courses.CreateModule(s.ctx, s.learner, other.ID, "M1")
	require.NoError(t, err)
	foreign, err := courses.CreateBlock(s.ctx, s.learner, mod.ID, course.CreateBlockInput{
		Title: "Forces", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	_, err = s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, foreign.ID,
		progress.Signal{MarkedDone: true})
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"a block must belong to the enrolment's course")
}
```

- [ ] **Step 8: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: progress.New`.

- [ ] **Step 9: Write `api/internal/progress/service.go`**

```go
package progress

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type BlockProgress struct {
	BlockID     uuid.UUID
	Status      string
	Score       *int
	Attempts    int
	CompletedAt *time.Time
}

type Service struct {
	runner     db.Runner
	enrolments *enrolment.Service
}

func New(runner db.Runner, enrolments *enrolment.Service) *Service {
	return &Service{runner: runner, enrolments: enrolments}
}

// MarkComplete records a completion signal for a block. It is idempotent: the
// event fires only when status transitions to completed, and the original
// completion timestamp is preserved.
func (s *Service) MarkComplete(ctx context.Context, userID, enrolmentID, blockID uuid.UUID, sig Signal) (BlockProgress, error) {
	var out BlockProgress
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		enrol, err := s.enrolments.Load(ctx, q, userID, enrolmentID)
		if err != nil {
			return err
		}

		block, err := q.GetBlockForProgress(ctx, blockID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("block %s", blockID)
		}
		if err != nil {
			return fmt.Errorf("get block: %w", err)
		}
		if block.CourseID != enrol.CourseID {
			return apperr.Invalid("block_id", "block %s is not part of this enrolment's course", blockID)
		}

		existing, err := q.GetBlockProgress(ctx, sqlcgen.GetBlockProgressParams{
			EnrolmentID: enrolmentID, BlockID: blockID,
		})
		alreadyCompleted := err == nil && existing.Status == StatusCompleted
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get block progress: %w", err)
		}

		done, err := Evaluate(block.Type, block.Completion, sig)
		if err != nil {
			return err
		}

		status := StatusInProgress
		var completedAt *time.Time
		if done || alreadyCompleted {
			status = StatusCompleted
			now := time.Now().UTC()
			completedAt = &now
		}

		attempts := int32(1)
		if err == nil {
			attempts = existing.Attempts + 1
		}

		var score *int32
		if sig.Score != nil {
			v := int32(*sig.Score)
			score = &v
		} else if err == nil {
			score = existing.Score
		}

		row, err := q.UpsertBlockProgress(ctx, sqlcgen.UpsertBlockProgressParams{
			EnrolmentID: enrolmentID,
			BlockID:     blockID,
			Status:      status,
			Score:       score,
			Attempts:    attempts,
			CompletedAt: pgconv.Timestamptz(completedAt),
		})
		if err != nil {
			return fmt.Errorf("upsert block progress: %w", err)
		}

		// The event fires on the transition only — never on a repeat.
		if done && !alreadyCompleted {
			if err := events.Append(ctx, q, events.Event{
				UserID:      userID,
				EnrolmentID: &enrolmentID,
				Kind:        events.KindBlockCompleted,
				Payload: map[string]any{
					"block_id":   blockID.String(),
					"block_type": block.Type,
				},
			}); err != nil {
				return err
			}
		}

		out = toBlockProgress(row)
		return nil
	})
	return out, err
}

func toBlockProgress(row sqlcgen.BlockProgress) BlockProgress {
	bp := BlockProgress{
		BlockID:  row.BlockID,
		Status:   row.Status,
		Attempts: int(row.Attempts),
	}
	if row.Score != nil {
		v := int(*row.Score)
		bp.Score = &v
	}
	bp.CompletedAt = pgconv.TimePtr(row.CompletedAt)
	return bp
}
```

- [ ] **Step 10: Confirm the nullable-timestamp conversion uses `pgconv`**

`CompletedAt` is a nullable `timestamptz`, so sqlc's parameter type is `pgtype.Timestamptz`. Use the Task 3 helpers rather than writing another converter — `pgconv.Timestamptz(completedAt)` on the way in, `pgconv.TimePtr(row.CompletedAt)` on the way out. Import `"github.com/Fikhri02/wikipetia/internal/platform/pgconv"`.

There is no `completedAtParam` function: one conversion helper for the whole codebase, in one package.

- [ ] **Step 11: Run the tests and confirm they pass**

Run: `make test`
Expected: all six mark-complete tests PASS.

- [ ] **Step 12: Write `api/cmd/api/handlers_progress.go`**

```go
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

type blockProgressBody struct {
	BlockID     uuid.UUID  `json:"block_id"`
	Status      string     `json:"status"`
	Score       *int       `json:"score,omitempty"`
	Attempts    int        `json:"attempts"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type blockProgressOutput struct {
	Body blockProgressBody
}

type completeBlockInput struct {
	EnrolmentID uuid.UUID `path:"enrolment_id"`
	BlockID     uuid.UUID `path:"block_id"`
	Body        struct {
		MarkedDone   bool `json:"marked_done,omitempty" doc:"The learner pressed done (reading blocks)"`
		WatchedToEnd bool `json:"watched_to_end,omitempty" doc:"The YouTube player reported the end event"`
	}
}

func (s *server) registerProgressRoutes(api huma.API) {
	svc := progress.New(s.runner, enrolment.New(s.runner))

	huma.Register(api, huma.Operation{
		OperationID: "completeBlock", Method: http.MethodPost,
		Path:    "/enrolments/{enrolment_id}/blocks/{block_id}/complete",
		Summary: "Report a completion signal for a reading or video block",
	}, func(ctx context.Context, in *completeBlockInput) (*blockProgressOutput, error) {
		bp, err := svc.MarkComplete(ctx, auth.MustUser(ctx), in.EnrolmentID, in.BlockID, progress.Signal{
			MarkedDone:   in.Body.MarkedDone,
			WatchedToEnd: in.Body.WatchedToEnd,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &blockProgressOutput{Body: blockProgressBody{
			BlockID: bp.BlockID, Status: bp.Status, Score: bp.Score,
			Attempts: bp.Attempts, CompletedAt: bp.CompletedAt,
		}}, nil
	})
}
```

- [ ] **Step 13: Register, test, regenerate, commit**

Add `s.registerProgressRoutes(api)` to `registerRoutes`.

```bash
make test && make generate
git add -A && git commit -m "feat(api): completion evaluators and idempotent mark-complete"
```

---

### Task 15: Quiz grading and per-topic mastery

The central write of the whole slice. Grading happens server-side, mastery updates per topic, and both plus the events land in one transaction.

**Files:**
- Create: `api/db/queries/mastery.sql`
- Create: `api/internal/progress/mastery.go`, `api/internal/progress/mastery_test.go`
- Create: `api/internal/progress/grading.go`, `api/internal/progress/grading_test.go`
- Create: `api/internal/progress/submit.go`, `api/internal/progress/submit_test.go`
- Modify: `api/cmd/api/handlers_progress.go`

**Interfaces:**
- Produces:
  - `progress.UpdateStrength(prev float64, evidenceCount int, score int) float64`
  - `progress.DecayedStrength(strength float64, lastEvidence, now time.Time) float64`
  - `progress.Level(strength float64, evidenceCount int) string` → `"new" | "learning" | "proficient" | "mastered"`
  - `progress.MasteredThreshold = 0.8`, `progress.ProficientThreshold = 0.5`
  - `progress.Answer{QuestionID uuid.UUID, ChoiceIndex *int, Boolean *bool, Text *string}`
  - `progress.QuestionResult{QuestionID uuid.UUID, TopicID *uuid.UUID, Correct bool}`
  - `progress.Grade(questions []course.Question, answers []Answer) ([]QuestionResult, int, error)`
  - `(*Service).SubmitQuiz(ctx, userID, enrolmentID, blockID uuid.UUID, answers []Answer) (QuizResult, error)`
  - `progress.QuizResult{Score int, Progress BlockProgress, Results []QuestionResult, Mastery []TopicStrength}`
  - `progress.TopicStrength{TopicID uuid.UUID, Strength float64, Level string}`

- [ ] **Step 1: Write the failing mastery test** — `api/internal/progress/mastery_test.go`

```go
package progress_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Fikhri02/wikipetia/internal/progress"
)

func TestFirstEvidenceSetsStrengthDirectly(t *testing.T) {
	// With no history, the first score IS the strength — no smoothing toward zero.
	assert.InDelta(t, 0.8, progress.UpdateStrength(0, 0, 80), 0.0001)
	assert.InDelta(t, 0.0, progress.UpdateStrength(0, 0, 0), 0.0001)
}

func TestLaterEvidenceMovesStrengthPartway(t *testing.T) {
	// alpha = 0.3: prev 0.5, score 100 -> 0.5*0.7 + 1.0*0.3 = 0.65
	assert.InDelta(t, 0.65, progress.UpdateStrength(0.5, 3, 100), 0.0001)
	// A bad result pulls it down but does not erase history.
	assert.InDelta(t, 0.35, progress.UpdateStrength(0.5, 3, 0), 0.0001)
}

func TestStrengthStaysWithinZeroAndOne(t *testing.T) {
	assert.LessOrEqual(t, progress.UpdateStrength(1.0, 10, 100), 1.0)
	assert.GreaterOrEqual(t, progress.UpdateStrength(0.0, 10, 0), 0.0)
}

func TestDecayHalvesStrengthOverTheHalfLife(t *testing.T) {
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	assert.InDelta(t, 0.4, progress.DecayedStrength(0.8, thirtyDaysAgo, now), 0.01,
		"a 30-day half-life halves strength after 30 days")
}

func TestDecayDoesNothingForFreshEvidence(t *testing.T) {
	now := time.Now()
	assert.InDelta(t, 0.8, progress.DecayedStrength(0.8, now, now), 0.0001)
	assert.InDelta(t, 0.8, progress.DecayedStrength(0.8, now.Add(time.Hour), now), 0.0001,
		"a future timestamp must not inflate strength")
}

func TestLevelThresholds(t *testing.T) {
	assert.Equal(t, "new", progress.Level(0, 0))
	assert.Equal(t, "learning", progress.Level(0.2, 1))
	assert.Equal(t, "learning", progress.Level(0.49, 4))
	assert.Equal(t, "proficient", progress.Level(0.5, 4))
	assert.Equal(t, "proficient", progress.Level(0.79, 4))
	assert.Equal(t, "mastered", progress.Level(0.8, 4))
	assert.Equal(t, "mastered", progress.Level(1.0, 9))
}
```

- [ ] **Step 2: Run and confirm failure**

Run: `cd api && go test ./internal/progress/ -run "TestFirstEvidence|TestLater|TestStrength|TestDecay|TestLevel" -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `api/internal/progress/mastery.go`**

```go
package progress

import (
	"math"
	"time"
)

// The mastery model is deliberately simple and lives in one place, because
// slice 2's spaced-repetition engine replaces it while writing the same column.
// It is an exponentially weighted moving average with time decay. It does NOT
// model forgetting curves; do not treat its output as a retention prediction.
const (
	masteryAlpha        = 0.3  // weight of the newest evidence
	masteryHalfLifeDays = 30.0 // strength halves after this long without evidence

	ProficientThreshold = 0.5
	MasteredThreshold   = 0.8
)

// UpdateStrength folds a new score (0-100) into an existing strength (0-1).
// With no prior evidence the score becomes the strength outright, so a first
// perfect quiz does not read as half-learned.
func UpdateStrength(prev float64, evidenceCount int, score int) float64 {
	s := float64(score) / 100.0
	if evidenceCount <= 0 {
		return clamp01(s)
	}
	return clamp01(prev*(1-masteryAlpha) + s*masteryAlpha)
}

// DecayedStrength applies time decay for display. It is computed on read rather
// than stored, because a stored level goes stale the moment nobody looks at it.
func DecayedStrength(strength float64, lastEvidence, now time.Time) float64 {
	days := now.Sub(lastEvidence).Hours() / 24.0
	if days <= 0 {
		return clamp01(strength)
	}
	return clamp01(strength * math.Pow(0.5, days/masteryHalfLifeDays))
}

// Level names a strength for the UI.
func Level(strength float64, evidenceCount int) string {
	switch {
	case evidenceCount <= 0:
		return "new"
	case strength >= MasteredThreshold:
		return "mastered"
	case strength >= ProficientThreshold:
		return "proficient"
	default:
		return "learning"
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
```

- [ ] **Step 4: Run and confirm the mastery tests pass**

Run: the command from Step 2.
Expected: PASS.

- [ ] **Step 5: Write the failing grading test** — `api/internal/progress/grading_test.go`

```go
package progress_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

func mcq(id uuid.UUID, topicID *uuid.UUID) course.Question {
	return course.Question{
		ID: id, TopicID: topicID, Kind: course.QuestionMCQSingle,
		Prompt: json.RawMessage(`{"text":"Which is an element?","options":["Water","Iron"]}`),
		Answer: json.RawMessage(`{"correct_index":1}`),
	}
}

func trueFalse(id uuid.UUID, topicID *uuid.UUID) course.Question {
	return course.Question{
		ID: id, TopicID: topicID, Kind: course.QuestionTrueFalse,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	}
}

func typed(id uuid.UUID, topicID *uuid.UUID) course.Question {
	return course.Question{
		ID: id, TopicID: topicID, Kind: course.QuestionTyped,
		Prompt: json.RawMessage(`{"text":"Symbol for iron?"}`),
		Answer: json.RawMessage(`{"accepted":["Fe"],"case_sensitive":false}`),
	}
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }
func strPtr(v string) *string { return &v }

func TestGradeAllCorrectIsOneHundred(t *testing.T) {
	q1, q2, q3 := uuid.New(), uuid.New(), uuid.New()
	questions := []course.Question{mcq(q1, nil), trueFalse(q2, nil), typed(q3, nil)}

	results, score, err := progress.Grade(questions, []progress.Answer{
		{QuestionID: q1, ChoiceIndex: intPtr(1)},
		{QuestionID: q2, Boolean: boolPtr(true)},
		{QuestionID: q3, Text: strPtr("Fe")},
	})
	require.NoError(t, err)
	assert.Equal(t, 100, score)
	require.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Correct)
	}
}

func TestGradeRoundsScoreToNearestPercent(t *testing.T) {
	q1, q2, q3 := uuid.New(), uuid.New(), uuid.New()
	questions := []course.Question{mcq(q1, nil), mcq(q2, nil), mcq(q3, nil)}

	_, score, err := progress.Grade(questions, []progress.Answer{
		{QuestionID: q1, ChoiceIndex: intPtr(1)},
		{QuestionID: q2, ChoiceIndex: intPtr(1)},
		{QuestionID: q3, ChoiceIndex: intPtr(0)},
	})
	require.NoError(t, err)
	assert.Equal(t, 67, score, "2 of 3 rounds to 67, not 66")
}

func TestTypedAnswerIsCaseInsensitiveByDefault(t *testing.T) {
	q := uuid.New()
	_, score, err := progress.Grade([]course.Question{typed(q, nil)},
		[]progress.Answer{{QuestionID: q, Text: strPtr("  fe  ")}})
	require.NoError(t, err)
	assert.Equal(t, 100, score, "whitespace is trimmed and case ignored unless case_sensitive")
}

func TestUnansweredQuestionCountsWrong(t *testing.T) {
	q1, q2 := uuid.New(), uuid.New()
	results, score, err := progress.Grade(
		[]course.Question{mcq(q1, nil), mcq(q2, nil)},
		[]progress.Answer{{QuestionID: q1, ChoiceIndex: intPtr(1)}})
	require.NoError(t, err)
	assert.Equal(t, 50, score)
	require.Len(t, results, 2)
}

func TestAnswerForUnknownQuestionIsRejected(t *testing.T) {
	q := uuid.New()
	_, _, err := progress.Grade([]course.Question{mcq(q, nil)},
		[]progress.Answer{{QuestionID: uuid.New(), ChoiceIndex: intPtr(0)}})
	assert.Error(t, err, "an answer must refer to a question in this block")
}

func TestGradeEmptyQuizIsAnError(t *testing.T) {
	_, _, err := progress.Grade(nil, nil)
	assert.Error(t, err, "a quiz with no questions cannot be scored")
}

func TestGradeCarriesTopicThrough(t *testing.T) {
	topicID := uuid.New()
	q := uuid.New()

	results, _, err := progress.Grade([]course.Question{mcq(q, &topicID)},
		[]progress.Answer{{QuestionID: q, ChoiceIndex: intPtr(1)}})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].TopicID)
	assert.Equal(t, topicID, *results[0].TopicID)
}
```

- [ ] **Step 6: Run and confirm failure**

Run: `cd api && go test ./internal/progress/ -run TestGrade -v`
Expected: FAIL — `undefined: progress.Grade`.

- [ ] **Step 7: Write `api/internal/progress/grading.go`**

```go
package progress

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

// Answer is one submitted answer. Exactly one payload field is meaningful,
// chosen by the question's kind.
type Answer struct {
	QuestionID  uuid.UUID
	ChoiceIndex *int
	Boolean     *bool
	Text        *string
}

type QuestionResult struct {
	QuestionID uuid.UUID
	TopicID    *uuid.UUID
	Correct    bool
}

// Grade scores a submission server-side. Answer keys never leave the server,
// so this is the only place correctness is decided.
//
// Unanswered questions count as wrong: a submission is a submission.
func Grade(questions []course.Question, answers []Answer) ([]QuestionResult, int, error) {
	if len(questions) == 0 {
		return nil, 0, apperr.Invalid("block", "this quiz has no questions")
	}

	byID := make(map[uuid.UUID]course.Question, len(questions))
	for _, q := range questions {
		byID[q.ID] = q
	}

	submitted := make(map[uuid.UUID]Answer, len(answers))
	for _, a := range answers {
		if _, ok := byID[a.QuestionID]; !ok {
			return nil, 0, apperr.Invalid("answers",
				"question %s is not part of this quiz", a.QuestionID)
		}
		submitted[a.QuestionID] = a
	}

	results := make([]QuestionResult, 0, len(questions))
	correct := 0
	for _, q := range questions {
		answer, answered := submitted[q.ID]
		isCorrect := false
		if answered {
			var err error
			isCorrect, err = gradeOne(q, answer)
			if err != nil {
				return nil, 0, err
			}
		}
		if isCorrect {
			correct++
		}
		results = append(results, QuestionResult{
			QuestionID: q.ID, TopicID: q.TopicID, Correct: isCorrect,
		})
	}

	score := int(math.Round(float64(correct) / float64(len(questions)) * 100))
	return results, score, nil
}

func gradeOne(q course.Question, a Answer) (bool, error) {
	switch q.Kind {
	case course.QuestionMCQSingle:
		var key course.MCQAnswer
		if err := json.Unmarshal(q.Answer, &key); err != nil {
			return false, fmt.Errorf("grading: mcq key for %s: %w", q.ID, err)
		}
		if a.ChoiceIndex == nil {
			return false, nil
		}
		return *a.ChoiceIndex == key.CorrectIndex, nil

	case course.QuestionTrueFalse:
		var key course.BoolAnswer
		if err := json.Unmarshal(q.Answer, &key); err != nil {
			return false, fmt.Errorf("grading: true/false key for %s: %w", q.ID, err)
		}
		if a.Boolean == nil {
			return false, nil
		}
		return *a.Boolean == key.Correct, nil

	case course.QuestionTyped:
		var key course.TypedAnswer
		if err := json.Unmarshal(q.Answer, &key); err != nil {
			return false, fmt.Errorf("grading: typed key for %s: %w", q.ID, err)
		}
		if a.Text == nil {
			return false, nil
		}
		given := strings.TrimSpace(*a.Text)
		for _, accepted := range key.Accepted {
			if key.CaseSensitive {
				if given == strings.TrimSpace(accepted) {
					return true, nil
				}
				continue
			}
			if strings.EqualFold(given, strings.TrimSpace(accepted)) {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("grading: unknown question kind %q", q.Kind)
	}
}
```

- [ ] **Step 8: Run and confirm the grading tests pass**

Run: `cd api && go test ./internal/progress/ -run TestGrade -v` plus the typed and unanswered tests.
Expected: PASS.

- [ ] **Step 9: Write `api/db/queries/mastery.sql`**

```sql
-- name: GetTopicMastery :one
SELECT * FROM topic_mastery WHERE enrolment_id = $1 AND topic_id = $2;

-- name: UpsertTopicMastery :one
INSERT INTO topic_mastery (enrolment_id, topic_id, strength, evidence_count, last_evidence_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
ON CONFLICT (enrolment_id, topic_id) DO UPDATE SET
    strength         = EXCLUDED.strength,
    evidence_count   = EXCLUDED.evidence_count,
    last_evidence_at = now(),
    updated_at       = now()
RETURNING *;

-- name: ListTopicMasteryByEnrolment :many
SELECT tm.topic_id, tm.strength, tm.evidence_count, tm.last_evidence_at
FROM topic_mastery tm
JOIN topics t ON t.id = tm.topic_id AND t.archived_at IS NULL
WHERE tm.enrolment_id = $1
ORDER BY t.sort_key;
```

- [ ] **Step 10: Generate, then write the failing submit test** — `api/internal/progress/submit_test.go`

```bash
make generate && cd api && go build ./...
```

```go
package progress_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

// quizScene extends newScene with a topic and a two-question quiz block.
type quizScene struct {
	scene
	topicID    uuid.UUID
	quizID     uuid.UUID
	questionA  uuid.UUID
	questionB  uuid.UUID
}

func newQuizScene(t *testing.T) quizScene {
	t.Helper()
	s := newScene(t)
	courses := course.New(s.runner)

	// The reading block's course is the enrolment's course; reuse it.
	var courseID uuid.UUID
	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		id, err := q.GetBlockCourseID(s.ctx, s.reading)
		if err != nil {
			return err
		}
		courseID = id
		return nil
	}))

	topic, err := courses.CreateTopic(s.ctx, s.learner, courseID, "Atoms")
	require.NoError(t, err)

	mods, err := courses.ListModules(s.ctx, s.learner, courseID)
	require.NoError(t, err)
	require.NotEmpty(t, mods)

	quiz, err := courses.CreateBlock(s.ctx, s.learner, mods[0].ID, course.CreateBlockInput{
		Title: "Quiz", Type: course.BlockQuiz,
		Completion: json.RawMessage(`{"kind":"score_threshold","threshold":70}`),
	})
	require.NoError(t, err)

	qa, err := courses.CreateQuestion(s.ctx, s.learner, quiz.ID, course.QuestionInput{
		Kind: course.QuestionTrueFalse, TopicID: &topic.ID,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	})
	require.NoError(t, err)

	qb, err := courses.CreateQuestion(s.ctx, s.learner, quiz.ID, course.QuestionInput{
		Kind: course.QuestionTrueFalse, TopicID: &topic.ID,
		Prompt: json.RawMessage(`{"text":"Water is an element"}`),
		Answer: json.RawMessage(`{"correct":false}`),
	})
	require.NoError(t, err)

	return quizScene{scene: s, topicID: topic.ID, quizID: quiz.ID, questionA: qa.ID, questionB: qb.ID}
}

func TestSubmitQuizAllCorrectCompletesAndMasters(t *testing.T) {
	s := newQuizScene(t)

	result, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(false)},
	})
	require.NoError(t, err)

	assert.Equal(t, 100, result.Score)
	assert.Equal(t, progress.StatusCompleted, result.Progress.Status)
	require.Len(t, result.Mastery, 1)
	assert.Equal(t, s.topicID, result.Mastery[0].TopicID)
	assert.InDelta(t, 1.0, result.Mastery[0].Strength, 0.0001,
		"first evidence sets strength outright")
	assert.Equal(t, "mastered", result.Mastery[0].Level)
}

func TestSubmitQuizBelowThresholdDoesNotComplete(t *testing.T) {
	s := newQuizScene(t)

	result, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(true)}, // wrong
	})
	require.NoError(t, err)

	assert.Equal(t, 50, result.Score)
	assert.Equal(t, progress.StatusInProgress, result.Progress.Status,
		"50 is below the block's 70 threshold")
	assert.Equal(t, "proficient", result.Mastery[0].Level)
}

func TestSubmitQuizEmitsBlockCompletedAndTopicMastered(t *testing.T) {
	s := newQuizScene(t)

	_, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(false)},
	})
	require.NoError(t, err)

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 10)
		require.NoError(t, err)

		kinds := map[string]int{}
		for _, r := range records {
			kinds[r.Kind]++
		}
		assert.Equal(t, 1, kinds[events.KindBlockCompleted])
		assert.Equal(t, 1, kinds[events.KindTopicMastered])
		return nil
	}))
}

func TestResubmitDoesNotRepeatEvents(t *testing.T) {
	s := newQuizScene(t)

	perfect := []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(false)},
	}

	_, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, perfect)
	require.NoError(t, err)
	_, err = s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, perfect)
	require.NoError(t, err)

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 20)
		require.NoError(t, err)
		kinds := map[string]int{}
		for _, r := range records {
			kinds[r.Kind]++
		}
		assert.Equal(t, 1, kinds[events.KindBlockCompleted],
			"block.completed fires on the transition only")
		assert.Equal(t, 1, kinds[events.KindTopicMastered],
			"topic.mastered fires when the threshold is crossed, not on every submission above it")
		return nil
	}))
}

func TestSubmitQuizCountsAttempts(t *testing.T) {
	s := newQuizScene(t)

	wrong := []progress.Answer{{QuestionID: s.questionA, Boolean: boolPtr(false)}}

	first, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, wrong)
	require.NoError(t, err)
	assert.Equal(t, 1, first.Progress.Attempts)

	second, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, wrong)
	require.NoError(t, err)
	assert.Equal(t, 2, second.Progress.Attempts)
}

func TestSubmitQuizRejectsNonQuizBlock(t *testing.T) {
	s := newQuizScene(t)

	_, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.reading, nil)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestSubmitQuizRejectsForeignEnrolment(t *testing.T) {
	s := newQuizScene(t)

	_, err := s.progress.SubmitQuiz(s.ctx, s.stranger, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
	})
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}
```

- [ ] **Step 11: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: SubmitQuiz`.

- [ ] **Step 12: Write `api/internal/progress/submit.go`**

```go
package progress

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type TopicStrength struct {
	TopicID  uuid.UUID
	Strength float64
	Level    string
}

type QuizResult struct {
	Score    int
	Progress BlockProgress
	Results  []QuestionResult
	Mastery  []TopicStrength
}

// SubmitQuiz grades a submission and records everything it implies — block
// progress, per-topic mastery, and the events describing both — in ONE
// transaction. See the spec's data-flow section: this is the pattern every
// other write follows.
func (s *Service) SubmitQuiz(ctx context.Context, userID, enrolmentID, blockID uuid.UUID, answers []Answer) (QuizResult, error) {
	var out QuizResult

	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		enrol, err := s.enrolments.Load(ctx, q, userID, enrolmentID)
		if err != nil {
			return err
		}

		block, err := q.GetBlockForProgress(ctx, blockID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("block %s", blockID)
		}
		if err != nil {
			return fmt.Errorf("get block: %w", err)
		}
		if block.CourseID != enrol.CourseID {
			return apperr.Invalid("block_id", "block %s is not part of this enrolment's course", blockID)
		}
		if block.Type != course.BlockQuiz {
			return apperr.Invalid("block_id", "block %s is not a quiz", blockID)
		}

		// Answer keys are loaded here and never serialised outward.
		questionRows, err := q.ListQuestionsByBlock(ctx, blockID)
		if err != nil {
			return fmt.Errorf("list questions: %w", err)
		}
		questions := make([]course.Question, 0, len(questionRows))
		for _, row := range questionRows {
			questions = append(questions, course.Question{
				ID: row.ID, BlockID: row.BlockID, TopicID: row.TopicID,
				Kind: row.Kind, Prompt: row.Prompt, Answer: row.Answer, SortKey: row.SortKey,
			})
		}

		results, score, err := Grade(questions, answers)
		if err != nil {
			return err
		}
		out.Score = score
		out.Results = results

		// ---- Block progress ----
		existing, getErr := q.GetBlockProgress(ctx, sqlcgen.GetBlockProgressParams{
			EnrolmentID: enrolmentID, BlockID: blockID,
		})
		if getErr != nil && !errors.Is(getErr, pgx.ErrNoRows) {
			return fmt.Errorf("get block progress: %w", getErr)
		}
		hadProgress := getErr == nil
		alreadyCompleted := hadProgress && existing.Status == StatusCompleted

		done, err := Evaluate(block.Type, block.Completion, Signal{Score: &score})
		if err != nil {
			return err
		}

		status := StatusInProgress
		var completedAt *time.Time
		if done || alreadyCompleted {
			status = StatusCompleted
			now := time.Now().UTC()
			completedAt = &now
		}

		attempts := int32(1)
		if hadProgress {
			attempts = existing.Attempts + 1
		}
		score32 := int32(score)

		progressRow, err := q.UpsertBlockProgress(ctx, sqlcgen.UpsertBlockProgressParams{
			EnrolmentID: enrolmentID,
			BlockID:     blockID,
			Status:      status,
			Score:       &score32,
			Attempts:    attempts,
			CompletedAt: pgconv.Timestamptz(completedAt),
		})
		if err != nil {
			return fmt.Errorf("upsert block progress: %w", err)
		}
		out.Progress = toBlockProgress(progressRow)

		if done && !alreadyCompleted {
			if err := events.Append(ctx, q, events.Event{
				UserID: userID, EnrolmentID: &enrolmentID,
				Kind: events.KindBlockCompleted,
				Payload: map[string]any{
					"block_id": blockID.String(), "block_type": block.Type, "score": score,
				},
			}); err != nil {
				return err
			}
		}

		// ---- Per-topic mastery ----
		// Score each topic on this submission alone, then fold it into that
		// topic's running strength.
		type tally struct{ correct, total int }
		perTopic := map[uuid.UUID]*tally{}
		order := []uuid.UUID{}
		for _, r := range results {
			if r.TopicID == nil {
				continue // untagged questions count toward completion, not mastery
			}
			t, ok := perTopic[*r.TopicID]
			if !ok {
				t = &tally{}
				perTopic[*r.TopicID] = t
				order = append(order, *r.TopicID)
			}
			t.total++
			if r.Correct {
				t.correct++
			}
		}

		for _, topicID := range order {
			t := perTopic[topicID]
			topicScore := int(float64(t.correct) / float64(t.total) * 100)

			prevStrength := 0.0
			prevCount := 0
			prevMastered := false
			current, err := q.GetTopicMastery(ctx, sqlcgen.GetTopicMasteryParams{
				EnrolmentID: enrolmentID, TopicID: topicID,
			})
			if err == nil {
				prevStrength = current.Strength
				prevCount = int(current.EvidenceCount)
				prevMastered = current.Strength >= MasteredThreshold
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("get topic mastery: %w", err)
			}

			newStrength := UpdateStrength(prevStrength, prevCount, topicScore)
			row, err := q.UpsertTopicMastery(ctx, sqlcgen.UpsertTopicMasteryParams{
				EnrolmentID:   enrolmentID,
				TopicID:       topicID,
				Strength:      newStrength,
				EvidenceCount: int32(prevCount + 1),
			})
			if err != nil {
				return fmt.Errorf("upsert topic mastery: %w", err)
			}

			out.Mastery = append(out.Mastery, TopicStrength{
				TopicID:  row.TopicID,
				Strength: row.Strength,
				Level:    Level(row.Strength, int(row.EvidenceCount)),
			})

			// Fires on the crossing only, so a learner who stays mastered does
			// not generate an event per submission.
			if newStrength >= MasteredThreshold && !prevMastered {
				if err := events.Append(ctx, q, events.Event{
					UserID: userID, EnrolmentID: &enrolmentID,
					Kind: events.KindTopicMastered,
					Payload: map[string]any{
						"topic_id": topicID.String(), "strength": newStrength,
					},
				}); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return QuizResult{}, err
	}
	return out, nil
}
```

- [ ] **Step 13: Run the tests and confirm they pass**

Run: `make test`
Expected: all seven submit tests PASS.

- [ ] **Step 14: Add the submit endpoint to `api/cmd/api/handlers_progress.go`**

```go
type submitQuizInput struct {
	EnrolmentID uuid.UUID `path:"enrolment_id"`
	BlockID     uuid.UUID `path:"block_id"`
	Body        struct {
		Answers []struct {
			QuestionID  uuid.UUID `json:"question_id"`
			ChoiceIndex *int      `json:"choice_index,omitempty"`
			Boolean     *bool     `json:"boolean,omitempty"`
			Text        *string   `json:"text,omitempty"`
		} `json:"answers"`
	}
}

type questionResultBody struct {
	QuestionID uuid.UUID `json:"question_id"`
	Correct    bool      `json:"correct"`
}

type topicStrengthBody struct {
	TopicID  uuid.UUID `json:"topic_id"`
	Strength float64   `json:"strength"`
	Level    string    `json:"level"`
}

type submitQuizOutput struct {
	Body struct {
		Score    int                  `json:"score"`
		Progress blockProgressBody    `json:"progress"`
		Results  []questionResultBody `json:"results"`
		Mastery  []topicStrengthBody  `json:"mastery"`
	}
}
```

and register it inside `registerProgressRoutes`:

```go
	huma.Register(api, huma.Operation{
		OperationID: "submitQuiz", Method: http.MethodPost,
		Path:    "/enrolments/{enrolment_id}/blocks/{block_id}/submit",
		Summary: "Submit quiz answers for server-side grading",
	}, func(ctx context.Context, in *submitQuizInput) (*submitQuizOutput, error) {
		answers := make([]progress.Answer, 0, len(in.Body.Answers))
		for _, a := range in.Body.Answers {
			answers = append(answers, progress.Answer{
				QuestionID: a.QuestionID, ChoiceIndex: a.ChoiceIndex,
				Boolean: a.Boolean, Text: a.Text,
			})
		}

		result, err := svc.SubmitQuiz(ctx, auth.MustUser(ctx), in.EnrolmentID, in.BlockID, answers)
		if err != nil {
			return nil, toHTTP(err)
		}

		out := &submitQuizOutput{}
		out.Body.Score = result.Score
		out.Body.Progress = blockProgressBody{
			BlockID: result.Progress.BlockID, Status: result.Progress.Status,
			Score: result.Progress.Score, Attempts: result.Progress.Attempts,
			CompletedAt: result.Progress.CompletedAt,
		}
		out.Body.Results = make([]questionResultBody, 0, len(result.Results))
		for _, r := range result.Results {
			out.Body.Results = append(out.Body.Results, questionResultBody{
				QuestionID: r.QuestionID, Correct: r.Correct,
			})
		}
		out.Body.Mastery = make([]topicStrengthBody, 0, len(result.Mastery))
		for _, m := range result.Mastery {
			out.Body.Mastery = append(out.Body.Mastery, topicStrengthBody{
				TopicID: m.TopicID, Strength: m.Strength, Level: m.Level,
			})
		}
		return out, nil
	})
```

Note `questionResultBody` deliberately carries no correct-answer field: the learner learns *whether* they were right, not what the key was.

- [ ] **Step 15: Run, regenerate, commit**

```bash
make test && make generate
git add -A && git commit -m "feat(api): server-side quiz grading and per-topic mastery"
```

---

### Task 16: The player read — one request, no answer keys

Everything the learning surface needs in one round trip, with the answer keys stripped. The test that proves they are absent is the most important test in this plan.

**Files:**
- Create: `api/db/queries/player.sql`
- Create: `api/internal/progress/player.go`, `api/internal/progress/player_test.go`
- Create: `api/cmd/api/handlers_player.go`, `api/cmd/api/handlers_player_test.go`
- Modify: `api/cmd/api/main.go`

**Interfaces:**
- Produces:
  - `(*Service).Player(ctx, userID, enrolmentID uuid.UUID) (PlayerView, error)`
  - `progress.PlayerView{Enrolment, CourseTitle, StartedAt, Topics []PlayerTopic, Modules []PlayerModule, BlocksCompleted, BlocksTotal int}`
  - `progress.PlayerModule{ID uuid.UUID, Title string, Blocks []PlayerBlock}`
  - `progress.PlayerBlock{ID uuid.UUID, Title, Type string, Content json.RawMessage, Completion json.RawMessage, TopicIDs []uuid.UUID, Status string, Score *int, Attempts int, Questions []PlayerQuestion}`
  - `progress.PlayerQuestion{ID uuid.UUID, Kind string, Prompt json.RawMessage}` — **no Answer field, by construction**
  - `progress.PlayerTopic{TopicID uuid.UUID, Title string, Strength float64, Level string}`

- [ ] **Step 1: Write `api/db/queries/player.sql`**

```sql
-- name: PlayerCourseHeader :one
SELECT c.id AS course_id, c.title AS course_title, c.pet_enabled, e.started_at, e.mode
FROM enrolments e
JOIN courses c ON c.id = e.course_id
WHERE e.id = $1;

-- name: PlayerBlocks :many
SELECT
    b.id,
    b.module_id,
    m.title      AS module_title,
    m.sort_key   AS module_sort_key,
    b.title,
    b.type,
    b.sort_key,
    b.content,
    b.completion,
    COALESCE(
        (SELECT array_agg(bt.topic_id ORDER BY t.sort_key)
         FROM block_topics bt
         JOIN topics t ON t.id = bt.topic_id AND t.archived_at IS NULL
         WHERE bt.block_id = b.id),
        '{}'
    )::uuid[]   AS topic_ids,
    bp.status,
    bp.score,
    COALESCE(bp.attempts, 0) AS attempts
FROM blocks b
JOIN modules m ON m.id = b.module_id AND m.archived_at IS NULL
JOIN enrolments e ON e.course_id = m.course_id
LEFT JOIN block_progress bp ON bp.block_id = b.id AND bp.enrolment_id = e.id
WHERE e.id = $1 AND b.archived_at IS NULL
ORDER BY m.sort_key, b.sort_key;

-- name: PlayerQuestions :many
SELECT q.id, q.block_id, q.kind, q.prompt, q.sort_key
FROM questions q
JOIN blocks b ON b.id = q.block_id AND b.archived_at IS NULL
JOIN modules m ON m.id = b.module_id AND m.archived_at IS NULL
JOIN enrolments e ON e.course_id = m.course_id
WHERE e.id = $1 AND q.archived_at IS NULL
ORDER BY q.block_id, q.sort_key;

-- name: PlayerTopics :many
SELECT
    t.id AS topic_id,
    t.title,
    COALESCE(tm.strength, 0)::double precision AS strength,
    COALESCE(tm.evidence_count, 0)::integer    AS evidence_count,
    tm.last_evidence_at
FROM enrolments e
JOIN topics t ON t.course_id = e.course_id AND t.archived_at IS NULL
LEFT JOIN topic_mastery tm ON tm.topic_id = t.id AND tm.enrolment_id = e.id
WHERE e.id = $1
ORDER BY t.sort_key;
```

**`PlayerQuestions` selects no `answer` column.** That is the enforcement: the learner-facing query cannot leak a key it never reads. Anyone adding `q.answer` here breaks the test in Step 3.

- [ ] **Step 2: Generate and build**

```bash
make generate && cd api && go build ./...
```

- [ ] **Step 3: Write the failing test** — `api/internal/progress/player_test.go`

```go
package progress_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

func TestPlayerViewCarriesTheWholeCourse(t *testing.T) {
	s := newQuizScene(t)

	view, err := s.progress.Player(s.ctx, s.learner, s.enrolmentID)
	require.NoError(t, err)

	assert.Equal(t, "Chemistry", view.CourseTitle)
	assert.False(t, view.StartedAt.IsZero())
	require.Len(t, view.Modules, 1)
	assert.Equal(t, 3, view.BlocksTotal, "reading, video and quiz")
	assert.Equal(t, 0, view.BlocksCompleted)

	require.Len(t, view.Topics, 1)
	assert.Equal(t, "Atoms", view.Topics[0].Title)
	assert.Equal(t, "new", view.Topics[0].Level, "no evidence yet")
}

func TestPlayerViewNeverContainsAnswerKeys(t *testing.T) {
	s := newQuizScene(t)

	view, err := s.progress.Player(s.ctx, s.learner, s.enrolmentID)
	require.NoError(t, err)

	// Serialise the whole view and assert no answer key survives anywhere in it.
	encoded, err := json.Marshal(view)
	require.NoError(t, err)
	body := string(encoded)

	assert.NotContains(t, body, "correct_index")
	assert.NotContains(t, body, `"correct"`)
	assert.NotContains(t, body, "accepted")
	assert.False(t, strings.Contains(body, `"answer"`),
		"the player view must never carry an answer field — devtools would defeat every quiz")

	// And the questions are still usable: prompts are present.
	var found bool
	for _, mod := range view.Modules {
		for _, block := range mod.Blocks {
			if len(block.Questions) > 0 {
				found = true
				assert.Contains(t, string(block.Questions[0].Prompt), "Iron")
			}
		}
	}
	assert.True(t, found, "the quiz block must expose its questions")
}

func TestPlayerViewReflectsProgressAndMastery(t *testing.T) {
	s := newQuizScene(t)

	_, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	require.NoError(t, err)

	_, err = s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(false)},
	})
	require.NoError(t, err)

	view, err := s.progress.Player(s.ctx, s.learner, s.enrolmentID)
	require.NoError(t, err)

	assert.Equal(t, 2, view.BlocksCompleted, "the reading and the quiz")
	require.Len(t, view.Topics, 1)
	assert.Equal(t, "mastered", view.Topics[0].Level)
	assert.InDelta(t, 1.0, view.Topics[0].Strength, 0.0001)

	var quizStatus string
	for _, mod := range view.Modules {
		for _, block := range mod.Blocks {
			if block.ID == s.quizID {
				quizStatus = block.Status
				require.NotNil(t, block.Score)
				assert.Equal(t, 100, *block.Score)
			}
		}
	}
	assert.Equal(t, progress.StatusCompleted, quizStatus)
}

func TestPlayerViewRejectsForeignEnrolment(t *testing.T) {
	s := newQuizScene(t)
	_, err := s.progress.Player(s.ctx, s.stranger, s.enrolmentID)
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}

func TestPlayerViewUsesNotStartedForUntouchedBlocks(t *testing.T) {
	s := newQuizScene(t)

	view, err := s.progress.Player(s.ctx, s.learner, s.enrolmentID)
	require.NoError(t, err)

	for _, mod := range view.Modules {
		for _, block := range mod.Blocks {
			assert.Equal(t, "not_started", block.Status,
				"a block with no progress row reads as not_started, not empty string")
			assert.Equal(t, 0, block.Attempts)
		}
	}
}
```

Note `"not_started"` exists only in the view: the database stores `in_progress` or `completed`, and the absence of a row means not started. Encoding that absence as a status in the API keeps the front end free of null checks.

- [ ] **Step 4: Run and confirm failure**

Run: `make test`
Expected: FAIL — `undefined: s.progress.Player`.

- [ ] **Step 5: Write `api/internal/progress/player.go`**

```go
package progress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

// StatusNotStarted exists only in the player view. The database has no such
// row; its absence means not started.
const StatusNotStarted = "not_started"

// PlayerQuestion has no Answer field. That is deliberate and structural: the
// type cannot carry a key, so no refactor can accidentally serialise one.
type PlayerQuestion struct {
	ID     uuid.UUID       `json:"id"`
	Kind   string          `json:"kind"`
	Prompt json.RawMessage `json:"prompt"`
}

type PlayerBlock struct {
	ID         uuid.UUID        `json:"id"`
	Title      string           `json:"title"`
	Type       string           `json:"type"`
	Content    json.RawMessage  `json:"content"`
	Completion json.RawMessage  `json:"completion"`
	TopicIDs   []uuid.UUID      `json:"topic_ids"`
	Status     string           `json:"status"`
	Score      *int             `json:"score,omitempty"`
	Attempts   int              `json:"attempts"`
	Questions  []PlayerQuestion `json:"questions,omitempty"`
}

type PlayerModule struct {
	ID     uuid.UUID     `json:"id"`
	Title  string        `json:"title"`
	Blocks []PlayerBlock `json:"blocks"`
}

type PlayerTopic struct {
	TopicID  uuid.UUID `json:"topic_id"`
	Title    string    `json:"title"`
	Strength float64   `json:"strength"`
	Level    string    `json:"level"`
}

type PlayerView struct {
	EnrolmentID     uuid.UUID      `json:"enrolment_id"`
	CourseID        uuid.UUID      `json:"course_id"`
	CourseTitle     string         `json:"course_title"`
	PetEnabled      bool           `json:"pet_enabled"`
	Mode            string         `json:"mode"`
	StartedAt       time.Time      `json:"started_at"`
	BlocksTotal     int            `json:"blocks_total"`
	BlocksCompleted int            `json:"blocks_completed"`
	Topics          []PlayerTopic  `json:"topics"`
	Modules         []PlayerModule `json:"modules"`
}

// Player returns everything the learning surface needs in one round trip:
// the course structure, this learner's progress on every block, the questions
// (prompts only), and per-topic mastery with decay applied.
func (s *Service) Player(ctx context.Context, userID, enrolmentID uuid.UUID) (PlayerView, error) {
	var out PlayerView

	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.enrolments.Load(ctx, q, userID, enrolmentID); err != nil {
			return err
		}

		header, err := q.PlayerCourseHeader(ctx, enrolmentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("enrolment %s", enrolmentID)
		}
		if err != nil {
			return fmt.Errorf("player header: %w", err)
		}

		out.EnrolmentID = enrolmentID
		out.CourseID = header.CourseID
		out.CourseTitle = header.CourseTitle
		out.PetEnabled = header.PetEnabled
		out.Mode = header.Mode
		out.StartedAt = header.StartedAt

		questionRows, err := q.PlayerQuestions(ctx, enrolmentID)
		if err != nil {
			return fmt.Errorf("player questions: %w", err)
		}
		questionsByBlock := map[uuid.UUID][]PlayerQuestion{}
		for _, row := range questionRows {
			questionsByBlock[row.BlockID] = append(questionsByBlock[row.BlockID], PlayerQuestion{
				ID: row.ID, Kind: row.Kind, Prompt: row.Prompt,
			})
		}

		blockRows, err := q.PlayerBlocks(ctx, enrolmentID)
		if err != nil {
			return fmt.Errorf("player blocks: %w", err)
		}

		// The query orders by module then block sort key, so a single pass
		// builds the modules in order without sorting again.
		var modules []PlayerModule
		indexByModule := map[uuid.UUID]int{}

		for _, row := range blockRows {
			status := StatusNotStarted
			if row.Status != nil {
				status = *row.Status
			}
			var score *int
			if row.Score != nil {
				v := int(*row.Score)
				score = &v
			}
			topicIDs := row.TopicIds
			if topicIDs == nil {
				topicIDs = []uuid.UUID{}
			}

			block := PlayerBlock{
				ID: row.ID, Title: row.Title, Type: row.Type,
				Content: row.Content, Completion: row.Completion,
				TopicIDs: topicIDs, Status: status, Score: score,
				Attempts:  int(row.Attempts),
				Questions: questionsByBlock[row.ID],
			}

			out.BlocksTotal++
			if status == StatusCompleted {
				out.BlocksCompleted++
			}

			idx, seen := indexByModule[row.ModuleID]
			if !seen {
				modules = append(modules, PlayerModule{
					ID: row.ModuleID, Title: row.ModuleTitle, Blocks: []PlayerBlock{},
				})
				idx = len(modules) - 1
				indexByModule[row.ModuleID] = idx
			}
			modules[idx].Blocks = append(modules[idx].Blocks, block)
		}
		if modules == nil {
			modules = []PlayerModule{}
		}
		out.Modules = modules

		topicRows, err := q.PlayerTopics(ctx, enrolmentID)
		if err != nil {
			return fmt.Errorf("player topics: %w", err)
		}
		now := time.Now().UTC()
		out.Topics = make([]PlayerTopic, 0, len(topicRows))
		for _, row := range topicRows {
			strength := row.Strength
			// Decay is computed on read, never stored.
			if last := pgconv.TimePtr(row.LastEvidenceAt); last != nil {
				strength = DecayedStrength(strength, *last, now)
			}
			out.Topics = append(out.Topics, PlayerTopic{
				TopicID:  row.TopicID,
				Title:    row.Title,
				Strength: strength,
				Level:    Level(strength, int(row.EvidenceCount)),
			})
		}
		return nil
	})

	if err != nil {
		return PlayerView{}, err
	}
	return out, nil
}
```

Note: a module with no blocks will not appear in the player view, because the query is block-driven. That is correct for a learner — there is nothing to do in an empty module — and differs deliberately from the builder's outline, which must show empty modules so they can be filled.

- [ ] **Step 6: Run the tests and confirm they pass**

Run: `make test`
Expected: PASS, including `TestPlayerViewNeverContainsAnswerKeys`.

- [ ] **Step 7: Write `api/cmd/api/handlers_player.go`**

```go
package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

type playerInput struct {
	ID uuid.UUID `path:"id" doc:"Enrolment id"`
}

// The view type is already shaped for the wire, with its own json tags and no
// answer fields, so the handler returns it directly rather than remapping it.
type playerOutput struct {
	Body progress.PlayerView
}

func (s *server) registerPlayerRoutes(api huma.API) {
	svc := progress.New(s.runner, enrolment.New(s.runner))

	huma.Register(api, huma.Operation{
		OperationID: "getEnrolment", Method: http.MethodGet, Path: "/enrolments/{id}",
		Summary: "Everything the learning surface needs, in one request",
	}, func(ctx context.Context, in *playerInput) (*playerOutput, error) {
		view, err := svc.Player(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &playerOutput{Body: view}, nil
	})
}
```

- [ ] **Step 8: Write the handler test** — `api/cmd/api/handlers_player_test.go`

```go
package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayerEndpointOmitsAnswerKeys(t *testing.T) {
	api, _ := newTestAPI(t, "learner")

	courseID := newCourse(t, api, "Chemistry")

	modResp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": "M1"})
	require.Equal(t, http.StatusCreated, modResp.Code)
	moduleID := idFromBody(t, modResp.Body.Bytes())

	quizResp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Quiz", "type": "quiz",
	})
	require.Equal(t, http.StatusCreated, quizResp.Code)
	quizID := idFromBody(t, quizResp.Body.Bytes())

	qResp := api.Post("/blocks/"+quizID+"/questions", map[string]any{
		"kind":   "mcq_single",
		"prompt": map[string]any{"text": "Which is an element?", "options": []string{"Water", "Iron"}},
		"answer": map[string]any{"correct_index": 1},
	})
	require.Equal(t, http.StatusCreated, qResp.Code, qResp.Body.String())

	enrolResp := api.Post("/enrolments", map[string]any{
		"course_id": courseID, "mode": "self_study",
	})
	require.Equal(t, http.StatusCreated, enrolResp.Code, enrolResp.Body.String())
	enrolmentID := idFromBody(t, enrolResp.Body.Bytes())

	playerResp := api.Get("/enrolments/" + enrolmentID)
	require.Equal(t, http.StatusOK, playerResp.Code, playerResp.Body.String())

	body := playerResp.Body.String()
	assert.Contains(t, body, "Which is an element?", "prompts must be present")
	assert.Contains(t, body, "Iron", "options must be present")
	assert.NotContains(t, body, "correct_index",
		"the answer key must never reach the learner over HTTP")
}
```

- [ ] **Step 9: Register, test, regenerate, commit**

Add `s.registerPlayerRoutes(api)` to `registerRoutes`.

```bash
make test && make generate
git add -A && git commit -m "feat(api): player aggregate read with answer keys excluded"
```

---

### Task 17: CI gates, the import-boundary check, and the seed script

The boundary rule the spec promised is only real if a machine enforces it. This task makes the architecture self-defending and gives you a course to click through.

**Files:**
- Create: `scripts/check-boundaries.sh`
- Create: `.github/workflows/ci.yml`
- Create: `api/cmd/seed/main.go`
- Modify: `Makefile` (add `seed`, `check`, `boundaries`)

**Interfaces:**
- Consumes: everything above.
- Produces: `make check` — the single command CI and you both run.

- [ ] **Step 1: Write `scripts/check-boundaries.sh`**

```bash
#!/usr/bin/env bash
# Enforces the architecture rules the spec states. These are not style
# preferences: each one, if violated, undoes a decision the design depends on.
set -euo pipefail

cd "$(dirname "$0")/.."
failed=0

fail() {
    echo "BOUNDARY VIOLATION: $1" >&2
    failed=1
}

# Rule 1: no learning package may import engagement.
# This is what keeps pets opt-in and deletable as a directory (spec section 3).
for pkg in course enrolment progress srs events; do
    dir="api/internal/$pkg"
    [ -d "$dir" ] || continue
    if grep -rn --include='*.go' 'wikipetia/internal/engagement' "$dir" 2>/dev/null; then
        fail "$pkg imports internal/engagement"
    fi
done

# Rule 2: no domain package may import net/http.
# Handlers own HTTP so the domain stays reusable by a CLI, a job, or another
# client (spec section 5). internal/platform/auth is the documented exception:
# middleware is HTTP by definition.
while IFS= read -r file; do
    case "$file" in
        api/internal/platform/auth/*) continue ;;
    esac
    if grep -qE '^\s*"net/http"$|^\s*_ "net/http"$' "$file"; then
        fail "$file imports net/http"
    fi
done < <(find api/internal -name '*.go' ! -name '*_test.go')

# Rule 3: no SQL outside the generated code and the query files.
# All SQL lives in db/queries as sqlc sources (spec section 5).
while IFS= read -r file; do
    case "$file" in
        api/internal/platform/sqlcgen/*) continue ;;
    esac
    if grep -qiE '(SELECT|INSERT INTO|UPDATE .* SET|DELETE FROM)[[:space:]]' "$file"; then
        fail "$file appears to contain inline SQL"
    fi
done < <(find api/internal api/cmd -name '*.go' ! -name '*_test.go')

if [ "$failed" -ne 0 ]; then
    echo "" >&2
    echo "These rules come from the design spec. If a rule is genuinely wrong," >&2
    echo "change the spec and this script together — do not silence it locally." >&2
    exit 1
fi

echo "boundaries OK"
```

```bash
chmod +x scripts/check-boundaries.sh
```

- [ ] **Step 2: Add targets to the `Makefile`**

```makefile
boundaries:
	./scripts/check-boundaries.sh

check: boundaries
	cd api && go vet ./...
	cd api && $(GOBIN)/sqlc diff
	cd api && go run ./cmd/api -dump-openapi > /tmp/openapi.check.yaml
	diff -q api/openapi.yaml /tmp/openapi.check.yaml || \
		(echo "openapi.yaml is stale — run 'make generate' and commit it" && exit 1)
	$(MAKE) test

seed: migrate-up
	cd api && DATABASE_URL="$(DB_URL)" go run ./cmd/seed
```

`sqlc diff` fails if the SQL changed without regenerating; the `openapi.yaml` diff fails if a handler changed without regenerating. Both are drift the web plan would otherwise inherit silently.

- [ ] **Step 3: Verify the boundary check catches a real violation**

Prove the check works before trusting it:

```bash
cat >> api/internal/course/service.go <<'EOF'

// TEMPORARY: deliberate violation to prove the boundary check works.
func boundaryCanary() string { return "SELECT 1 FROM courses" }
EOF
make boundaries
```
Expected: FAIL with `api/internal/course/service.go appears to contain inline SQL`.

Now remove it:

```bash
git checkout api/internal/course/service.go
make boundaries
```
Expected: `boundaries OK`. A check you have never seen fail is not a check.

- [ ] **Step 4: Write `api/cmd/seed/main.go`**

```go
// Command seed creates one small course so manual testing and demos do not
// start with twenty minutes of clicking. Safe to run repeatedly: it makes a new
// course each time.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/config"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
	"github.com/google/uuid"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	runner := db.PoolRunner{Pool: pool}

	var owner uuid.UUID
	if err := runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		owner, err = user.Store{}.EnsureBySubject(ctx, q, "dev|dev", "dev")
		return err
	}); err != nil {
		return err
	}

	courses := course.New(runner)

	c, err := courses.Create(ctx, owner, course.CreateInput{
		Title:       "Form 4 Chemistry — seed",
		Description: "A seeded course for manual testing.",
		PetEnabled:  true,
	})
	if err != nil {
		return err
	}

	atoms, err := courses.CreateTopic(ctx, owner, c.ID, "Atomic structure")
	if err != nil {
		return err
	}
	bonding, err := courses.CreateTopic(ctx, owner, c.ID, "Chemical bonding")
	if err != nil {
		return err
	}

	mod, err := courses.CreateModule(ctx, owner, c.ID, "Module 1 — Matter")
	if err != nil {
		return err
	}

	reading, err := courses.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "What is an atom?",
		Type:  course.BlockReading,
		Content: json.RawMessage(`{"body_md":"# Atoms\n\nAn atom is the smallest unit of an element."}`),
	})
	if err != nil {
		return err
	}
	if err := courses.SetBlockTopics(ctx, owner, reading.ID, []uuid.UUID{atoms.ID}); err != nil {
		return err
	}

	video, err := courses.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title:   "Atomic structure explained",
		Type:    course.BlockVideoYouTube,
		Content: json.RawMessage(`{"video_id":"FSyAehMdpyI"}`),
	})
	if err != nil {
		return err
	}
	if err := courses.SetBlockTopics(ctx, owner, video.ID, []uuid.UUID{atoms.ID}); err != nil {
		return err
	}

	quiz, err := courses.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "Check yourself",
		Type:  course.BlockQuiz,
	})
	if err != nil {
		return err
	}
	if err := courses.SetBlockTopics(ctx, owner, quiz.ID, []uuid.UUID{atoms.ID, bonding.ID}); err != nil {
		return err
	}

	questions := []course.QuestionInput{
		{
			Kind: course.QuestionMCQSingle, TopicID: &atoms.ID,
			Prompt: json.RawMessage(`{"text":"Which of these is an element?","options":["Water","Iron","Salt"]}`),
			Answer: json.RawMessage(`{"correct_index":1}`),
		},
		{
			Kind: course.QuestionTrueFalse, TopicID: &atoms.ID,
			Prompt: json.RawMessage(`{"text":"An atom is smaller than a molecule of water."}`),
			Answer: json.RawMessage(`{"correct":true}`),
		},
		{
			Kind: course.QuestionTyped, TopicID: &bonding.ID,
			Prompt: json.RawMessage(`{"text":"What is the chemical symbol for iron?"}`),
			Answer: json.RawMessage(`{"accepted":["Fe"],"case_sensitive":false}`),
		},
	}
	for _, in := range questions {
		if _, err := courses.CreateQuestion(ctx, owner, quiz.ID, in); err != nil {
			return err
		}
	}

	e, err := enrolment.New(runner).Enrol(ctx, owner, c.ID, enrolment.ModeSelfStudy)
	if err != nil {
		return err
	}

	fmt.Printf("seeded\n  course     %s\n  enrolment  %s\n  try        curl -s localhost:8080/enrolments/%s | head -40\n",
		c.ID, e.ID, e.ID)
	return nil
}
```

- [ ] **Step 5: Write `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
  pull_request:

jobs:
  api:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:18
        env:
          POSTGRES_USER: wikipetia
          POSTGRES_PASSWORD: wikipetia
          POSTGRES_DB: wikipetia_test
        ports:
          - 5433:5432
        options: >-
          --health-cmd "pg_isready -U wikipetia"
          --health-interval 2s
          --health-timeout 3s
          --health-retries 20

    env:
      TEST_DB_URL: postgres://wikipetia:wikipetia@localhost:5433/wikipetia_test?sslmode=disable

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.1'
          cache-dependency-path: api/go.sum

      - name: Install tools
        run: |
          go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
          go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1

      - name: Migrate test database
        run: $(go env GOPATH)/bin/migrate -path api/db/migrations -database "$TEST_DB_URL" up

      - name: Boundaries
        run: ./scripts/check-boundaries.sh

      - name: Vet
        run: cd api && go vet ./...

      - name: sqlc is up to date
        run: cd api && $(go env GOPATH)/bin/sqlc diff

      - name: openapi.yaml is up to date
        run: |
          cd api && go run ./cmd/api -dump-openapi > /tmp/openapi.check.yaml
          diff -u openapi.yaml /tmp/openapi.check.yaml

      - name: Test
        run: cd api && TEST_DATABASE_URL="$TEST_DB_URL" go test ./... -count=1
```

The CI job migrates the test database directly rather than using `make test`, because the Makefile's `testdb` target assumes Docker Compose while CI provides Postgres as a service.

- [ ] **Step 6: Run the whole gate locally**

```bash
make check
```
Expected: `boundaries OK`, no vet output, no sqlc diff, no openapi diff, all tests pass.

- [ ] **Step 7: Seed and walk the definition of done by hand**

```bash
make seed
make run   # in another terminal
```

Then confirm each numbered item from the spec's definition of done, against the API:

```bash
# 1-3: the seed created a course with topics, a module, three blocks and an enrolment.
ENROL=<enrolment id printed by make seed>

# The player view: structure, progress, mastery, and NO answer keys.
curl -s localhost:8080/enrolments/$ENROL | python3 -m json.tool | head -60
curl -s localhost:8080/enrolments/$ENROL | grep -c correct_index   # must print 0

# 4: complete the reading, then the video via the player's end event.
READING=<reading block id from the player view>
curl -s -X POST localhost:8080/enrolments/$ENROL/blocks/$READING/complete \
  -H 'content-type: application/json' -d '{"marked_done":true}'

VIDEO=<video block id>
curl -s -X POST localhost:8080/enrolments/$ENROL/blocks/$VIDEO/complete \
  -H 'content-type: application/json' -d '{"watched_to_end":true}'

# Take the quiz.
QUIZ=<quiz block id>;  Q1=<question 1 id>;  Q2=<question 2 id>;  Q3=<question 3 id>
curl -s -X POST localhost:8080/enrolments/$ENROL/blocks/$QUIZ/submit \
  -H 'content-type: application/json' \
  -d "{\"answers\":[{\"question_id\":\"$Q1\",\"choice_index\":1},{\"question_id\":\"$Q2\",\"boolean\":true},{\"question_id\":\"$Q3\",\"text\":\"Fe\"}]}"

# 5-6: progress and mastery reflect it, and survive a restart.
curl -s localhost:8080/enrolments/$ENROL | python3 -c \
  'import json,sys; v=json.load(sys.stdin); print(v["blocks_completed"],"/",v["blocks_total"]); print([(t["title"],round(t["strength"],2),t["level"]) for t in v["topics"]])'

# 7: the events were written in the same transactions.
make psql
# then: SELECT kind, payload->>'block_id', payload->>'topic_id' FROM events ORDER BY id;
```

Expected: `3 / 3`; both topics at `mastered`; and the `events` table holding `block.completed` rows for all three blocks plus `topic.mastered` for both topics.

- [ ] **Step 8: Audit authorization coverage**

The spec requires that no endpoint can be reached with someone else's data. Rather than one test per endpoint — which would mostly re-test huma's routing — authorization is enforced on five paths, each tested at the service layer. Confirm every endpoint routes through one of them:

| Authorization path | Tested by | Endpoints it guards |
|---|---|---|
| `course.RequireOwner` / `load` | `TestGetRejectsNonOwner`, `TestArchiveRejectsNonOwner` | every `/courses/*` read and write, plus the outline |
| `loadTopic` → `RequireOwner` | `TestCreateTopicRejectsStranger`, `TestArchiveTopicRejectsStranger` | `/topics/*`, `/courses/{id}/topics` |
| `loadModule` → `RequireOwner` | `TestCreateBlockRejectsStranger` (via module) | `/modules/*`, `/courses/{id}/modules` |
| `loadBlock` → `GetBlockCourseID` → `RequireOwner` | `TestCreateBlockRejectsStranger` | `/blocks/*`, `/questions/*`, block topics, questions |
| `enrolment.Load` | `TestMarkCompleteRejectsForeignEnrolment`, `TestSubmitQuizRejectsForeignEnrolment`, `TestPlayerViewRejectsForeignEnrolment` | `/enrolments/{id}`, `complete`, `submit` |

Run this to confirm no handler reaches a service method without one of those guards:

```bash
# Every domain method called from a handler must take a user id as its first
# argument after ctx. This greps for calls that do not.
grep -rn 'svc\.' api/cmd/api/handlers_*.go | grep -v 'auth.MustUser(ctx)' || echo "all handler calls pass the authenticated user"
```
Expected: `all handler calls pass the authenticated user`.

If that prints any line, a handler is calling a service without the caller's identity — which means authorization is being skipped. Fix it before committing.

- [ ] **Step 9: Commit**

```bash
git add -A && git commit -m "ci: boundary checks, drift gates, seed script"
```

- [ ] **Step 10: Verify the branch is green end to end**

```bash
make down && make up && make migrate-up && make check
```
Expected: everything passes from a cold database. A plan that only works on a warm machine is not finished.

---

## Slice 1 API — done

At this point the API satisfies every item of the spec's definition of done except the browser itself, and `api/openapi.yaml` is the contract the web plan consumes. The remaining work — the Vite/React PWA, the block component registry, the YouTube IFrame player integration, and the single Playwright smoke test — is the companion **web plan**, written next.

Do not start the web plan until `make check` is green from a cold database on this branch.
