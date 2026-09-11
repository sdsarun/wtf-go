# Project 16 — Bounded Blocking Queue (two ways)

| | |
|---|---|
| **Difficulty** | 4 / 5 |
| **Estimated time** | 6–9 hours |
| **Prerequisites** | [11](11-generic-containers.md), [14](14-link-checker.md), [15](15-log-pipeline.md) |
| **Builds toward** | [17 – Cache](17-in-memory-cache.md), [19 – Pub/Sub Broker](19-pubsub-broker.md) |

## Why this project

Channels are Go's usual answer to "bounded queue", but `sync.Cond` is what channels are
*built from*, and understanding it is what lets you build things channels can't (e.g. a
priority-aware wait, or `Broadcast`-to-all-waiters). You will implement the **same
interface** twice — once on `sync.Cond`, once on a buffered channel — and confront the one
genuinely awkward thing about `Cond`: it has no built-in way to stop waiting on
`context` cancellation, so you build the pattern that fixes that.

## Go concepts you MUST use

- [ ] `sync.Mutex` + `sync.Cond` (`NewCond`, `L.Lock`/`Unlock`, `Wait`, `Signal`,
      `Broadcast`)
- [ ] the same behaviour on a **buffered channel** instead, for comparison
- [ ] `context` cancellation of a blocked `Cond.Wait` via a watcher goroutine that calls
      `Broadcast` on `ctx.Done()`
- [ ] `sync.WaitGroup` for the stress tests (N producers, M consumers)
- [ ] generics (`Queue[T any]`)
- [ ] a benchmark comparing both implementations at several sizes/concurrency levels
- [ ] deadlock and starvation reasoning, written down in the README

## Background

**Why `Cond.Wait` and `context` don't mix natively.** `Wait` atomically unlocks the mutex
and blocks until `Signal`/`Broadcast`, then re-locks. There is no `WaitContext`. The fix:
run one extra goroutine per blocked call that does
`select { case <-ctx.Done(): cond.Broadcast() }`; every waiter, once woken (by the real
event *or* by this), re-checks its condition **and** `ctx.Err()` before deciding to
actually proceed or return the cancellation. This wastes a goroutine per pending wait —
say so in the README as the cost of this approach.

**Spurious wakeups.** Always call `Wait` inside a `for !condition { cond.Wait() }` loop,
never `if`.

**Fairness.** Neither implementation here guarantees FIFO wake order among waiters
(`Broadcast` wakes everyone, who then race for the lock; `Signal` picks one but Go doesn't
document which). Document this rather than trying to fix it.

## Requirements

### Shared interface

```go
type Queue[T any] interface {
    Push(ctx context.Context, v T) error  // blocks while full; ctx cancel -> ctx.Err()
    Pop(ctx context.Context) (T, error)    // blocks while empty; ctx cancel -> ctx.Err()
    Close()                                 // idempotent; wakes every blocked caller
    Len() int
    Cap() int
}
var ErrClosed = errors.New("queue: closed")
```

`NewCondQueue[T](capacity int) Queue[T]` and `NewChanQueue[T](capacity int) Queue[T]`.
`capacity <= 0` → panic `queue: capacity must be >= 1`.

### Behaviour specification

| Scenario | Expectation (both implementations) |
|---|---|
| capacity 2; `Push(a)`, `Push(b)` | both return `nil` immediately, `Len()==2` |
| capacity 2, full; `Push(c)` with a background `Pop()` arriving 10ms later | `Push(c)` blocks ~10ms then returns `nil` |
| capacity 2, full; `Push(c, ctx)` with `ctx` timeout 5ms and no consumer | returns `context.DeadlineExceeded` within ~5ms (not the full test timeout) |
| empty; `Pop(ctx)` with `ctx` already cancelled | returns `context.Canceled` immediately |
| `Close()` then `Pop()` on a queue with 2 items left | returns both items normally, `nil` error, **then** the next `Pop` returns `ErrClosed` |
| `Close()` then `Push()` | returns `ErrClosed` immediately, item not enqueued |
| `Close()` called twice | second call is a no-op, no panic |
| `Close()` while 5 goroutines are blocked in `Push` (queue full, no consumer) | all 5 wake and return `ErrClosed` within a bounded time (no goroutine left blocked) |
| `Close()` while goroutines are blocked in `Pop` (queue empty) | all wake and return `ErrClosed` |
| `Len()` / `Cap()` under concurrent Push/Pop | never negative, never exceeds `Cap()`, matches item count at quiescence |

### Stress / correctness requirements (graded)

- [ ] `go test -race ./...` clean.
- [ ] **No lost updates:** N producers each push `M` unique tokens; total popped by all
      consumers equals `N*M`, with no duplicates and no missing token (`TestNoLostItems`,
      both implementations, e.g. 8 producers × 8 consumers × 10k items, capacity 16).
- [ ] **No deadlock:** the stress test above completes within a generous timeout under
      `-race` (use `t.Deadline()`-aware timeout, or a hard 30s watchdog that fails the
      test rather than hanging CI forever).
- [ ] **No goroutine leak:** after `Close()` and draining, `runtime.NumGoroutine()`
      returns to baseline (this is where the "watcher goroutine per blocked call" design
      must clean itself up correctly).
- [ ] **Cancellation is prompt:** a `Push`/`Pop` blocked forever (no counterpart, no
      Close) with a 20ms context deadline must return within ~25ms even under heavy
      unrelated load from the stress test.

### Demo binary (optional but required for the milestones below)

`pcdemo -impl cond|chan -producers N -consumers M -capacity C -items K` runs the stress
scenario and prints throughput (`items/sec`) and p50/p99 latency of `Push`. Used for the
benchmark write-up.

### Exit codes (demo binary only)

| Code | Meaning |
|---|---|
| `0` | completed |
| `2` | bad flags |

## Suggested milestones

1. `ChanQueue` first — it's the easy one and pins down the semantics (closed behaviour,
   error values) you'll replicate.
2. `CondQueue` without cancellation: `Push`/`Pop`/`Close`/`Len` using one `Mutex` + one
   `Cond` (both "not full" and "not empty" waits on the same cond, woken by
   `Broadcast` — simpler and correct, if slightly less efficient than two Conds; note the
   trade-off).
3. Add the `ctx` watcher-goroutine pattern to `CondQueue`; make sure it's cleaned up
   (use a `done chan struct{}` + a second `select` to stop the watcher once the real wait
   resolves — don't leak it).
4. `Close()` semantics on both: drain-then-`ErrClosed`, idempotent, wakes all blocked.
5. Stress tests: no-lost-items, no-deadlock, no-leak, prompt-cancellation.
6. Benchmarks: `BenchmarkPush`/`BenchmarkPop` per impl at capacities 1/16/1024 and
   concurrency 1/8/64.
7. `pcdemo`; write the README comparison (throughput table + the fairness/goroutine-count
   trade-off in prose).

## Project layout

```
projects/16-blocking-queue/
  cmd/pcdemo/main.go
  internal/bq/queue.go       // interface, ErrClosed
  internal/bq/cond_queue.go
  internal/bq/chan_queue.go
  internal/bq/*_test.go
  internal/bq/bench_test.go
  cmd/pcdemo/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every behaviour-table row is a test, run against **both** implementations via a
      table of constructors (`for _, ctor := range []func(int) Queue[int]{...}`).
- [ ] Every stress/correctness checkbox passes for both implementations.
- [ ] The README states, in your own words: (a) why `Cond` needs a watcher goroutine for
      cancellation and what it costs, (b) a throughput comparison table from your
      benchmarks, (c) which implementation you'd actually ship and why.
- [ ] `CondQueue`'s watcher goroutines are provably cleaned up (leak test).

## Test requirements

- `TestBehaviour` — the full behaviour table, parameterized over both implementations.
- `TestCancellationPromptness` — asserts wall-clock bound, not just "eventually returns".
- `TestCloseWakesAllBlocked`.
- `TestNoLostItems` (stress).
- `TestNoGoroutineLeak`.
- `BenchmarkPush` / `BenchmarkPop` — `b.Run` matrix of capacity × concurrency,
  `b.ReportAllocs()`.
- `ExampleQueue` (using whichever impl; interface-typed).

## Stretch goals

- A priority-aware `CondQueue` (`Cond` shines here — a channel can't easily do "wake the
  waiter for the highest-priority slot").
- Two separate `Cond`s (`notFull`, `notEmpty`) instead of one `Broadcast`-everyone cond;
  benchmark the difference under high contention.
- A `TryPush`/`TryPop` non-blocking variant.
- Batch operations: `PushN`/`PopN`.
- A lock-free SPSC ring buffer variant (single producer, single consumer) using
  `sync/atomic` only — benchmark against both.

## Self-check questions

1. Why must `Wait` be called inside `for !cond { Wait() }` and never
   `if !cond { Wait() }`? Construct a concrete interleaving where the `if` version reads
   stale state.
2. Your `CondQueue.Push` spawns a watcher goroutine per call when a `ctx` is provided.
   Trace exactly how it exits in the "won the race, item enqueued normally" path — what
   stops it from leaking?
3. `Broadcast` vs `Signal` — your single-cond design must use `Broadcast`. Why would
   `Signal` be wrong here (hint: a waiter woken by `Signal` might be waiting for the
   *opposite* condition)?
4. In `ChanQueue`, how does `Close()` make blocked `Pop`s return `ErrClosed` only *after*
   draining, while blocked `Push`es return `ErrClosed` immediately? (Hint: closing a
   channel doesn't discard what's already buffered.)
5. You measure `CondQueue` slower than `ChanQueue` at high contention but faster at low
   contention with large payloads. Sketch why (think: what a channel send actually does
   internally vs a mutex + cond).
6. `runtime.NumGoroutine()` is flaky as a leak signal (GC workers, etc.). What did you do
   to make `TestNoGoroutineLeak` reliable rather than occasionally flaky?
7. Is your bounded queue **fair** (FIFO among waiters)? Design an experiment that would
   catch unfairness if it existed, and report what you found.
