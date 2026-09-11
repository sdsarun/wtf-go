# Project 15 — Log Aggregation Pipeline

| | |
|---|---|
| **Difficulty** | 4 / 5 |
| **Estimated time** | 10–14 hours |
| **Prerequisites** | [09](09-mini-grep.md), [13](13-report-engine.md), [14](14-link-checker.md) |
| **Builds toward** | [19 – Pub/Sub Broker](19-pubsub-broker.md), [29 – Task Scheduler](29-task-scheduler.md) |

## Why this project

The **pipeline pattern**: independent stages, each a goroutine, connected by channels,
with `context` cancellation threaded through every stage and a clean drain-and-flush on
shutdown. You'll fan a stream of raw log lines out to a pool of parsers, fan their results
back in, enrich, window-aggregate, and write to multiple sinks — all `-race` clean, with a
**goroutine-leak test** and a reusable hand-rolled `errgroup`.

## Go concepts you MUST use

- [ ] the **pipeline** pattern: `read → parse(pool) → filter → enrich → fan-out → sinks`,
      each stage a goroutine communicating only via channels
- [ ] **fan-out** (one channel, K parse workers) and **fan-in** (merge K channels into one)
- [ ] `select` with a `case <-ctx.Done(): return` in **every** stage's loop
- [ ] `context` cancellation propagated to all stages; cancel triggered by SIGINT
      (`signal.NotifyContext`) **or** by all sources reaching EOF (batch mode)
- [ ] `sync/atomic` (`atomic.Int64`) for the hot counters; a `sync.Mutex`-guarded map for
      the aggregation windows — and a README paragraph on why each
- [ ] a reusable **`Group`** type (hand-rolled errgroup): `Go(func() error)`, `Wait() error`,
      cancels a shared context on the first non-nil error
- [ ] bounded (buffered) channels → **backpressure**: a slow sink slows the whole pipeline
- [ ] a dead-letter channel for unparseable lines
- [ ] a `TestNoGoroutineLeak`

## Background

**Event model:**

```go
type Level int // Trace < Debug < Info < Warn < Error < Fatal
type Event struct {
    Time   time.Time
    Level  Level
    Msg    string
    Fields map[string]string
    Source string
    Seq    int64      // assigned by enrich
    Raw    string
}
```

**Parse formats** (config `"format"`):

| format | example line |
|---|---|
| `logfmt` | `ts=2024-03-09T10:00:01Z level=warn msg="disk 85%" host=db1` |
| `json` | `{"time":"...","level":"WARNING","message":"...","host":"db1"}` |
| `access` | `10.0.0.1 - alice [09/Mar/2024:10:00:01 +0000] "GET /x HTTP/1.1" 500 12` |
| `regex` | config supplies `"pattern"` with named groups `(?P<level>…)` etc. |

Level aliases: `WARNING→warn`, `ERR/ERROR→error`, `CRIT/FATAL→fatal`, `INFORMATIONAL→info`,
numbers `0–7` → syslog severities.

**Windowing:** events are bucketed by `Time.Truncate(window)` (default `1m`). The
aggregate sink keeps `map[windowKey]map[groupKey]int64` behind a mutex and flushes windows
older than `now - 2*window` (or all of them at shutdown).

**Shutdown order:** cancel context → `read` stops → close its output → each downstream
stage drains its input, does its work, closes its output → sinks flush → stats printed.
No stage may exit while its input channel still has buffered items.

## Requirements

### Functional requirements

1. `logpipe run -config PIPE.json [-follow] [-workers N] [-window 1m]`.
   - batch (default): process every source to EOF, then shut down and print stats.
   - `-follow`: tail sources (`os.File` + poll for growth), run until SIGINT.
2. Config describes: `sources` (`[]{name, path}`; `path: "-"` = stdin), `format`,
   optional `pattern`, `filters`, `enrichers`, `sinks`.
3. **Filters** (all must pass): `min_level`, `match` (`field=glob`), `not_match`,
   `since`/`until` (RFC3339), `sample` (keep 1 in N).
4. **Enrichers**: `seq` (monotonic `Seq`), `hour` (adds `hour` field), `geo` (adds
   `country` from a static `ip→cc` prefix table in config), `rename`, `redact`
   (`field` → `***`).
5. **Sinks** (fan-out; each on its own buffered channel):
   - `write` — `{path, format: raw|logfmt|json}`; write matched events.
   - `aggregate` — `{by: ["level"], path}`; windowed counts; flush on window close +
     shutdown; output sorted by `(window, groupKey)`.
   - `alert` — `{by: ["level"], threshold: 10, path}`; when a window's count for a group
     ≥ threshold, emit one alert line for that window.
6. `-workers N` (default `NumCPU`) parse workers. `-workers 1` ⇒ deterministic sink
   output for a fixed input (golden-testable).
7. Stats block to stderr on shutdown (batch: always; follow: on SIGINT).
8. `-max-dead-rate F` (default `1.0`): if `dead / read > F`, exit 1 after printing stats.

### Exact contract — stats block

```
sources        2
lines read     50000
parsed         49873
filtered out   30012
dead-lettered  127
written        19861
windows        42
alerts         3
duration       1.42s
```

### Exact contract — aggregate output (`by: ["level"]`, window 1m)

```
window                level  count
2024-03-09T10:00:00Z  info   1423
2024-03-09T10:00:00Z  warn   57
2024-03-09T10:00:00Z  error  12
2024-03-09T10:01:00Z  info   1388
```

### Exact contract — alert line

```
ALERT  2024-03-09T10:00:00Z  level=error  count=12  threshold=10
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | batch completed (dead-rate within limit); or `-follow` interrupted cleanly |
| `1` | dead-rate exceeded `-max-dead-rate`; a stage returned an error; a sink path unwritable |
| `2` | usage — bad config, unknown format, unreadable source, bad flag |
| `130` | second SIGINT (force quit) |

### Concurrency requirements (graded)

- [ ] `go test -race ./...` clean under `-workers 32`.
- [ ] `TestNoGoroutineLeak` — goroutine count returns to baseline after a batch run **and**
      after a cancelled run.
- [ ] Cancelling mid-stream: `Run` returns within ~`window` (not "after every line").
- [ ] No stage drops buffered input on shutdown (batch run over a fixed file:
      `lines read == parsed + dead-lettered`, always).
- [ ] Every channel has a documented capacity; every worker loop selects on `ctx.Done()`.
- [ ] The `Group` type: first error cancels the context; `Wait` returns that first error;
      later errors are discarded (documented).

### Case specification — SUCCESS (batch, `-workers 1`)

Fixture `testdata/app.log` — 20 lines of `logfmt`, mixed levels, spanning 2 minutes, plus
2 malformed lines.

| # | Command | assertion | exit |
|---|---|---|---|
| S1 | `logpipe run -config testdata/basic.json` | stats: `lines read 22`, `parsed 20`, `dead-lettered 2` | 0 |
| S2 | `logpipe run -config testdata/filter_warn.json` (`min_level: warn`) | `write` sink contains only warn+error+fatal | 0 |
| S3 | `logpipe run -config testdata/agg_level.json` | aggregate file matches the golden window/level/count table | 0 |
| S4 | `logpipe run -config testdata/alert.json` (`threshold 3` on error) | alert file has one `ALERT` line for the window with ≥3 errors | 0 |
| S5 | `logpipe run -config testdata/enrich.json` | `write` events have `seq` 1..20 and an `hour` field | 0 |
| S6 | `logpipe run -config testdata/json_format.json` (fixture `app.jsonl`) | parses JSON lines, level aliases resolved | 0 |
| S7 | `logpipe run -config testdata/access_format.json` (fixture `access.log`) | `status` field present; `500`s filterable | 0 |
| S8 | `logpipe run -config testdata/multi_sink.json` | `write` **and** `aggregate` both produced, counts consistent | 0 |
| S9 | `logpipe run -config testdata/stdin.json < testdata/app.log` | same as S1 | 0 |
| S10 | `logpipe run -config testdata/basic.json` twice | byte-identical sink files (deterministic at `-workers 1`) | 0 |
| S11 | `cat testdata/app.log \| logpipe run -config testdata/basic.json -workers 8` | same **counts** as S1 (order may differ) | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | `logpipe run -config testdata/dead_heavy.json` (`-max-dead-rate 0.1`, fixture 50% junk) | stats + `error: dead-letter rate 0.50 exceeds 0.10` | 1 |
| F2 | `logpipe run -config testdata/bad_sink_path.json` (`/root/x.log`) | `error: sink "write": open /root/x.log: permission denied` | 1 |
| F3 | `logpipe run -config testdata/unknown_format.json` | `error: config: unknown format "syslog"` | 2 |
| F4 | `logpipe run -config testdata/bad_regex.json` | `error: config: invalid pattern: ...` | 2 |
| F5 | `logpipe run -config testdata/missing_source.json` | `error: source "app": open testdata/nope.log: no such file or directory` | 2 |
| F6 | `logpipe run -config testdata/not_json.json` | `error: testdata/not_json.json: invalid config: ...` | 2 |
| F7 | `logpipe run` | `error: -config is required` | 2 |
| F8 | `logpipe` | usage | 2 |

### Error catalogue

| Trigger | Format |
|---|---|
| dead-rate | `error: dead-letter rate %.2f exceeds %.2f` |
| sink open | `error: sink %q: %v` |
| config format | `error: config: unknown format %q` |
| config pattern | `error: config: invalid pattern: %v` |
| source open | `error: source %q: %v` |
| bad config file | `error: %s: invalid config: %v` |
| missing flag | `error: -config is required` |

## Suggested milestones

1. `internal/pipe/group.go` — the `Group` (errgroup). Test: first error cancels, `Wait`
   returns it, no leak.
2. `internal/logpipe/parse.go` — `logfmt`, `json`, `access`, `regex`; level normalization;
   dead-letter for failures. Table tests.
3. Stages as functions returning channels:
   `read(ctx, srcs) <-chan rawLine` · `parse(ctx, in, workers) (<-chan Event, <-chan deadLine)` ·
   `filter(ctx, in, f) <-chan Event` · `enrich(ctx, in, e) <-chan Event`.
4. Fan-in `merge(ctx, ...<-chan Event) <-chan Event`.
5. Fan-out to sinks: `split(ctx, in, n) []<-chan Event`; the three sink implementations.
6. `Windower` (mutex map) shared by `aggregate` and `alert`.
7. `Pipeline.Run(ctx)` wiring it all with the `Group`; shutdown order; stats.
8. `-follow` (tailer), `signal.NotifyContext`, double-SIGINT.
9. Concurrency tests + golden CLI tests.

## Project layout

```
projects/15-logpipe/
  cmd/logpipe/main.go
  internal/pipe/group.go          // reusable errgroup
  internal/logpipe/parse.go
  internal/logpipe/event.go       // Event, Level, Level.String / parse
  internal/logpipe/stages.go      // read/parse/filter/enrich/merge/split
  internal/logpipe/sinks.go       // write / aggregate / alert
  internal/logpipe/window.go      // Windower (mutex map)
  internal/logpipe/pipeline.go    // config -> Run(ctx)
  internal/logpipe/tail.go        // -follow
  internal/*/**_test.go
  cmd/logpipe/main_test.go
  testdata/
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly (golden sink files under `testdata/golden/`).
- [ ] Every concurrency-requirements checkbox has a test.
- [ ] `lines read == parsed + dead-lettered` holds for every batch run (invariant test).
- [ ] `go test -race -count=5 ./...` is clean (flush out flaky races).
- [ ] `-workers 1` output is byte-deterministic; `-workers N` counts match.
- [ ] The `Group` type is genuinely reusable (no `logpipe` imports) and has its own tests.

## Test requirements

- `TestGroup` — success, first-error-cancels, panic-in-Go propagation (your choice: rethrow
  or convert — document), no leak.
- `TestParse*` — one per format incl. malformed → dead letter, level aliases, missing
  timestamp (→ use `time.Now()` or zero? decide + test).
- `TestStagesCancel` — each stage returns promptly on `ctx.Done()` with a full input
  channel.
- `TestMergeFairness` — `merge` doesn't starve a slow input.
- `TestWindower` — bucketing, flush-on-close, concurrent `Add` under `-race`.
- `TestPipelineInvariant` — the `read == parsed + dead` property over a generated 100k-line
  file.
- `TestNoGoroutineLeak`, `TestPromptCancel`.
- `TestDeterministicWorkers1`.
- `ExampleGroup`.

## Stretch goals

- A `tcp` source: accept syslog over TCP (bridge to Project 23).
- Adaptive parse-worker pool that grows/shrinks with queue depth.
- Exactly-once window flush with a WAL so a crash mid-window doesn't lose counts
  (Project 22).
- Expose live stats at `/debug/vars` via `expvar` (Project 26/29).
- Replace the poll-based tailer with `fsnotify`-style behaviour — document why that needs
  a third-party package and what the stdlib alternative costs.

## Self-check questions

1. Your `parse` stage has 8 workers reading one channel and writing one channel. Draw
   where fan-out happens and where fan-in happens. Why does the output channel need to be
   closed by a *separate* goroutine, not by any worker?
2. `select { case ev := <-in: …; case <-ctx.Done(): return }` — if `in` is closed (not
   just empty), which case fires, and how do you tell "closed" from "got a zero Event"?
3. Batch mode must cancel the context when all sources hit EOF. Where do you detect that,
   and why can't `read` just `close` its channel and let the rest cascade (what about the
   `alert` sink's timer)?
4. `atomic.Int64` for `linesRead` but a `sync.Mutex` for the windows map. Why not atomic
   for both? Why not a mutex for both?
5. A sink's channel fills up (slow disk). Describe exactly how backpressure reaches
   `read`. Is that the behaviour you want for logs? When would drop-oldest be better?
6. On SIGINT you cancel the context. A parse worker is mid-`select`, the `filter` stage
   has 500 buffered events. What happens to those 500, and is that acceptable?
7. `TestNoGoroutineLeak` passes locally but flakes in CI. Name two timing assumptions it
   probably makes and how to remove them.
8. Your `Group.Go` runs a func that panics. What should `Wait` do — and what does
   `golang.org/x/sync/errgroup` actually do? Pick a behaviour and defend it.
