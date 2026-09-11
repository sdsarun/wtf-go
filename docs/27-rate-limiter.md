# Project 27 — Rate Limiter Package (own module)

| | |
|---|---|
| **Difficulty** | 5 / 5 |
| **Estimated time** | 10–14 hours |
| **Prerequisites** | [11](11-generic-containers.md), [17](17-in-memory-cache.md), [20](20-rest-api.md) |
| **Builds toward** | [21 – Auth & Sessions](21-auth-sessions.md) (stretch), [26 – Reverse Proxy](26-reverse-proxy.md) (stretch) |

## Why this project

Every other project has lived inside the `learning/go` module. This one is a
**publishable package** in its own right — its own `go.mod`, its own semantic version,
its own `go doc` output, `Example` functions that are actually run by `go test`, and a
deliberate `internal/` boundary so consumers only see the API you intend. Algorithmically
it's two rate-limiting strategies (token bucket, sliding-window counter) plus a generic
per-key wrapper — real algorithms, but the *point* of this project is packaging and API
design, graded as seriously as the code.

## Go concepts you MUST use

- [ ] a **separate module**: its own `go.mod` (module path e.g. `ratelimit`), wired into
      this repo via a root **`go.work`** (`go work use . ./projects/27-ratelimiter`) so
      everything still builds/tests together locally, while the module itself is
      independently `go get`-able
- [ ] **semantic versioning** discipline — document in the README what would bump major
      vs minor vs patch for a change to this specific API, and tag `v0.1.0` in your notes
- [ ] generics: `KeyedLimiter[K comparable]`
- [ ] the functional-options pattern (`WithClock`, `WithBurst`, …)
- [ ] **godoc-quality doc comments** on every exported symbol — package doc in `doc.go`,
      each comment starting with the symbol's name, runnable via `go doc ./...`
- [ ] `example_test.go` with `Example` functions that have a `// Output:` comment `go
      test` actually verifies
- [ ] **benchmarks** (`Allow`/`AllowN`/`Wait` under contention) and a **fuzz target**
      (`FuzzAllowN`) that must never panic and never let the token count go negative
- [ ] an `internal/` package holding the actual math, with only clean types exported from
      the package root

## Background

**Token bucket.** A bucket holds up to `burst` tokens, refilling at `rate` tokens/second.
`Allow()` succeeds and consumes one token iff at least one is available *right now*,
computed lazily: `elapsed := now.Sub(lastRefill); tokens = min(burst, tokens +
elapsed.Seconds()*rate); lastRefill = now`.

**Sliding-window counter.** Divide time into fixed windows of length `window`. Track a
count for the **current** window and the **previous** one. At time `t` inside the current
window, the estimated count over the trailing `window` duration is
`prevCount*(1 - elapsedFractionOfCurrentWindow) + currCount`. If that estimate is `<
limit`, the request is allowed and `currCount` increments. This is an *approximation*
(not exact like a sliding log) — O(1) memory, and you must document the approximation
error in the README.

**`Wait`.** Blocks the caller until a token would be available (or `ctx` is done),
without busy-polling: for the token bucket, compute the exact duration until enough
tokens accrue and sleep via a `context`-aware timer; for the sliding window, poll at a
bounded interval (document why an exact wait isn't possible for the counter
approximation).

## Requirements

### Public API (package root, e.g. `ratelimit`)

```go
type Limiter interface {
    Allow() bool
    AllowN(n int) bool
    Wait(ctx context.Context) error
    WaitN(ctx context.Context, n int) error
}

func NewTokenBucket(rate float64, burst int, opts ...Option) *TokenBucket
func NewSlidingWindow(limit int, window time.Duration, opts ...Option) *SlidingWindow

func WithClock(now func() time.Time) Option // for both constructors; default time.Now

type KeyedLimiter[K comparable] struct { /* unexported */ }
func NewKeyed[K comparable](factory func() Limiter, opts ...KeyedOption[K]) *KeyedLimiter[K]
func (kl *KeyedLimiter[K]) Allow(key K) bool
func (kl *KeyedLimiter[K]) Len() int             // number of tracked keys
func WithIdleEviction[K comparable](after time.Duration) KeyedOption[K] // reuses Project 17's sweep idea
```

`NewTokenBucket`/`NewSlidingWindow` both satisfy `Limiter`. `internal/limitmath` holds the
refill/window arithmetic; the exported types are thin wrappers with locking and the
public API.

### Behaviour specification

| Scenario | Expectation |
|---|---|
| `NewTokenBucket(10, 5)`; 5 immediate `Allow()` calls | all `true` (full burst) |
| same, 6th immediate call | `false` |
| same, wait 100ms (1 token at rate 10/s), 1 more `Allow()` | `true` |
| `AllowN(3)` with 5 tokens available | `true`, 2 tokens remain |
| `AllowN(3)` with 2 tokens available | `false`, tokens **unchanged** (all-or-nothing) |
| `AllowN(0)` | `true`, no state change (degenerate but valid) |
| `AllowN(-1)` | **panics** `ratelimit: n must be >= 0` |
| `Wait(ctx)` with an already-cancelled `ctx`, 0 tokens available | returns `ctx.Err()` immediately, no sleep |
| `Wait(ctx)` with tokens available now | returns `nil` immediately |
| `Wait(ctx)` with 0 tokens, ctx has a 2s timeout, 1 token due in 100ms | returns `nil` after ~100ms |
| `NewSlidingWindow(5, time.Second)`; 5 calls within one window | all `true` |
| same, 6th call same window | `false` |
| same, wait until just past the window boundary, 1 more call | `true` (old window's weight has decayed) |
| `KeyedLimiter` with a token-bucket factory; `Allow("a")` and `Allow("b")` | independent buckets — exhausting `"a"`'s doesn't affect `"b"` |
| `KeyedLimiter` with `WithIdleEviction(1s)`; key unused for >1s | evicted; `Len()` drops; a fresh `Allow` for that key starts a new bucket at full burst |
| `NewTokenBucket(0, 5)` (rate 0 — never refills) | 5 initial `Allow()`s succeed, then always `false` |
| concurrent `Allow()` from many goroutines on one limiter | total successes over a time window never exceeds what the math allows (property test, not exact count) |

### Package-level requirements

- `doc.go` with a package comment explaining what the package is, its two strategies, and
  a short usage example.
- Every exported identifier documented; `go vet` and `golint`-style comment-format checks
  pass (start with the name, full sentence).
- `example_test.go`: at least `ExampleNewTokenBucket`, `ExampleKeyedLimiter`, each with a
  `// Output:` block that `go test` verifies (use a fixed `WithClock` so output is
  deterministic).
- `go.mod` with a real module path; a `README.md` stating the current version and what a
  breaking API change would look like (so you've thought about it, even without actually
  publishing).

## Suggested milestones

1. **Module setup first.** Create `projects/27-ratelimiter/go.mod` (its own module), add
   a root `go.work` (`go work init .` then `go work use ./projects/27-ratelimiter`).
   Verify `go build ./...` and `go test ./...` both work from the repo root *and* from
   inside the submodule directory alone.
2. `internal/limitmath`: pure refill/window math, no locking, no clock injection games —
   just functions of `(state, elapsed) -> newState`. Exhaustively unit tested.
3. `TokenBucket` wrapping it with a `sync.Mutex`, `WithClock`.
4. `SlidingWindow` likewise.
5. `Wait`/`WaitN` for both.
6. `KeyedLimiter[K]` + idle eviction (a small sweeper, same shape as Project 17's).
7. Doc comments, `doc.go`, `example_test.go` with deterministic clocks.
8. Benchmarks + `FuzzAllowN`.

## Project layout

```
projects/27-ratelimiter/
  go.mod                       // its OWN module
  doc.go
  limiter.go                   // Limiter interface, Option, WithClock
  tokenbucket.go
  slidingwindow.go
  keyed.go
  internal/limitmath/refill.go
  internal/limitmath/window.go
  internal/limitmath/*_test.go
  *_test.go
  bench_test.go
  fuzz_test.go
  example_test.go
  README.md
  Makefile
go.work                        // at the repo root, added in this project
go.work.sum
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every behaviour-table row is a test.
- [ ] `go build ./...` and `go test ./...` succeed **both** from the repo root (via
      `go.work`) **and** from `cd projects/27-ratelimiter && go test ./...` standalone
      (proves the module is genuinely independent).
- [ ] `go vet ./...` and `gofmt -l .` clean; `go doc ./projects/27-ratelimiter` (or from
      inside the module, `go doc .`) renders sensible output for every exported symbol —
      paste it into the README.
- [ ] `example_test.go`'s `Example` functions pass under plain `go test` (their
      `// Output:` is checked automatically — this is not optional tooling, it's a test).
- [ ] `FuzzAllowN` runs ≥3 minutes clean; a property assertion inside the fuzz target
      checks tokens/counts never go negative.
- [ ] `go test -race ./...` clean.
- [ ] No exported symbol reaches into `internal/limitmath`'s types directly from outside
      this module (there's no way to — prove you understand why by explaining it in the
      README).

## Test requirements

- `TestLimitMath` (in `internal/limitmath`) — refill arithmetic, window weighting,
  exhaustive small cases.
- `TestTokenBucketBurstAndRefill`, `TestTokenBucketAllowNAllOrNothing`,
  `TestTokenBucketZeroRate`.
- `TestSlidingWindowBoundary` — behaviour right at a window edge.
- `TestWaitRespectsContext`, `TestWaitReturnsWhenTokenAvailable`.
- `TestKeyedIndependence`, `TestKeyedIdleEviction`.
- `TestConcurrentAllowProperty` (`-race`, statistical bound, not an exact count).
- `BenchmarkTokenBucketAllow`, `BenchmarkSlidingWindowAllow`,
  `BenchmarkKeyedAllowManyKeys`.
- `FuzzAllowN`.
- `ExampleNewTokenBucket`, `ExampleKeyedLimiter` (with `// Output:`).

## Stretch goals

- A third strategy: exact sliding-window log (`O(n)` memory, no approximation error) —
  benchmark memory/CPU against the counter approximation and document the trade-off.
- A `Limiter` that wraps another and adds jittered denial (avoid thundering-herd retries).
- An HTTP middleware helper (`func Middleware(lim Limiter) func(http.Handler)
  http.Handler`) — but keep it in a **separate** sub-package so the core module has zero
  `net/http` dependency.
- Publish it for real: push to a public repo, tag `v0.1.0`, `go get` it into a scratch
  project to prove it resolves.
- A distributed variant backed by Project 25's KV store (`INCR` + `EXPIRE` is literally
  a sliding-window-ish limiter — implement it and compare).

## Self-check questions

1. Why does this project need its **own** `go.mod` instead of just being another
   `projects/NN-x/` package in the root module? What would a consumer's `go get` look
   like in each case, and why does that difference matter for a package meant to be
   reused elsewhere?
2. `go.work` lets your repo build both modules together locally. What happens to someone
   who clones just `projects/27-ratelimiter/` on its own (no `go.work`) — does it still
   build? What would break their build if you'd accidentally imported something from the
   root module?
3. `AllowN(3)` with only 2 tokens must change **nothing** (not partially consume). Where
   exactly in your code does the check happen relative to the mutation, and how do you
   guarantee no other goroutine's `Allow` sneaks in between your check and your update?
4. Your sliding-window counter is an *approximation*. Construct a concrete timeline where
   it allows more than `limit` requests in some real trailing-`window`-duration interval,
   and explain why that's an accepted trade-off for O(1) memory.
5. `WithClock` takes a `func() time.Time`. Why does injecting the clock make
   `ExampleNewTokenBucket`'s `// Output:` deterministic, when the real `Allow()` behaviour
   depends on wall-clock time?
6. `KeyedLimiter[K comparable]`'s idle eviction needs to know when a key was last used
   without adding a lock-per-Allow-call bottleneck. What did you choose, and what's its
   worst case if `Allow` is called at extremely high frequency across millions of keys?
7. `internal/limitmath` has no mutex — `TokenBucket` does. Why does splitting "the math"
   from "the concurrency-safe wrapper" make both halves easier to test, specifically for
   the fuzz target?
