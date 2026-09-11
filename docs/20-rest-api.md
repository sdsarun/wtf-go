# Project 20 — HTTP REST API (stdlib only)

| | |
|---|---|
| **Difficulty** | 4 / 5 |
| **Estimated time** | 12–16 hours |
| **Prerequisites** | [12](12-json-config-loader.md), [13](13-report-engine.md), [17](17-in-memory-cache.md) |
| **Builds toward** | [21 – Auth & Sessions](21-auth-sessions.md), [26 – Reverse Proxy](26-reverse-proxy.md) |

## Why this project

A real HTTP service built on **only** `net/http` — Go 1.22+'s `ServeMux` gives you real
routing (method + path patterns + `{id}` variables) without a framework. This is where
middleware chaining, panic recovery, structured logging, conditional requests
(`ETag`/`If-Match`), and `httptest`-driven testing all come together. Project 21 bolts
auth onto exactly this service.

## Go concepts you MUST use

- [ ] `net/http` server: `http.ServeMux` with Go 1.22+ method+path patterns
      (`"POST /tasks"`, `"GET /tasks/{id}"`), `r.PathValue`, `http.Server` with
      `ReadHeaderTimeout`/`WriteTimeout`, `Server.Shutdown(ctx)`
- [ ] **middleware chaining**: `func(http.Handler) http.Handler`, composed in order
- [ ] a **panic-recovery** middleware (never let a handler panic take the process down;
      never leak the panic message to the client)
- [ ] `context` values — a per-request ID generated in middleware, read by the logger and
      returned in a response header
- [ ] `encoding/json` request/response bodies; `json.Decoder.DisallowUnknownFields`;
      `http.MaxBytesReader` for body-size limits
- [ ] `mime/multipart` — a file-upload endpoint
- [ ] `net/http/httptest` — both `httptest.NewRecorder()` (unit-test a handler directly)
      and `httptest.NewServer` (integration test over a real socket)
- [ ] `testing.M` / `TestMain` for suite-wide setup
- [ ] `log/slog` — one structured line per request (method, path, status, duration,
      request_id), `slog.NewJSONHandler`
- [ ] `signal.NotifyContext` + `Server.Shutdown` for graceful shutdown

## Background

**Resource:** a `Task` — `id, title, done, priority, tags, created_at, updated_at,
version`. `version` is an integer bumped on every mutation and echoed as the `ETag`
header (`ETag: "3"`); a mutating request may send `If-Match: "3"` and gets `412
Precondition Failed` if the current version differs (optimistic concurrency).

**Response envelope.** Success bodies are the resource (or a list envelope) directly, no
wrapper. Errors are always:

```json
{ "error": { "code": "validation_failed", "message": "...", "fields": {"title": "..."} } }
```

`fields` is present only for `validation_failed`.

**Storage:** in-memory, a `TaskStore` interface backed by a mutex-guarded map — this is a
service layer you could later swap for Project 22's storage engine without touching
handlers.

## Requirements

### Routes

| Method | Path | Body | Success | Notes |
|---|---|---|---|---|
| `POST` | `/tasks` | `{title, priority?, tags?}` | `201` + `Location`, `ETag` | |
| `GET` | `/tasks` | — | `200` | query: `done, priority, tag, limit, offset, sort` |
| `GET` | `/tasks/{id}` | — | `200` + `ETag` | |
| `PUT` | `/tasks/{id}` | `{title, priority?, tags?, done?}` | `200` + `ETag` | full replace; honors `If-Match` |
| `PATCH` | `/tasks/{id}` | any subset of fields | `200` + `ETag` | partial merge; honors `If-Match` |
| `DELETE` | `/tasks/{id}` | — | `204` | honors `If-Match` |
| `POST` | `/tasks/{id}/attachment` | multipart, field `file`, ≤5MiB | `201` `{filename,size,content_type}` | |
| `GET` | `/tasks/{id}/attachment` | — | `200` binary | `Content-Type`/`Content-Disposition` set |
| `GET` | `/healthz` | — | `200` `{"status":"ok","uptime":"..."}` | never touches the store |

### Validation

`title`: required, 1–200 runes. `priority`: `low`/`med`/`high`, default `med`.
`tags`: ≤10 items, each 1–30 runes, deduplicated. `done`: bool, default `false`.

### List query params

`done=true|false` · `priority=P` · `tag=T` (repeatable, AND semantics) ·
`limit` (default `20`, max `100`) · `offset` (default `0`) ·
`sort` = `created_at|title|priority` optionally prefixed `-` for descending (default
`-created_at`). Response:

```json
{"data": [ {task}, ... ], "total": 37, "limit": 20, "offset": 0}
```

### Status codes

| Code | When |
|---|---|
| `200` | GET/PUT/PATCH success |
| `201` | POST create success (resource or attachment) |
| `204` | DELETE success |
| `400` | malformed JSON, unknown field, validation failure, bad query param |
| `404` | unknown id, unknown route |
| `405` | known route, wrong method (native `ServeMux` behaviour — verify, don't hand-roll) |
| `412` | `If-Match` present and doesn't match current version |
| `413` | body exceeds the size limit |
| `415` | `Content-Type` on a JSON body isn't `application/json` |
| `500` | recovered panic or unexpected internal error |

### Exit codes (the binary)

| Code | Meaning |
|---|---|
| `0` | clean shutdown (SIGINT/SIGTERM, `Server.Shutdown` completed within `-shutdown-timeout`) |
| `1` | failed to bind the listener, or shutdown deadline exceeded |
| `2` | bad flags / config |

### Case specification — SUCCESS (via `httptest.NewServer`)

| # | Request | Response | 
|---|---|---|
| S1 | `POST /tasks {"title":"buy milk"}` | `201`, `id` assigned, `priority:"med"`, `version:1`, `ETag:"1"`, `Location:/tasks/{id}` |
| S2 | `GET /tasks/{id}` (from S1) | `200`, same body, `ETag:"1"` |
| S3 | `PUT /tasks/{id} {"title":"buy oat milk"}` `If-Match:"1"` | `200`, `title` updated, `version:2`, `ETag:"2"` |
| S4 | `PATCH /tasks/{id} {"done":true}` | `200`, `done:true`, other fields unchanged, `version:3` |
| S5 | `DELETE /tasks/{id}` `If-Match:"3"` | `204`, empty body |
| S6 | `GET /tasks/{id}` (after S5) | `404` |
| S7 | create 3 tasks (priorities low/med/high), `GET /tasks?priority=high` | `200`, `total:1`, `data` has 1 item |
| S8 | `GET /tasks?limit=1&offset=1&sort=title` after creating 3 with distinct titles | `200`, the second title alphabetically |
| S9 | `POST /tasks {}` — missing title | `400 validation_failed`, `fields.title` present |
| S10 | `POST /tasks/{id}/attachment` multipart `file=hello.txt` (12 bytes) | `201 {"filename":"hello.txt","size":12,...}` |
| S11 | `GET /tasks/{id}/attachment` (after S10) | `200`, body = the 12 bytes, correct `Content-Type` |
| S12 | `GET /healthz` | `200 {"status":"ok",...}` |
| S13 | `DELETE /tasks/{id}` `If-Match:"1"` on a task now at version 2 | `412 precondition_failed` |
| S14 | any request, inspect response headers | `X-Request-Id` present and matches the `request_id` in the server's log line for that request |
| S15 | trigger a handler panic (a test-only route or injected fault) | `500`, generic `internal_error` body, process still alive, next request succeeds |
| S16 | SIGTERM the running server mid-request (integration test with a slow handler) | in-flight request completes, new connections refused, process exits 0 within timeout |

### Case specification — FAILURE

| # | Request | Response |
|---|---|---|
| F1 | `POST /tasks` with `Content-Type: text/plain` | `415 unsupported_media_type` |
| F2 | `POST /tasks` with a 2 MiB JSON body (limit 1 MiB) | `413 payload_too_large` |
| F3 | `POST /tasks {"title":"x","nope":1}` | `400 unknown_field` (strict decoding) |
| F4 | `POST /tasks` with truncated JSON | `400 invalid_json` |
| F5 | `GET /tasks/not-a-real-id` | `404 not_found` |
| F6 | `POST /tasks/{id}` (wrong method for that path — only `GET`/attachment `POST` exist under it, but plain `/tasks/{id}` has no `POST`) | `405`, `Allow: GET, PUT, PATCH, DELETE` |
| F7 | `GET /nope` | `404 not_found` (generic envelope, not the mux's default text) |
| F8 | `GET /tasks?limit=1000` | `400 validation_failed` (`limit` max 100) |
| F9 | `GET /tasks?sort=color` | `400 validation_failed` (`sort` not one of the allowed fields) |
| F10 | `PUT /tasks/{id} {"title":"x"}` `If-Match:"99"` (actual version 1) | `412 precondition_failed` |
| F11 | `POST /tasks/{id}/attachment` with a 10 MiB file (limit 5 MiB) | `413 payload_too_large` |
| F12 | `POST /tasks/{id}/attachment` no `file` field | `400 validation_failed` |

### Error catalogue

| `code` | HTTP | message pattern |
|---|---|---|
| `validation_failed` | 400 | `"validation failed"` + `fields` map |
| `unknown_field` | 400 | `"unknown field %q"` |
| `invalid_json` | 400 | `"invalid JSON: %v"` |
| `unsupported_media_type` | 415 | `"Content-Type must be application/json"` |
| `payload_too_large` | 413 | `"request body exceeds %d bytes"` |
| `not_found` | 404 | `"task %s not found"` / `"not found"` |
| `precondition_failed` | 412 | `"version mismatch: have %d, want %d"` |
| `internal_error` | 500 | `"internal server error"` (generic — details go to the log, never the client) |

## Suggested milestones

1. `internal/task`: `Task`, `TaskStore` interface, an in-memory implementation
   (`RWMutex`-guarded map, `version` bump on mutation).
2. Handlers for the 5 core CRUD routes on a **bare** `ServeMux`, JSON encode/decode,
   validation, the error envelope. Test with `httptest.NewRecorder()`.
3. Middleware: request-ID, `slog` logging, panic recovery — compose them around the mux.
4. `If-Match`/`ETag` optimistic concurrency.
5. List filtering/sorting/pagination.
6. Multipart attachment endpoints (`http.MaxBytesReader`, `r.FormFile`).
7. `/healthz`; wire `main` with `http.Server`, flags, `signal.NotifyContext` + graceful
   shutdown.
8. Integration tests over `httptest.NewServer` (`TestMain` spins it up once per package
   test run, or per test — document your choice).

## Project layout

```
projects/20-rest-api/
  cmd/api/main.go
  internal/task/task.go        // Task, validation
  internal/task/store.go       // TaskStore + in-memory impl
  internal/httpapi/middleware.go
  internal/httpapi/handlers.go
  internal/httpapi/router.go   // builds the ServeMux
  internal/httpapi/envelope.go // error/response encoding
  internal/httpapi/*_test.go
  cmd/api/main_test.go         // TestMain + full-server integration tests
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] Every handler is reachable and testable **without** starting a real listener
      (`httptest.NewRecorder`), plus at least the S-table is also run against a real
      `httptest.NewServer`.
- [ ] The panic-recovery middleware is proven by S15: the process survives, the client
      gets a generic 500, and the panic detail appears **only** in the server log.
- [ ] Every log line is one JSON object with `method, path, status, duration_ms,
      request_id`; `request_id` matches the `X-Request-Id` response header.
- [ ] `Server.Shutdown` is used (not `os.Exit` while requests are in flight); S16 passes.
- [ ] `go test -race ./...` clean (the store is accessed concurrently by table-driven
      parallel handler tests).

## Test requirements

- `TestValidation` — every validation rule, both directions.
- `TestCRUDHandlers` — S1–S6 via `httptest.NewRecorder`.
- `TestListFilterSortPaginate` — S7–S9.
- `TestOptimisticConcurrency` — S13, F10.
- `TestAttachment` — S10, S11, F11, F12.
- `TestMiddlewareOrder` — request ID is set before logging reads it; recovery wraps
  everything (a panicking handler still gets logged with status 500).
- `TestErrorEnvelope` — F1–F5, F7–F9 exact JSON shape.
- `TestMethodNotAllowed` — F6, asserting the `Allow` header.
- `TestGracefulShutdown` — S16.
- `TestConcurrentStoreAccess` — parallel creates/reads/updates, `-race`.
- `ExampleTaskStore`.

## Stretch goals

- Rate limiting per client IP (bridge to Project 27, once it exists — for now, a simple
  token bucket per `RemoteAddr`).
- `Idempotency-Key` support on `POST /tasks`.
- OpenAPI-ish `/schema` endpoint generated from your validation rules.
- Server-Sent Events on `GET /tasks/events` streaming create/update/delete notifications
  (bridge to Project 19's broker).
- Replace the in-memory store with Project 22's storage engine behind the same
  `TaskStore` interface — prove the handlers don't change.

## Self-check questions

1. `http.ServeMux` returns `405` automatically when a path matches a different method's
   registered pattern. How does it know to do that, and what `Allow` header value does it
   compute?
2. Your recovery middleware must run **outermost** (wrap everything else) or a panic in
   the logging middleware itself would still crash the server. Trace the wrapping order
   in your `router.go` and confirm this.
3. `context.WithValue` for the request ID — why a typed, unexported key type instead of a
   `string` key? What collision could a `string` key cause across packages?
4. `http.MaxBytesReader(w, r.Body, limit)` — what does it actually do when the limit is
   exceeded, and how does that differ from just checking `r.ContentLength`?
5. `ETag: "3"` and `If-Match: "3"` — why are these quoted strings in real HTTP, and what
   would a byte-for-byte comparison miss if you forgot to strip the quotes?
6. Two `PUT` requests for the same task both read version `1` concurrently, then both
   write. Walk through how `If-Match` plus your store's locking prevents a lost update —
   where exactly is the compare-and-swap?
7. `Server.Shutdown(ctx)` vs `Server.Close()` — which one lets in-flight handlers finish,
   and what does your `-shutdown-timeout` flag actually bound?
8. Your JSON decoder uses `DisallowUnknownFields`. A client sends `{"title":"x","Title":"y"}`
   (case variant). Does strict mode catch that? Why or why not, and does it matter for your
   validation?
