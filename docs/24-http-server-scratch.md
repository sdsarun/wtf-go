# Project 24 — HTTP/1.1 Server From Scratch (on raw TCP)

| | |
|---|---|
| **Difficulty** | 5 / 5 |
| **Estimated time** | 14–18 hours |
| **Prerequisites** | [09](09-mini-grep.md), [20](20-rest-api.md), [23](23-tcp-chat-server.md) |
| **Builds toward** | [26 – Reverse Proxy](26-reverse-proxy.md), [28 – Static Site Generator](28-static-site-generator.md) |

## Why this project

You've used `net/http` since Project 20. Now build the part it was hiding: parse the
request line and headers **by hand** off a `net.Conn`, decode `Content-Length` and
**chunked** request bodies, handle keep-alive correctly, write chunked *response* bodies
for handlers that don't know their length upfront, and serve a real templated site with
`html/template` on top of your own `Handler` interface. This is the deepest project in the
curriculum so far — budget real time for it.

## Go concepts you MUST use

- [ ] `net.Listener` / `net.Conn` directly — no `net/http` server type anywhere in the
      server code (using `net/http`'s **client**, or `httptest`, to test it is fine and
      expected)
- [ ] hand-parsing the request line and headers with `bufio.Reader` (`ReadString`,
      `ReadLine`) — optionally `net/textproto.Reader.ReadMIMEHeader` for header parsing,
      your choice, document it
- [ ] `Content-Length` bodies via `io.LimitReader` + `io.ReadFull`; **chunked**
      `Transfer-Encoding` decoded by hand (hex size, CRLF, data, CRLF, repeat, terminal
      `0\r\n\r\n`)
- [ ] chunked **response** encoding for handlers that write without a known length
- [ ] `Connection: keep-alive` / `close` handling; per-connection read/idle deadlines
- [ ] `net.Pipe()` for fast, allocation-free tests of your protocol code with no real
      socket
- [ ] `html/template` — `{{define}}`/`{{template}}`/`{{block}}` composition, a
      `template.FuncMap`, `ParseFS`
- [ ] `mime.TypeByExtension` for static file `Content-Type`
- [ ] your **own** `Handler`/`HandlerFunc`/middleware types (conceptually parallel to
      `net/http`'s, but yours)

## Background

**Request line:** `METHOD SP request-target SP HTTP-VERSION CRLF` — exactly two single
spaces. `HTTP-VERSION` is `HTTP/1.1` or `HTTP/1.0`.

**Headers:** `Name: value CRLF`, repeated, terminated by a bare `CRLF`. Names are
case-insensitive; canonicalize them (`Content-Length` however written). Obsolete line
folding (a continuation line starting with whitespace) is **not** supported — treat it as
malformed.

**Body framing** (in this priority order): `Transfer-Encoding: chunked` wins if present;
else `Content-Length: N` reads exactly `N` bytes; else the request has **no body**
(`GET`/`HEAD`/`DELETE` never do; `POST`/`PUT`/`PATCH` without either header → `400`).

**Chunk format:**
```
<hex-size>\r\n
<size bytes of data>\r\n
... (repeat) ...
0\r\n
\r\n
```
(No trailers required to support; reject a request that never terminates within your max
body size.)

**Keep-alive:** HTTP/1.1 defaults to keep-alive unless `Connection: close` is sent;
HTTP/1.0 defaults to close unless `Connection: keep-alive` is sent. After each response,
if keep-alive, loop reading the next request on the **same** connection (with an idle
read deadline); else close.

**Response framing:** if the handler sets `Content-Length` before the first `Write`, write
exactly that many bytes verbatim (extra writes are an error; fewer are a caller bug —
document your behaviour, e.g. pad or error). If `Content-Length` was **not** set, switch
to `Transfer-Encoding: chunked` for the response automatically, framing each `Write` call
as one chunk, with a final `0\r\n\r\n` on `Close`/handler return.

## Requirements

### Your API

```go
type Header = textproto.MIMEHeader // or your own map[string][]string — document your choice
type Request struct {
    Method, Target, Proto string
    Header                Header
    Body                  io.Reader
    RemoteAddr             string
}
type ResponseWriter interface {
    Header() Header
    WriteHeader(status int)
    Write(p []byte) (int, error)
}
type Handler interface { ServeHTTP(ResponseWriter, *Request) }
type HandlerFunc func(ResponseWriter, *Request)
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) { f(w, r) }

func Serve(ln net.Listener, h Handler) error
func ListenAndServe(addr string, h Handler) error
```

Plus a small `Router` (exact-path map keyed by `"METHOD path"`, and a single dynamic route
`"GET /post/"` prefix match that hands the remainder to the handler as `r.Target`'s
suffix — no full pattern language, this is intentionally simpler than Project 20's).

### Parsing rules → status codes

| Condition | Response |
|---|---|
| request line has ≠ 3 space-separated tokens | `400 Bad Request` |
| method contains a lowercase letter or non-token character | `400 Bad Request` |
| HTTP-VERSION is not `HTTP/1.1` or `HTTP/1.0` | `400 Bad Request` |
| a header line has no `:` | `400 Bad Request` |
| a header line starts with whitespace (obsolete folding) | `400 Bad Request` |
| more than 100 headers, or a header line > 8 KiB | `431 Request Header Fields Too Large` |
| `Content-Length` is not a valid non-negative integer | `400 Bad Request` |
| body-bearing method (`POST`/`PUT`/`PATCH`) with neither `Content-Length` nor chunked `Transfer-Encoding` | `400 Bad Request` |
| a chunk size line is not valid hex | `400 Bad Request` |
| connection idle > `-idle-timeout` waiting for a request line | connection closed, no response (not an error — this is normal keep-alive expiry) |
| reading the request line/headers takes > `-header-timeout` | connection closed |
| no route matches | `404 Not Found` |
| route matches, wrong method | `405 Method Not Allowed` with `Allow` |
| `Host` header absent on an `HTTP/1.1` request | `400 Bad Request` |
| static file path contains `..` after cleaning | `400 Bad Request` (never touch the filesystem with it) |

### The demo site

Routes: `GET /` (index, lists post titles), `GET /post/{slug}` (renders
`templates/post.html` with that post's content, `404` if unknown), `GET /static/*`
(serves files from an embedded/`static/` directory with correct `Content-Type` via
`mime.TypeByExtension`), `GET /healthz` (`200 ok`).

Templates use a shared `{{define "layout"}}...{{block "content" .}}{{end}}...{{end}}`
base with per-page content blocks and a `FuncMap` providing at least one helper (e.g.
`upper`, or a date formatter).

### Exit codes

| Code | Meaning |
|---|---|
| `0` | clean shutdown |
| `1` | listener bind failure |
| `2` | bad flags |

### Case specification — SUCCESS (raw bytes over `net.Pipe`, and a subset over a real listener)

| # | Request bytes (abbreviated, `\r\n` implied) | Response assertion |
|---|---|---|
| S1 | `GET / HTTP/1.1` + `Host: x` | `200`, body contains the index content, `Connection` absent or `keep-alive` |
| S2 | `GET /post/hello-world HTTP/1.1` + `Host: x` | `200`, body contains that post's content (templated) |
| S3 | `GET /post/nope HTTP/1.1` + `Host: x` | `404` |
| S4 | `GET /static/site.css HTTP/1.1` + `Host: x` | `200`, `Content-Type: text/css; charset=utf-8` |
| S5 | `GET /static/../../etc/passwd HTTP/1.1` + `Host: x` | `400` (path traversal rejected before any file access) |
| S6 | `POST /echo HTTP/1.1` + `Host: x` + `Content-Length: 5` + body `hello` (a test-only echo handler) | `200`, body `hello` |
| S7 | same as S6 but chunked: `Transfer-Encoding: chunked` + `2\r\nhe\r\n3\r\nllo\r\n0\r\n\r\n` | `200`, body `hello` |
| S8 | two pipelined requests on the same connection, second with `Connection: close` | both answered correctly, in order, connection closed after the second |
| S9 | `HTTP/1.0` request with no `Connection` header | connection closed after the response (HTTP/1.0 default) |
| S10 | `HTTP/1.0` request with `Connection: keep-alive` | connection stays open for a second request |
| S11 | a handler that never sets `Content-Length` and writes in 3 separate `Write` calls | response uses `Transfer-Encoding: chunked` with exactly 3 chunks matching each `Write`'s bytes |
| S12 | `GET /nope HTTP/1.1` + `Host: x` | `404` |
| S13 | `POST /echo HTTP/1.1` + `Host: x` (no route for POST /echo but GET exists — use a route that only supports GET for this case) → `PUT /` | `405`, `Allow: GET` |
| S14 | idle connection, no request sent for the configured idle timeout (short in tests) | connection closed by the server, no response bytes sent |
| S15 | `GET / HTTP/1.1` with no `Host` header | `400` |

### Case specification — FAILURE (malformed input)

| # | Bytes sent | Response |
|---|---|---|
| F1 | `GET /` (no version token) | `400` |
| F2 | `get / HTTP/1.1` (lowercase method) | `400` |
| F3 | `GET / HTTP/2.0` | `400` |
| F4 | a header line `X-Bad-Header-No-Colon` | `400` |
| F5 | a header line starting with a space (folded continuation) | `400` |
| F6 | `Content-Length: -5` | `400` |
| F7 | `Content-Length: abc` | `400` |
| F8 | `POST /echo HTTP/1.1` + `Host: x` + no `Content-Length`/chunked | `400` |
| F9 | a chunk size line `zz\r\n` | `400` |
| F10 | 150 header lines | `431` |
| F11 | one header line of 9000 bytes | `431` |

### Error catalogue

Bodies for error responses are a plain text line: `<status> <reason phrase>\n` (e.g.
`400 Bad Request\n`) — keep it simple and consistent; this is not the templated site.

## Suggested milestones

1. Request-line + header parser off a `bufio.Reader`, returning `(*Request, error)` —
   pure, tested with `net.Pipe` from the start (write bytes on one end, parse on the
   other, no real socket needed).
2. `Content-Length` body reading.
3. Chunked body **decoding**.
4. `ResponseWriter` — buffered-header, `Content-Length` path first (simplest).
5. Chunked **response** encoding for the no-`Content-Length` path.
6. Keep-alive loop per connection + deadlines; `405`/`404`/error-status plumbing.
7. `Router`.
8. `html/template` site + `mime`-typed static file serving with traversal protection.
9. `Serve`/`ListenAndServe`, flags, graceful shutdown (same pattern as Project 23).
10. Full protocol test suite over `net.Pipe`; a handful of end-to-end tests over a real
    `net.Listen("tcp", "127.0.0.1:0")`.

## Project layout

```
projects/24-httpd/
  cmd/httpd/main.go
  internal/httpd/request.go     // parser: request line, headers, body framing
  internal/httpd/chunked.go     // chunked decode + encode
  internal/httpd/response.go    // ResponseWriter impl
  internal/httpd/conn.go        // per-connection loop, keep-alive, deadlines
  internal/httpd/server.go      // Serve/ListenAndServe, shutdown
  internal/httpd/router.go
  internal/site/handlers.go     // the demo site's handlers
  internal/site/templates/      // layout.html, index.html, post.html
  internal/site/static/         // site.css etc.
  internal/httpd/*_test.go
  cmd/httpd/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly, the S-rows primarily via `net.Pipe`, with S1,
      S3, S4, S8 also verified over a real `net.Listener` using `net/http.Client` (proves
      you're byte-compatible with a real HTTP client, not just your own test harness).
- [ ] Chunked request decoding and chunked response encoding each have a round-trip test:
      encode then decode gets back the original bytes.
- [ ] Path-traversal protection is proven with a fuzz-ish table (`..`, `%2e%2e`-if-you-
      decode-percent-escapes, absolute paths, backslashes on the test host if relevant).
- [ ] `go test -race ./...` clean under concurrent connections.
- [ ] No goroutine or file-descriptor leak across 1000 connect/request/close cycles.

## Test requirements

- `TestParseRequestLine`, `TestParseHeaders` — every F-row.
- `TestContentLengthBody`, `TestChunkedDecode`, `FuzzChunkedDecode` (must never hang or
  panic on malformed chunk data).
- `TestChunkedEncodeRoundTrip`.
- `TestKeepAlive`, `TestConnectionClose`, `TestHTTP10Defaults`, `TestPipelining`.
- `TestRouter` — exact match, prefix route, 404, 405 + `Allow`.
- `TestStaticFileServing`, `TestPathTraversalRejected`.
- `TestTemplateRendering` — index and post pages contain expected fragments.
- `TestIdleTimeout`, `TestHeaderTimeout`, `TestHeaderLimits`.
- `TestEndToEndOverRealSocket` — S1, S3, S4, S8 via `net/http.Client`.
- `TestNoConnectionLeak`.
- `ExampleServe`.

## Stretch goals

- `Expect: 100-continue` support.
- Range requests for static files (`Range`/`Content-Range`, `206`).
- `If-Modified-Since` / `Last-Modified` (`304`).
- Gzip response compression (`compress/gzip`, content negotiation via `Accept-Encoding`).
- TLS (`crypto/tls`) — same server logic, wrapped listener.
- Compare your implementation's behaviour against `net/http`'s for the same requests
  using `httptest` as an oracle, and document any deliberate deviations.

## Self-check questions

1. Why parse `Content-Length` and reject `Transfer-Encoding: chunked` **combined with** a
   `Content-Length` header as malformed (both present is actually a known HTTP request
   -smuggling vector in real servers) — what could an attacker exploit if your parser
   picked one and a downstream proxy picked the other?
2. Your chunked decoder must never allocate memory proportional to an attacker-claimed
   chunk size before validating it. Where's the check, and what's the failure mode if you
   skip it (tie this back to Project 10's binary-parsing lesson)?
3. `net.Pipe()` gives you two directly-connected `net.Conn`s with no OS socket. Why does
   that make your parser tests both **faster** and **more deterministic** than using a
   real `net.Listener`, and what does it *not* test that a real socket would (hint: think
   about partial reads / TCP segmentation)?
4. A handler writes 10 bytes, calls `Write` again with 10 more, and never sets
   `Content-Length`. Walk through exactly what bytes hit the wire for each `Write` call
   under your chunked response encoder.
5. HTTP/1.0 without `Connection: keep-alive` vs HTTP/1.1 without `Connection: close` — you
   default to opposite behaviours. Where in your code does that one `if` live, and why do
   both versions exist historically?
6. Two pipelined requests arrive back-to-back in a single TCP segment. Your
   `bufio.Reader` reads more than one request's worth of bytes in one `Read` syscall. How
   does your per-connection loop make sure the *second* request's bytes aren't lost or
   misparsed as part of the first request's body?
7. Why is `path.Clean` alone insufficient to stop `../../../etc/passwd`-style traversal
   if you then naively `filepath.Join(staticDir, cleaned)` — walk through a concrete input
   that still escapes, and state the actual fix you used.
