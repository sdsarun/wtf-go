# Project 19 — In-Process Pub/Sub Broker

| | |
|---|---|
| **Difficulty** | 4 / 5 |
| **Estimated time** | 9–12 hours |
| **Prerequisites** | [16](16-bounded-blocking-queue.md), [17](17-in-memory-cache.md) |
| **Builds toward** | [25 – KV Store](25-kv-store.md) `SUBSCRIBE`, [29 – Task Scheduler](29-task-scheduler.md) |

## Why this project

An in-memory publish/subscribe broker is where channel design decisions actually matter:
what happens when a subscriber can't keep up? You'll implement four different backpressure
policies, use a **nil channel to disable a `select` case** (making one function handle
"block forever" and "block with a deadline" through the same `select`), and build a
`Close` that drains cleanly under normal load but has a **bounded escape hatch** when a
misbehaving subscriber would otherwise block it forever.

## Go concepts you MUST use

- [ ] channel-heavy design: one buffered channel per subscriber, sized per-subscription
- [ ] a **nil channel disabling a `select` case** — the `trySend` helper below must use
      this, not an `if timeout > 0 { … } else { … }` branch
- [ ] `select` with `default` (the `DropNewest` policy's non-blocking attempt)
- [ ] `select` + `time.After` (the `Block`-with-deadline and `Disconnect` grace period)
- [ ] `sync.RWMutex` used **deliberately** so that `Close`'s exclusive `Lock()` cannot
      proceed while any `Publish` is mid-fan-out (`RLock`) — and a bounded escape hatch
      via `context` when that would block too long
- [ ] `sync.Map` is **not** the right tool here — say why in the README (mutation of a
      per-topic slice on every subscribe/unsubscribe) and use `RWMutex` + plain map
- [ ] closing subscriber channels safely (never send-after-close; never double-close)

## Background

**Topics** are dot-separated (`orders.eu.created`). A subscription pattern may use `*` to
match exactly one segment and a trailing `>` to match one-or-more remaining segments
(`orders.*` matches `orders.created` but not `orders.eu.created`; `orders.>` matches both).
Write your own matcher — split on `.`, compare segment by segment.

**The `trySend` helper — the required nil-channel pattern:**

```go
func trySend(ch chan<- Message, msg Message, timeout time.Duration) bool {
    var timeoutC <-chan time.Time
    if timeout > 0 {
        timeoutC = time.After(timeout)
    }
    select {
    case ch <- msg:
        return true
    case <-timeoutC: // nil when timeout <= 0: this case can never fire, i.e. "block forever"
        return false
    }
}
```

**Overflow policies**, applied when a subscriber's buffered channel is full at publish
time:

| Policy | Behaviour |
|---|---|
| `Block` | `trySend` with the subscriber's `-block-timeout` (`0` = block forever) |
| `DropNewest` | non-blocking send (`select … default:`); if full, the **new** message is dropped |
| `DropOldest` | non-blocking receive-then-send: pop one buffered message to make room, then send the new one (never blocks) |
| `Disconnect` | `trySend` with a short grace period; on timeout, the subscriber is unsubscribed and its channel closed |

## Requirements

### API

```go
type Message struct { Topic string; Data any; Seq uint64; Time time.Time }

type Broker struct { /* unexported: mu sync.RWMutex, subs map[string][]*subscriber, seq atomic.Uint64, closed bool */ }
func New() *Broker

type SubOption func(*subscriber)
func WithBuffer(n int) SubOption           // default 16
func WithPolicy(p Policy) SubOption        // default DropNewest
func WithTimeout(d time.Duration) SubOption // for Block / Disconnect grace

func (b *Broker) Subscribe(pattern string, opts ...SubOption) (*Subscription, error)
type Subscription struct{ /* ... */ }
func (s *Subscription) C() <-chan Message
func (s *Subscription) Pattern() string
func (s *Subscription) Unsubscribe()

func (b *Broker) Publish(topic string, data any) PublishResult
type PublishResult struct{ Matched, Delivered, Dropped, Disconnected int }

func (b *Broker) Close(ctx context.Context) error   // see Close semantics below
func (b *Broker) NumSubscribers(pattern string) int // exact-pattern count, for tests/metrics
```

### Behaviour specification

| Scenario | Expectation |
|---|---|
| subscribe `orders.*`, publish `orders.created` | delivered |
| subscribe `orders.*`, publish `orders.eu.created` | **not** delivered (one segment only) |
| subscribe `orders.>`, publish `orders.eu.created` | delivered |
| subscribe `orders.>`, publish `orders` | **not** delivered (`>` needs ≥1 segment) |
| two subscribers to `orders.created`, one to `orders.*` | publish `orders.created` → `Matched=3, Delivered=3` |
| `DropNewest`, buffer 1, subscriber not reading; publish twice | `Delivered=1` on first, `Dropped=1` on second; subscriber's channel holds the **first** message |
| `DropOldest`, buffer 1, same setup | after both publishes, channel holds the **second** message |
| `Block` with `WithTimeout(20ms)`, subscriber not reading; publish once | `Publish` returns after ~20ms with `Dropped=1` (not delivered) |
| `Block` with `WithTimeout(0)` (block forever), subscriber reads after 20ms | `Publish` returns only after the read, `Delivered=1`; **do not** test with an unbounded timeout and no reader — that's a hang, not a test |
| `Disconnect`, `WithTimeout(20ms)`, subscriber never reads; publish once | after ~20ms, `Disconnected=1`; `NumSubscribers` for that pattern drops by one; the subscriber's channel is **closed** (a `range` over it ends) |
| `Unsubscribe` called twice | second call is a no-op, no panic |
| publish with zero matching subscribers | `PublishResult{}` all zero, no error |
| `Publish` after `Close` | `PublishResult{}` all zero (or a documented sentinel — pick one, test it) |
| `Subscribe` after `Close` | returns `ErrClosed` |

### `Close` semantics

```go
func (b *Broker) Close(ctx context.Context) error
```

1. Mark the broker closed (further `Subscribe` → `ErrClosed`; further `Publish` → no-op).
2. Acquire the exclusive `Lock()` — this **blocks until every in-flight `Publish` call
   completes its fan-out**, which is exactly why `Publish` holds `RLock` for its whole
   duration.
3. Once acquired: `close()` every subscriber's channel (safe — no `Publish` is running).
4. If step 2 doesn't complete before `ctx` is done (e.g. some subscriber's `Block`-forever
   policy is wedging a `Publish` call): give up waiting, return `ctx.Err()`, and leave the
   broker's internal state such that channels are closed on a **best-effort** basis in a
   separate path (document exactly what you do — e.g. a `forceClose` that closes what it
   can without a lock, accepting the small risk, is acceptable *if you explain the
   trade-off in the README*; do not silently leak).

### Exit / error catalogue

| Trigger | Value |
|---|---|
| subscribe after close | `ErrClosed = errors.New("broker: closed")` |
| bad pattern | `ErrInvalidPattern = errors.New("broker: invalid pattern")` (e.g. empty segment `orders..created`, `>` not last) |

### Demo binary (optional runnable artifact)

`pubsubdemo` reads `topic: message` lines from stdin, publishes each, and prints delivery
counts; `-sub PATTERN` (repeatable) pre-registers a subscriber that prints what it
receives. Not the graded surface — the package is.

## Suggested milestones

1. Pattern matcher (`match(pattern, topic string) bool`) — table-tested exhaustively.
2. `Broker`/`subscriber` types, `Subscribe`/`Unsubscribe`, `RWMutex`-guarded registry.
3. `trySend` (the nil-channel version — write it exactly as specified, don't take a
   shortcut) and `Publish` for `Block` and `DropNewest`.
4. `DropOldest`, `Disconnect`.
5. `Close(ctx)` with the RLock/Lock coordination; the bounded escape hatch.
6. Concurrency tests; the demo binary.

## Project layout

```
projects/19-pubsub/
  cmd/pubsubdemo/main.go
  internal/broker/pattern.go
  internal/broker/broker.go     // Subscribe/Unsubscribe/Publish/Close
  internal/broker/trysend.go
  internal/broker/*_test.go
  cmd/pubsubdemo/main_test.go
  README.md   // must explain the Close escape-hatch trade-off
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every behaviour-table row is a test.
- [ ] `go test -race ./...` clean.
- [ ] `trySend` is implemented with a nil channel, not an `if`/`else` branching to two
      different `select` statements (review your own diff for this).
- [ ] No `Publish` call can ever send on a channel that `Close` has already closed
      (property test: hammer `Publish` and `Close` concurrently under `-race` — should
      never panic with "send on closed channel").
- [ ] `TestNoGoroutineLeak` — no goroutine remains blocked after `Close` returns `nil`
      (the bounded-timeout case is allowed to leave a documented, explained situation —
      state it, don't hide it).
- [ ] The README's `Close` trade-off paragraph exists and is accurate to what the code
      does.

## Test requirements

- `TestPatternMatch` — exact, `*`, `>`, invalid patterns, exhaustive table.
- `TestFanOutToMultipleSubscribers`.
- `TestDropNewest`, `TestDropOldest`, `TestBlockWithTimeout`, `TestBlockForever`,
  `TestDisconnectOnSlowSubscriber`.
- `TestUnsubscribeIdempotent`.
- `TestPublishAfterClose`, `TestSubscribeAfterClose`.
- `TestCloseWaitsForInFlightPublish` — start a slow `Publish` (`Block`, generous timeout,
  reader arrives late), call `Close` concurrently, assert it waits and then succeeds.
- `TestCloseTimeoutEscapeHatch` — a wedged `Block`-forever `Publish` with no reader;
  `Close(ctx)` with a short deadline returns `ctx.Err()` promptly (bounded wall-clock
  assertion, not "eventually").
- `TestConcurrentPublishSubscribeUnsubscribe` (`-race`, many goroutines).
- `TestNoGoroutineLeak`.
- `ExampleBroker`.

## Stretch goals

- Message replay: `WithReplay(n int)` keeps the last `n` messages per topic and delivers
  them to a new subscriber immediately.
- Per-topic metrics via `expvar` (bridge to Project 26/29).
- A `RequestReply(ctx, topic, data) (Message, error)` built on a one-shot reply
  subscription.
- Cross-process: expose the broker over the TCP protocol you'll design in Project 25's
  `SUBSCRIBE` command.
- Priority subscribers that get delivered before `DropNewest`/`DropOldest` kicks in for
  everyone else.

## Self-check questions

1. In `trySend`, what is the *type* and *value* of `timeoutC` when `timeout <= 0`? Why does
   a receive on a nil channel block forever instead of panicking or returning immediately?
2. Walk through why `Close` acquiring `Lock()` is guaranteed not to proceed while a
   `Publish` is running, given `Publish` holds `RLock` for its *entire* fan-out (not just
   while reading the subscriber list). What would go wrong if `Publish` released the
   `RLock` before sending to subscriber channels?
3. `DropOldest` does a non-blocking receive then a non-blocking send on the same channel.
   Between those two operations, could another goroutine sneak in and fill the slot you
   just freed? Does that matter here, and why (or why not)?
4. Your `Disconnect` policy closes a subscriber's channel from inside `Publish`, which
   holds `RLock`. Is closing a channel while holding a read lock on unrelated broker state
   safe? What *would* be unsafe to do to the subscriber registry itself while only holding
   `RLock`?
5. `TestCloseTimeoutEscapeHatch` — after that test, is the wedged `Publish` goroutine
   still running? If so, is that a bug or an accepted trade-off, and how does your README
   justify it?
6. Why is `sync.Map` a poor fit for the subscriber registry here, specifically — what
   operation do you need that `sync.Map` doesn't give you efficiently?
7. Pattern `orders.>` — write the matcher logic for `>` in words: what does it require
   about segment count, and why does a bare `>` alone (matching zero-or-more) behave
   differently from what this spec asked for?
