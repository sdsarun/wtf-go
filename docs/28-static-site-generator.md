# Project 28 — Static Site Generator CLI

| | |
|---|---|
| **Difficulty** | 5 / 5 |
| **Estimated time** | 12–16 hours |
| **Prerequisites** | [09](09-mini-grep.md), [13](13-report-engine.md), [14](14-link-checker.md) |
| **Builds toward** | [29 – Task Scheduler](29-task-scheduler.md) |

## Why this project

A static site generator forces you through the filesystem-abstraction side of Go
(`io/fs`, so "read templates from disk" and "read templates baked into the binary" are the
*same code path*), `html/template`'s escaping rules (and the deliberate, narrow way you
opt **out** of escaping for content you trust), `encoding/xml` for a real RSS feed,
concurrent page rendering, and **build tags** to compile an optional dev-preview server in
or out of the binary entirely.

Content bodies in this project are **HTML fragments you author directly** — a Markdown
parser is out of scope for the standard library and is offered as a stretch goal, not the
core.

## Go concepts you MUST use

- [ ] `io/fs` / `fs.FS` — your template renderer takes an `fs.FS`, so `embed.FS` (the
      built-in theme) and `os.DirFS` (a custom theme directory) are interchangeable
- [ ] `//go:embed` for the default theme (`embed.FS`)
- [ ] `filepath.WalkDir` to discover content files; `path` (for the **URL** paths you
      generate, which are always `/`-separated) vs `path/filepath` (for **OS** paths) —
      used correctly, not interchangeably
- [ ] `text/template` for non-HTML output (RSS, sitemap) and `html/template` for pages,
      including a deliberate `template.HTML(...)` cast for the trusted body content, with
      a code comment explaining exactly why it's safe *here* and dangerous in general
- [ ] `encoding/xml` for the RSS feed and sitemap
- [ ] a concurrent **worker pool** rendering pages in parallel (`-workers N`)
- [ ] poll-based file **watching** (`-watch`) — stat mtimes on an interval, no OS-specific
      notification API
- [ ] **build tags** — the `serve` subcommand exists only in a binary built with
      `-tags devserver` (`//go:build devserver` / a matching stub for the default build)
- [ ] `os/exec` + **filename-suffix** build constraints (`open_darwin.go`,
      `open_linux.go`) for an "open in browser" helper after `serve`

## Background

**Content file** (`content/**/*.html`), front matter delimited by `---` lines:

```
---
title: Hello, World
date: 2024-03-09
tags: go, learning
layout: post
---
<p>This is the <strong>body</strong>, verbatim HTML.</p>
```

`title` (required), `date` (required, `2006-01-02`), `tags` (optional, comma-separated),
`layout` (optional, `page` or `post`, default `page`), `draft` (optional bool, default
`false` — excluded from `build` unless `-drafts`).

**Site structure:**

```
content/            *.html with front matter (may be nested — nesting becomes URL nesting)
static/             copied verbatim to the output root
templates/           optional custom theme (else the embedded default)
out/                 build output (recreated each build)
```

**Why `template.HTML`.** `html/template` escapes everything by default *because it
doesn't know what's safe*. Your front-matter body is authored by you (or a trusted
contributor with commit access) and deliberately contains HTML — cast it once, at one
well-documented boundary (`template.HTML(page.Body)`), and nowhere else. Every *other*
field (title, tags) stays a plain `string` and gets escaped normally — prove this with a
test using a title containing `<script>`.

## Requirements

### Commands

| Command | Behaviour |
|---|---|
| `ssg build [-content D] [-templates D] [-static D] [-out D] [-base-url URL] [-workers N] [-drafts]` | full build |
| `ssg watch` (same flags) | build once, then poll every 500ms; on any content/template/static change, rebuild and print what changed |
| `ssg new SLUG -title "..." [-layout page\|post]` | scaffold `content/SLUG.html` with front matter filled in and today's date |
| `ssg serve [-addr :8000] [-out D]` | **only exists in `devserver`-tagged builds**; serves `-out` over HTTP for local preview |

Defaults: `-content content`, `-templates ""` (empty = embedded theme), `-static static`,
`-out out`, `-workers NumCPU`, `-base-url http://localhost/`.

### Build output

- `out/index.html` — list of non-draft pages/posts, newest `date` first.
- `out/<slug>/index.html` per content file (pretty URLs; nested content paths preserve
  nesting: `content/blog/hello.html` → `out/blog/hello/index.html`).
- `out/feed.xml` — RSS 2.0, one `<item>` per **post**-layout page (not `page`-layout),
  newest first, absolute URLs built from `-base-url`.
- `out/sitemap.xml` — one `<url><loc>` per output page, absolute URLs.
- everything under `static/` copied byte-for-byte to the matching path under `out/`.

### RSS item fields

`<title>`, `<link>` (absolute), `<pubDate>` (RFC 1123Z, from the front-matter `date`),
`<description>` (the rendered HTML body, CDATA-wrapped or entity-escaped — your choice,
document it and be consistent).

### Exit codes

| Code | Meaning |
|---|---|
| `0` | build succeeded (`watch` exits `0` only on a clean `Ctrl-C`) |
| `1` | a content file failed to parse (front matter or bad date), a template error, an unwritable `-out` |
| `2` | usage — unknown subcommand/flag, `serve` used in a non-`devserver` build |

### Case specification — SUCCESS

Fixture `testdata/site/` with 2 posts, 1 page, 1 draft, one static asset.

| # | Command | Assertion |
|---|---|---|
| S1 | `ssg build -content testdata/site/content -static testdata/site/static -out /tmp/out` | `out/index.html`, both post directories, the page directory, `feed.xml`, `sitemap.xml`, and the copied static file all exist |
| S2 | inspect `out/index.html` | draft **excluded**; the 2 posts appear newest-first |
| S3 | `ssg build ... -drafts` | draft **included** |
| S4 | inspect a post whose title is `Hello & <Welcome>` | the rendered `<title>` in HTML is escaped (`Hello &amp; &lt;Welcome&gt;`); the **body**, which contains real `<strong>` tags, is **not** escaped |
| S5 | inspect `feed.xml` | well-formed XML (parses with `encoding/xml`); 2 `<item>`s (posts only, page excluded); dates in RFC 1123Z |
| S6 | inspect `sitemap.xml` | one `<url>` per generated page, absolute URLs starting with `-base-url` |
| S7 | `ssg build -templates testdata/customtheme -out /tmp/out2` | output reflects the custom theme's markup, not the embedded default |
| S8 | `ssg build -out /tmp/out -workers 1` vs `-workers 8` | **byte-identical** output trees (rendering is embarrassingly parallel and order-independent — verify with a checksum walk) |
| S9 | `ssg new my-post -title "My Post"` | creates `content/my-post.html` with front matter incl. today's date; running it again with the same slug → error, doesn't overwrite |
| S10 | `ssg watch`, then touch a content file, wait for the poll interval | the tool detects the change, rebuilds, prints something like `rebuilt: content/hello.html changed` |
| S11 | `ssg serve -out /tmp/out` (built **with** `-tags devserver`) | serves `index.html` at `/`, static assets at their paths, `200` |
| S12 | `ssg serve` (built **without** the tag) | `error: serve is not available in this build (built without -tags devserver)`, exit `2` |

### Case specification — FAILURE

| # | Command | stderr | exit |
|---|---|---|---|
| F1 | a content file missing the closing `---` | `error: content/x.html: unterminated front matter` | 1 |
| F2 | a content file missing `title` | `error: content/x.html: front matter: title is required` | 1 |
| F3 | a content file with `date: not-a-date` | `error: content/x.html: front matter: invalid date "not-a-date"` | 1 |
| F4 | `-layout` value on `ssg new` other than `page`/`post` | `error: -layout must be page or post` | 2 |
| F5 | `ssg new existing-slug` (file already present) | `error: content/existing-slug.html already exists` | 1 |
| F6 | `-out` points at an unwritable path | `error: <path>: <os error>` | 1 |
| F7 | a template references a field that doesn't exist (typo) | `error: template: page: ...` (the `html/template` execution error, surfaced) | 1 |
| F8 | `ssg` (no subcommand) | usage | 2 |
| F9 | `ssg frob` | `error: unknown command "frob"` + usage | 2 |

### Error catalogue

| Trigger | Format |
|---|---|
| unterminated front matter | `error: %s: unterminated front matter` |
| missing title | `error: %s: front matter: title is required` |
| bad date | `error: %s: front matter: invalid date %q` |
| duplicate `new` slug | `error: %s already exists` |
| bad layout flag | `error: -layout must be page or post` |
| template exec | `error: %v` (pass through `html/template`'s own error) |
| unknown command | `error: unknown command %q` |

## Suggested milestones

1. Front-matter parser (pure, over an `io.Reader`) → `(FrontMatter, body string, error)`.
   Exhaustive tests incl. F1–F3.
2. Content discovery: `filepath.WalkDir` over `-content`, building `[]Page` (single pass,
   not concurrent yet) — this is also what `index.html`/RSS/sitemap need, so build it
   first and get it right before parallelizing rendering.
3. Template renderer taking an `fs.FS` (works identically for `embed.FS` and
   `os.DirFS`); the embedded default theme.
4. `template.HTML` boundary for the body; the escaping test (S4).
5. `build`: static copy, page rendering (single-threaded first, get output correct).
6. Parallelize rendering with a worker pool; prove determinism (S8).
7. RSS (`encoding/xml`) and sitemap.
8. `new` scaffolding.
9. `watch` (poll mtimes of every discovered file + the template dir).
10. `serve` behind the `devserver` build tag; the OS-specific "open in browser" helper
    behind filename-suffix build constraints.

## Project layout

```
projects/28-ssg/
  cmd/ssg/main.go
  cmd/ssg/serve.go            // //go:build devserver
  cmd/ssg/serve_stub.go       // //go:build !devserver
  cmd/ssg/open_darwin.go
  cmd/ssg/open_linux.go
  cmd/ssg/open_other.go       // //go:build !darwin && !linux
  internal/ssg/frontmatter.go
  internal/ssg/discover.go    // WalkDir -> []Page
  internal/ssg/render.go      // fs.FS-based renderer, template.HTML boundary
  internal/ssg/build.go       // orchestration + worker pool
  internal/ssg/feed.go        // RSS + sitemap
  internal/ssg/watch.go
  internal/ssg/theme/         // //go:embed default templates
  internal/ssg/*_test.go
  cmd/ssg/main_test.go
  testdata/site/…
  testdata/customtheme/…
  README.md
  Makefile                    // include a `devserver` build target using -tags
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] S4's escaping test passes: a malicious/careless title never produces raw HTML, the
      body's real HTML always renders as HTML.
- [ ] S8's determinism check passes at `-workers 1, 4, 16`.
- [ ] `go build ./...` (no tags) and `go build -tags devserver ./...` **both** succeed;
      only the tagged build's binary responds to `serve`.
- [ ] `feed.xml` and `sitemap.xml` parse cleanly with `encoding/xml.Unmarshal` in a test
      (not just "looks right" — actually parse your own output).
- [ ] `go test -race ./...` clean for the concurrent build path.

## Test requirements

- `TestParseFrontMatter` — F1–F3 plus every optional field.
- `TestDiscoverPages` — nesting, draft flagging, sort order.
- `TestRenderFSAbstraction` — the exact same renderer function against an `embed.FS` and
  an `os.DirFS` pointed at equivalent fixture content, asserting identical output.
- `TestHTMLEscapingBoundary` — S4.
- `TestBuildDeterminism` — S8 (checksum every output file, compare across worker counts).
- `TestRSSValid`, `TestSitemapValid` — parse your own output back.
- `TestNewScaffold` — S9, F5.
- `TestWatchDetectsChange` — S10 (short poll interval in tests).
- `TestServeBuildTag` — this one is special: it's really two `go build` invocations
  compared (document how you test a build-tag difference — e.g. a small shell/`go test`
  harness that builds both and checks binary behaviour, or a unit test that only runs
  under `devserver` and a companion asserting the stub's error under the default build).
- `ExampleRender`.

## Stretch goals

- A tiny Markdown-to-HTML pass (bold/italic/links/headers only — hand-rolled, not a full
  CommonMark implementation) as an alternative content format.
- Incremental builds: skip re-rendering pages whose source + templates haven't changed
  since the last build (mtime-based).
- Syntax highlighting for `<pre><code>` blocks (hand-rolled, small language subset).
- `-minify` (strip redundant whitespace from output HTML — careful with `<pre>`).
- Replace poll-based watching with a documented note on what `fsnotify` (third-party)
  would buy you and why you didn't reach for it.

## Self-check questions

1. Your renderer's signature takes `fs.FS`, not a directory path. What concretely lets you
   pass `embed.FS` and `os.DirFS("templates")` to the *same* function, and what stdlib
   interface makes that possible?
2. `template.HTML(page.Body)` — if `page.Body` could ever come from an **untrusted**
   source (say, a public comment form) instead of a file only you commit, what attack
   becomes possible, and why is "it's fine because it's just my own blog" the actual load-
   bearing assumption here (not something the type system protects you from)?
3. `path.Join` vs `filepath.Join` — your RSS `<link>` URLs and your on-disk output paths
   both involve joining segments. Which function for which, and what breaks on Windows if
   you mix them up?
4. Your worker pool renders pages concurrently, but `index.html`/`feed.xml`/`sitemap.xml`
   all need the **full** list of pages. Where in your pipeline does that full list get
   built, and why must it happen before the parallel phase starts rather than being
   assembled *by* the parallel workers?
5. `//go:build devserver` on one file and `//go:build !devserver` on its stub — what
   happens if you typo one of these tags such that *neither* file is included in a given
   build? What error do you get, and how early?
6. `filename_darwin.go` (no `//go:build` line) vs an explicit `//go:build darwin` comment
   — are these equivalent? When would you need the explicit comment even on a
   correctly-suffixed file (hint: combining OS **and** another condition)?
7. Poll-based watching checks mtimes every 500ms. What kind of file change could this
   *miss* entirely (not just "notice late"), and why does a real editor's "atomic save"
   (write a temp file, rename over the original) actually make polling *more* reliable
   here, not less?
