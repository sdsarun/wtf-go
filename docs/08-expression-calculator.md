# Project 08 — Expression Calculator (recursive descent)

| | |
|---|---|
| **Difficulty** | 3 / 5 |
| **Estimated time** | 6–9 hours |
| **Prerequisites** | [01](01-unit-converter-cli.md)–[07](07-todo-cli.md) |
| **Builds toward** | [13 – Report Engine](13-report-engine.md), [30 – Interpreter](30-interpreter.md) |

## Why this project

Your first parser. A recursive-descent expression evaluator is small enough to finish in a
weekend and rich enough to force **recursion**, an **interface-based AST**, **custom error
types with position info**, error **wrapping** (`%w`, `errors.Is`, `errors.As`), and
`panic`/`recover` used deliberately as parser control flow with a clean boundary. Project
30 is the same idea at 10× scale — do this one first.

## Go concepts you MUST use

- [ ] recursion (the parser functions call each other)
- [ ] a hand-written **lexer** producing a `[]Token` (no `regexp`, no `text/scanner`)
- [ ] an **interface** `Node` with several implementing struct types (`Num`, `BinOp`,
      `Unary`, `Call`, `Var`, `Assign`)
- [ ] `fmt.Stringer` on `Node` (pretty-print the parsed tree)
- [ ] **custom error types**: `*ParseError{Pos int, Msg string}`, `*EvalError`
- [ ] a **sentinel error** `ErrDivByZero`, wrapped with `%w`, detected with `errors.Is`
- [ ] `errors.As` in the REPL to pull `Pos` out of a `*ParseError` and draw a caret
- [ ] `panic` for parser error unwinding + `recover` at the single parse entry point that
      converts it to a returned `*ParseError`
- [ ] first-class functions: `map[string]Builtin` where `type Builtin func([]float64) (float64, error)`
- [ ] a closure and a higher-order use (e.g. a `reduce` helper for `min`/`max`)

## Background

**Grammar** (EBNF; `^` is right-associative, everything else left):

```
expr    = term   { ("+" | "-") term } ;
term    = power  { ("*" | "/" | "%") power } ;
power   = unary  [ "^" power ] ;
unary   = ("+" | "-") unary | primary ;
primary = NUMBER
        | IDENT
        | IDENT "(" [ expr { "," expr } ] ")"
        | IDENT "=" expr
        | "(" expr ")" ;
```

**Precedence, lowest → highest:** `= ` (assignment) · `+ -` · `* / %` · unary `- +` ·
`^`. `2^3^2` = `2^(3^2)` = `512`. `-2^2` = `-(2^2)` = `-4`. `2 + 3 * 4` = `14`.

**Semantics.** All values are `float64`. `/` by `0` → `ErrDivByZero`. `%` is
`math.Mod`. `^` is `math.Pow`. Assignment `x = expr` evaluates `expr`, stores it in the
environment, and *is* an expression returning that value (`y = x = 5` sets both).

**Builtins:** constants `pi`, `e`; the special variable `ans` (last result); functions
`sqrt abs sin cos tan asin acos atan ln log exp floor ceil round pow(x,y) min(...) max(...) hypot(x,y)`.
`ln` is natural log, `log` is base-10. Arity errors are `*EvalError`.

**Number formatting:** `strconv.FormatFloat(v, 'g', -1, 64)` — `14`, `0.3333333333333333`,
`1024`, `1.5`, `-4`. `NaN` prints `NaN`, `+Inf` prints `+Inf`.

## Requirements

### Functional requirements

1. Modes:
   - `calc "<expr>"` — evaluate one expression, print the result, exit.
   - `calc` with a TTY or piped stdin — REPL: read a line, evaluate, print `= <result>`
     or an error block; loop until EOF or `:quit`.
   - `calc -f FILE` — evaluate each non-empty, non-comment (`#`) line; print each result;
     stop at the first error (exit 1) unless `-k` (keep going).
2. The environment persists across REPL lines and across file lines: variables set on one
   line are visible on the next. `ans` is updated after every successful evaluation.
3. REPL meta-commands (line starts with `:`): `:vars` (list variables, sorted),
   `:reset` (clear variables, keep `ans`? no — clear everything), `:help`, `:quit`.
   Unknown → `error: unknown command ":foo"`.
4. Parse errors carry a 1-based **rune** position. The REPL prints:
   ```
   <the input line>
   <spaces><caret>
   error: <message> at position <n>
   ```
5. Eval errors (`div by zero`, unknown identifier, unknown function, wrong arity,
   `sqrt(-1)` → `NaN` is *not* an error) print `error: <message>` (no caret).

### Exact contract — output

| Mode | On success | On error |
|---|---|---|
| one-shot | `<result>\n` to stdout, exit 0 | `error: <msg>\n` (+ caret block for parse errors) to stderr, exit 1 |
| REPL | `= <result>\n` | error block to stdout (it's interactive), loop continues |
| file | `<result>\n` per line | error block to stderr; exit 1 (or continue with `-k`) |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | success; REPL exit via `:quit`/EOF |
| `1` | one-shot or file evaluation error |
| `2` | usage — unknown flag, `-f` file missing, both an expr arg and `-f` given |

### Case specification — one-shot SUCCESS

| # | `calc <arg>` | stdout | exit |
|---|---|---|---|
| S1 | `"2 + 3 * 4"` | `14` | 0 |
| S2 | `"(2 + 3) * 4"` | `20` | 0 |
| S3 | `"2 ^ 3 ^ 2"` | `512` | 0 |
| S4 | `"-2 ^ 2"` | `-4` | 0 |
| S5 | `"10 / 4"` | `2.5` | 0 |
| S6 | `"1 / 3"` | `0.3333333333333333` | 0 |
| S7 | `"10 % 3"` | `1` | 0 |
| S8 | `"10.5 % 3"` | `1.5` | 0 |
| S9 | `"2 * pi"` | `6.283185307179586` | 0 |
| S10 | `"sqrt(2)"` | `1.4142135623730951` | 0 |
| S11 | `"pow(2, 10)"` | `1024` | 0 |
| S12 | `"max(3, 7, 1, 9, 2)"` | `9` | 0 |
| S13 | `"min(3, 7, 1)"` | `1` | 0 |
| S14 | `"abs(-5) + floor(3.9)"` | `8` | 0 |
| S15 | `"1e3 + 2.5e-1"` | `1000.25` | 0 |
| S16 | `"sqrt(-1)"` | `NaN` | 0 |
| S17 | `"1 / 0"` — wait, this is an error | see F-table | 1 |
| S18 | `"x = 21"` | `21` | 0 |

### Case specification — REPL SUCCESS (one session, `\n`-separated input)

Input:
```
x = 6
y = 7
x * y
ans + 1
:vars
sqrt(ans)
:quit
```
Output:
```
= 6
= 7
= 42
= 43
x = 6
y = 7
= 6.557438524302
```
*(last value shown to full `%g` precision — your exact digits from `math.Sqrt(43)`)*

### Case specification — FAILURE

| # | Input | message | exit (one-shot) |
|---|---|---|---|
| F1 | `"2 +"` | `error: unexpected end of input at position 4` | 1 |
| F2 | `"2 + * 3"` | `error: unexpected "*" at position 5` | 1 |
| F3 | `"(2 + 3"` | `error: expected ")" at position 7` | 1 |
| F4 | `"2 @ 3"` | `error: unexpected character "@" at position 3` | 1 |
| F5 | `"1 / 0"` | `error: division by zero` | 1 |
| F6 | `"1 / (3 - 3)"` | `error: division by zero` | 1 |
| F7 | `"foo + 1"` | `error: unknown identifier "foo"` | 1 |
| F8 | `"bar(2)"` | `error: unknown function "bar"` | 1 |
| F9 | `"sqrt(2, 3)"` | `error: sqrt takes 1 argument, got 2` | 1 |
| F10 | `"pow(2)"` | `error: pow takes 2 arguments, got 1` | 1 |
| F11 | `"3 = 4"` | `error: cannot assign to a non-identifier at position 3` | 1 |
| F12 | `calc -f missing.txt` | `error: open missing.txt: no such file or directory` | 2 |
| F13 | `calc "1+1" -f x.txt` | `error: give an expression or -f, not both` | 2 |

For a parse error in the **REPL**, the caret block is printed and the loop continues; the
exit code is unaffected.

### Error catalogue

| Trigger | Format |
|---|---|
| unexpected EOF | `unexpected end of input at position %d` |
| unexpected token | `unexpected %q at position %d` |
| unexpected char (lexer) | `unexpected character %q at position %d` |
| expected token | `expected %q at position %d` |
| assign target | `cannot assign to a non-identifier at position %d` |
| div by zero | `division by zero` (wraps `ErrDivByZero`) |
| unknown identifier | `unknown identifier %q` |
| unknown function | `unknown function %q` |
| arity | `%s takes %d argument(s), got %d` |
| REPL meta | `unknown command %q` |

## Suggested milestones

1. `internal/calc/lexer.go` — `Token{Kind, Text, Pos}`; `Lex(string) ([]Token, error)`.
   Test against operators, numbers (incl. `1e3`), idents, bad chars.
2. `ast.go` — the `Node` interface + implementations + `String()`.
3. `parser.go` — one function per grammar rule; internal errors via `panic(parseError)`;
   `Parse(string) (Node, error)` recovers and returns a `*ParseError`.
4. `env.go` — `Env{vars map[string]float64}`; builtins map; `ans`.
5. `eval` — `Node.eval(*Env)`; `%w` wrap `ErrDivByZero`.
6. `repl.go` — line loop, meta-commands, the caret block using `errors.As`.
7. `cmd/calc/main.go` — mode selection, `-f`, `-k`.
8. Full test suite incl. `FuzzLex` and `FuzzParse` (should never panic).

## Project layout

```
projects/08-calc/
  cmd/calc/main.go
  internal/calc/lexer.go
  internal/calc/ast.go
  internal/calc/parser.go
  internal/calc/env.go
  internal/calc/eval.go
  internal/calc/repl.go
  internal/calc/*_test.go
  cmd/calc/main_test.go
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] `Parse` **never panics** on any input — verified by `FuzzParse` running ≥60s clean.
      Internal `panic(parseError)` is always caught by the entry-point `recover`.
- [ ] `errors.Is(err, ErrDivByZero)` is true for F5/F6 even though the message is wrapped.
- [ ] The REPL survives every error in the F-table and keeps its variable environment.
- [ ] `Node.String()` round-trips: `Parse(n.String())` produces an equal tree for a set of
      sample expressions (modulo added parens).

## Test requirements

- `TestLex` — operators, `-`/`+`, numbers incl. `1e-3`, idents, whitespace, bad chars
  with position.
- `TestParsePrecedence` — S1–S4 as tree shape assertions (not just the number).
- `TestEval` — every S row; `TestEvalErrors` — every F row's message.
- `TestAssignmentIsExpression` — `y = x = 5` sets both; result is `5`.
- `TestBuiltins` — arity, `ln` vs `log`, `min`/`max` variadic, `sqrt(-1)` = `NaN`.
- `TestREPLSession` — the session above; `:vars`, `:reset`, `:quit`, unknown command.
- `TestCaret` — parse error at position `n` puts the caret under the right rune (test a
  line with a multi-byte rune before the error).
- `FuzzLex`, `FuzzParse` — no panic, no hang.
- `ExampleParse`.

## Stretch goals

- Comparison + boolean ops (`< > == && ||`) and an `if(cond, a, b)` builtin.
- User-defined functions: `f(x) = x^2 + 1`.
- A `:tree <expr>` meta-command printing the AST indented.
- Rational mode (`-rational`) using `math/big.Rat` so `1/3 + 1/3 + 1/3 == 1` exactly.
- Units: `3 m + 20 cm` (bolt Project 01's converter on).

## Self-check questions

1. `power = unary [ "^" power ]` — the right-hand side recurses into `power`, not `unary`.
   Why does that one choice make `^` right-associative?
2. You parse `-2^2` as `-(2^2)`. Which grammar rule enforces that unary minus binds looser
   than `^`, and where would you move it to get `(-2)^2`?
3. Your parser `panic`s a `parseError` deep in `parsePrimary` and `recover`s in `Parse`.
   Why is that not "using panic for normal errors" in the bad sense — what's the boundary?
4. `fmt.Errorf("division by zero")` vs `fmt.Errorf("division by zero: %w", ErrDivByZero)` —
   the message is identical. Which one lets a caller write `errors.Is(err, ErrDivByZero)`,
   and why?
5. The caret must sit under the *rune* at `Pos`, not the byte. Given input `"café + @"`,
   what byte offset and what rune offset does `@` have, and which does your lexer track?
6. `map[string]Builtin` — why is a map of functions nicer here than a giant `switch` in
   `eval`? When would the `switch` be better?
7. `x = ans` before any evaluation — what should `ans` be, and where do you initialize it?
