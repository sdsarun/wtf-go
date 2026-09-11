# Project 17 — In-Memory Cache: TTL + Single-Flight

| | |
|---|---|
| **Difficulty** | 4 / 5 |
| **Estimated time** | 8–11 hours |
| **Prerequisites** | [11](11-generic-containers.md), [16](16-bounded-blocking-queue.md) |
| **Builds toward** | [25 – KV Store](25-kv-store.md), [26 – Reverse Proxy](26-reverse-proxy.md) |

## Why this project

The cache every service ends up writing: generic, TTL-expiring, capacity-bounded with LRU
eviction, a background sweeper, and — the interesting part — **single-flight**: when 50
goroutines ask for the same missing key at once, exactly one of them should do the
expensive load and the other 49 should wait for its result. You'll use `RWMutex` where
reads genuinely outnumber writes, `Once` where "exactly once" is the actual requirement
(not everywhere, as a habit), and benchmark your mutex+map design against `sync.Map` to
find out — empirically — when each wins.

## Go concepts you MUST use

- [ ] `sync.RWMutex` — `RLock` for a **non-mutating** peek path, full `Lock` for anything
      that touches LRU order or the map
- [ ] `sync.Once` for **exactly one** thing that must happen exactly once (`Close`
      stopping the background goroutine) — not sprinkled in as a habit
- [ ] `sync.Map` — a second, simpler cache variant, benchmarked against the mutex+map one
- [ ] `time.Timer` / `time.Ticker` / `time.AfterFunc` — the background eviction sweep
- [ ] generics (`Cache[K comparable, V any]`)
- [ ] the **single-flight** pattern (hand-rolled, coordinating waiters with a
      `sync.WaitGroup` per in-flight key)
- [ ] `t.Parallel()` in your test suite
- [ ] `-race`-clean concurrent tests; benchmarks with `b.ReportAllocs()`

## Background

**Why not import Project 11's `List`?** It lives under
`projects/11-containers/internal/containers`, and Go's `internal/` rule only allows
imports from within the tree rooted at `internal`'s parent — i.e. only code under
`projects/11-containers/` can import it. This project **cannot** import it. Write your own
small intrusive doubly-linked list here (a dozen lines: it only needs
`moveToFront`/`removeTail`) — see self-check 7 for why this restriction exists and is
useful.

**TTL semantics.** `ttl <= 0` means "never expires". An expired entry is invisible to
`Get`/`Peek` even before the sweeper removes it (**lazy** expiry on read) — the background
**sweep** is only there to reclaim memory for keys nobody reads again.

**Capacity & LRU.** With `WithCapacity(n)`, once the cache holds `n` live (non-expired)
entries, a `Set` for a new key evicts the least-recently-**used** entry first. `Get`
(but not `Peek`) counts as a use and moves the entry to the front.

**Single-flight.** `GetOrLoad(ctx, key, ttl, load)`: if a live value exists, return it. If
not, and no load for that key is already in flight, start one (calling `load` exactly
once) and register it as in-flight; any other caller for the same key **while it's in
flight** waits for that call's result instead of starting its own. On success, the result
is cached with `ttl` and returned to every waiter. On error, it is returned to every
waiter and **nothing is cached**.

## Requirements

### API

```go
type Cache[K comparable, V any] struct { /* unexported */ }

func New[K comparable, V any](opts ...Option[K, V]) *Cache[K, V]
func WithCapacity[K comparable, V any](n int) Option[K, V]      // 0 = unbounded
func WithSweepInterval[K comparable, V any](d time.Duration) Option[K, V] // default 1s

func (c *Cache[K, V]) Set(key K, val V, ttl time.Duration)
func (c *Cache[K, V]) Get(key K) (V, bool)     // live value; touches LRU; RLock-ineligible
func (c *Cache[K, V]) Peek(key K) (V, bool)    // like Get but never touches LRU order; RLock
func (c *Cache[K, V]) Delete(key K)
func (c *Cache[K, V]) Len() int                // live entries only
func (c *Cache[K, V]) Close()                  // idempotent; stops the sweeper

type Loader[K comparable, V any] func(ctx context.Context, key K) (V, error)
func (c *Cache[K, V]) GetOrLoad(ctx context.Context, key K, ttl time.Duration, load Loader[K, V]) (V, error)

// A second, simpler implementation for comparison — same read/write API minus LRU/capacity.
type MapCache[K comparable, V any] struct { /* backed by sync.Map */ }
func NewMapCache[K comparable, V any]() *MapCache[K, V]
```

### Behaviour specification

| Scenario | Expectation |
|---|---|
| `Set("a", 1, time.Hour)`; `Get("a")` | `1, true` |
| `Get("missing")` | zero value, `false` |
| `Set("a", 1, 10*time.Millisecond)`; sleep 20ms; `Get("a")` | zero value, `false` (lazily expired) |
| same, but check `Len()` **before** the sweep has run | still counts as 0 live (Len excludes expired even pre-sweep) |
| `Set("a", 1, 10ms)`; sleep past sweep interval; inspect internal map size (test-only hook) | entry physically removed by the sweeper |
| capacity 2; `Set(a)`, `Set(b)`, `Get(a)`, `Set(c)` | `b` is evicted (a was touched, so b is now LRU) |
| capacity 2; `Set(a)`, `Set(b)`, `Peek(a)`, `Set(c)` | `a` is evicted (`Peek` did **not** count as use) |
| `Delete` a missing key | no-op, no panic |
| `Close()` twice | second call is a no-op |
| `Close()` then `Set`/`Get` | still work as a plain cache; only the sweeper is stopped |
| `GetOrLoad` with an existing live value | `load` is **not** called |
| `GetOrLoad` on a missing key, 20 goroutines simultaneously | `load` is called **exactly once**; all 20 get its result |
| `GetOrLoad` where `load` returns an error | all waiters get that error; the key is **not** cached; a subsequent `GetOrLoad` calls `load` again |
| `GetOrLoad` where `load` respects `ctx` and the caller's `ctx` is cancelled | that caller returns `ctx.Err()` promptly **without** cancelling the in-flight load for other waiters (only cancel the load if **every** waiter's context is done — document your choice if you simplify this) |

### Concurrency requirements (graded)

- [ ] `go test -race ./...` clean.
- [ ] `TestSingleFlightExactlyOnce` — 100 goroutines, `GetOrLoad` same key, an
      `atomic.Int64` counter inside `load` incremented — asserts it equals `1`.
- [ ] `TestNoGoroutineLeak` — `Close()` stops the sweeper; goroutine count returns to
      baseline.
- [ ] `TestConcurrentSetGetDelete` under `-race` with `t.Parallel()` subtests, 8+ goroutines
      hammering a shared cache with random keys.
- [ ] `Peek` is proven to use `RLock` (not `Lock`) via a benchmark showing concurrent
      `Peek`s scale better than concurrent `Get`s under high read concurrency (report the
      numbers in the README, don't just assert).

### Exit criteria for `MapCache` vs `Cache` benchmark write-up

Run `BenchmarkCache` and `BenchmarkMapCache` for: read-heavy (95% Get / 5% Set), and
write-heavy (50/50), at 1/8/64 concurrent goroutines. Put the resulting table + one
paragraph of interpretation in the README (this is a required deliverable, not a stretch
goal).

## Suggested milestones

1. Plain `Cache` (no capacity, no eviction): `Set`/`Get`/`Delete`/`Len`, guarded by
   `RWMutex`, TTL lazy-expiry on read.
2. The intrusive LRU list + `WithCapacity`; `Peek` vs `Get` and the RLock/Lock split.
3. The background sweeper (`time.Ticker`) + `Close` guarded by `sync.Once`.
4. Single-flight: an in-flight registry `map[K]*call` (own small mutex or reuse the main
   one), `call{wg sync.WaitGroup; val V; err error}`.
5. `MapCache` on `sync.Map`.
6. Concurrency tests, then benchmarks, then the README write-up.

## Project layout

```
projects/17-cache/
  internal/cache/cache.go       // Cache, options, Set/Get/Peek/Delete/Len
  internal/cache/lru.go         // tiny intrusive doubly-linked list
  internal/cache/sweep.go       // background eviction
  internal/cache/singleflight.go
  internal/cache/mapcache.go    // sync.Map variant
  internal/cache/*_test.go
  internal/cache/bench_test.go
  README.md                     // must include the benchmark table + interpretation
  Makefile
```

No `cmd/` needed — pure library. (A tiny `cmd/cachedemo` is fine if you want something to
run by hand.)

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every behaviour-table row is a test.
- [ ] Every concurrency-requirements checkbox has a passing test.
- [ ] The README's benchmark table and write-up are present and reflect your actual
      `go test -bench` output (paste real numbers, not estimates).
- [ ] `Peek` never calls anything that requires the write lock.
- [ ] `Close` is safe to call zero, one, or many times, from any goroutine.
- [ ] `GetOrLoad`'s error path never caches a value.

## Test requirements

- `TestTTLExpiry`, `TestLazyExpiryBeforeSweep`, `TestSweepReclaimsMemory` (a test hook to
  peek the raw map size).
- `TestLRUEviction`, `TestPeekDoesNotTouchLRU`.
- `TestCloseIdempotent`, `TestUsableAfterClose`.
- `TestSingleFlightExactlyOnce`, `TestSingleFlightErrorNotCached`,
  `TestSingleFlightContextCancel`.
- `TestConcurrentSetGetDelete` (`-race`, `t.Parallel`).
- `TestNoGoroutineLeak`.
- `BenchmarkCache`/`BenchmarkMapCache` — `b.Run` matrix (workload × concurrency),
  `b.ReportAllocs()`.
- `ExampleCache_GetOrLoad`.

## Stretch goals

- A second eviction policy (LFU) selectable via an `Option`.
- `GetOrLoadMulti(ctx, keys, ttl, batchLoad)` — single-flight a batch loader too.
- Metrics: hit/miss/eviction counters exposed via a `Stats()` method (`atomic.Int64`s).
- A `WithOnEvict(func(K, V))` callback.
- Persist-to-disk snapshot/restore (small preview of Project 22).

## Self-check questions

1. `Get` moves the entry to the front of the LRU list — a mutation. Why can't `Get` use
   `RLock`, and what would go wrong (concretely, with two goroutines) if it did?
2. Walk through `GetOrLoad` for key `"x"` with 3 concurrent callers arriving within
   microseconds of each other, one of whom is the "first". What data structure detects
   "someone's already loading this key", and what do callers 2 and 3 actually block on?
3. Why does a failed `load` **not** get cached? What would break if you cached the error
   as a value with a short TTL instead of just not caching?
4. `sync.Once` guards `Close`. What actually breaks if two goroutines call `Close`
   concurrently **without** the `Once` — walk through the race on the ticker's stop
   channel.
5. Your benchmark shows `sync.Map` winning read-heavy/high-concurrency and losing
   write-heavy/low-concurrency. Explain *why*, in terms of what `sync.Map` actually does
   internally (read-mostly map + amortized promotion), not just "the numbers say so".
6. `WithCapacity(2)`: `Set(a)`, `Set(b)`, expire `a` (TTL), `Set(c)`. Is anything evicted?
   Why or why not — what counts toward "capacity", live entries or total map entries?
7. Why couldn't you `import "learning/go/projects/11-containers/internal/containers"`?
   What is the *exact* rule (which directory levels), and what would you have to do to the
   repo layout to make that import legal?
