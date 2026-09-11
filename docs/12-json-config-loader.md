# Project 12 — JSON Config Loader + Validator

| | |
|---|---|
| **Difficulty** | 3 / 5 |
| **Estimated time** | 8–11 hours |
| **Prerequisites** | [05](05-grade-book-csv-report.md), [07](07-todo-cli.md) |
| **Builds toward** | [20 – REST API](20-rest-api.md), [26 – Reverse Proxy](26-reverse-proxy.md), [29 – Task Scheduler](29-task-scheduler.md) |

## Why this project

Every real service has a config loader, and it's where `encoding/json` gets interesting:
custom scalar types, `json.RawMessage` for polymorphic sections, `Decoder` options,
collecting **all** validation errors instead of failing on the first, layering
defaults + environment overrides, and a pinch of `reflect` to drive it from struct tags.
The `Config` type and loader you build here are reused by Projects 20, 26 and 29.

## Go concepts you MUST use

- [ ] `encoding/json`: struct tags (`json:"name,omitempty"`, `json:"-"`), embedded
      structs, `json.MarshalIndent`
- [ ] custom scalar types with `UnmarshalJSON`/`MarshalJSON` **and**
      `encoding.TextMarshaler`/`TextUnmarshaler` (`Duration`, `Bytes`, `LogLevel`)
- [ ] `json.RawMessage` — defer decoding a polymorphic `rate_limits` array, then dispatch
      on a discriminator field
- [ ] `json.Decoder` with `DisallowUnknownFields()` (strict mode) and `UseNumber()`;
      streaming a file that may contain **one object or NDJSON**
- [ ] `errors.Join` — return every validation failure at once
- [ ] the **functional-options** pattern: `Load(path string, opts ...Option)`
- [ ] `reflect` — walk `env:"..."` tags to apply environment overrides; walk
      `validate:"..."` tags to run constraint checks; `reflect.DeepEqual` in tests
- [ ] `encoding/xml` — `-format xml` loads the same `Config` from XML

## Background

**Layering** (each step overrides the previous):

1. **Zero value** of `Config`.
2. **Defaults** (unless `WithDefaults(false)`): sensible values for unset fields.
3. **File** (`config.json` or `config.xml`).
4. **Environment** (only with `WithEnv(getenv)`): any field with an `env` tag whose
   variable is set and non-empty.

Then **validate** and return `errors.Join(all failures...)`.

**Polymorphic section.** `rate_limits` is a JSON array where each element has a `type`:

```json
{ "type": "token_bucket", "key": "ip", "rate": 10.0, "burst": 20 }
{ "type": "fixed_window", "key": "user", "limit": 100, "window": "1m" }
```

Decode the array as `[]json.RawMessage`, peek `{"type": "..."}` in each, then unmarshal
into the concrete rule struct.

## Requirements

### The `Config` type (`internal/config`)

```go
type Config struct {
    Server     ServerConfig     `json:"server" xml:"server"`
    Database   DatabaseConfig   `json:"database" xml:"database"`
    Logging    LoggingConfig    `json:"logging" xml:"logging"`
    Features   map[string]bool  `json:"features,omitempty" xml:"-"`
    RateLimits []RateLimitRule  `json:"rate_limits,omitempty" xml:"-"` // via RawMessage
}

type ServerConfig struct {
    Host         string     `json:"host"          env:"APP_SERVER_HOST" validate:"nonempty"`
    Port         int        `json:"port"          env:"APP_SERVER_PORT" validate:"min=1,max=65535"`
    ReadTimeout  Duration   `json:"read_timeout"  env:"APP_SERVER_READ_TIMEOUT" validate:"positive"`
    WriteTimeout Duration   `json:"write_timeout" validate:"positive"`
    MaxBodyBytes Bytes      `json:"max_body_bytes" validate:"min=1"`
    TLS          *TLSConfig `json:"tls,omitempty"`
}
type TLSConfig struct {
    CertFile string `json:"cert_file" validate:"nonempty"`
    KeyFile  string `json:"key_file"  validate:"nonempty"`
}
type DatabaseConfig struct {
    DSN             string   `json:"dsn"       env:"APP_DB_DSN" validate:"nonempty"`
    PoolSize        int      `json:"pool_size" env:"APP_DB_POOL" validate:"min=1,max=1000"`
    ConnMaxLifetime Duration `json:"conn_max_lifetime" validate:"nonneg"`
}
type LoggingConfig struct {
    Level   LogLevel `json:"level" env:"APP_LOG_LEVEL" validate:"nonempty"`
    Outputs []string `json:"outputs" validate:"nonempty"`  // each ∈ {stdout,stderr,file}
    File    string   `json:"file,omitempty"`               // required iff Outputs contains "file"
}

type Duration time.Duration   // JSON: "30s" or a number of seconds
type Bytes    int64           // JSON: "10MB"/"1KiB" or a number of bytes
type LogLevel int             // debug < info < warn < error; Text: the name

type RateLimitRule interface{ isRule(); Key() string }
type TokenBucketRule struct{ K string; Rate float64; Burst int }
type FixedWindowRule  struct{ K string; Limit int; Window Duration }
```

**Options:**
`WithDefaults(bool)` · `WithEnv(getenv func(string) string)` · `WithStrict()`
(sets `DisallowUnknownFields`) · `WithFormat(f Format)` (`FormatJSON` default, `FormatXML`).

**Entry point:** `Load(path string, opts ...Option) (*Config, error)`. The error, if any,
is either an I/O / parse error **or** a joined `*ValidationError` wrapping many
`*FieldError{Path, Message string}`.

### Defaults

| Field | Default |
|---|---|
| `server.host` | `"0.0.0.0"` |
| `server.port` | `8080` |
| `server.read_timeout` / `write_timeout` | `15s` |
| `server.max_body_bytes` | `1MB` (1048576) |
| `database.pool_size` | `10` |
| `logging.level` | `info` |
| `logging.outputs` | `["stdout"]` |

### Cross-field validation rules (beyond the tag constraints)

1. `logging.outputs` may only contain `stdout`, `stderr`, `file`.
2. if `logging.outputs` contains `file`, `logging.file` must be non-empty.
3. `server.tls`, if present, needs **both** `cert_file` and `key_file`.
4. every `features` key must be non-empty and match `^[a-z][a-z0-9_]*$`.
5. `rate_limits[i].key` must be non-empty; `token_bucket.rate > 0` and `burst >= 1`;
   `fixed_window.limit >= 1` and `window > 0`.
6. unknown `rate_limits[i].type` → `unsupported rate limit type "..."`.

### The CLI (`cfgcheck`)

| Command | Behaviour |
|---|---|
| `cfgcheck FILE` | load + validate; print `OK` (exit 0) or `config is invalid:` + a sorted bullet list of `path: message` (exit 1) |
| `cfgcheck -format xml FILE` | same, XML input |
| `cfgcheck -strict FILE` | unknown JSON fields become errors |
| `cfgcheck -env FILE` | apply real `os.Getenv` overrides before validating |
| `cfgcheck -dump FILE` | print the fully-resolved config as canonical indented JSON, exit 0 (still 1 if invalid, after printing) |
| `cfgcheck -schema` | print every field: `path  type  default  env-var  constraints` (from reflection), exit 0 |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | valid (or `-schema`) |
| `1` | invalid config, or a parse/IO error |
| `2` | usage — no FILE where required, unknown flag, unknown `-format` |

### Case specification — SUCCESS

Fixtures in `testdata/`. `valid_min.json` sets only `database.dsn` (everything else via
defaults).

| # | Command | key output | exit |
|---|---|---|---|
| S1 | `cfgcheck testdata/valid_min.json` | `OK` | 0 |
| S2 | `cfgcheck -dump testdata/valid_min.json` | JSON with `"port": 8080`, `"read_timeout": "15s"`, `"level": "info"`, `"outputs": ["stdout"]` | 0 |
| S3 | `cfgcheck testdata/valid_full.json` (all sections, TLS, 2 rate limits) | `OK` | 0 |
| S4 | `cfgcheck -format xml testdata/valid_min.xml` | `OK` | 0 |
| S5 | `APP_SERVER_PORT=9090 cfgcheck -env -dump testdata/valid_min.json` | dump shows `"port": 9090` | 0 |
| S6 | `cfgcheck -schema` | a table incl. `server.port  int  8080  APP_SERVER_PORT  min=1,max=65535` | 0 |
| S7 | `cfgcheck testdata/rate_limits.json` | `OK`; `-dump` round-trips both rule types | 0 |
| S8 | `cfgcheck testdata/timeout_as_number.json` (`"read_timeout": 20`) | `OK`; dump shows `"20s"` | 0 |
| S9 | `cfgcheck testdata/bytes_as_string.json` (`"max_body_bytes": "2MB"`) | `OK`; dump shows `2097152` | 0 |

### Case specification — FAILURE

| # | Command | stderr (bullets, sorted by path) | exit |
|---|---|---|---|
| F1 | `cfgcheck testdata/bad_port.json` (`"port": 70000`) | `- server.port: must be between 1 and 65535, got 70000` | 1 |
| F2 | `cfgcheck testdata/many_errors.json` | multiple bullets: `database.dsn: must not be empty`, `logging.level: must be one of debug, info, warn, error`, `server.read_timeout: must be > 0` — all in one run | 1 |
| F3 | `cfgcheck testdata/file_output_no_path.json` | `- logging.file: required when outputs includes "file"` | 1 |
| F4 | `cfgcheck testdata/tls_half.json` (cert only) | `- server.tls.key_file: must not be empty` | 1 |
| F5 | `cfgcheck testdata/bad_feature_key.json` (`"Feature-1": true`) | `- features: key "Feature-1" is not a valid identifier` | 1 |
| F6 | `cfgcheck testdata/bad_rule_type.json` | `- rate_limits[0]: unsupported rate limit type "leaky_bucket"` | 1 |
| F7 | `cfgcheck testdata/bad_duration.json` (`"read_timeout": "20 seconds"`) | `error: server.read_timeout: invalid duration "20 seconds"` | 1 |
| F8 | `cfgcheck -strict testdata/unknown_field.json` | `error: json: unknown field "colour"` | 1 |
| F9 | `cfgcheck testdata/not_json.json` | `error: testdata/not_json.json: invalid JSON: <detail>` | 1 |
| F10 | `cfgcheck missing.json` | `error: open missing.json: no such file or directory` | 1 |
| F11 | `cfgcheck` | usage | 2 |
| F12 | `cfgcheck -format toml x` | `error: -format must be json or xml` | 2 |

### Error catalogue

| Trigger | Format |
|---|---|
| joined validation | `config is invalid:` then `  - %s: %s` per `FieldError`, path-sorted |
| scalar parse (Duration/Bytes/Level) | `error: %s: invalid duration %q` / `invalid size %q` / `invalid level %q` |
| strict unknown field | `error: %v` (the `json` package's message) |
| bad JSON/XML | `error: %s: invalid JSON: %v` / `invalid XML: %v` |
| file | `error: %v` |
| unsupported rule | `%s: unsupported rate limit type %q` (a `FieldError`) |

## Suggested milestones

1. Scalar types: `Duration`, `Bytes`, `LogLevel` — `UnmarshalJSON`/`MarshalJSON` +
   `Text` methods; exhaustive tests (string form, numeric form, bad form).
2. `Config` structs; `decodeJSON` / `decodeXML`; polymorphic `rate_limits` via
   `[]json.RawMessage` + discriminator.
3. `applyDefaults(*Config)`.
4. `applyEnv(*Config, getenv)` — reflect over `env` tags, parse into the field's kind.
5. `Validate(*Config) error` — reflect over `validate` tags for the simple constraints +
   the hand-written cross-field rules; collect into `errors.Join`.
6. `Load` wiring the options; `-schema` via reflection.
7. `cmd/cfgcheck`. Golden CLI test with fixtures for every S/F row.

## Project layout

```
projects/12-config/
  cmd/cfgcheck/main.go
  internal/config/config.go      // Config + section structs
  internal/config/scalars.go     // Duration, Bytes, LogLevel
  internal/config/rules.go       // RateLimitRule + concrete + RawMessage dispatch
  internal/config/load.go        // Load, options, decode json/xml, defaults
  internal/config/env.go         // reflect env overrides
  internal/config/validate.go    // reflect tag checks + cross-field rules + errors.Join
  internal/config/schema.go      // reflect -> schema table
  internal/config/*_test.go
  cmd/cfgcheck/main_test.go
  testdata/*.json testdata/*.xml
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] `Validate` on `testdata/many_errors.json` returns **all** failures in one call
      (count them in the test), sorted by path in the CLI output.
- [ ] Round-trip: `Load` → `json.MarshalIndent` → `Load` again → `reflect.DeepEqual`
      (for every valid fixture).
- [ ] `Duration`/`Bytes`/`LogLevel` each accept both their string and numeric JSON forms
      and marshal back to the string form.
- [ ] `-strict` rejects unknown fields; without it they're silently ignored (both tested).
- [ ] The env-override code reads `env` tags via reflection — no hand-written
      `if os.Getenv(...) != ""` ladder.

## Test requirements

- `TestScalars` — table per type: string, number, invalid, marshal output.
- `TestDecodePolymorphic` — token_bucket + fixed_window + unknown type.
- `TestDefaults` — a near-empty config gets every documented default.
- `TestEnvOverride` — injected `getenv` changes `port`, `dsn`, `read_timeout`; unset vars
  don't clobber file values.
- `TestValidateCollectsAll` — F2 fixture → N `FieldError`s.
- `TestCrossFieldRules` — F3, F4, F5, F6.
- `TestRoundTrip` — every valid fixture.
- `TestStrictMode` — F8 with/without `-strict`.
- `TestSchemaReflection` — schema contains the right env var + constraints for 3 fields.
- `ExampleLoad`.

## Stretch goals

- `WithFileRefs` — a string value `"@secrets/db.txt"` is replaced by that file's contents.
- Hot reload: `Watch(path, func(*Config))` polling mtime (bridge to Project 26/28).
- `${VAR}` interpolation inside string values.
- Emit a JSON Schema document from the reflected type.
- Support a third format via `encoding/gob` for a fast binary cache of the parsed config.

## Self-check questions

1. `Duration` implements both `UnmarshalJSON` and `UnmarshalText`. Which does
   `encoding/json` call for `"read_timeout": "30s"`? Which for a map key of type
   `Duration`? Why have both?
2. `rate_limits` uses `[]json.RawMessage`. Why can't you just make `RateLimitRule` an
   interface and let `json.Unmarshal` figure it out?
3. `errors.Join(a, b, c)` — what does `.Error()` look like, and how does a caller pull out
   just the `*FieldError`s? (`errors.As` in a loop? `Unwrap() []error`?)
4. Your env-override loop uses `reflect.Value.SetInt` etc. What must be true about the
   `reflect.Value` for `Set*` not to panic, and how do you get an addressable value from
   `*Config`?
5. `json.Decoder.UseNumber()` vs default — for `"port": 8080`, what type lands in an
   `any`, and why does that matter if you're decoding into `map[string]any` for `-schema`?
6. `DisallowUnknownFields` catches `"colour"` but not a misspelled nested key under a
   `map[string]bool`. Why, and what would catch the map case?
7. Defaults run before the file, env runs after. Walk through what happens to
   `server.port` when: default is 8080, the file omits it, and `APP_SERVER_PORT=0` is set.
   Is `0` "unset" or "explicitly zero"? How does your code tell?
