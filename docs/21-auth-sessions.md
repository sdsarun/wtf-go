# Project 21 — Auth & Sessions

| | |
|---|---|
| **Difficulty** | 4 / 5 |
| **Estimated time** | 9–13 hours |
| **Prerequisites** | [20 – REST API](20-rest-api.md) |
| **Builds toward** | [26 – Reverse Proxy](26-reverse-proxy.md) |

## Why this project

Bolt real authentication onto Project 20's API: password hashing that isn't a rookie
mistake, HMAC-signed session cookies you verify in **constant time**, and CSRF protection
for the cookie-based flow. Every crypto primitive here is stdlib — no third-party auth
library, so you see exactly what one would be doing for you.

## Go concepts you MUST use

- [ ] `crypto/rand` — password salts, CSRF tokens, the server's HMAC secret
- [ ] `crypto/pbkdf2` (stdlib since Go 1.24) — password hashing with a fixed iteration
      count and per-user salt
- [ ] `crypto/sha256` (as PBKDF2's hash and HMAC's hash)
- [ ] `crypto/hmac` — sign and verify the session cookie payload
- [ ] `crypto/subtle.ConstantTimeCompare` — **every** secret comparison (signature check,
      CSRF token check, password-hash-adjacent comparisons) goes through it, never `==`
      or `bytes.Equal` on secret material
- [ ] `encoding/base64` (`URLEncoding`, unpadded via `RawURLEncoding`) for cookie/token
      encoding
- [ ] `net/http.Cookie` — `HttpOnly`, `Secure`, `SameSite`, `Path`, `MaxAge`/`Expires`
- [ ] `context` — the authenticated user flows through middleware → handlers via a typed
      context key

## Background

**Password storage.** Never store or compare plaintext. On register:
`salt := random 16 bytes`; `hash := pbkdf2.Key(sha256.New, password, salt, 210_000, 32)`;
store `pbkdf2$210000$base64(salt)$base64(hash)`. On login: re-derive with the *stored*
salt and iteration count, compare with `subtle.ConstantTimeCompare`.

**Session token.** `payload := json.Marshal({"uid": id, "exp": unixSeconds})`;
`sig := hmac.New(sha256.New, serverSecret); sig.Write(payload)`;
`token := base64url(payload) + "." + base64url(sig.Sum(nil))`. To verify: split on `.`,
recompute the HMAC over the decoded payload, compare with `subtle.ConstantTimeCompare`,
**then** check `exp`. (Verify the signature *before* trusting `exp` — never parse
untrusted JSON you haven't authenticated first... except you must decode it to get `exp`
to check freshness; the signature check comes first regardless, and a tampered payload
fails signature verification regardless of what `exp` says.)

**CSRF — double-submit cookie.** The `csrf_token` cookie is **not** `HttpOnly` (so
JS can read it) and its value must be echoed by the client in an `X-CSRF-Token` header
on every state-changing request. The server compares cookie value to header value
(`subtle.ConstantTimeCompare`) — no server-side session storage needed for this check.

**Per-user data isolation.** Tasks (from Project 20) gain an `owner_id`. A user requesting
a task they don't own gets `404`, **not** `403` — don't reveal that the resource exists.

## Requirements

### Routes

| Method | Path | Auth | Body | Success |
|---|---|---|---|---|
| `POST` | `/auth/register` | none | `{username,password}` | `201 {"user_id":...}` |
| `POST` | `/auth/login` | none | `{username,password}` | `200 {"user_id":...,"expires_at":...}` + 2 cookies |
| `POST` | `/auth/logout` | session | — | `204`, cookies cleared |
| `POST` | `/auth/refresh` | session (not expired) | — | `200`, new cookies (new `exp`) |
| `GET` | `/auth/me` | session | — | `200 {"user_id":...,"username":...}` |
| `*` | `/tasks*` | session (+ CSRF on mutating) | — | as Project 20, scoped to `owner_id` |

### Validation

`username`: `^[a-z0-9_]{3,32}$`, unique (case-insensitive uniqueness). `password`: ≥8
bytes (no upper bound stated — document why you don't cap it).

### Cookies (on login/refresh)

| Cookie | Flags | Contents |
|---|---|---|
| `session` | `HttpOnly; Path=/; SameSite=Strict; Max-Age=3600` | the signed token |
| `csrf_token` | `Path=/; SameSite=Strict; Max-Age=3600` (no `HttpOnly`) | a fresh random value, unrelated to the session's own signature |

On logout: both re-set with `Max-Age=-1` (delete) and empty value.

### Status codes

| Code | When |
|---|---|
| `200`/`201`/`204` | as tabled above |
| `400` | validation failure on register/login body |
| `401` | missing/invalid/expired session cookie; wrong username or password |
| `403` | valid session, but missing/mismatched CSRF token on a mutating request |
| `404` | unknown username lookups leak nothing beyond `401`; a task not owned by the caller |
| `409` | username already registered |

### Exit codes (the binary — extends Project 20's)

Same as Project 20; additionally `1` if `-secret` (the HMAC key) is empty or shorter than
32 bytes at startup.

### Case specification — SUCCESS

| # | Request(s) | Assertion |
|---|---|---|
| S1 | `POST /auth/register {"username":"alice","password":"correcthorse"}` | `201`, `user_id` assigned |
| S2 | `POST /auth/register` same username again | `409` |
| S3 | `POST /auth/login {"username":"alice","password":"correcthorse"}` | `200`; `Set-Cookie: session=...`, `Set-Cookie: csrf_token=...` both present |
| S4 | `GET /auth/me` with the `session` cookie from S3 | `200 {"user_id":...,"username":"alice"}` |
| S5 | `POST /tasks` with `session` cookie **and** matching `X-CSRF-Token` header | `201` (task owned by alice) |
| S6 | `GET /tasks` as alice after S5, and separately as a second user `bob` | alice sees her task; bob's list is empty |
| S7 | alice `GET /tasks/{aliceTaskID}` | `200` |
| S8 | bob `GET /tasks/{aliceTaskID}` | `404` (not `403`) |
| S9 | `POST /auth/refresh` with a valid, not-yet-expired session | `200`, new `session`/`csrf_token` cookies with a later `exp` |
| S10 | `POST /auth/logout` then `GET /auth/me` with the old cookie | logout → `204`; the follow-up → `401` |

### Case specification — FAILURE

| # | Request | Response |
|---|---|---|
| F1 | `POST /auth/register {"username":"AL","password":"x"}` | `400` — username too short AND fails the character class; password too short (both reported) |
| F2 | `POST /auth/login {"username":"alice","password":"wrong"}` | `401 {"error":{"code":"invalid_credentials",...}}` |
| F3 | `POST /auth/login {"username":"ghost","password":"whatever"}` | `401`, **identical body** to F2 (don't leak whether the username exists — assert byte-equal error bodies) |
| F4 | `GET /auth/me` with no `session` cookie | `401` |
| F5 | `GET /auth/me` with a `session` cookie whose last byte is flipped (tampered signature) | `401` |
| F6 | `GET /auth/me` with a `session` cookie whose `exp` is in the past (valid signature) | `401` |
| F7 | `POST /tasks` with a valid `session` cookie but **no** `X-CSRF-Token` header | `403 {"error":{"code":"csrf_failed",...}}` |
| F8 | `POST /tasks` with a valid `session` cookie and a `X-CSRF-Token` that doesn't match the `csrf_token` cookie | `403` |
| F9 | `GET /tasks` (a non-mutating request) with a valid session but no CSRF cookie/header at all | `200` — CSRF is only enforced on mutating methods |
| F10 | `POST /auth/refresh` with an **expired** session cookie | `401` (refresh requires a still-valid session — that's what `refresh` means here; a fully expired session must re-login) |
| F11 | server started with `-secret short` (< 32 bytes) | process exits `1` at startup with a clear message, never binds a listener |

### Error catalogue

| `code` | HTTP | message |
|---|---|---|
| `validation_failed` | 400 | field errors, same shape as Project 20 |
| `invalid_credentials` | 401 | `"invalid username or password"` (identical for both causes) |
| `unauthenticated` | 401 | `"missing or invalid session"` |
| `session_expired` | 401 | `"session expired"` |
| `csrf_failed` | 403 | `"missing or invalid CSRF token"` |
| `username_taken` | 409 | `"username already registered"` |
| `not_found` | 404 | (reused from Project 20 — task not owned/not found look identical) |

## Suggested milestones

1. `internal/authcrypto`: PBKDF2 hash+verify, HMAC token sign+verify, CSRF token
   generation — pure functions, no HTTP, exhaustively tested first (including the
   tamper-detection and expiry cases).
2. `internal/user`: `User` store (username→id, id→passwordHash), register/authenticate.
3. `POST /auth/register`, `/login`, `/logout`, `/refresh`, `/me` handlers.
4. `RequireAuth` middleware — extracts + verifies the session, injects the user into
   `context`, else `401`.
5. `RequireCSRF` middleware — only on mutating methods, wraps the task routes.
6. Retrofit Project 20's task store with `owner_id`; every task handler filters by the
   context user; ownership check → `404` on mismatch.
7. `-secret` flag/env validation at startup.
8. Full integration tests (`httptest.NewServer`, real `net/http.CookieJar` to carry
   cookies across requests like a browser would).

## Project layout

```
projects/21-auth/
  cmd/api/main.go                     // extends Project 20's main
  internal/authcrypto/password.go     // pbkdf2 hash/verify
  internal/authcrypto/token.go        // hmac sign/verify session token
  internal/authcrypto/csrf.go
  internal/user/user.go               // store, register, authenticate
  internal/httpapi/auth_handlers.go
  internal/httpapi/auth_middleware.go
  internal/httpapi/*_test.go
  cmd/api/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md) and Project 20's DoD. Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] Every secret comparison in the codebase uses `subtle.ConstantTimeCompare` — grep
      your own diff for `==` or `bytes.Equal` near tokens/hashes and justify or fix each
      hit.
- [ ] F3's assertion (byte-identical error body for "wrong password" vs "unknown user")
      actually passes — this is the whole point of that test.
- [ ] Password hashes are never logged, never returned in any response body (verify with
      a test that greps every response for the raw password/hash).
- [ ] A tampered session cookie is rejected **before** its `exp` claim is trusted
      (test: a tampered-but-"not yet expired" token still fails).
- [ ] `owner_id` isolation is airtight: a fuzz-ish test creates 5 users × 5 tasks each and
      asserts every cross-user `GET`/`PUT`/`DELETE`/`PATCH` returns `404`.

## Test requirements

- `TestPBKDF2HashVerify` — correct password, wrong password, wrong salt, cost parameters
  round-trip through the stored format string.
- `TestTokenSignVerify` — valid, tampered payload, tampered signature, expired, wrong
  secret.
- `TestCSRFDoubleSubmit` — match, mismatch, missing, non-mutating method bypass.
- `TestRegisterLogin` — S1–S3, F1–F3 (incl. the byte-equal check).
- `TestSessionLifecycle` — S9, S10, F4–F6, F10.
- `TestTaskOwnershipIsolation` — S6–S8, the 5×5 cross-user fuzz-ish test.
- `TestStartupSecretValidation` — F11.
- `TestNoSecretLeakage` — response-body grep.
- `ExampleAuthcrypto_hashVerify` (as `Example...` if it fits the pattern; otherwise a
  regular test is fine — document your choice).

## Stretch goals

- Refresh-token rotation (a separate long-lived token; `session` becomes short-lived).
- Rate-limit `/auth/login` per IP to blunt brute force (bridge to Project 27).
- Multiple sessions per user with a `GET /auth/sessions` + revoke-one endpoint (needs
  server-side session storage — a real trade-off vs the stateless token above; discuss it
  in the README).
- Argon2id instead of PBKDF2 — note that it's `golang.org/x/crypto`, not stdlib, and why
  it's nonetheless the better real-world choice.
- Email verification flow using a second signed, short-lived token type.

## Self-check questions

1. Why constant-time compare a signature but *not* the plaintext password itself (you
   never have the plaintext to compare — you compare *hashes*, and PBKDF2 already makes
   timing differences irrelevant there because of the hash's avalanche property; discuss
   why the *hash comparison* still uses `subtle.ConstantTimeCompare` anyway)?
2. Walk through exactly what an attacker learns if `/auth/login` returned a different
   error for "no such user" vs "wrong password" — and why F3's identical-body test closes
   that gap only if the **timing** is also similar (do you need to worry about timing
   here, given PBKDF2's cost, even after the bodies match?).
3. Why is `csrf_token` **not** `HttpOnly` while `session` **is**? What attack does each
   flag prevent, and why does CSRF protection specifically *require* JS to read the
   cookie?
4. A tampered session cookie has a *future* `exp` (attacker guessed a payload) but a
   wrong signature. Your code must reject it on signature alone. What's the exploit if you
   checked `exp` first and only checked the signature when `exp` looked valid (hint: it's
   not actually exploitable here, but explain why checking signature first is still the
   correct habit)?
5. `pbkdf2.Key(sha256.New, password, salt, 210_000, 32)` — what does the iteration count
   actually cost an attacker doing offline brute force, and what does it cost your login
   endpoint's p99 latency? How would you choose the number for a real service?
6. `SameSite=Strict` on the session cookie — what legitimate cross-site scenario does this
   break (think: a link from another site into your app while logged in), and why is
   `Strict` still the right default for this project?
7. Task ownership returns `404` instead of `403` for another user's task. What's the
   information-disclosure argument for that, and can you think of a case where `404`
   itself leaks something (hint: response timing between "task doesn't exist" and "task
   exists but isn't yours")?
