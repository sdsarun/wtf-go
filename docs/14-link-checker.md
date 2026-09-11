# Project 14 — Concurrent Link Checker / Crawler

| | |
|---|---|
| **Difficulty** | 3 / 5 |
| **Estimated time** | 8–12 hours |
| **Prerequisites** | [09](09-mini-grep.md), [12](12-json-config-loader.md), [13](13-report-engine.md) |
| **Builds toward** | [18 – Download Manager](18-download-manager.md), [26 – Reverse Proxy](26-reverse-proxy.md) |

## Why this project

Your first real concurrency: a bounded **worker pool** of HTTP fetchers, a **coordinator**
goroutine that owns the visited-set (so there's no shared-mutable-map to race on),
`context` cancellation wired from `SIGINT` and a deadline all the way down to each request,
per-host **rate limiting**, and jittered retry backoff. It must be `-race` clean and it
must stop *promptly* on Ctrl-C, printing a partial report.

## Go concepts you MUST use

- [ ] goroutines + `sync.WaitGroup`
- [ ] channels: a buffered job queue, **directional** channel parameters
      (`jobs <-chan Job`, `results chan<- Result`), `close` to signal "no more work",
      `range` over a channel
- [ ] the **worker-pool** pattern with a fixed concurrency `-c`
- [ ] a **coordinator / actor** goroutine that solely owns `visited map[string]visit` and
      the frontier — workers talk to it over channels, never touch the map
      (also implement the `sync.Mutex` alternative in a branch and benchmark both)
- [ ] `context`: `signal.NotifyContext(SIGINT, SIGTERM)`, `context.WithTimeout`,
      `ctx.Done()` selected in every blocking loop, `context.Cause`
- [ ] `context.WithValue` — a per-run `crawlID` carried in the context and included in
      every log line
- [ ] `net/http` client: explicit `Timeout`, a custom `http.Transport`
      (`MaxIdleConnsPerHost`), `CheckRedirect` to capture the redirect chain, `HEAD` with
      `GET` fallback
- [ ] `net/url`: `Parse`, `base.ResolveReference(ref)`, host/scheme comparison, dropping
      `#fragments`, normalizing trailing slashes
- [ ] `time.Ticker` for the global rate limit; `math/rand/v2` for backoff jitter
- [ ] the race detector — CI runs `go test -race`

## Background

**Link extraction is best-effort.** With only the standard library there is no HTML
parser, so `crawl` mode extracts `href=`/`src=` with a `regexp` and documents that it
misses JS-built links and malformed markup. `sitemap` mode uses `encoding/xml` and is
exact.

**Redirects** are followed (up to 10) but the final URL and the chain are recorded. A
2xx after redirects is `ok`; `-strict-redirects` makes any redirect count as a problem.

**Rate limiting** is global: at most `-rate` requests start per second, enforced by a
`time.Ticker` (a worker takes a tick before each request). `-delay` adds a per-host
minimum gap on top.

**Graceful cancel.** On `ctx` cancellation: stop handing out new jobs, let in-flight
requests finish or time out, drain results, print whatever report you have, exit `130`.

## Requirements

### Functional requirements

1. Subcommands:
   - `linkcheck check [-f FILE]` — read URLs (one per line, `#` comments) from `FILE` or
     stdin; check each once; no crawling.
   - `linkcheck crawl URL` — BFS crawl from `URL`, extracting links, bounded by `-depth`
     and `-same-host`.
   - `linkcheck sitemap URL` — GET `URL` (or `URL/sitemap.xml` if `URL` has no path),
     parse `<urlset><url><loc>…`, check every `loc`.
2. Concurrency `-c` (default `min(8, NumCPU*2)`), rate `-rate` (default `20`, `0` =
   unlimited), per-request `-timeout` (default `10s`), whole-run `-deadline` (default
   `0` = none), `-retries` (default `2`, on network error or `>=500`), `-depth`
   (default `3`), `-same-host` (default `true`), `-strict-redirects` (default `false`).
3. Each URL is fetched at most once (keyed by normalized URL). The visited set is owned by
   one goroutine.
4. Output `-format text|json|csv` (default `text`), to stdout; `-o FILE` optional.
5. Progress to stderr every second (unless `-quiet`): `checked=NN broken=NN inflight=NN`.
6. Exit codes below.

### Result & report model

```go
type Visit struct {
    URL       string
    Depth     int
    Status    int        // 0 if never got a response
    Err       string     // "" on success
    Redirects []string   // chain, final excluded
    Referrers []string   // sorted, unique
    Latency   time.Duration
}
```

Classification: `ok` (2xx, or 3xx when not strict), `redirect` (3xx, only meaningful when
strict off — shown separately), `broken` (4xx, 5xx, or `Err != ""`).

### Exact contract — `text` report

```
checked 42 urls in 3.14s (8 workers, rate 20/s)
ok        37
redirect   2
broken     3

BROKEN
  https://example.com/gone         404   from: /, /about
  https://example.com/slow         error: context deadline exceeded   from: /
  https://cdn.example.com/x.js     error: dial tcp: lookup cdn.example.com: no such host   from: /assets

REDIRECT
  https://example.com/old  ->  https://example.com/new   (301)
```

`from:` lists up to 5 referrers, sorted, `+N more` if truncated. Lists are sorted by URL.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | every checked URL is `ok` (redirects allowed unless `-strict-redirects`) |
| `1` | at least one `broken` (or `redirect` under `-strict-redirects`) |
| `2` | usage — bad subcommand, invalid seed URL, unreadable `-f` file, bad flag |
| `130` | interrupted (SIGINT/SIGTERM) — a partial report was printed |

### Concurrency requirements (these are graded)

- [ ] `go test -race ./...` is clean.
- [ ] No goroutine outlives `Run` — a `TestNoGoroutineLeak` compares
      `runtime.NumGoroutine()` before/after (with a small settle delay) and after a
      cancelled run.
- [ ] Cancelling the context makes `Run` return within `2 * timeout`, not "after every
      queued URL is fetched".
- [ ] The number of concurrently in-flight HTTP requests never exceeds `-c` (assert with
      an atomic counter + max-watermark in a test against an `httptest` server that
      sleeps).
- [ ] At most `-rate` requests start per second (measure start timestamps in a test).
- [ ] Every channel send/receive in a worker loop is inside a `select { case …: ; case
      <-ctx.Done(): return }` — no unconditional blocking sends.

### Case specification — SUCCESS (against an `httptest` fake site)

The test site: `/` links to `/a`, `/b`, `/gone` (404), `/old` (301 → `/new`), and
`https://external.test/x` (off-host). `/a` links back to `/` and to `/b`.

| # | Command | assertion | exit |
|---|---|---|---|
| S1 | `linkcheck crawl http://SITE/` (same-host, depth 3) | visits `/`, `/a`, `/b`, `/gone`, `/old`, `/new`; **not** `external.test` | 1 (`/gone`) |
| S2 | `linkcheck crawl http://SITE/ -same-host=false` | also visits `external.test/x` | 1 |
| S3 | `linkcheck crawl http://SITE/ -depth 0` | visits only `/` | 1 |
| S4 | `linkcheck check -f urls.txt` (3 good URLs) | `ok 3`, no `BROKEN` section | 0 |
| S5 | `linkcheck check` with stdin containing `/gone` | `broken 1` | 1 |
| S6 | `linkcheck sitemap http://SITE/sitemap.xml` (3 `<loc>`s) | all three checked | 0/1 per contents |
| S7 | `linkcheck crawl http://SITE/ -format json` | valid JSON: `{summary, visits: [...]}` | 1 |
| S8 | `linkcheck crawl http://SITE/ -c 1 -rate 5` | still correct, just slower; concurrency never exceeds 1 | 1 |
| S9 | `linkcheck crawl http://SITE/ -strict-redirects` | `/old` now counts as broken | 1 |
| S10 | SIGINT during a crawl of a slow site | partial report to stdout, exit 130, no leaked goroutines | 130 |
| S11 | `linkcheck crawl http://SITE/ -retries 2` against `/flaky` (500 twice then 200) | `/flaky` ends `ok` after 2 retries | 1 (still `/gone`) |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | `linkcheck` | usage | 2 |
| F2 | `linkcheck crawl not-a-url` | `error: invalid seed URL "not-a-url": missing scheme` | 2 |
| F3 | `linkcheck crawl ftp://x/` | `error: unsupported scheme "ftp" (want http or https)` | 2 |
| F4 | `linkcheck check -f nope.txt` | `error: open nope.txt: no such file or directory` | 2 |
| F5 | `linkcheck crawl http://SITE/ -c 0` | `error: -c must be >= 1` | 2 |
| F6 | `linkcheck crawl http://SITE/ -format yaml` | `error: -format must be text, json, or csv` | 2 |

### Error catalogue

| Trigger | Format |
|---|---|
| invalid seed | `error: invalid seed URL %q: %v` |
| scheme | `error: unsupported scheme %q (want http or https)` |
| file | `error: %v` |
| `-c` | `error: -c must be >= 1` |
| `-rate` | `error: -rate must be >= 0` |
| `-format` | `error: -format must be text, json, or csv` |

## Suggested milestones

1. `internal/crawl`: `normalizeURL`, `sameHost`, `resolveLinks(base, body)` (regexp),
   `parseSitemap` (`encoding/xml`).
2. `Fetcher` interface (`Fetch(ctx, url) (*Response, error)`), a real HTTP impl, and a
   fake for tests. `Response{Status, Body, FinalURL, Chain}`.
3. Single-worker crawl (no concurrency) to nail correctness: BFS, depth, visited, report.
4. The coordinator goroutine: owns `visited` + frontier; channels `nextJob`, `foundLinks`,
   `done`.
5. The worker pool: `-c` workers, each `select`ing on jobs / `ctx.Done()`; results back to
   the coordinator; `WaitGroup` for shutdown.
6. Rate limiter (`time.Ticker`), per-host delay, retries with `rand/v2` jitter.
7. `signal.NotifyContext`, `-deadline`, partial report on cancel, exit 130.
8. Renderers (text/json/csv); progress line; `main`.
9. Concurrency tests: race, leak, concurrency cap, rate cap, prompt cancel.

## Project layout

```
projects/14-linkcheck/
  cmd/linkcheck/main.go
  internal/crawl/url.go         // normalize, sameHost, resolve
  internal/crawl/fetch.go       // Fetcher interface + HTTP impl
  internal/crawl/sitemap.go
  internal/crawl/crawler.go     // coordinator + worker pool + Run(ctx)
  internal/crawl/ratelimit.go
  internal/crawl/report.go
  internal/crawl/*_test.go
  internal/crawl/fake_test.go   // in-memory site + fake fetcher / httptest server
  cmd/linkcheck/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly (S-rows against an `httptest` server built in the
      test).
- [ ] Every concurrency-requirements checkbox above has a passing test.
- [ ] `go test -race ./...` clean; `go vet` clean.
- [ ] The report is **deterministic** given the same site (sort everything) even though
      fetch order isn't.
- [ ] `main` is thin; `Run(ctx, cfg) (Report, error)` is the testable core.
- [ ] Ctrl-C twice force-exits immediately (second signal → `os.Exit(130)`).

## Test requirements

- `TestNormalizeURL` — fragments dropped, trailing slash, default ports, case of host.
- `TestResolveLinks` — relative, absolute, protocol-relative, `mailto:` skipped,
  duplicates.
- `TestParseSitemap` — valid, empty, malformed XML.
- `TestCrawlCorrectness` — S1–S3, S9 against the fake site, single-worker.
- `TestWorkerPoolConcurrencyCap`, `TestRateLimit`, `TestNoGoroutineLeak`,
  `TestPromptCancel`, `TestRetryWithBackoff`.
- `TestRaceStress` — `-race`, `-c 32`, 500-page generated site.
- `TestReportDeterminism` — run twice, identical output.
- `TestRenderers` — text/json/csv golden.
- `ExampleCrawler`.

## Stretch goals

- Respect `robots.txt` (`Disallow` rules) — write the tiny parser.
- `-max-pages N` global cap.
- Persist a crawl and resume it (bridge to Project 07/22).
- Swap the regexp extractor for a hand-written tokenizer over `<a …>` tags (Project 30
  skills).
- `errgroup`-style: replace your hand-rolled `WaitGroup`+error-channel with a small
  generic `Group` type (you'll formalize this in Project 15).

## Self-check questions

1. You chose a coordinator goroutine over `sync.Mutex` for `visited`. What's the argument
   for each? Which scales better to 64 workers, and why might the mutex actually be
   *faster* for this workload?
2. A worker does `results <- r` with no `select`. The coordinator has exited (context
   cancelled). What happens to the worker, and how does that become a goroutine leak?
3. `context.WithTimeout(ctx, 10*time.Second)` returns a `cancel` func you must call. What
   leaks if you don't, and where exactly do you put the `defer cancel()`?
4. `http.Client{Timeout: 10s}` vs passing a context with a 10s deadline to the request —
   what's the difference in *what* gets cancelled (the whole request vs just the dial)?
5. Ctrl-C fires. Your `-rate` ticker is blocked in a worker waiting for the next tick.
   How does that worker learn to stop, and what does the `select` look like?
6. `base.ResolveReference(ref)` on `base = http://x/a/b` and `ref = ../c` — what's the
   result, and what does `ref = //cdn/x` resolve to?
7. Two workers both discover `/b` before either reports it visited. How does your design
   guarantee `/b` is fetched exactly once?
8. `runtime.NumGoroutine()` right after `Run` returns is 3 higher than before. Name three
   things that could still be running and how you'd make each stop.
