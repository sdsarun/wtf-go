# Project 29 — Task Scheduler / Cron Daemon

| | |
|---|---|
| **Difficulty** | 5 / 5 |
| **Estimated time** | 13–17 hours |
| **Prerequisites** | [03](03-date-duration-calculator.md), [13](13-report-engine.md), [15](15-log-pipeline.md), [26](26-reverse-proxy.md) |
| **Builds toward** | [30 – Interpreter](30-interpreter.md) (as a "what's next" capstone lead-in) |

## Why this project

A cron daemon: a **hand-written cron expression parser** (a small, exhaustively-testable
pure function, in the spirit of Project 01), a scheduler goroutine per job computing its
next fire time without drifting, `os/exec` for shelling out, an **interface-based plugin
registry** for job types (echoing Project 13), `pprof`/`runtime` introspection, and one
genuine OS-specific behaviour (killing a job's whole process group on timeout) behind a
build tag.

## Go concepts you MUST use

- [ ] a hand-written **cron expression parser** (`minute hour day month weekday`,
      `* , - /`) — pure, exhaustively tested
- [ ] `time.Timer` per job, recomputed each cycle from `time.Now()` (never accumulate
      drift by repeatedly adding a fixed interval)
- [ ] `context` — per-run timeout via `context.WithTimeout`, whole-daemon cancellation
- [ ] `os/exec.CommandContext` — capturing stdout/stderr, exit code, `*exec.ExitError`
- [ ] an **interface-based job-plugin registry** (`Job` interface, `Register(kind,
      factory)`, same shape as Project 13's registry)
- [ ] `os/signal` — `SIGHUP` reloads the job list, `SIGINT`/`SIGTERM` graceful shutdown
- [ ] `log/slog` — one structured line per job run (name, start, duration, status,
      exit code)
- [ ] `net/http/pprof` registered on a separate debug listener; `runtime.NumGoroutine`,
      `runtime.GOMAXPROCS`, `runtime.ReadMemStats` surfaced on a `/status` page
- [ ] a build-tag-gated OS-specific behaviour: killing a job's **process group** on
      timeout (unix) vs a documented no-op fallback (other platforms)

## Background

**Cron fields**, in order: `minute (0-59) hour (0-23) day-of-month (1-31) month (1-12)
weekday (0-6, 0=Sunday)`. Each field accepts `*`, a number, a range `a-b`, a step `*/n` or
`a-b/n`, or a comma-separated list of any of those. **Quirk to implement faithfully:** if
**both** day-of-month and weekday are restricted (neither is `*`), a time matches if
**either** matches (OR), not both (AND) — this is standard cron behaviour and it trips
people up; you must get it right and test it explicitly.

**No-drift scheduling.** Don't do `next = last + interval` repeatedly (drifts under
scheduling jitter and breaks across DST). Instead, each cycle: `next :=
NextAfter(schedule, time.Now())`; sleep until `next`; run; repeat, always recomputing from
the current wall-clock time.

**Overlap policy** — what happens if a job's next scheduled fire time arrives while its
previous run is still executing:

| Policy | Behaviour |
|---|---|
| `skip` | log `skipped (still running)`, don't run, keep the schedule |
| `queue` | at most **one** run queues; it starts the instant the current run finishes |
| `allow` | runs concurrently, no limit |

## Requirements

### Config

```json
{
  "jobs": [
    {"name": "backup", "schedule": "0 2 * * *", "kind": "command",
     "config": {"cmd": ["/bin/sh", "-c", "echo backing up"]},
     "timeout": "10m", "overlap": "skip", "retries": 1},
    {"name": "healthcheck", "schedule": "*/5 * * * *", "kind": "http",
     "config": {"url": "http://localhost:8080/healthz"}, "timeout": "5s", "overlap": "skip"}
  ]
}
```

### Job kinds (registry)

| kind | config | `Run` behaviour |
|---|---|---|
| `command` | `{"cmd": ["prog","arg",...]}` | `exec.CommandContext`; non-zero exit → error carrying the exit code |
| `http` | `{"url": "..."}` | `GET`; status `>=400` → error |
| `log` | `{"message": "..."}` | `slog.Info(message)`; test/demo kind |
| `sleep` | `{"duration": "..."}` | sleeps (respecting `ctx`); **test-only** kind, used to make overlap/timeout deterministic in tests |

`Job interface { Run(ctx context.Context) error }`; `Register(kind string, factory
func(json.RawMessage) (Job, error))`; a duplicate `Register` panics (same discipline as
Project 13).

### Retries

On failure, retry up to `retries` additional times with a fixed 1s gap between attempts
(a real backoff is a stretch goal), all within the same run's timeout budget if any
remains, else the run is marked failed without exhausting retries.

### `/status` (debug HTTP listener, `-debug-addr`, default `:6060`, separate from any
application traffic — this daemon has none)

`GET /status`: JSON — per job: `name, schedule, kind, next_run, last_run, last_status,
last_duration_ms, running bool`. `GET /debug/pprof/*`: the standard `net/http/pprof`
handlers, registered on the same mux.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | clean shutdown (SIGINT/SIGTERM, all jobs finished or force-stopped within `-shutdown-grace`) |
| `1` | config parse/validation error at startup or on a rejected reload |
| `2` | bad flags |

### Case specification — SUCCESS: cron parser (`NextAfter(expr string, from time.Time) (time.Time, error)`)

All `from` values are UTC. This table **is** the primary spec for the parser — treat every
row as a required test case.

| # | Expression | `from` | Expected next |
|---|---|---|---|
| C1 | `* * * * *` | `2024-03-09T10:00:30Z` | `2024-03-09T10:01:00Z` |
| C2 | `0 * * * *` | `2024-03-09T10:00:00Z` | `2024-03-09T11:00:00Z` (exactly `from` doesn't count — strictly after) |
| C3 | `0 * * * *` | `2024-03-09T10:30:00Z` | `2024-03-09T11:00:00Z` |
| C4 | `*/15 * * * *` | `2024-03-09T10:16:00Z` | `2024-03-09T10:30:00Z` |
| C5 | `0 2 * * *` | `2024-03-09T10:00:00Z` | `2024-03-10T02:00:00Z` |
| C6 | `0 2 * * *` | `2024-03-09T01:00:00Z` | `2024-03-09T02:00:00Z` |
| C7 | `0 9-17 * * *` | `2024-03-09T20:00:00Z` | `2024-03-10T09:00:00Z` |
| C8 | `0 9-17/2 * * *` | `2024-03-09T09:00:00Z` | `2024-03-09T11:00:00Z` |
| C9 | `0 0 1 * *` | `2024-03-09T00:00:00Z` | `2024-04-01T00:00:00Z` |
| C10 | `0 0 1 * *` | `2024-12-31T23:59:59Z` | `2025-01-01T00:00:00Z` |
| C11 | `0 0 * * 1` (Mondays) | `2024-03-09T00:00:00Z` (a Saturday) | `2024-03-11T00:00:00Z` |
| C12 | `0 0 * * 0` (Sundays) | `2024-03-11T00:00:00Z` (a Monday) | `2024-03-17T00:00:00Z` |
| C13 | `0 0 1,15 * *` | `2024-03-02T00:00:00Z` | `2024-03-15T00:00:00Z` |
| C14 | `0 0 1,15 * *` | `2024-03-16T00:00:00Z` | `2024-04-01T00:00:00Z` |
| C15 | `0 0 13 * 5` (13th **or** Friday — the OR quirk) | `2024-03-09T00:00:00Z` (Saturday) | `2024-03-13T00:00:00Z` (the 13th, a Wednesday — arrives *before* the next Friday) |
| C16 | `0 0 13 * 5` same expr | `2024-03-14T00:00:00Z` | `2024-03-15T00:00:00Z` (a Friday — the next match after the 13th has passed) |
| C17 | `30 4 * * 2` (Tuesdays 04:30) | `2024-03-09T00:00:00Z` | `2024-03-12T04:30:00Z` |
| C18 | `59 23 31 12 *` (only Dec 31, 23:59) | `2024-01-01T00:00:00Z` | `2024-12-31T23:59:00Z` |
| C19 | `0 0 30 2 *` (Feb 30 — never exists) | any `from` | `error: schedule never matches` (don't infinite-loop searching — bound the search, e.g. 5 years, then error) |
| C20 | malformed: `*/0 * * * *` | — | `error: step must be >= 1` |
| C21 | malformed: `60 * * * *` | — | `error: minute 60 out of range 0-59` |
| C22 | malformed: `* * * *` (4 fields) | — | `error: expected 5 fields, got 4` |

### Case specification — SUCCESS: scheduler behaviour (using the `sleep`/`log` test job kinds and short real intervals, e.g. schedules like `every second` via a test-only helper that accepts a `time.Duration` directly instead of parsing cron strings, so tests aren't tied to wall-clock minute boundaries)

| # | Scenario | Assertion |
|---|---|---|
| S1 | a `log` job fires on schedule 3 times over a short window | 3 `slog` records observed, `status=ok` |
| S2 | a `sleep` job whose duration exceeds its own interval, `overlap: skip` | the second scheduled fire is skipped (logged `skipped`), never runs concurrently with itself |
| S3 | same but `overlap: queue` | the second run starts immediately after the first finishes (not at its original scheduled time) |
| S4 | same but `overlap: allow` | both runs are observed executing concurrently (a shared counter peaks at ≥2) |
| S5 | a `command` job with `cmd: ["false"]` on unix (or an equivalent guaranteed-failing command) and `retries: 2` | 3 total attempts observed (1 + 2 retries), final status `failed` |
| S6 | a `command` job exceeding its `timeout` | the process is killed, run marked `failed: timeout`, and (unix) its **process group** is confirmed gone (no orphaned children — test with a `cmd` that spawns a child and sleeps) |
| S7 | `SIGHUP` with an edited config (job added) | the new job starts firing on its schedule without restarting the process; existing jobs' in-flight runs are unaffected |
| S8 | `SIGHUP` with an invalid new config | reload rejected, an error logged, previous jobs keep running unchanged |
| S9 | `GET /status` | JSON listing every job with a `next_run` in the future and, after at least one fire, a populated `last_run`/`last_status` |
| S10 | `GET /debug/pprof/goroutine?debug=1` | valid pprof text output |
| S11 | SIGINT with one job mid-run (a `sleep` shorter than `-shutdown-grace`) | the daemon waits for it, then exits `0` |
| S12 | SIGINT with a job mid-run **longer** than `-shutdown-grace` | the daemon force-cancels it (context) and exits `0` after the grace period, not before |

### Case specification — FAILURE

| # | Scenario | Result |
|---|---|---|
| F1 | config references an unregistered `kind` | startup error, exit `1` |
| F2 | a job's `config` fails its kind's factory validation (e.g. `command` with empty `cmd`) | startup error, exit `1` |
| F3 | duplicate job `name`s in config | startup error, exit `1` |
| F4 | `overlap` value not one of `skip`/`queue`/`allow` | startup error, exit `1` |
| F5 | `-config` file missing | startup error, exit `1` |
| F6 | `schedd -config` (missing value) | usage, exit `2` |

### Error catalogue

| Trigger | Format |
|---|---|
| field count | `error: expected 5 fields, got %d` |
| range | `error: %s %d out of range %d-%d` |
| step | `error: step must be >= 1` |
| never matches | `error: schedule never matches` |
| unknown kind | `error: job %q: unknown kind %q` |
| bad job config | `error: job %q: %v` |
| duplicate name | `error: duplicate job name %q` |
| bad overlap | `error: job %q: overlap must be skip, queue, or allow` |

## Suggested milestones

1. Cron parser: field parsing (`*`, `N`, `a-b`, `*/n`, lists) → a `matcher` per field;
   `NextAfter` by minute-stepping forward with a bound (this is the C-table — nail it
   before anything else).
2. `Job` interface + registry + the four built-in kinds.
3. Single-job scheduler loop: compute next, sleep via `time.Timer`, run once, repeat —
   no overlap handling yet.
4. Overlap policies (`skip`/`queue`/`allow`) with a per-job mutex/semaphore.
5. Retries.
6. `os/exec.CommandContext` timeout handling + the unix process-group kill (build-tagged;
   a no-op stub elsewhere).
7. `slog` run logging; `/status` + `pprof` debug listener.
8. `SIGHUP` reload (build new job set, diff against running, start new/stop removed —
   document what happens to a running job whose config changed: let it finish under the
   old config, or not — your choice, document and test it).
9. Graceful shutdown with `-shutdown-grace`.

## Project layout

```
projects/29-schedd/
  cmd/schedd/main.go
  internal/cron/parse.go       // NextAfter, field parsing
  internal/cron/*_test.go
  internal/job/job.go          // Job interface, registry
  internal/job/command.go
  internal/job/http.go
  internal/job/testkinds.go    // log, sleep (build-tag them out of non-test builds if you prefer, or just document they're test-only)
  internal/scheduler/scheduler.go  // per-job loop, overlap policy, retries
  internal/scheduler/reload.go
  internal/scheduler/status.go     // /status + pprof mux
  internal/procgroup/kill_unix.go  // //go:build unix
  internal/procgroup/kill_other.go // //go:build !unix
  internal/*/**_test.go
  cmd/schedd/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every cron-parser row (C1–C22) passes exactly.
- [ ] Every scheduler-behaviour row (S1–S12) and failure row (F1–F6) passes.
- [ ] `go test -race ./...` clean.
- [ ] S6's process-group test actually proves no orphaned child survives (check via
      `/proc` on Linux, or document the equivalent check you used — skip gracefully with
      `t.Skip` on platforms where the check isn't feasible, but don't skip the *kill*
      behaviour itself, only the verification technique).
- [ ] Reload (S7/S8) never drops a currently-running job's ability to finish and log its
      result.
- [ ] `NextAfter` never loops forever on an unsatisfiable schedule (C19) — bounded search,
      tested with a timeout-guarded test.

## Test requirements

- `TestNextAfter` — C1–C22, exhaustively.
- `TestJobRegistry` — built-ins present, duplicate panics, unknown kind error.
- `TestOverlapSkip`, `TestOverlapQueue`, `TestOverlapAllow` — S2–S4.
- `TestRetries` — S5.
- `TestTimeoutKillsProcessGroup` — S6 (platform-guarded verification).
- `TestReloadAddsJob`, `TestReloadRejectsBadConfig` — S7, S8.
- `TestStatusEndpoint`, `TestPprofRegistered`.
- `TestGracefulShutdownWaits`, `TestGracefulShutdownForcesAfterGrace` — S11, S12.
- `ExampleNextAfter`.

## Stretch goals

- Exponential backoff with jitter for retries (bridge to Project 27's rate limiter math,
  inverted).
- A `once` job type: runs a single time at a specific `time.Time`, then removes itself.
- Persist run history (bridge to Project 22's storage engine) so `/status` survives a
  restart.
- A `dependency` field: job B only fires after job A's most recent run succeeded.
- Distribute across multiple daemon instances with leader election (far beyond this
  curriculum, but worth naming as "what real cron replacements like Nomad/Airflow do").

## Self-check questions

1. Walk through why `0 0 13 * 5` matching "the 13th OR a Friday" (not AND) is the correct
   cron semantics, and construct an expression where a naive AND implementation would
   never fire at all (a real footgun in real cron).
2. `NextAfter` must not infinite-loop on `0 0 30 2 *`. What bound did you choose (a
   maximum number of days/years searched), and why is any finite bound "wrong" in
   principle but fine in practice?
3. Recomputing `next := NextAfter(schedule, time.Now())` every cycle instead of
   `next += interval` — what specific bug does this avoid across a daylight-saving-time
   transition (pick a concrete schedule and DST date and trace both approaches)?
4. `overlap: queue` allows **at most one** queued run. What data structure/field holds
   "there's a queued run pending," and what happens to a second-arriving scheduled fire
   while one is already queued (should it also skip, per the spec's intent)?
5. Your `command` job kind kills the whole process group on timeout, not just the direct
   child. Why does `cmd.Process.Kill()` alone leave orphans for a job that spawns its own
   subprocesses, and what `SysProcAttr` field fixes it on unix?
6. `SIGHUP` reload: a job named `"backup"` exists in both the old and new config with a
   *different* schedule. Does your reload logic recognize this as "the same job, updated"
   or "remove old backup, add new backup"? Which did you implement, and does an in-flight
   run of the old `"backup"` get cancelled or allowed to finish?
7. Two debug surfaces exist: `/status` (your own JSON) and `/debug/pprof/*` (stdlib).
   Why register both on the **same** `-debug-addr` mux rather than exposing `/status` on
   the (nonexistent) application port — what's the security/operational argument for
   keeping debug endpoints off any port that might ever be internet-facing?
