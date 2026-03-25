# Completed Features

This file tracks implemented roadmap items and delivery depth.

Depth legend:

- `Scaffolded`: command surface/basic wiring exists.
- `Partial`: usable core behavior, not feature-complete.
- `Solid`: implemented and tested for current scope.

## P0

| Feature | Depth | Notes |
|---|---|---|
| Rename & Branding (`fkn` -> `qp`) | Solid | Binary/config/schema/module/docs/tests renamed; branding now `Quickly Please`. |
| Daemon Mode (Windows) | Solid | `qp daemon start/stop/status/restart`; `qp setup --windows`; named-pipe IPC server/client; PowerShell shim install; auto-proxy from normal invocations when daemon is running; setup now also registers a Task Scheduler auto-start entry and first-run nudge output is throttled. |
| Coloured & Formatted Output | Solid | Styled task/guard/error/watch output via lipgloss; `NO_COLOR` + `--no-color`; `--verbose` command preview; `--quiet` informational suppression. |
| DAG Execution Syntax (`run`) | Solid | `par(...)`, `->`, nested refs, parser + validation + execution. |

## P1

| Feature | Depth | Notes |
|---|---|---|
| CEL Expression Engine | Partial | `internal/cel` evaluator with bool evaluation and validation helpers; registered `branch()`, `env()`, `profile()`, `tag()`, `param()`, `has_param()`, `file_exists()`, and `os` variable. |
| Conditional Branching | Solid | Task-level `when`; DAG-level `when(expr, if_true[, if_false])`; `switch(...)` multi-branch run-expression support. |
| NDJSON Event Stream | Solid | `--events` emits `plan/start/output/done/skipped/complete` with plan graph `nodes`/`edges`; includes `retry`, `iteration`, and `approval_required` event types. |
| Variables | Solid | Top-level `vars` (static or shell-resolved), env overrides via `QP_VAR_*`, and CLI overrides via `--var` with precedence `CLI > env > YAML`. |
| Templates | Solid | String snippets via `{{template.<name>}}` plus parameterized task templates expanded via `use`, instance `params`, and per-task `override`. |
| Profiles | Solid | Profile overlays for vars/task `when`/`timeout`/`env`; selection via `QP_PROFILE`, CLI `--profile` (including stacking), and `_default` profile expression support. |
| Task Caching / Skip | Solid | Opt-in command-task cache with optional `cache.paths` content hashing, runtime `--no-cache`, cache hit signaling/events, `qp cache status/clean`, and upstream-fresh invalidation. |
| Task Retry | Solid | Task-level retry policy via `retry`, `retry_delay`, `retry_backoff`, `retry_on`, including `retry` events in NDJSON streams. |
| Secrets Management | Partial | Top-level `secrets` from env/file sources, `{{secret.<name>}}` interpolation, and output/event/result redaction. |

## In Progress / Not Yet

| Feature | Status |
|---|---|
| Harness Engineering Support | Partial |
| Scaffolding (harness-focused) | Partial |
| LLM Node Type | Not started |
| Shared State | Not started |
| CEL + LLM Evaluation | Not started |
| MCP Client Node Type | Not started |
| `until` Cycle Support | Not started |
| Expression Node Type | Not started |
| Approval Gate Node Type | Not started |
| Flows as MCP Tools | Not started |
| Declarative MCP Server Builder | Not started |
| Concurrency Control | Not started |
| Structured Logging | Not started |
| Context Dump | Not started |
| Advanced Dry Run / Advanced Diff-Plan / Harness extras | Not started |

### Harness Engineering Notes (Current)

- `qp arch-check` added with JSON output (`--json`) and non-zero exit on violations.
- `architecture` config now supports:
  - `layers`
  - `domains.<name>.root`
  - `domains.<name>.layers`
  - `rules` with `direction: forward`, `cross_domain: allow|deny`, and `cross_cutting`.
- Initial implementation currently checks Go imports resolved via module path in `go.mod`.
- `qp init --harness` now scaffolds:
  - an `arch-check` task
  - a starter `architecture` block
  - `check` pipeline wiring that includes `arch-check`
