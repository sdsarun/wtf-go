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

**No AI/Claude co-author attribution** — commits and PR descriptions in this
repo must not carry a `Co-Authored-By: Claude` trailer or a "Generated with
Claude Code" line. This is enforced via `.claude/settings.local.json`
(`attribution.commit` / `attribution.pr` both set to `""`), not by asking each
time.
