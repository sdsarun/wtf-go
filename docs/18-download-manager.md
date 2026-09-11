# Project 18 — Concurrent Download Manager

| | |
|---|---|
| **Difficulty** | 4 / 5 |
| **Estimated time** | 9–13 hours |
| **Prerequisites** | [14](14-link-checker.md), [17](17-in-memory-cache.md) |
| **Builds toward** | [25 – KV Store](25-kv-store.md), [26 – Reverse Proxy](26-reverse-proxy.md) |

## Why this project

A segmented downloader — split one file into N byte ranges, fetch them concurrently, and
write each straight to its offset in the output file — forces you through the `io`
plumbing that most projects only touch lightly: `io.WriterAt`/`ReaderAt`, `io.CopyBuffer`
with a **pooled** buffer (`sync.Pool`), `io.Pipe` to decouple a slow network reader from a
writer in another goroutine, and resumable state that survives a `Ctrl-C`.

## Go concepts you MUST use

- [ ] HTTP `Range` requests (`Range: bytes=X-Y`), `206 Partial Content`, the
      `Content-Range` / `Accept-Ranges` response headers
- [ ] `io.Copy`, `io.CopyN`, `io.CopyBuffer`
- [ ] `io.WriterAt` / `io.ReaderAt` — write each chunk directly to its file offset with
      `*os.File` (which implements `WriterAt`), no in-memory reassembly
- [ ] `io.Pipe` — in the single-stream fallback, decouple the network-read goroutine from
      the disk-write+hash goroutine
- [ ] `sync.Pool` of reusable `[]byte` buffers for `io.CopyBuffer`, shared across chunk
      workers
- [ ] progress reporting via a channel, aggregated by a ticking goroutine
- [ ] `context` cancel on `SIGINT` → save resume state, exit cleanly
- [ ] resume-from-offset using a JSON sidecar state file
- [ ] `errors.Join` of per-chunk failures

## Background

**Segmented download.** `HEAD` the URL. If the response has `Accept-Ranges: bytes` and a
`Content-Length`, split `[0, size)` into `-c` near-equal chunks. Each worker does
`GET` with `Range: bytes=start-end`, expects `206`, and writes the body to
`file.WriteAt(buf, offset)` as it streams — never buffering a whole chunk in memory.

**No range support fallback.** A single worker streams the whole body through
`io.MultiWriter(file, hasher)`, or — to exercise `io.Pipe` — through a pipe: the read side
is drained by a `io.Copy(io.MultiWriter(file, hasher), pipeReader)` goroutine while the
network response is copied into `pipeWriter` by another; this decouples network
back-pressure from disk write speed and is the shape you'd extend for e.g. an in-flight
decompressor.

**Resume.** A sidecar `<output>.dlstate` (JSON) records the URL, total size, `ETag` (or
`Last-Modified`), and each chunk's `{start, end, done}`. `-resume` re-fetches only chunks
not marked done, after confirming the server's current `ETag`/size still match (otherwise
error — the remote file changed).

## Requirements

### Functional requirements

1. `dl URL -o FILE [flags]`. `HEAD URL` first (fallback to a `GET` with `Range: bytes=0-0`
   if `HEAD` is rejected) to learn size, range support, `ETag`.
2. `-c N` (default `4`) concurrent chunks when range-capable; ignored (forced to `1`)
   otherwise. `-timeout D` per-request timeout (default `30s`). `-retries N` (default `3`)
   per chunk, exponential backoff with jitter.
3. `-resume`: if `<FILE>.dlstate` exists and its URL/size/ETag match the server, skip
   `done` chunks; otherwise error unless `-force` (which restarts from scratch).
4. `-sha256 HASH`: after completion, compute the file's SHA-256 (streamed, not by
   re-reading the whole file into memory beyond a copy buffer) and compare; mismatch is a
   failure and the `.dlstate`/partial file are **kept** (so the user can inspect/retry).
5. `-rate BYTES_PER_SEC` (default `0` = unlimited): a shared token-bucket limiter across
   all chunk workers.
6. Progress to stderr every 200ms (suppressed by `-quiet`), one line, carriage-return
   overwritten:
   ```
   downloading file.bin  42.3%  12.3 MB/s  eta 8s  [====>     ]
   ```
7. On `SIGINT`: stop starting new work, let in-flight chunk writes finish (or the current
   `WriteAt` call complete), write `.dlstate` with progress so far, print
   `interrupted — resume with -resume`, exit `130`.
8. On full success: remove the `.dlstate` sidecar; print `done: <FILE> (<size> bytes in
   <duration>, <avg-throughput>/s)`.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | download complete (and checksum matched, if given) |
| `1` | HTTP error (4xx/5xx after retries), size/ETag mismatch on resume, checksum mismatch, write error |
| `2` | usage — bad URL, missing `-o`, bad flag values |
| `130` | interrupted (state saved) |

### Exact contract — `.dlstate` schema

```json
{
  "url": "http://host/file.bin",
  "size": 10485760,
  "etag": "\"abc123\"",
  "chunks": [
    {"start": 0, "end": 2621439, "done": true},
    {"start": 2621440, "end": 5242879, "done": false}
  ]
}
```

### Case specification — SUCCESS (against `httptest` servers)

`RANGE_SRV` serves a known 10 MiB byte pattern with `Accept-Ranges: bytes` and an `ETag`.
`NORANGE_SRV` serves the same bytes but without range support. `FLAKY_SRV` fails each
chunk's first attempt with `500`, then succeeds.

| # | Command | assertion | exit |
|---|---|---|---|
| S1 | `dl http://RANGE_SRV/f -o out.bin -c 4 -quiet` | `out.bin` bytes equal the source exactly; 4 `Range` requests observed by the test server | 0 |
| S2 | `dl http://RANGE_SRV/f -o out.bin -c 1 -quiet` | correct output with a single chunk | 0 |
| S3 | `dl http://NORANGE_SRV/f -o out.bin -c 8 -quiet` | falls back to 1 stream (server saw exactly 1 request); correct output | 0 |
| S4 | `dl http://RANGE_SRV/f -o out.bin -sha256 <correct> -quiet` | `done:` message, exit 0 | 0 |
| S5 | interrupt mid-download (send the test's cancel signal after 2 of 4 chunks land) then `dl http://RANGE_SRV/f -o out.bin -resume -quiet` | second run only re-requests the missing byte ranges (assert via the test server's request log); final file correct | 0 |
| S6 | `dl http://FLAKY_SRV/f -o out.bin -c 4 -retries 2 -quiet` | succeeds after retries; final file correct | 0 |
| S7 | `dl http://RANGE_SRV/f -o out.bin -c 4 -rate 1048576 -quiet` | completes; total wall time is consistent with the rate cap (within tolerance) | 0 |
| S8 | full download then run again with the **same** command (no `-resume`) | overwrites cleanly, `.dlstate` absent both before and after | 0 |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | `dl http://RANGE_SRV/f -o out.bin -sha256 deadbeef -quiet` | `error: checksum mismatch: got <actual>, want deadbeef` (`out.bin` and `.dlstate` retained) | 1 |
| F2 | `dl http://RANGE_SRV/missing -o out.bin -quiet` | `error: HEAD http://RANGE_SRV/missing: 404 Not Found` | 1 |
| F3 | resume after the source file changed (different `ETag`) | `error: resume failed: remote file changed (etag was "abc", now "def"); rerun with -force` | 1 |
| F4 | `dl http://ALWAYS500/f -o out.bin -retries 2 -quiet` | `error: chunk 2 [2621440-5242879]: giving up after 3 attempts: 500 Internal Server Error` (first failing chunk reported; all chunk errors joined in `.dlstate`'s companion log, only one summarized to stderr) | 1 |
| F5 | `dl not-a-url -o out.bin` | `error: invalid URL "not-a-url"` | 2 |
| F6 | `dl http://RANGE_SRV/f` (no `-o`) | `error: -o is required` | 2 |
| F7 | `dl http://RANGE_SRV/f -o out.bin -c 0` | `error: -c must be >= 1` | 2 |
| F8 | `dl http://RANGE_SRV/f -o /root/nope/out.bin` | `error: open /root/nope/out.bin: ...` | 1 |

### Error catalogue

| Trigger | Format |
|---|---|
| HEAD failure | `error: HEAD %s: %s` |
| checksum | `error: checksum mismatch: got %s, want %s` |
| resume mismatch | `error: resume failed: remote file changed (etag was %q, now %q); rerun with -force` |
| chunk exhausted | `error: chunk %d [%d-%d]: giving up after %d attempts: %s` |
| bad URL | `error: invalid URL %q` |
| missing `-o` | `error: -o is required` |
| `-c` | `error: -c must be >= 1` |
| file open | `error: %v` |

## Suggested milestones

1. `internal/dl`: `probe(ctx, url) (size int64, ranges bool, etag string, err error)`.
2. Single-stream path (no ranges): `io.Pipe` + `io.MultiWriter` + SHA-256, correctness
   first, no concurrency.
3. Segmented path: chunk planning, one worker per chunk, `WriteAt`, `sync.Pool` buffers,
   retries with backoff.
4. `.dlstate` read/write; `-resume` matching logic.
5. Progress: a `progress chan progressMsg{chunk int, delta int64}`, an aggregator
   goroutine rendering the line via `\r`.
6. Rate limiting (reuse/port the token-bucket idea from Project 14).
7. `signal.NotifyContext`, save-state-and-exit on interrupt.
8. `main`; build the three `httptest` fixtures (range, no-range, flaky) for tests.

## Project layout

```
projects/18-download-manager/
  cmd/dl/main.go
  internal/dl/probe.go
  internal/dl/segmented.go     // WriterAt-based multi-chunk path
  internal/dl/singlestream.go  // io.Pipe fallback path
  internal/dl/state.go         // .dlstate
  internal/dl/progress.go
  internal/dl/ratelimit.go
  internal/dl/*_test.go
  cmd/dl/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly against the `httptest` fixtures.
- [ ] `go test -race ./...` clean.
- [ ] Peak memory does not scale with file size for either path (verify with a 200 MiB
      fixture and `-memprofile`; buffers come from the pool, not per-chunk allocation).
- [ ] Resume never re-downloads a `done` chunk (assert via request-count on the test
      server).
- [ ] `SIGINT` mid-download always leaves a `.dlstate` from which resume completes
      correctly (property test: interrupt at a random point, resume, compare bytes).
- [ ] `sync.Pool` buffers are correctly sized and never cause a data race
      (`-race` over concurrent chunk workers).

## Test requirements

- `TestProbe` — range-capable, no-range, 404.
- `TestSegmentedDownload` — S1–S2, correct byte-for-byte output (hash comparison against
  the known source).
- `TestFallbackSingleStream` — S3, exactly one request observed.
- `TestResume` — S5, F3.
- `TestRetryBackoff` — S6, F4.
- `TestChecksum` — S4, F1.
- `TestRateLimit` — S7 timing bound.
- `TestInterruptSaveState` — property test: interrupt at byte offset `k` for several `k`,
  resume, compare full output to source.
- `BenchmarkSegmentedThroughput` — `b.Run` over `-c` 1/4/16.
- `ExampleDownload`.

## Stretch goals

- Mirror lists: try several URLs for the same content, race the fastest, cancel the rest.
- Adaptive chunk count based on measured per-chunk throughput.
- A `dl batch urls.txt` mode reusing Project 14's worker-pool ideas.
- Bandwidth-fair scheduling across multiple simultaneous downloads in one process.
- A resumable **upload** counterpart using chunked `PUT`.

## Self-check questions

1. `os.File` implements `io.WriterAt`. Two goroutines call `WriteAt` on the *same* file at
   non-overlapping offsets concurrently — is that safe? What if the offsets overlap?
2. Your single-stream path uses `io.Pipe`. What happens if the goroutine reading from
   `pipeReader` stops (e.g. disk full) while another goroutine is still writing to
   `pipeWriter`? Which call blocks, and how do you unblock it on cancellation?
3. `sync.Pool` buffers "may be garbage collected at any time" between `Put` and `Get`. Why
   is that acceptable for this use case, and what would be unsafe about putting something
   *stateful* (not just reusable bytes) in the pool?
4. Chunk 2 fails after all retries while chunks 1, 3, 4 succeed. What does your code do
   with the *other* workers — do they keep running, and does `errors.Join` end up with 1
   error or does the whole download abort immediately? Which did you choose and why?
5. You resume and the server's `ETag` differs. Why is re-downloading anyway (ignoring the
   mismatch) actively dangerous for a segmented download specifically (think about what
   "chunk 2 of the old file" plus "chunk 3 of the new file" produces)?
6. Your progress aggregator reads from a channel written by up to `-c` workers. Could two
   workers' progress updates for the *same* chunk race each other into a wrong percentage?
   How does your design prevent (or not need to prevent) that?
7. `Content-Range: bytes 0-2621439/10485760` on a `206` — what three numbers does this
   give you, and which do you double-check against what you asked for in the `Range`
   header?
