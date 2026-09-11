# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this repo is

A personal Go-learning curriculum, worked through project by project.

- `docs/` — the curriculum: 30 project briefs (`01-...md` fundamentals →
  `30-interpreter.md` capstone), `README.md` (repo/toolchain conventions and the
  universal Definition of Done), and `concept-coverage.md` (which project forces
  which Go concept). These are reference specs — implement code to match them,
  don't edit them to match whatever the code currently does.
- `projects/NN-slug/` — the actual implementations, one directory per brief,
  laid out exactly as that brief's own "Project layout" section specifies
  (typically `cmd/<bin>/`, `internal/<pkg>/`, `README.md`, `Makefile`).
- `scripts/new-project.sh` — scaffolds the generic skeleton for a new
  `projects/NN-slug/` (see its header comment / run with no args for usage).

## Git commits

**Every commit, and every pull request description, created in this repository
must end with a `Co-Authored-By: Claude` trailer** — always, without being
asked each time:

```
Co-Authored-By: Claude <noreply@anthropic.com>
```

(Use the specific model name in place of `Claude` if the harness's own
per-session instructions give one, e.g. `Claude Sonnet 5 <noreply@anthropic.com>`
— but the trailer itself is never optional.)
