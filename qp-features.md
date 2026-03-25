# qp — Quickly Please

> From task runner to agent runtime in one file.

qp (Quickly Please) is a local-first, declarative task runner, pipeline engine, and agentic workflow runtime. One Go binary, one YAML file, runs from the command line.

---

## Principles

### The Three Tests

Every feature in `qp` must pass these tests:

1. **Can a human understand it in 60 seconds?** If explaining it requires a taxonomy, it is too diffuse.
2. **Can an agent use it without scraping prose?** Structured output matters more than clever markdown.
3. **Does it reinforce the same mental model?** The model is: *this is how the repo works.* Not: *here are some useful tools.*

### Status and Exit Code Contract

Formalised once; applied consistently throughout. This is a v1 stability commitment.

| State | Terminal | JSON `status` | Process exit code |
|---|---|---|---|
| Success | `PASS` | `pass` | `0` |
| Command failed | `FAIL` | `fail` | `1+` (actual exit code) |
| Binary not found | `FAIL` | `fail` | `127` (standard) |
| Cancelled (parallel) | `CANCELLED` | `cancelled` | `130` (standard) |
| Timeout | `TIMEOUT` | `timeout` | `124` (standard) |

There is no `SKIP` or `skip`. A missing binary is a broken environment — it is a `fail` with exit code `127`, not an intentional non-execution.

---

## Table of Contents

1. [Low-Hanging Fruit](#1-low-hanging-fruit)
2. [Completed Features](#2-completed-features)
3. [In-Progress Features](#3-in-progress-features)
4. [Workflow Runtime (P2)](#4-workflow-runtime-p2)
5. [Server & Orchestration (P1–P3)](#5-server--orchestration-p1p3)
6. [Observability (P1)](#6-observability-p1)
7. [Scaffolding (P1)](#7-scaffolding-p1)
8. [Future: qp Studio (P4)](#8-future-qp-studio-p4)
9. [Open Questions](#9-open-questions)
10. [Implementation Order](#10-implementation-order)
11. [Node Type Summary](#11-node-type-summary)
12. [Full Example: Autonomous Bug Fix Agent](#12-full-example-autonomous-bug-fix-agent)

---

## 1. Low-Hanging Fruit

High-impact, low-effort items that strengthen qp's position in both conventional task running and agent-assisted development. Roughly ordered by effort-to-impact ratio within each section.

---

### 1.1 Competitive Gaps

Features that just, task, or make offer and that users switching to qp will notice are missing.

#### Content-Addressed File Hashing for Cache

**Gap:** task (go-task) and make both support file-based dependency tracking. task's `sources` and `generates` fields let you declare input/output globs, and the task is skipped if inputs haven't changed since the last successful run. make does the same with file modification times. qp's current cache is config-hash based — it knows whether the command/params/env changed but not whether the source files changed.

**Assessment:** Medium effort, but a simplified v1 is achievable. The core loop is: glob the declared paths, hash the file contents, compare against the stored hash. Go's `filepath.WalkDir` + `crypto/sha256` make the mechanics straightforward.

**Suggested shape:**

```yaml
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    cache:
      paths:
        - "**/*.go"
        - go.mod
        - go.sum
```

When `cache.paths` is declared, the cache key includes a content hash of all matching files. When it's absent, the current config-hash behaviour is the fallback. This is additive — existing configs don't change.

The stored hash lives in `.qp/cache/` alongside the existing cache files. On a hit, stdout/stderr are replayed (as they are today). On a miss or when `--no-cache` is passed, the task runs normally.

**Scope it down:** Don't try to handle `generates` (output tracking) in v1. Input hashing alone covers the main use case. Output tracking is a separate, harder problem that can come later.

**Files:** `internal/runner/cache.go` — extend `cacheKeyInput` and `makeCacheKey`, new file globbing + hashing helper

#### Dynamic Variables from Shell Output

**Gap:** task supports `vars` with `sh` values that execute a shell command and capture the output. just has backtick expressions. Both let you do things like:

```yaml
vars:
  git_sha: { sh: "git rev-parse --short HEAD" }
  go_version: { sh: "go env GOVERSION" }
```

qp's `vars` are static strings only. This means common patterns like stamping a build with the current commit hash require inlining the shell command into every task that needs it.

**Suggested shape:**

```yaml
vars:
  region: us-east-1
  git_sha:
    sh: "git rev-parse --short HEAD"
```

If the value is a string, it's static (as today). If it's a mapping with a `sh` key, execute the command at config load time and use the output as the value. Errors during resolution should fail fast with a clear message.

**Effort:** Low-medium. The config unmarshalling needs a custom `UnmarshalYAML` on the vars map to handle both string and mapping values.

**Files:** `internal/config/config.go` — `Vars` type and unmarshalling, resolution in `Load`

#### OS-Specific Task Variants

**Gap:** just has `[linux]`, `[macos]`, and `[windows]` attributes that select recipes based on the current OS. task has a `platforms` field. qp has no equivalent.

**Suggested shape:** Use the existing `when` field with a built-in `os` variable:

```yaml
tasks:
  open-docs:
    desc: Open docs in browser
    cmd: open docs/index.html
    when: os == "darwin"

  open-docs-linux:
    desc: Open docs in browser
    cmd: xdg-open docs/index.html
    when: os == "linux"
```

This already works if `os` is exposed as a CEL variable — which it currently isn't. Adding `"os": runtime.GOOS` to `celVars()` is a one-line fix. No new syntax needed.

**Files:** `internal/runner/runner.go` — `celVars()`

#### Quiet/Silent Task Mode

**Gap:** just has the `@` prefix to suppress echoing a command before execution. make has `@` too. task has `silent: true`.

**Suggested shape:** A `silent: true` field on tasks that suppresses the resolved command from being shown in verbose/event output. Cleaner and more consistent with the YAML-first approach than a prefix character.

**Effort:** Trivial.

**Files:** `internal/config/config.go` — add `Silent` field to `Task`, `internal/runner/command.go` — respect it during output

#### Cleanup / Teardown Steps

**Gap:** task has `defer` which runs cleanup commands even when the task fails — useful for stopping Docker containers, removing temp files, or tearing down test fixtures. qp has no equivalent.

**Suggested shape:**

```yaml
tasks:
  integration:
    desc: Run integration tests
    cmd: go test -tags=integration ./...
    defer: docker compose down
```

`defer` runs after the task completes regardless of success or failure. If `defer` itself fails, the original task status is preserved but the defer failure is logged.

**Effort:** Low-medium.

**Files:** `internal/config/config.go` — add `Defer` field, `internal/runner/command.go` — execute after main command

#### Multi-File Config / Includes

**Gap:** task supports `includes` to split config across multiple files.

**Suggested shape:**

```yaml
includes:
  - tasks/backend.yaml
  - tasks/frontend.yaml
```

Included files are merged into the main config. Task name collisions are an error. Keep it simple — flat merging, no inheritance, no overrides. The main decision is whether included files can define anything (tasks, guards, scopes) or only tasks.

**Files:** `internal/config/config.go` — `Load()`, new merge logic

---

### 1.2 Conventional Task Running

#### Homebrew Tap

The release workflow, cross-platform binaries, and checksums are already in place. A homebrew tap removes the Go toolchain as a prerequisite for anyone on macOS. `brew install neural-chilli/tap/qp` is a much lower barrier than `go install`. A GitHub-hosted tap repo with a single formula is roughly 30 minutes of work.

#### Upward Directory Walk for `qp.yaml`

Every time someone `cd`s into `internal/runner` and types `qp test`, it should just work. This is table stakes for git, cargo, and npm. Walk upward from the current working directory to filesystem root, stop at the first directory containing `qp.yaml`, and set `repoRoot` to that directory. Small change, large ergonomic payoff.

**Files:** `cmd/qp/commands_exec.go` — `loadConfig()`

#### `qp run` as an Explicit Verb

Right now `qp test` works because unknown args fall through to task dispatch. Adding `qp run test` as an explicit synonym costs almost nothing (one case in the switch) and makes the CLI more self-documenting. Both forms should work.

**Files:** `cmd/qp/main.go` — `run()` switch

#### dotenv Loading Feedback

`env_file` exists but is completely silent. A `--verbose` or debug mode that reports `loaded 3 vars from .env` would save real debugging time when a task fails because an env var isn't set. Even logging it to stderr when `--events` is active would help.

**Files:** `internal/runner/command.go` — `loadEnvFile()`

#### Task Timing Summary After Pipelines

When `qp check` finishes, print a one-line summary:

```
3 tasks passed in 4.2s (test: 2.1s, vet: 0.3s, build: 1.8s)
```

Guards already do this with the styled report. Pipelines should too.

**Files:** `cmd/qp/commands_exec.go` — `runTask()`, `cmd/qp/cli_output.go`

---

### 1.3 Agent-Assisted Development

#### Align CLAUDE.md Output with Claude Code Conventions

Claude Code looks for specific patterns in CLAUDE.md — things like "always run X before committing" or "never modify files in Y". The generated CLAUDE.md is good but could be tighter. Study what Claude Code actually picks up on and optimise the generated output to hit those patterns. This is zero new code — just tuning the template strings.

**Files:** `internal/initcmd/agents.go` — `renderAgentDoc()`

#### Single Golden Command in Generated Agent Docs

The generated AGENTS.md says "use `qp guard`" but the golden path for most agent workflows is simpler: run one command, get a pass/fail. Make sure the generated docs prominently feature the single command an agent should run after making changes. Make it unmissable — first thing an agent reads, not buried in a list.

**Files:** `internal/initcmd/agents.go` — `renderAgentDoc()`

#### `qp repair --brief`

The current repair output is thorough but verbose. A `--brief` mode that emits just the failing task, the parsed errors, and the scoped file paths — nothing else — would be ideal for feeding back into an agent's context window without burning tokens on the guard status table and git diff.

**Files:** `internal/repair/repair.go` — `render()`, `cmd/qp/commands_exec.go` — `runRepair()`

#### Multi-Language Codemap Inference

Codemap entries are currently hand-written. A `qp init --codemap` that scans source directories and generates a starter codemap would save significant manual effort and demonstrate that qp is language-agnostic, not Go-specific.

The pattern is already established in `internal/initcmd/` — the import inferrers for Make, just, package.json, Rust, Python, Java, and Docker Compose each implement a small language-specific detector behind a common flow. Codemap inference should follow the same architecture.

**Approach:** Define a common interface:

```go
type CodemapInferrer interface {
    Extensions() []string
    InferPackage(dir string, files []os.DirEntry) *config.CodemapPackage
}
```

Each language implements this by extracting what it can from source files in a directory. The walker scans the repo, matches directories to inferrers by file extension, and assembles the codemap.

**What each language can extract without a full parser:**

The key insight is that you don't need a full AST parser for any of these. First-line doc comments, exported/public type names, and directory structure get you 80% of the value. Simple line-by-line scanning or basic regex is sufficient.

| Language | desc source | key_types | entry_points | Detection |
|---|---|---|---|---|
| **Go** | Package godoc (`// Package foo ...`) | Exported type declarations | Exported functions | `.go` files |
| **Python** | Module docstring in `__init__.py` | Class declarations | Top-level function defs | `__init__.py` or `.py` files |
| **TypeScript/JS** | First JSDoc block in `index.ts`/`index.js` | Exported class/interface/type | Exported functions, default exports | `.ts`, `.tsx`, `.js`, `.jsx` files |
| **Rust** | `//!` doc comment in `lib.rs`/`mod.rs` | `pub struct`/`enum`/`trait` | `pub fn`, `pub async fn` | `.rs` files |
| **Java** | First Javadoc on primary class | Public class/interface/enum | Public static methods | `.java` files |
| **C/C++** | First block comment in header | Class/struct in headers | Functions in headers | `.h`, `.hpp`, `.cpp`, `.c` files |

**Output quality:** Getting 70% right on the first pass is the goal. The user edits the rest. This matches the philosophy of `init --from-repo` which is already explicitly positioned as "a strong starting point, not a source of truth."

Generated output is explicitly marked as a starter:

```yaml
# Generated by qp init --codemap — review and refine
codemap:
  packages:
    internal/runner:
      desc: Core task execution engine
      key_types:
        - Runner
        - Result
      entry_points:
        - New
        - Runner.Run
```

**Effort:** Each language inferrer is 40–80 lines. The walker and assembly logic is another 60–80 lines. Total is roughly 400–600 lines for all six languages plus the framework.

**Files:** new `internal/codemap/infer.go` (walker + interface), new `internal/codemap/infer_*.go` per language, `cmd/qp/main.go` or `internal/initcmd/init.go` for the CLI surface

#### `qp validate --suggest`

When validation passes, optionally suggest improvements: tasks without scopes, scopes without descriptions, guards that don't cover all scoped tasks, codemap packages that don't match any scope. This is the knowledge-accrual flywheel applied to the config itself.

**Files:** `internal/config/config.go` or new `internal/config/suggest.go`, `cmd/qp/commands_basic.go` — `runValidate()`

#### Scope Coverage Reporting

A `qp scope --coverage` or a section in `qp list` that shows which source directories are covered by at least one scope and which are orphaned. Agents work best when scopes are comprehensive. Showing the gaps makes it obvious what to add next.

**Files:** `internal/scope/scope.go`, `cmd/qp/commands_exec.go` — `runList()`

#### Resolved File List in Task Context JSON

When an agent asks for context about a task via `qp context --task X --json`, include the actual file paths within the scope as a flat `files` array. Currently the agent gets scope paths (directory prefixes) but not the resolved file list.

**Files:** `internal/context/sections_general.go` — `agentSection()`

---

### 1.4 Distribution and Discoverability

#### One-Line Install Script

A standard curl-pipe-sh installer that detects OS/arch, downloads the right binary from GitHub releases, and puts it in `/usr/local/bin`:

```bash
curl -sSfL https://raw.githubusercontent.com/neural-chilli/qp/main/install.sh | sh
```

#### Use `qp.yaml` in Other Public Projects

The best marketing for qp is using it visibly in other public repos. Each one is a worked example that demonstrates the value proposition without needing to be explained.

---

### 1.5 Low-Hanging Fruit Summary

| Suggestion | Effort | Impact | Category |
|---|---|---|---|
| Content-addressed file hashing | Medium | High | Competitive gap |
| Dynamic vars from shell output | Low-Med | High | Competitive gap |
| OS-specific tasks via CEL `os` var | Trivial | Medium | Competitive gap |
| Quiet/silent task mode | Trivial | Low | Competitive gap |
| Cleanup/defer steps | Low-Med | Medium | Competitive gap |
| Multi-file config includes | Low-Med | Medium | Competitive gap |
| Homebrew tap | Low | High | Distribution |
| Upward directory walk | Low | High | Ergonomics |
| `qp run` verb | Trivial | Medium | CLI polish |
| dotenv loading feedback | Trivial | Medium | Debugging |
| Pipeline timing summary | Low | Medium | CLI polish |
| CLAUDE.md alignment | Low | High | Agent support |
| Golden command in agent docs | Trivial | High | Agent support |
| `qp repair --brief` | Low | High | Agent support |
| Multi-language codemap inference | Medium | High | Agent support |
| `qp validate --suggest` | Medium | High | Agent support |
| Scope coverage reporting | Low | Medium | Agent support |
| Resolved file list in context JSON | Low | Medium | Agent support |
| One-line install script | Low | High | Distribution |
| qp.yaml in other projects | Trivial | Medium | Discoverability |

---

## 2. Completed Features

These features are implemented and tested. Kept here as a record.

### Rename & Branding (P0) — Solid

Binary, config, module path, docs, tests, and internal references renamed from `fkn` to `qp`. Branding is `Quickly Please`.

### DAG Execution Syntax (P0) — Solid

The `run` field on tasks expresses an arbitrary DAG: `par(...)`, `->`, nested refs, `when(expr, if_true[, if_false])`. Parser, validation, and execution are all implemented.

### diff-plan (P1) — Solid

`qp diff-plan` reads unstaged, staged, and untracked files from git, maps them to scopes/tasks/guards/codemap, and outputs structured results as markdown or JSON.

### Dry Run (P0) — Solid

`--dry-run` prints the resolved command without executing it. Works with safety approval bypass.

### Harness Engineering Support (P1) — Solid

Architecture enforcement, tiered documentation, invariant enforcement, quality scoring, and knowledge accrual are implemented.

### JSON Schema (P1) — Partial

Base `qp.schema.json` shipped. **Remaining:** add `run` to task `oneOf`, add `vars`, `templates`, and `profiles` as top-level properties.

### Generated Docs and Knowledge Accrual (P0) — Solid

`qp init --docs` generates `HUMANS.md`, `AGENTS.md`, and `CLAUDE.md` from `qp.yaml` with managed block markers. `agent.accrue_knowledge` controls whether agent docs instruct agents to propose structured updates.

---

## 3. In-Progress Features

These features have usable core behaviour but are not feature-complete.

---

### 3.1 Daemon Mode (P0)

**Status:** Partial — `qp daemon start/stop/status/restart`; `qp setup --windows`; named-pipe IPC server/client; PowerShell shim install; auto-proxy from normal invocations when daemon is running.

**Remaining:**

- Register as Windows Task Scheduler task for auto-start on login
- Polish first-run nudge behaviour

**Design:** A long-lived background process that takes the Windows AV scan cost once on startup, then serves all subsequent `qp` invocations over a named pipe. On Mac/Linux, direct invocation is already fast (~10ms) so the daemon is not needed — it's a transparent Windows optimisation.

---

### 3.2 Coloured & Formatted Output (P0)

**Status:** Partial — Styled task/guard/error/watch output via lipgloss; `NO_COLOR` + `--no-color` support.

**Remaining:**

- `--verbose` flag to show resolved commands before execution
- `--quiet` flag to suppress informational output

---

### 3.3 CEL Expression Engine (P1)

**Status:** Partial — `internal/cel` evaluator with bool evaluation and validation helpers. `branch()` and `env()` normalisation. Task-level `when` works.

**Remaining:**

- Register `branch` and `env` as proper CEL custom functions instead of string rewriting (current normalisation is fragile)
- Expose `os` as a CEL variable (`runtime.GOOS`) — see Low-Hanging Fruit
- Expose `profile()` function for active profile name
- Richer built-in function set (see CEL built-ins table below)

**Built-in functions (target set):**

| Function | Returns | Description |
|---|---|---|
| `env(name)` | `string` | Environment variable value |
| `branch()` | `string` | Current git branch |
| `os` | `string` | Current OS: `linux`, `darwin`, `windows` |
| `profile()` | `string` | Active profile name |
| `changed(glob)` | `list<string>` | Files changed (git diff) matching glob |
| `tag()` | `string` | Current git tag (empty if none) |
| `param(name)` | `string` | Task parameter value |
| `has_param(name)` | `bool` | Whether parameter was provided |
| `file_exists(p)` | `bool` | Whether a file exists at path p |
| `deps(task)` | `list<Result>` | Results of dependency tasks |
| `state` | `map` | Shared state object (in flows) |

---

### 3.4 Conditional Branching (P1)

**Status:** Partial — Task-level `when` and DAG-level `when(expr, if_true[, if_false])` work.

**Remaining:** `switch(...)` multi-branch node type.

```
switch(cel_expr,
  "value1": subgraph1,
  "value2": subgraph2,
  ...
)
```

Example:

```yaml
run: >
  par(lint, test)
  -> switch(env("TARGET"),
       "api": build-api -> deploy-api,
       "web": build-web -> deploy-web,
       "all": par(build-api, build-web) -> par(deploy-api, deploy-web)
     )
```

Resolution model: branching is resolved in a dedicated phase before execution. Parse → resolve (evaluate all CEL expressions, prune branches, produce a concrete DAG) → execute. The execution engine never sees conditional nodes.

---

### 3.5 NDJSON Event Stream (P1)

**Status:** Partial — `--events` emits `plan/start/output/done/skipped/complete` for task/guard execution.

**Remaining:**

- Richer `plan` event with full resolved graph structure (`nodes` and `edges` arrays)
- `iteration` event for `until` loops (when cycle support lands)
- `approval_required` event for approval gates (when approval gates land)

---

### 3.6 Variables (P0)

**Status:** Partial — Top-level `vars` supported in task command/env interpolation and CEL eval context.

**Remaining:**

- Dynamic `sh` values — see Low-Hanging Fruit
- Environment variable override: `QP_VAR_REGISTRY=gcr.io/production qp run deploy`
- CLI override: `qp run deploy --var registry=gcr.io/production`
- Precedence: CLI > env > YAML defaults

---

### 3.7 Templates (P1)

**Status:** Partial — Top-level `templates` string snippets supported via `{{template.<name>}}` interpolation.

**Remaining:**

- Parameterised templates (reusable task patterns stamped out with different parameters):

```yaml
templates:
  go-service:
    params:
      service: { type: string, required: true }
      port: { type: int, default: 8080 }
    tasks:
      build:
        cmd: go build -o bin/{{param.service}} ./cmd/{{param.service}}
      test:
        cmd: go test ./internal/{{param.service}}/...

tasks:
  auth:
    use: go-service
    params: { service: auth, port: 8081 }
```

Generated task names are namespaced: `auth:build`, `auth:test`. Individual instances can override template-level settings via `override`.

---

### 3.8 Profiles (P1)

**Status:** Partial — Top-level `profiles` overlays for vars and task `when`/`timeout`/`env`; selected via `QP_PROFILE`.

**Remaining:**

- CLI selection: `qp run deploy --profile staging`
- Profile stacking: `qp run deploy --profile staging --profile high-memory`
- `profile()` CEL function for branching on active profile
- Environment-based default: `profiles: { _default: "{{env.QP_PROFILE}}" }`

---

### 3.9 Task Caching / Skip (P0)

**Status:** Partial — Opt-in `cache: true` for cmd tasks; local cache in `.qp/cache`; runtime bypass via `--no-cache`; cache-hit surfaced via skip/event signals.

**Remaining:**

- Content-addressed file hashing via `cache.paths` globs — see Low-Hanging Fruit
- Cache status command: `qp cache status`, `qp cache clean`, `qp cache clean --all`
- Upstream dependency invalidation: if an upstream task ran fresh, downstream must also run fresh

---

### 3.10 Task Retry (P0)

**Status:** Not started.

Simple retry for tasks that fail — covers flaky tests, transient network errors, rate-limited API calls. Simpler than `until` loops, which are for agentic retry with state and reasoning.

```yaml
tasks:
  test:
    cmd: go test ./...
    retry: 3
    retry_delay: 2s

  call-api:
    cmd: curl -f https://api.example.com/webhook
    retry: 5
    retry_delay: 5s
    retry_backoff: exponential  # 5s, 10s, 20s, 40s, 80s
```

| Field | Type | Default | Description |
|---|---|---|---|
| `retry` | `int` | `0` | Max retry attempts (0 = no retry) |
| `retry_delay` | `duration` | `0s` | Wait between retries |
| `retry_backoff` | `string` | `fixed` | `fixed` or `exponential` |
| `retry_on` | `list` | `[any]` | Retry conditions: `any`, `exit_code:N`, `stderr_contains:pattern` |

Each retry attempt emits events:

```jsonl
{"type":"retry","task":"test","attempt":2,"max":3,"reason":"exit_code:1","delay_ms":2000,"ts":"..."}
```

---

### 3.11 Secrets Management (P1)

**Status:** Not started.

Dedicated handling for sensitive values with clear separation from regular config.

```yaml
secrets:
  openai_key:
    from: env
    env: OPENAI_API_KEY

  deploy_token:
    from: env
    env: DEPLOY_TOKEN

  db_password:
    from: file
    path: .qp-secrets
    key: DB_PASSWORD
```

Usage via `{{secret.name}}` interpolation. Secrets are automatically redacted in structured logs, context dumps, event streams, CLI output, and plan JSON.

`.qp-secrets` is a simple key=value file for local development. `qp init` adds it to `.gitignore` automatically.

Convention-based LLM provider integration: secret names `anthropic`, `openai`, `google` are automatically used by the corresponding provider. Explicit override via `api_key: "{{secret.my_custom_key}}"` if needed.

---

## 4. Workflow Runtime (P2)

These features transform qp from a task runner into an agentic workflow runtime. They depend on the P0/P1 foundation being solid.

---

### 4.1 LLM Node Type

A built-in task type that calls an LLM API as a graph node.

**Design philosophy:** No framework dependencies. No langchaingo, no langchain, no SDKs. Each LLM provider is a thin HTTP client (~80 lines) behind a simple interface:

```go
type LLMProvider interface {
    Complete(ctx context.Context, req CompletionReq) (CompletionResp, error)
}
```

**Supported providers:**

| Provider | Env var for API key | Models |
|---|---|---|
| Anthropic | `ANTHROPIC_API_KEY` | claude-sonnet-4-20250514 etc |
| OpenAI | `OPENAI_API_KEY` | gpt-4o, gpt-4o-mini etc |
| Google | `GOOGLE_API_KEY` | gemini-2.0-flash etc |

**YAML syntax:**

```yaml
tasks:
  analyze:
    type: llm
    provider: openai
    model: gpt-4o
    system: "You are a code reviewer."
    prompt: |
      Review this code for bugs:
      {{state.code}}
    output: state.review
    structured: true
    temperature: 0.2
    max_tokens: 2000
```

Prompts support `{{state.field}}` interpolation using Go's `text/template`. When `structured: true`, the response is parsed as JSON before writing to state. HTTP errors are retried with exponential backoff (3 attempts).

---

### 4.2 Shared State

A JSON state object that flows between nodes in a flow, enabling data passing and accumulation.

```yaml
flows:
  my-flow:
    state:
      input_file: "{{param.file}}"
      analysis: null
      decision: null

    run: analyze -> decide -> act
```

**State mechanics:**

- Initialised from the `state` block at flow start
- Each task reads state (via template interpolation in prompts/commands, or via CEL)
- Each task writes to state (via the `output` field)
- Writes are merged, not replaced (JSON merge patch semantics)
- Parallel tasks each receive a snapshot; outputs are merged after all complete (last-finished-wins with a warning on conflicts)

**State access by node type:**

| Node type | Reads state via | Writes state via |
|---|---|---|
| `cmd` | `state_input` (template/env/file/stdin) | stdout → `output` path |
| `llm` | `{{state.x}}` in prompt templates | Response → `output` path |
| `mcp` | `{{state.x}}` in `input` templates | Result → `output` path |
| `expr` | `state.x` natively in CEL | CEL eval → named fields |
| `approve` | `{{state.x}}` in `message` | Decision → `output` path |

**How cmd tasks read state (`state_input`):**

| Value | Behaviour |
|---|---|
| `template` | Default. `{{state.x}}` resolved in `cmd` string before execution |
| `env` | `QP_STATE` env var contains full state as JSON string |
| `file` | `QP_STATE_FILE` env var points to a temp JSON file |
| `stdin` | State JSON piped to the command's stdin |

**How cmd tasks write state (`output_format`):**

| Value | Behaviour |
|---|---|
| `json` | Default. Parse entire stdout as one JSON value |
| `text` | Store stdout as a raw string at the output path |
| `ndjson` | Parse each line of stdout as JSON, collect into array |

**State access in CEL:** CEL's native access syntax covers most JSONPath patterns without needing a separate query language. CEL's `filter`, `map`, `exists`, `all` macros are more expressive than JSONPath for workflow conditions:

```yaml
when: >
  state.test_results
    .filter(r, r.status == 'fail')
    .filter(r, r.module.startsWith('core/'))
    .size() > 3
```

---

### 4.3 CEL + LLM Evaluation (Smart Branching)

The pattern where an LLM produces a structured evaluation and CEL uses that evaluation to make routing decisions. Not a separate feature — it emerges from combining LLM nodes, shared state, and CEL branching.

**Pattern:** LLM receives context → returns structured JSON → CEL reads evaluation from state → `when`/`switch` routes execution.

```yaml
flows:
  triage:
    state:
      error_log: "{{param.log}}"
      evaluation: null

    run: >
      evaluate
      -> switch(state.evaluation.severity,
           "critical": when(state.evaluation.fixable, auto-fix, page-oncall),
           "medium": create-ticket,
           "low": log-and-continue
         )

    tasks:
      evaluate:
        type: llm
        model: gpt-4o
        prompt: |
          Analyse this error log and classify it.
          Error: {{state.error_log}}
          Respond with JSON:
          {"severity": "low"|"medium"|"critical", "fixable": true|false, "reasoning": "..."}
        output: state.evaluation
        structured: true
```

**Key insight:** The LLM provides the judgement, CEL provides the deterministic routing logic, state provides the data flow. The branching decision is auditable — you can inspect `state.evaluation` in the event stream and see exactly why the flow took the path it did.

---

### 4.4 MCP Client Node Type

A task type that calls tools on external MCP servers, making qp an orchestrator for any MCP-compatible service.

```yaml
flows:
  research:
    mcp_servers:
      - name: knowledge
        url: "http://localhost:8100/sse"
      - name: jira
        url: "http://localhost:8300/sse"

    run: search -> analyze -> create-ticket

    tasks:
      search:
        type: mcp
        server: knowledge
        tool: semantic_search
        input:
          query: "{{state.topic}}"
          top_k: 5
        output: state.search_results
```

**Transports:** `http://` or `https://` for SSE, `stdio://command` for subprocess. Auth via headers.

---

### 4.5 Cycle Support (`until`)

A DAG node type that enables bounded loops — the primitive required for agentic retry/refine patterns.

```
until(subgraph, cel_condition, max: N)
```

- Runs `subgraph`
- Evaluates `cel_condition` against current state
- If true: exits the loop
- If false: runs `subgraph` again
- Hard stop at `max` iterations (required — no unbounded loops)

```yaml
run: >
  diagnose
  -> until(
       generate-fix -> apply -> test,
       state.tests_pass == true,
       max: 5
     )
  -> when(state.tests_pass, create-pr, escalate)
```

**Safety:** `max` is required. CEL's termination guarantee means the exit condition always evaluates. Static analysis warns if the subgraph can't modify the state fields referenced in the condition.

Each iteration emits an event:

```jsonl
{"type":"iteration","loop":"until_1","attempt":2,"max":5,"condition_result":false,"ts":"..."}
```

---

### 4.6 Expression Node Type

A task type that evaluates a CEL expression and updates state. For lightweight transformations between nodes.

```yaml
tasks:
  prepare:
    type: expr
    eval:
      total_failures: "state.test_results.filter(r, r.status == 'fail').size()"
      needs_review: "state.total_failures > 5 || state.coverage < 80.0"
      summary: "'Found ' + string(state.total_failures) + ' failures'"
```

Each key-value pair is a state field and a CEL expression. Evaluated in declaration order so later expressions can reference earlier ones.

---

## 5. Server & Orchestration (P1–P3)

---

### 5.1 Declarative MCP Server Builder (P1)

**Status:** Dormant internally. The MCP implementation exists in `internal/mcp/` with full tool, resource, prompt, and transport support. It is intentionally hidden from the public CLI and docs surface while the product direction focuses on direct CLI use and generated docs.

Any task or flow with `expose: true` is automatically registered as an MCP tool when `qp serve` is running. This makes qp a zero-code MCP server builder.

```yaml
tasks:
  health-check:
    expose: true
    description: "Check service health"
    cmd: curl -s https://api.internal/health

  restart-service:
    expose: true
    description: "Restart a named service"
    params:
      service: { type: string, required: true, description: "Service name" }
    cmd: systemctl restart {{param.service}}
```

```bash
# Stdio transport (default — for Claude Code, IDE integrations)
$ qp serve

# SSE transport (for remote/HTTP clients)
$ qp serve --transport sse --addr :9119
```

**Security:**

- Opt-in exposure: `expose: true` required
- Auth: API key or mTLS for remote, open for localhost
- Parameter validation via CEL
- Execution limits: max concurrent flows, per-flow timeout ceiling
- Audit stream: every invocation logged

---

### 5.2 Concurrency Control (P1)

Per-task and global concurrency limits for `qp serve` mode.

```yaml
tasks:
  health-check:
    expose: true
    concurrency: 0          # unlimited (default)
    cmd: curl -s https://api.internal/health

  restart-service:
    expose: true
    concurrency: 1          # mutex — only one restart at a time
    cmd: systemctl restart {{param.service}}

  deploy:
    expose: true
    concurrency: 1
    queue: true              # don't reject when full, queue and wait
    queue_timeout: 5m
    cmd: ./deploy.sh
```

| Value | Meaning |
|---|---|
| `0` | Unlimited (default) |
| `1` | Mutex |
| `N` | Bounded — up to N concurrent invocations |

Global serve configuration:

```yaml
serve:
  max_concurrent: 20
  default_task_concurrency: 0
  drain_timeout: 30s
  request_timeout: 5m
  transport: stdio
  addr: ":9119"
```

Implementation: `map[string]chan struct{}` — one buffered channel per task, sized to the concurrency limit.

---

### 5.3 Approval Gate Node Type (P3)

A task type that pauses execution and waits for human input.

```yaml
tasks:
  approve-deploy:
    type: approve
    message: "Deploy {{state.version}} to production?"
    options: [approve, reject, defer]
    output: state.approval_decision
    timeout: 30m
```

| Context | Behaviour |
|---|---|
| CLI | Interactive prompt on stdin, or skip with `--approve-all` |
| Event stream | Emits `{"type":"approval_required",...}` and blocks |
| MCP | Returns tool result requesting approval, resumes on reply |

---

### 5.4 Flows as MCP Tools (P3)

Flows with `expose: true` are callable via qp's MCP server interface, making every flow a tool that any agent or MCP client can invoke.

```yaml
flows:
  bug-fix:
    expose: true
    description: "Autonomously diagnose and fix a bug"
    params:
      issue: { type: string, required: true, description: "Issue description" }
      repo: { type: string, default: "." }

    run: gather -> diagnose -> until(fix -> test, state.pass, max: 5) -> pr
```

Distributed composition: flows can call other qp instances' flows via `type: mcp`.

---

## 6. Observability (P1)

---

### 6.1 Structured Logging

Structured JSON logging via `github.com/rs/zerolog` across all task execution, flow orchestration, and LLM/MCP interactions.

```bash
$ qp run release --log-format json
$ qp run release --log-format json --log-target stderr
$ qp run release --log-file /var/log/qp/release.jsonl
```

| Flag | Values | Default | Description |
|---|---|---|---|
| `--log-format` | `text`, `json` | `text` | Output format |
| `--log-target` | `stdout`, `stderr` | `stdout` | Where console log output is written |
| `--log-file` | file path | none | Additionally write structured JSON logs to a file (always JSON) |
| `--log-level` | `debug`, `info`, `warn`, `error` | `info` | Minimum log level |

**Standard fields:** `run_id`, `flow`, `task`, `status`, `duration_ms`, `attempt`, `trigger` (`cli`, `shell`, `mcp`, `watch`), `caller` (MCP identity), `params`.

**LLM call logging:** Provider, model, token usage, prompt hash. Full prompt/response logging is opt-in via `logging.log_llm_content: true`.

**Flow audit records:** At flow completion, a summary log entry captures the complete audit trail including params, final state, resolved graph, trigger, and caller.

---

### 6.2 Context Dump

Full execution context dump for debugging and audit.

```bash
$ qp run release --dump-context release-debug.json
$ qp run release --dump-context-at build context.json
$ qp run release --dump-context-on-fail debug.json
$ qp run release --dump-context -
```

**Context schema includes:** `run_id`, `params`, `environment`, `config` (file + hash), `resolved_graph`, `cel_evaluations` (every branching decision with expression, result, and branch taken), `state_snapshots` (state at every step boundary), `tasks` (full detail per task), `mcp_calls`.

The context dump is designed to be fed directly to an LLM for analysis. An LLM reading it has everything it needs to reason about what happened.

**Sensitive data handling:**

```yaml
logging:
  context_dump:
    redact_env: ["API_KEY", "SECRET_*"]
    include_llm_content: true
    include_task_output: true
    max_output_bytes: 102400
```

---

## 7. Scaffolding (P1)

---

### 7.1 Project Scaffolding

Generate project structure, documentation stubs, and domain skeletons from the architecture declaration in `qp.yaml`.

```bash
$ qp init --scaffold
$ qp init --scaffold go-service
```

**Built-in scaffold templates:**

| Template | Description |
|---|---|
| `default` | Minimal harness: `qp.yaml`, AGENTS.md, docs stubs |
| `service` | Layered service with domain architecture |
| `monorepo` | Multi-service monorepo with shared infrastructure |
| `cli` | CLI tool with feature-based layout |
| `library` | Library with public API surface documentation |

Templates are language-agnostic — they generate harness infrastructure (config, docs, architecture rules) but not application code.

### 7.2 Domain Scaffolding

```bash
$ qp scaffold domain payments
```

Generates directory structure, documentation stubs, scope definitions, and task entries conforming to the declared architecture.

### 7.3 Sync Mode

```bash
$ qp scaffold sync
```

Reconcile the project structure against the architecture declaration. Generates missing directories and doc stubs without touching existing files.

### 7.4 External Templates

```bash
$ qp init --scaffold https://github.com/myorg/qp-templates/go-service
```

A scaffold template is a directory containing `template.yaml` (metadata and variable declarations) and `*.tmpl` files using Go `text/template` syntax.

---

## 8. Future: qp Studio (P4)

A visual editor for qp flows. Deferred until the YAML schema is stable.

### Why it works

- The YAML file IS the data model — no intermediate representation
- Round-trips perfectly: edit in UI → writes YAML, edit YAML → UI reflects changes
- JSON Schema drives form generation for task configuration panels
- Five node types with known schemas = simple palette

### Likely stack

- ReactFlow or Vue Flow for the graph canvas
- PrimeVue for panels and forms
- Reads and writes `qp.yaml` directly
- Ships as a separate binary or `qp studio` subcommand

### Not building now because

- Need the schema to stabilise through real usage first
- CLI-first ensures the data model is shaped by the domain, not by UI components

---

## 9. Open Questions

| # | Question | Notes |
|---|---|---|
| 1 | Should `init --from-repo` imply `--docs`? | A strong suggestion may be better than surprising users with generated files |
| 2 | Monorepo: per-subdirectory `qp.yaml` with inheritance? | Strong candidate for v1.1; relates to multi-file includes in Low-Hanging Fruit |
| 3 | Should `qp init` offer language-specific starter templates? | Nice to have; not critical path |
| 4 | `.qp/` directory — should location be configurable? | Defaulting to repo root is simplest; revisit for monorepo support |
| 5 | Should guard cache include `resolved_cmd` per step? | Low cost; useful for context doc accuracy |
| 6 | Parallel `continue_on_error` for v1.1? | Expected to be the #1 feature request; semantics are clear (wait for all, non-zero if any fail) |

---

## 10. Implementation Order

| Phase | Features | Competes with |
|---|---|---|
| **Now** | Low-hanging fruit: competitive gaps, agent doc alignment, codemap inference, ergonomic polish, distribution | — |
| **P0** | Task retry, remaining daemon polish, remaining coloured output | make, just |
| **P1** | CEL improvements, switch branching, profiles CLI, templates composition, content-addressed caching, secrets, structured logging, context dump, concurrency control, scaffolding | Jenkins, GitHub Actions |
| **P2** | LLM nodes, shared state, smart branching, MCP client, until, expr node | LangGraph, Dagger |
| **P3** | Approval gates, flows as MCP tools, security | Temporal, Prefect |
| **P4** | qp Studio | n8n, Retool |

Each phase is independently useful and validates the next. Ship now, get users. Ship P1, get teams. Ship P2, get believers.

---

## 11. Node Type Summary

| Type | Purpose | Input | Output |
|---|---|---|---|
| `cmd` | Run a shell command | Command string | stdout → state |
| `llm` | Call an LLM API | Prompt template | Response → state |
| `mcp` | Call a tool on an external MCP server | Tool + input | Result → state |
| `expr` | Evaluate CEL and update state | CEL expressions | State mutation |
| `approve` | Human-in-the-loop gate | Message + options | Decision → state |

---

## 12. Full Example: Autonomous Bug Fix Agent

```yaml
flows:
  bug-fix-agent:
    expose: true
    description: "Diagnose and fix a bug autonomously"
    params:
      issue: { type: string, required: true }

    state:
      issue: "{{param.issue}}"
      code_context: null
      diagnosis: null
      patch: null
      tests_pass: false
      attempt: 0

    run: >
      gather-context
      -> diagnose
      -> until(
           generate-fix -> apply-patch -> run-tests,
           state.tests_pass == true,
           max: 5
         )
      -> when(state.tests_pass, create-pr, escalate)

    tasks:
      gather-context:
        type: cmd
        cmd: qp context --task test --json
        output: state.code_context

      diagnose:
        type: llm
        model: gpt-4o
        prompt: |
          Issue: {{state.issue}}
          Relevant code: {{state.code_context}}
          Diagnose the root cause.
        output: state.diagnosis

      generate-fix:
        type: llm
        model: gpt-4o
        prompt: |
          Diagnosis: {{state.diagnosis}}
          Attempt: {{state.attempt}}
          Previous patch (if any): {{state.patch}}
          Generate a fix as a unified diff.
        output: state.patch

      apply-patch:
        type: cmd
        cmd: echo '{{state.patch}}' | git apply

      run-tests:
        type: cmd
        cmd: qp run test --json
        output: state.tests_pass

      create-pr:
        type: cmd
        cmd: gh pr create --title "fix: {{state.issue}}" --body "{{state.diagnosis}}"

      escalate:
        type: mcp
        server: slack
        tool: send_message
        input:
          channel: "#engineering"
          text: "Failed to auto-fix after 5 attempts: {{state.issue}}"
```

---

*qp — Quickly Please. qp never forgets.*
