# Project 26 — Reverse Proxy + Load Balancer

| | |
|---|---|
| **Difficulty** | 5 / 5 |
| **Estimated time** | 12–16 hours |
| **Prerequisites** | [12](12-json-config-loader.md), [14](14-link-checker.md), [20](20-rest-api.md) |
| **Builds toward** | [27 – Rate Limiter](27-rate-limiter.md), [29 – Task Scheduler](29-task-scheduler.md) |

## Why this project

`net/http/httputil.ReverseProxy` does the hard part (streaming request/response bodies
correctly, hop-by-hop header stripping); your job is everything around it: a pluggable
**balancing strategy**, background **health checks** with hysteresis so a single blip
doesn't flap a backend in and out, `expvar` metrics, and **zero-downtime config reload**
on `SIGHUP` via an atomically-swapped immutable config.

## Go concepts you MUST use

- [ ] `net/http/httputil.ReverseProxy` — the modern `Rewrite func(*ProxyRequest)` API
      (not the deprecated `Director`), `ErrorHandler`, `ModifyResponse`
- [ ] a `Balancer` **interface** with ≥3 implementations (round-robin, random,
      least-connections)
- [ ] background health-check goroutines (one per backend), `context`-cancellable,
      reporting into shared state safely
- [ ] `expvar` — `expvar.Map`, `expvar.Int`, `expvar.Func` for computed values (uptime)
- [ ] **hot config reload**: `SIGHUP` → parse the new config → build new backend/balancer
      state → swap it in via `atomic.Pointer[Config]` — in-flight requests keep using the
      old value, new requests immediately see the new one
- [ ] `net/url` for backend target construction
- [ ] retries with a bounded attempt count across backends on failure

## Background

**Routing.** Requests match the route whose `path_prefix` is the **longest** matching
prefix of the request path. Each route has its own backend pool and balancing strategy.

**Health check hysteresis.** A backend starts **healthy** (optimistic — see self-check 5).
Each check interval, a probe (`GET <health_check.path>` with `health_check.timeout`)
either succeeds or fails. `unhealthy_after` **consecutive** failures marks it Down;
`healthy_after` **consecutive** successes after that marks it back Up. A single flaky
probe does not flip status.

**Reload.** `SIGHUP` re-reads the config file. Build an entirely new immutable
`*Config` (routes, backend pools, fresh health-checker goroutines) and
`atomic.Pointer[Config].Store` it. The old config's health-check goroutines are stopped
via their own `context.CancelFunc`, tracked alongside the old `*Config` so they can be
cancelled *after* the swap (not before — don't leave a gap with no health data).

## Requirements

### Config

```json
{
  "listen": ":8080",
  "routes": [
    {"path_prefix": "/api", "backends": ["http://b1:9001", "http://b2:9002"], "strategy": "round_robin"},
    {"path_prefix": "/",    "backends": ["http://b3:9003"], "strategy": "random"}
  ],
  "health_check": {"path": "/healthz", "interval": "5s", "timeout": "2s", "unhealthy_after": 3, "healthy_after": 2},
  "retries": 2
}
```

`strategy` ∈ `round_robin`, `random`, `least_conn`. Validated with Project 12-style rules
(reuse that pattern; you may vendor a trimmed loader here per the `internal/` rule).

### Proxying behaviour

1. Match the longest `path_prefix`; no match → `404`.
2. Pick a **healthy** backend via the route's `Balancer`. If none healthy →
   `503 Service Unavailable` with `Retry-After: 5`.
3. Proxy the request via `ReverseProxy`, setting `X-Forwarded-For`/`X-Forwarded-Proto`
   (via `ProxyRequest.SetXForwarded()`) and `Via: 1.1 miniproxy`.
4. On a backend connection error (not an HTTP error status — a real dial/timeout
   failure), retry on a **different** healthy backend, up to `retries` times, then
   `502 Bad Gateway`.
5. `least_conn` tracks in-flight request counts per backend (`atomic.Int64`, incremented
   before proxying, decremented via `ModifyResponse`/`ErrorHandler`, whichever fires).

### `/metrics` (expvar, `GET /metrics` served by the proxy itself, not proxied)

JSON (the standard `expvar` handler shape) including at least:
`uptime_seconds` (a `Func`), and per-route, per-backend `requests`, `errors`, `healthy`
(0/1).

### Exit codes

| Code | Meaning |
|---|---|
| `0` | clean shutdown |
| `1` | listen failure; invalid config at startup |
| `2` | bad flags |

### Case specification — SUCCESS (`httptest` backends)

Two backends `B1`, `B2` behind `/api`; one backend `B3` behind `/`.

| # | Scenario | Assertion |
|---|---|---|
| S1 | 4 requests to `/api/x` with `round_robin`, both backends healthy | hits alternate `B1, B2, B1, B2` |
| S2 | `least_conn`: hold one slow request open on `B1`, send 2 more | both new requests go to `B2` (fewer in-flight) |
| S3 | request to `/` | routed to `B3` (longest-prefix beats the `/api` route only for non-`/api` paths) |
| S4 | request to `/apixyz` (not `/api` exactly, but shares the prefix) | matches `/api` route (prefix match, not exact) — document if you want stricter segment matching instead, and be consistent |
| S5 | `B1` starts failing health checks; after `unhealthy_after` consecutive failures | traffic goes only to `B2`; `/metrics` shows `B1 healthy=0` |
| S6 | `B1` recovers; after `healthy_after` consecutive successes | traffic resumes to `B1` |
| S7 | `B1` down, `B2` also goes down | `503` with `Retry-After: 5` |
| S8 | `B1` returns a connection error (test closes the listener mid-request) | proxy retries on `B2`, request still succeeds, response looks identical to the client |
| S9 | `GET /metrics` | valid JSON with the documented shape |
| S10 | edit the config file (change strategy), send `SIGHUP` | subsequent requests use the new strategy; a request that started **before** the signal completes using the old config's backend list |
| S11 | during reload, old health-check goroutines | are cancelled after the swap (no leaked goroutines — test it) |
| S12 | client sends `X-Forwarded-For: 1.2.3.4`; proxy forwards to backend | backend observes `X-Forwarded-For: 1.2.3.4, <proxy's remote addr>` (appended, not replaced) |

### Case specification — FAILURE

| # | Scenario | Response / behaviour |
|---|---|---|
| F1 | `GET /nope` (no matching prefix) | `404` |
| F2 | all backends for a route unhealthy | `503`, `Retry-After: 5` |
| F3 | backend dial fails on every retry attempt | `502 Bad Gateway` |
| F4 | config file has a backend URL that fails `url.Parse` | startup error, exit `1` |
| F5 | config file has an unknown `strategy` | startup error, exit `1` |
| F6 | `SIGHUP` with a config file that now fails to parse | reload is **rejected**, the proxy keeps serving the previous good config, an error is logged |
| F7 | `-config` file missing at startup | exit `1` |

### Error catalogue

Proxy-generated error bodies are plain text: `<status> <reason>\n`. Config errors:
`error: config: %v` to stderr.

## Suggested milestones

1. Config loading (adapt Project 12's approach) + validation; `Route`/`Backend` types.
2. `Balancer` interface + round-robin (simplest, gets routing working end to end first).
3. Wire `ReverseProxy` with `Rewrite`, `ErrorHandler` (→ 502), longest-prefix matching.
4. Random and least-conn balancers; per-backend in-flight counters.
5. Health checker: one goroutine per backend, hysteresis, shared status via an
   `atomic.Bool` or small mutex-guarded struct per backend.
6. Retries on connection failure.
7. `expvar` metrics.
8. `SIGHUP` reload: build-new-then-swap via `atomic.Pointer[Config]`; cancel old
   health-checkers post-swap.
9. Full test suite with `httptest` backends and a fake "flaky" backend for health-check
   tests.

## Project layout

```
projects/26-proxy/
  cmd/proxy/main.go
  internal/proxy/config.go       // adapted Project 12-style loader
  internal/proxy/balancer.go     // interface + round_robin/random/least_conn
  internal/proxy/backend.go      // Backend, health status
  internal/proxy/health.go       // checker goroutines, hysteresis
  internal/proxy/router.go       // longest-prefix match
  internal/proxy/handler.go      // ReverseProxy wiring, retries, metrics counters
  internal/proxy/reload.go       // SIGHUP, atomic.Pointer swap
  internal/proxy/*_test.go
  cmd/proxy/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] `go test -race ./...` clean (health checkers, request counters, and reload all
      touch shared state concurrently).
- [ ] `TestReloadNoDroppedRequests` — start a slow request, `SIGHUP` mid-flight, assert
      it completes successfully using the *old* config's backend.
- [ ] `TestNoGoroutineLeakAcrossReload` — reload 10 times, goroutine count stays flat.
- [ ] `least_conn`'s counters never go negative and settle to `0` at quiescence.
- [ ] `/metrics` numbers are internally consistent (sum of per-backend `requests` across
      a route equals the number of requests you sent it in a test).

## Test requirements

- `TestLongestPrefixMatch`.
- `TestBalancers` — round_robin sequence, random distribution (statistical, generous
  tolerance), least_conn under concurrent in-flight requests.
- `TestHealthCheckHysteresis` — S5, S6, and a "flaps once, doesn't flip" case.
- `TestAllBackendsDown` — F2.
- `TestRetryOnConnectionError` — S8, F3 (retries exhausted).
- `TestForwardedHeaders` — S12.
- `TestMetricsShape`, `TestMetricsConsistency`.
- `TestReloadSwapsAtomically`, `TestReloadNoDroppedRequests`,
  `TestReloadRejectsBadConfig` (F6), `TestNoGoroutineLeakAcrossReload`.
- `ExampleBalancer`.

## Stretch goals

- Sticky sessions (a cookie pinning a client to one backend).
- Circuit breaker per backend (open after N consecutive errors, half-open probe).
- TLS termination at the proxy (`crypto/tls`).
- Per-route rate limiting (bridge to Project 27, once built).
- A weighted-round-robin balancer using the generic containers from Project 11.

## Self-check questions

1. `httputil.ReverseProxy`'s `Rewrite` function receives a `*ProxyRequest` with both
   `In` (original) and `Out` (to be sent) requests. What do you copy from `In` to `Out`
   yourself vs what does `ReverseProxy` already handle before calling `Rewrite`?
2. Why build an entirely **new** `*Config` on reload rather than mutating the existing
   one in place? Walk through what a concurrent reader could observe if you mutated a
   `[]Backend` slice that a `Balancer` was iterating.
3. `atomic.Pointer[Config].Load()` — every request calls this once at the top of the
   handler and uses that *one* snapshot for the whole request. Why is calling `Load()`
   again partway through the same request a bug (even though each individual `Load()` is
   safe)?
4. Health checks default new backends to "healthy" rather than "unknown/down until
   proven." What's the argument for optimistic-healthy, and what's the risk? When would
   you flip the default?
5. `least_conn` increments a counter before proxying and decrements it in
   `ModifyResponse` **or** `ErrorHandler`. Why do you need both hooks — what request
   outcome would leak a permanently-incremented counter if you only implemented one?
6. Your retry logic must not retry a request that already reached the backend and might
   have had side effects (e.g. a `POST` that partially succeeded). What do you actually
   retry on — only dial/connect failures, or any error? Defend your choice.
7. Two goroutines: the reload handler cancelling old health-checkers, and a health-checker
   goroutine mid-probe using a `context` that's about to be cancelled. What happens to
   that in-flight probe's HTTP request, and does your code wait for it before moving on?
