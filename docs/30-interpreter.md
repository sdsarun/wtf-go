# Project 30 — Mini Programming-Language Interpreter

| | |
|---|---|
| **Difficulty** | 5 / 5 |
| **Estimated time** | 20–30 hours (spread over 1–2 weeks) |
| **Prerequisites** | [08](08-expression-calculator.md), [11](11-generic-containers.md), [13](13-report-engine.md) |
| **Builds toward** | nothing further in this curriculum — this is the grand capstone |

## Why this project

The capstone. A tree-walking interpreter for a small C-like language: a hand-written
**lexer**, a **Pratt parser** (precedence-climbing recursive descent), an AST built
entirely on interfaces, a tree-walking **evaluator** with a proper **environment** model
for variables and **closures**, first-class functions, arrays, hashes, and a REPL. There
is barely a Go feature from Projects 01–29 that doesn't show up here again, used for real:
recursion, interfaces, type switches, generics (in supporting containers you may reuse the
*shape* of, if not the exact package — see Project 11's `internal/` note), error wrapping,
`panic`/`recover` at the REPL boundary, closures over mutable environments, and heavy
table-driven + fuzz testing.

## Go concepts you MUST use

- [ ] a hand-written **lexer**: `[]byte`/rune scanning → a token stream, no `regexp`
- [ ] a **Pratt parser**: prefix/infix parse functions keyed by token type, a precedence
      table, precedence-climbing — not a naive recursive-descent grammar transcription
- [ ] an **AST** where every node type implements a common interface (`Node`, with
      `Statement`/`Expression` sub-interfaces), heavy interface + type-switch use in the
      evaluator
- [ ] an **environment** model for variables: `Environment{store map[string]Object;
      outer *Environment}`, and **closures** that capture their defining environment by
      reference (not by copying bindings)
- [ ] first-class, higher-order functions as a first-class `Object` type
- [ ] `panic`/`recover` **only** at the REPL's top-level boundary (parser/evaluator
      internals should return errors, not panic — recover exists to guarantee a crashed
      evaluation never kills the REPL, not as your primary error mechanism)
- [ ] large **table-driven test suites**; `FuzzLex` and `FuzzParse` (must never panic or
      hang on any input); benchmarks; a `pprof` profiling pass on a compute-heavy program
- [ ] `//go:generate` for the `TokenType` → name mapping (a stringer-style generator, as
      in Project 11)

## Background — the language

C-like syntax, dynamically typed, semicolon-terminated statements. Types: `Integer`
(int64), `Boolean`, `String`, `Null`, `Array`, `Hash`, `Function`.

```
let x = 5;
let add = fn(a, b) { a + b };
let result = add(x, 10);

if (x > 3) { "big" } else { "small" }

let numbers = [1, 2, 3];
numbers[1]

let scores = {"alice": 97, "bob": 84};
scores["alice"]

let fact = fn(n) { if (n == 0) { 1 } else { n * fact(n - 1) } };
fact(5)
```

**Truthiness:** only `false` and the `null` value are falsy. `0`, `""`, and `[]` are
**truthy** — this is deliberate and a required test case, not an oversight.

**Blocks and implicit return.** A `{ ... }` block's value is the value of its **last**
expression statement (function bodies, `if` bodies). A `return EXPR;` immediately unwinds
out of any nested blocks **up to the nearest enclosing function call**, not further.

**Grammar (EBNF, statements):**

```
Program        = { Statement } ;
Statement      = LetStatement | ReturnStatement | ExpressionStatement ;
LetStatement   = "let" IDENT "=" Expression ";" ;
ReturnStatement= "return" Expression ";" ;
ExpressionStatement = Expression ";" ;
BlockStatement = "{" { Statement } "}" ;
```

**Operators & precedence** (lowest → highest): `||` · `&&` · `== !=` · `< > <= >=` ·
`+ -` · `* / %` · prefix `! -` · call/index (`f(...)`, `a[...]`).

## Requirements

### Lexer

Token types: `ILLEGAL EOF IDENT INT STRING ASSIGN PLUS MINUS BANG ASTERISK SLASH PERCENT
LT GT LE GE EQ NOT_EQ AND OR COMMA SEMICOLON COLON LPAREN RPAREN LBRACE RBRACE LBRACKET
RBRACKET FUNCTION LET TRUE FALSE IF ELSE RETURN`. Keywords (`fn let true false if else
return`) are recognized by looking up a scanned identifier in a keyword table, not by
special-casing them in the scanner. String literals are `"..."` with no escape sequences
required (backslash handling is a stretch goal — document the omission).

### Parser

A Pratt parser: `prefixParseFns map[TokenType]func() Expression`,
`infixParseFns map[TokenType]func(Expression) Expression`, a `precedence(tok) int`
function, and `parseExpression(precedence int) Expression` that loops while the next
token's precedence exceeds the current one. Parse errors accumulate in a `[]error` on the
parser (don't stop at the first one; collect what you can, like a real compiler front end)
and are all surfaced together.

### AST node table

| Node | Fields |
|---|---|
| `Program` | `Statements []Statement` |
| `LetStatement` | `Name *Identifier; Value Expression` |
| `ReturnStatement` | `Value Expression` |
| `ExpressionStatement` | `Value Expression` |
| `BlockStatement` | `Statements []Statement` |
| `Identifier` | `Value string` |
| `IntegerLiteral` | `Value int64` |
| `StringLiteral` | `Value string` |
| `Boolean` | `Value bool` |
| `PrefixExpression` | `Operator string; Right Expression` |
| `InfixExpression` | `Left Expression; Operator string; Right Expression` |
| `IfExpression` | `Condition Expression; Consequence, Alternative *BlockStatement` |
| `FunctionLiteral` | `Parameters []*Identifier; Body *BlockStatement` |
| `CallExpression` | `Function Expression; Arguments []Expression` |
| `ArrayLiteral` | `Elements []Expression` |
| `IndexExpression` | `Left, Index Expression` |
| `HashLiteral` | `Pairs map[Expression]Expression` |

### Object model

`Object interface { Type() ObjectType; Inspect() string }`. Concrete types: `Integer`,
`Boolean`, `String`, `Null`, `ReturnValue` (internal unwrap-signal wrapper), `Error`
(carries a message), `Function` (`Parameters, Body, Env`), `Array` (`Elements []Object`),
`Hash` (`Pairs map[HashKey]HashPair`). `Integer`/`Boolean`/`String` implement a
`Hashable` interface (`HashKey() HashKey`) so they can be hash keys; anything else used as
a key is a runtime error.

### Evaluator semantics

- Identifiers resolve by walking `Environment.outer` chains outward; unresolved →
  `Error("identifier not found: %s")`.
- `let` creates/overwrites a binding in the **current** environment only.
- A `FunctionLiteral`, when evaluated, captures the **current environment by reference**
  as its closure environment. Calling it creates a **new enclosed environment** (outer =
  the closure env, not the caller's env — this is what makes closures work and lexical,
  not dynamic, scoping) and binds parameters into it.
- Arithmetic/comparison on mismatched types (`5 + true`) → `Error("type mismatch: %s %s
  %s")`. An operator not defined for a type pair (`"a" - "b"`, `-true`) →
  `Error("unknown operator: ...")`.
- Indexing an `Array` out of bounds (including negative) → `Null`, not an error. Indexing
  a `Hash` with a missing key → `Null`.
- Calling a non-`Function` → `Error("not a function: %s")`.
- An `Error` object, once produced, **propagates** through blocks/statements like a
  `ReturnValue` does — evaluation of the current top-level statement stops immediately.
- Built-ins: `len(x)` (rune count for `String`, element count for `Array`; error for
  anything else), `first(arr)`/`last(arr)` (`Null` on empty), `rest(arr)` (new array
  without the first element, `Null` on empty), `push(arr, x)` (returns a **new** array —
  arrays are not mutated in place; verify this with a test), `puts(...)` (writes each
  argument's `Inspect()` to the evaluator's configured `io.Writer`, returns `Null`).

## Case specification — the language behaviour (every row is a test)

**OK cases** (`input` → `.Inspect()` of the result, or a note about output for `puts`):

| # | Input | Result |
|---|---|---|
| L1 | `5` | `5` |
| L2 | `5 + 5 * 2` | `15` |
| L3 | `(5 + 5) * 2` | `20` |
| L4 | `-5 - -10` | `5` |
| L5 | `5 % 2` | `1` |
| L6 | `!true` | `false` |
| L7 | `!!5` | `true` (truthy) |
| L8 | `5 > 3 == true` | `true` |
| L9 | `1 < 2 && 3 < 4` | `true` |
| L10 | `1 > 2 \|\| 3 < 4` | `true` |
| L11 | `"foo" + "bar"` | `foobar` |
| L12 | `"a" == "a"` | `true` |
| L13 | `let x = 5; x + 1` | `6` |
| L14 | `let x = 5; let x = 10; x` | `10` (rebinding shadows in the same scope) |
| L15 | `let add = fn(a, b) { a + b }; add(2, 3)` | `5` |
| L16 | `let newAdder = fn(x) { fn(y) { x + y } }; let addTwo = newAdder(2); addTwo(3);` | `5` (closures) |
| L17 | `let fact = fn(n) { if (n == 0) { 1 } else { n * fact(n - 1) } }; fact(5)` | `120` (self-reference through the closure env — see self-check 3) |
| L18 | `if (false) { 10 }` | `null` |
| L19 | `if (1 < 2) { 10 } else { 20 }` | `10` |
| L20 | `let f = fn(x) { if (x > 5) { return "big"; } return "small"; }; f(10)` | `big` |
| L21 | `let f = fn() { if (true) { if (true) { return 1; } return 2; } return 3; }; f()` | `1` (return unwinds nested blocks, stops at the function boundary) |
| L22 | `let a = [1, 2, 3]; a[1]` | `2` |
| L23 | `let a = [1, 2, 3]; a[10]` | `null` |
| L24 | `let a = [1, 2, 3]; a[-1]` | `null` |
| L25 | `len("hello")` | `5` |
| L26 | `len([1, 2, 3])` | `3` |
| L27 | `first([1,2,3])`, `last([1,2,3])`, `rest([1,2,3])` | `1`, `3`, `[2, 3]` |
| L28 | `let a = [1,2]; let b = push(a, 3); len(a)` | `2` (original untouched) |
| L29 | `let h = {"a": 1, "b": 2}; h["a"]` | `1` |
| L30 | `let h = {"a": 1}; h["z"]` | `null` |
| L31 | `{true: 1, false: 2}[1 < 2]` | `1` (booleans as keys) |
| L32 | a `map`-over-array built entirely in the language (recursive helper using `rest`/`push`/`first`) applied with a doubling function | `[2, 4, 6]` for input `[1,2,3]` |

**ERROR cases** (evaluator errors — `Error.Inspect()` starts `ERROR: `):

| # | Input | Message |
|---|---|---|
| E1 | `5 + true` | `type mismatch: INTEGER + BOOLEAN` |
| E2 | `-true` | `unknown operator: -BOOLEAN` |
| E3 | `true + false` | `unknown operator: BOOLEAN + BOOLEAN` |
| E4 | `foobar` | `identifier not found: foobar` |
| E5 | `"a" - "b"` | `unknown operator: STRING - STRING` |
| E6 | `5(1, 2)` | `not a function: INTEGER` |
| E7 | `len(1)` | `argument to 'len' not supported, got INTEGER` |
| E8 | `len("a", "b")` | `wrong number of arguments. got=2, want=1` |
| E9 | `if (10 > 1) { if (10 > 1) { return true + false; } return 1; }` | `unknown operator: BOOLEAN + BOOLEAN` (propagates through the nested block, evaluation stops — `return 1` never runs) |
| E10 | `{fn(x){x}: 1}` | `unusable as hash key: FUNCTION` |

**PARSE-ERROR cases** (parser collects and reports these; evaluation never runs):

| # | Input | Error contains |
|---|---|---|
| P1 | `let x 5;` | `expected next token to be =, got INT instead` |
| P2 | `let = 5;` | `expected next token to be IDENT, got = instead` |
| P3 | `if (true { 1 }` | `expected next token to be ), got { instead` |
| P4 | `fn(x, { }` | a parse error mentioning the malformed parameter list |
| P5 | `(1 + 2` | `expected next token to be ), got EOF instead` |

### REPL

Reads a line, parses it as a full `Program`, evaluates it in a **persistent** environment
(bindings carry across lines, like Project 08's calculator). On a parse error: print
every collected parser error, one per line, prefixed `parse error: `. On an eval error:
print `ERROR: <message>`. On success: print the result's `.Inspect()`. The special
identifier `_` is automatically bound to the value of the last successfully evaluated
top-level expression statement (mirroring Project 08's `ans`). `:quit` exits.

### One-shot / file mode

`interp "let x = 5; x * 2;"` prints `10`, exit `0`. `interp -f script.mn` runs a file;
`puts(...)` output goes to stdout. Any parse or eval error → the message(s) to stderr,
exit `1`.

## Suggested milestones

*(This project is large — commit after each milestone, and lean hard on the case tables
above as your test suite as you go, not as an afterthought.)*

1. Lexer + `TestNextToken` against a program exercising every token; `FuzzLex` from day
   one.
2. AST node types; `String()` on each (for debugging/pretty-printing) — not graded
   directly but invaluable while building the parser.
3. Parser: statements first (`let`, `return`, expression statements with just literals),
   then the Pratt engine for expressions — prefix (literals, `!`, `-`, `(`, `fn`, `if`,
   `[`, `{`) then infix (binary operators, call, index) with the precedence table.
   `TestOperatorPrecedenceParsing` (assert the **parenthesized string form** of the
   parsed tree matches, e.g. `"a + b * c"` → `"(a + (b * c))"`).
4. Parser error collection; P1–P5.
5. Object model + a tree-walking `Eval(node, env)`; integers, booleans, `null`,
   arithmetic, comparisons, `!`/`-` prefix, `if`.
6. `let`, identifiers, environments (with `outer` chaining).
7. Functions, calls, **closures** — get L16/L17 passing; they're the real test of whether
   your environment model is correct.
8. `return` + error propagation through blocks (L21, E9) — this is the part most people
   get subtly wrong the first time; test it hard.
9. Strings, arrays, hashes, indexing, built-ins.
10. The REPL; file/one-shot mode.
11. `FuzzParse`; benchmarks (`BenchmarkEvalFib`); a `pprof` CPU profile on a naive
    recursive Fibonacci program, with the flame-graph takeaway written into the README.
12. `//go:generate` the `TokenType` name table.

## Project layout

```
projects/30-interpreter/
  cmd/interp/main.go
  internal/token/token.go
  internal/token/token_string.go   // generated
  internal/lexer/lexer.go
  internal/lexer/*_test.go
  internal/lexer/lexer_fuzz_test.go
  internal/ast/ast.go
  internal/parser/parser.go
  internal/parser/*_test.go
  internal/parser/parser_fuzz_test.go
  internal/object/object.go
  internal/object/environment.go
  internal/evaluator/evaluator.go
  internal/evaluator/builtins.go
  internal/evaluator/*_test.go
  internal/repl/repl.go
  internal/repl/*_test.go
  cmd/interp/main_test.go
  testdata/*.mn                    // sample programs
  README.md
  Makefile
```

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every row of L1–L32, E1–E10, and P1–P5 passes exactly.
- [ ] `TestOperatorPrecedenceParsing` covers at least 15 expressions of increasing
      complexity, asserting the exact parenthesized string form.
- [ ] Closures are proven correct by L16 (returns a *new* function each call, capturing
      its *own* `x`) and by a test that two closures created from the same outer call
      don't see each other's later mutations of a shared `let`.
- [ ] `return` inside deeply nested `if`/blocks stops exactly at the function boundary —
      L21 and E9 both pass, and a test with 4+ levels of nesting.
- [ ] `push` never mutates its input array (L28) — verified, not assumed.
- [ ] `FuzzLex` and `FuzzParse` each run ≥5 minutes clean: no panic, no hang, on any
      input including binary garbage.
- [ ] `go generate ./...` regenerates `token_string.go` identically to what's committed.
- [ ] The README includes your `pprof` findings for the Fibonacci benchmark (what
      fraction of time is spent in environment lookups vs allocation vs arithmetic) and
      one sentence on what you'd change in a tree-walking interpreter to speed it up
      (this is a lead-in to "write a bytecode VM instead," which is out of scope here).

## Test requirements

- `TestNextToken` — a program using every token type.
- `TestLetStatements`, `TestReturnStatements`, `TestIdentifierExpression`,
  `TestIntegerLiteralExpression`, `TestStringLiteralExpression`.
- `TestParsingPrefixExpressions`, `TestParsingInfixExpressions`,
  `TestOperatorPrecedenceParsing`.
- `TestIfExpression`, `TestFunctionLiteralParsing`, `TestCallExpressionParsing`,
  `TestArrayLiteralParsing`, `TestIndexExpressionParsing`, `TestHashLiteralParsing`.
- `TestParserErrors` — P1–P5.
- `TestEvalIntegerExpression`, `TestEvalBooleanExpression`, `TestBangOperator`,
  `TestIfElseExpressions`, `TestReturnStatements` (eval-level), `TestErrorHandling` (E1–E10),
  `TestLetStatements` (eval-level), `TestFunctionObject`, `TestFunctionApplication`,
  `TestClosures` (L16, L17), `TestStringLiteral`, `TestStringConcatenation`,
  `TestBuiltinFunctions`, `TestArrayLiterals`, `TestArrayIndexExpressions`,
  `TestHashLiterals`, `TestHashIndexExpressions`.
- `TestREPLPersistentEnv`, `TestREPLParseErrorReporting`, `TestUnderscoreBinding`.
- `FuzzLex`, `FuzzParse`.
- `BenchmarkEvalFib` (`fib(20)` or similar — deep recursion, exercises environment chains
  hard).
- `ExampleEval` or `ExampleREPL`.

## Stretch goals

- **This is the natural "what's next":** compile the AST to bytecode and run it on a
  small stack VM instead of tree-walking — a well-known, large step up in both
  performance and complexity. Not part of this curriculum; a good first project *after*
  it if you want to keep going.
- String escape sequences (`\n`, `\"`, `\\`) in the lexer.
- A `for` loop or a `while`-style construct (the language currently has none — recursion
  is the only iteration mechanism, which is itself a good self-check point).
- More built-ins: `map`, `reduce`, `filter` implemented as **Go** builtins (not
  in-language recursion) for comparison — benchmark against the in-language L32 version.
- Float support alongside `Integer`.
- A `quote`/`unquote` macro system (the classic follow-up chapter in interpreter books) —
  genuinely advanced; only attempt it once everything else is rock solid.

## Self-check questions

1. Walk through `let newAdder = fn(x) { fn(y) { x + y } }; let addTwo = newAdder(2);`
   step by step: when the inner `fn(y) { x + y }` is evaluated, what `Environment` does it
   capture, and why does `addTwo(3)` still see `x = 2` even though `newAdder`'s call has
   long since returned?
2. Your `Environment` has a `store map[string]Object` and an `outer *Environment`. Why
   `outer *Environment` (a pointer) and not `outer Environment` (a value)? What would
   break about closures if environments were copied instead of shared?
3. `let fact = fn(n) { ... fact(n-1) ... };` — at the moment the `FunctionLiteral` is
   evaluated (to produce the `Function` object that gets bound to `fact`), is `fact`
   itself already in the environment? Walk through exactly *when* the binding
   `env.Set("fact", ...)` happens relative to when the closure environment is captured,
   and why recursion still works despite the apparent chicken-and-egg problem.
4. `return` inside a `BlockStatement` produces a `*ReturnValue` wrapper object, not the
   raw value. Why does the evaluator need this wrapper — what would go wrong if
   `Eval(BlockStatement)` just returned the raw unwrapped value the instant it saw
   `return`, without a way to distinguish "this block finished normally with this value"
   from "a `return` happened, stop unwinding at the next function boundary"?
5. An `*object.Error`, once produced, must also propagate up through blocks similarly to
   `ReturnValue`. What test (in the E-table) specifically catches an evaluator that
   forgets this and lets execution continue past an error?
6. Your Pratt parser's `parseExpression(precedence)` loop condition compares the
   **upcoming** token's precedence against the precedence passed in. Trace `1 + 2 * 3`
   through it by hand: when parsing the `+` infix expression's right-hand side, why does
   the loop *not* also consume the `*`, `3` as part of `1`'s side, and how does that
   produce the correct tree `(1 + (2 * 3))`?
7. `push(arr, x)` returns a new array. What's the actual Go-level operation (hint: think
   about slice `append` and its capacity/aliasing behaviour from Project 06) that makes
   this safe even if two closures hold references to the "same" original array?
8. Name three Go features from *this specific* interpreter's codebase that came directly
   from an earlier project in this curriculum (not "Go in general" — literally: "I used
   the panic/recover pattern I first built in Project X for Y here").
