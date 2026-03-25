# qp — Quickly Please

> From task runner to agent runtime in one file.

qp (Quickly Please) is a local-first, declarative task runner, pipeline engine, and agentic workflow runtime. One Go binary, one YAML file, runs from the command line.

---

## Table of Contents

1. [Rename & Branding](#1-rename--branding)
2. [Daemon Mode](#2-daemon-mode)
3. [Coloured & Formatted Output](#3-coloured--formatted-output)
4. [DAG Execution Syntax](#4-dag-execution-syntax)
5. [CEL Expression Engine](#5-cel-expression-engine)
6. [Conditional Branching](#6-conditional-branching)
7. [NDJSON Event Stream](#7-ndjson-event-stream)
8. [LLM Node Type](#8-llm-node-type)
9. [Shared State](#9-shared-state)
10. [CEL + LLM Evaluation (Smart Branching)](#10-cel--llm-evaluation-smart-branching)
11. [MCP Client Node Type](#11-mcp-client-node-type)
12. [Cycle Support (until)](#12-cycle-support-until)
13. [Expression Node Type](#13-expression-node-type)
14. [Approval Gate Node Type](#14-approval-gate-node-type)
15. [Flows as MCP Tools (Server)](#15-flows-as-mcp-tools-server)
16. [Declarative MCP Server Builder](#16-declarative-mcp-server-builder)
17. [Concurrency Control](#17-concurrency-control)
18. [JSON Schema for qp.yaml](#18-json-schema-for-qpyaml)
19. [Structured Logging](#19-structured-logging)
20. [Context Dump](#20-context-dump)
21. [Variables](#21-variables)
22. [Profiles](#22-profiles)
23. [Templates](#23-templates)
24. [Task Caching / Skip](#24-task-caching--skip)
25. [Secrets Management](#25-secrets-management)
26. [diff-plan](#26-diff-plan)
27. [Task Retry](#27-task-retry)
28. [Dry Run](#28-dry-run)
29. [Harness Engineering Support](#29-harness-engineering-support)
30. [Scaffolding](#30-scaffolding)
31. [Future: qp Studio (UI)](#31-future-qp-studio-ui)

---

## 1. Rename & Branding

**Priority: P0 — do first**

Rename from `fkn` to `qp` across the entire codebase.

### What to change

- Binary name: `fkn` → `qp`
- Config file: `fkn.yaml` → `qp.yaml`
- Go module path
- All internal references, docs, README, MCP tool names
- AGENTS.md and any agent guidance files
- CLI help text and error messages

### Backward compatibility

- None required for initial rename in this repo; `qp.yaml` is the only supported config filename.

### Name

- **Q**uickly **P**lease
- Also works as **Q**ueue & **P**rocess (describes the DAG engine)
- README intro: *"qp (Quickly Please) — a task runner that gets on with it."*
- Mascot: an elephant (the letters q and p form an elephant face — two round eyes with a trunk in the middle)
- *qp never forgets.*

---

## 2. Daemon Mode

**Priority: P0**

A long-lived background process that takes the Windows AV scan cost once on startup, then serves all subsequent `qp` invocations over a named pipe. On Mac/Linux, direct invocation is already fast (~10ms) so the daemon is not needed — it's a transparent Windows optimisation.

### Motivation

On corporate Windows machines with endpoint protection (CrowdStrike, Carbon Black, Defender), every binary invocation pays ~1 second of AV scanning overhead. Even a Go "Hello World" takes the same hit. The daemon amortises this to a single scan on startup. All subsequent calls go through a PowerShell function that communicates over a named pipe — no `CreateProcess`, no scan, ~2-3ms.

### How it works

The `qp` Go binary runs in daemon mode, listening on a Windows named pipe. A lightweight PowerShell function (installed in the user's `$PROFILE`) replaces the binary in PowerShell's resolution order. Since PowerShell resolves functions before external binaries, `qp build` always hits the function, which connects to the daemon over the pipe, sends the arguments, proxies stdout/stderr/exit code back, and returns. The user never knows the daemon exists.

### PowerShell client function

```powershell
function qp {
    $pipe = [System.IO.Pipes.NamedPipeClientStream]::new(".", "qp-daemon", "InOut")
    $pipe.Connect(1000)
    $writer = [System.IO.StreamWriter]::new($pipe)
    $reader = [System.IO.StreamReader]::new($pipe)

    $writer.WriteLine(($args -join "`t"))
    $writer.Flush()

    while ($line = $reader.ReadLine()) {
        if ($line.StartsWith("ERR:")) { [Console]::Error.WriteLine($line.Substring(4)) }
        elseif ($line.StartsWith("EXIT:")) { exit [int]$line.Substring(5) }
        else { Write-Output $line }
    }
    $pipe.Dispose()
}
```

This function uses PowerShell's native .NET interop — no external dependencies, no binary to scan. It loads once when the profile loads and every call after is pure pipe I/O.

### CLI composability

Because the PowerShell function emits output via `Write-Output` and `[Console]::Error.WriteLine`, all standard CLI patterns work:

```powershell
qp lint | Select-String "unused"     # piping works
qp test 2>errors.log                  # redirection works
qp build && qp deploy                 # chaining works
```

Git hooks, VS Code tasks, Makefiles calling qp — all work transparently. No modal interface, no context switching. qp is just another command in your normal terminal.

### Setup

```bash
$ qp setup --windows
  ✓ Started qp daemon (PID 12345)
  ✓ Registered daemon as scheduled task (starts on login)
  ✓ Added qp function to PowerShell profile

  Restart your terminal to activate.
```

`qp setup --windows` is a one-time operation (taking the single AV scan hit). It:

1. Starts the daemon process
2. Registers it as a Windows Task Scheduler task to auto-start on login
3. Writes the PowerShell function to `$PROFILE`

From that point on, every `qp` call is 2-3ms.

### First-run nudge

On Windows, if the daemon isn't running, the Go binary runs normally (taking the 1s AV hit) and prints a hint:

```
Tip: run 'qp setup --windows' for faster execution (~2ms vs ~1000ms)
```

One gentle nudge, then it's the user's choice. The binary always works without the daemon — it's just slower.

### Daemon lifecycle

```bash
$ qp daemon start         # start manually
$ qp daemon stop          # stop manually
$ qp daemon status        # check if running, PID, uptime
$ qp daemon restart       # restart (picks up new qp binary version)
```

The daemon:

- Listens on a Windows named pipe (`\\.\pipe\qp-daemon`)
- Handles multiple concurrent requests (one goroutine per connection)
- Auto-restarts on crash (Task Scheduler handles this)
- Logs to `~/.qp/daemon.log` for diagnostics
- Respects `qp.yaml` changes (reloads config on each request, not on startup — so config changes are immediate)

### Mac/Linux

On Mac/Linux, `qp setup` is a no-op or just installs shell completions. No daemon is needed because direct binary invocation is already ~10ms. The daemon exists purely to solve the Windows AV problem.

### Implementation

The daemon adds approximately 200 lines on top of the existing task execution engine:

- Named pipe listener: ~40 lines
- Message framing (args in, stdout/stderr/exit code out): ~60 lines
- PowerShell client function: ~15 lines
- `qp daemon start/stop/status`: ~30 lines
- `qp setup --windows`: ~50 lines

---

## 3. Coloured & Formatted Output

**Priority: P0**

Use lipgloss for styled terminal output in both daemon mode responses and standard CLI execution.

### Requirements

- Task names in **bold**
- Status indicators: ✓ green for pass, ✗ red for fail, ⏭ yellow for skipped, ⟳ blue for running
- Timing in dim/grey
- Structured error output with highlighted file paths and line numbers
- Colour output auto-detected (disable if not a TTY or if `NO_COLOR` env is set)
- `--no-color` flag to force plain output

### Suggested library

- `github.com/charmbracelet/lipgloss` for styling

---

## 4. DAG Execution Syntax

**Priority: P0**

A `run` field on tasks that expresses an arbitrary DAG of execution in a single inline expression.

### Syntax

```
par(a, b, c)       # run a, b, c in parallel
a -> b -> c         # run a, b, c sequentially
par(a, b) -> c      # run a and b in parallel, then c
a -> par(b, c) -> d # a first, then b and c in parallel, then d
```

### Composition by reference

A task name inside `run` inlines that task's own graph:

```yaml
tasks:
  checks:
    run: par(lint, test, audit)
  release:
    run: checks -> build -> deploy
```

`checks` is expanded inline — the engine sees `par(lint, test, audit) -> build -> deploy`.

### Parser

The `run` string is parsed into an AST with the following node types:

| Node      | Syntax             | Meaning                          |
|-----------|--------------------|----------------------------------|
| Sequence  | `a -> b`           | Run a then b                     |
| Parallel  | `par(a, b, c)`     | Run a, b, c concurrently         |
| Reference | `taskname`         | Inline another task's graph      |

### Error handling

- Parallel groups fail fast: if any member fails, cancel siblings and propagate failure
- Sequential chains stop on first failure
- Cycle detection at config validation time (not runtime)

### Relationship to `needs`

The existing `needs` field continues to work as before. `run` is a more expressive alternative for complex topologies. A task should not use both `needs` and `run` — validate and error if both are present.

---

## 5. CEL Expression Engine

**Priority: P1**

Integrate Google's Common Expression Language (CEL) via `github.com/google/cel-go` as the expression engine for conditional logic.

### Why CEL

- Guaranteed to terminate (no loops, no recursion)
- No side effects
- Type-safe
- Clean syntax with macros: `exists`, `all`, `filter`, `map`, `has`, `size`
- `in` operator for list/map membership
- Used in Kubernetes, Firebase, Envoy — stable and well-maintained
- Performant (compiles to bytecode)

### Built-in functions exposed to CEL

| Function         | Returns       | Description                                  |
|------------------|---------------|----------------------------------------------|
| `env(name)`      | `string`      | Environment variable value                   |
| `branch()`       | `string`      | Current git branch                           |
| `changed(glob)`  | `list<string>`| Files changed (git diff) matching glob       |
| `tag()`          | `string`      | Current git tag (empty if none)              |
| `param(name)`    | `string`      | Task parameter value                         |
| `has_param(name)`| `bool`        | Whether parameter was provided               |
| `file_exists(p)` | `bool`        | Whether a file exists at path p              |
| `deps(task)`     | `list<Result>`| Results of dependency tasks                  |
| `state`          | `map`         | Shared state object (in flows)               |

### Usage in tasks

```yaml
tasks:
  deploy:
    when: env("BRANCH") == "main" && changed("src/").size() > 0
    cmd: ./deploy.sh
```

The `when` field is a CEL expression. If it evaluates to `false`, the task is skipped. The `skipped` reason is included in JSON output and the event stream.

---

## 6. Conditional Branching

**Priority: P1**

Two new node types in the `run` DAG syntax that use CEL for runtime branching.

### when (ternary)

```
when(cel_expr, if_true)
when(cel_expr, if_true, if_false)
```

If `if_false` is omitted and the condition is false, the node is skipped and the graph reconnects around it.

```yaml
run: >
  par(lint, test)
  -> build
  -> when(env("ENV") == "prod", par(canary, smoke), smoke)
  -> deploy
  -> when(branch() == "main", notify-slack)
```

### switch (multi-branch)

```
switch(cel_expr,
  "value1": subgraph1,
  "value2": subgraph2,
  ...
)
```

```yaml
run: >
  par(lint, test)
  -> switch(env("TARGET"),
       "api": build-api -> deploy-api,
       "web": build-web -> deploy-web,
       "all": par(build-api, build-web) -> par(deploy-api, deploy-web)
     )
```

### Resolution model

Branching is resolved in a dedicated phase before execution:

1. **Parse** — `run` string → AST
2. **Resolve** — evaluate all `when`/`switch` CEL expressions, prune branches, produce a concrete DAG
3. **Execute** — run the resolved DAG

This means the execution engine never sees conditional nodes. It always runs a plain DAG. This keeps the executor simple.

### Plan output

`qp plan` shows the resolved graph for the current environment:

```bash
$ qp plan release --env ENV=prod
  ✓ lint ─┐
  ✓ test ─┼─ build ─ canary ─┐
  ✓ audit─┘          smoke ──┴─ deploy ─ notify-slack

$ qp plan release --env ENV=staging
  ✓ lint ─┐
  ✓ test ─┼─ build ─ smoke ─ deploy
  ✓ audit─┘
```

`qp plan --json` returns the resolved graph as structured JSON — critical for agent consumption.

---

## 7. NDJSON Event Stream

**Priority: P1**

Real-time structured event output during task execution, enabling TUI dashboards, web visualisers, IDE extensions, and agent monitoring.

### Usage

```bash
# Events on stderr, task stdout/stderr on stdout
$ qp run release --events 2>events.jsonl

# Everything structured on stdout (machine consumption)
$ qp run release --events --json
```

### Event types

```jsonl
{"type":"plan","graph":{"nodes":[...],"edges":[...]},"ts":"..."}
{"type":"start","task":"lint","ts":"..."}
{"type":"output","task":"lint","stream":"stdout","line":"...","ts":"..."}
{"type":"done","task":"lint","status":"pass","duration_ms":320,"ts":"..."}
{"type":"skipped","task":"build","reason":"dependency 'test' failed","ts":"..."}
{"type":"complete","status":"fail","duration_ms":4210,"ts":"..."}
```

| Type       | Meaning                                          |
|------------|--------------------------------------------------|
| `plan`     | Resolved graph (always first event)              |
| `start`    | Task execution began                             |
| `output`   | A line of stdout/stderr from a task              |
| `done`     | Task completed (status: pass, fail, error)       |
| `skipped`  | Task pruned (dependency failure or CEL condition) |
| `complete` | Entire run finished                              |

### Design notes

- First event is always `plan` — consumers get the full topology before any execution
- Every event has a `ts` field (ISO 8601 with milliseconds)
- NDJSON chosen over SSE for simplicity and piping compatibility
- The stream is the foundation for all downstream visualisation (TUI, web, IDE)

---

## 8. LLM Node Type

**Priority: P2**

A built-in task type that calls an LLM API as a graph node.

### Design philosophy

**No framework dependencies.** No langchaingo, no langchain, no SDKs. Each LLM provider is a thin HTTP client (~80 lines) behind a simple interface:

```go
type LLMProvider interface {
    Complete(ctx context.Context, req CompletionReq) (CompletionResp, error)
}
```

### Supported providers

Implement three providers (thin HTTP wrappers, not SDK dependencies):

| Provider    | Env var for API key      | Models                     |
|-------------|--------------------------|----------------------------|
| Anthropic   | `ANTHROPIC_API_KEY`      | claude-sonnet-4-20250514 etc  |
| OpenAI      | `OPENAI_API_KEY`         | gpt-4o, gpt-4o-mini etc   |
| Google      | `GOOGLE_API_KEY`         | gemini-2.0-flash etc      |

### YAML syntax

```yaml
tasks:
  analyze:
    type: llm
    provider: openai          # or anthropic, google — inferred from model if omitted
    model: gpt-4o
    system: "You are a code reviewer."
    prompt: |
      Review this code for bugs:
      {{state.code}}
    output: state.review       # where in shared state to put the response
    structured: true           # request JSON output from the LLM
    temperature: 0.2           # optional, defaults to provider default
    max_tokens: 2000           # optional
```

### Templating

Prompts support `{{state.field}}` interpolation using Go's `text/template`. All state fields are available. Template evaluation happens at runtime, after upstream tasks have populated state.

### Structured output

When `structured: true` is set:
- System prompt includes instruction to return valid JSON only
- Response is parsed as JSON before writing to state
- Parse failure is a task error

### Error handling

- HTTP errors (rate limits, auth failures) are retried with exponential backoff (3 attempts)
- Final failure is a task error that propagates through the graph normally
- Timeout from task-level `timeout` field is respected

---

## 9. Shared State

**Priority: P2**

A JSON state object that flows between nodes in a flow, enabling data passing and accumulation.

### Declaration

```yaml
flows:
  my-flow:
    state:
      input_file: "{{param.file}}"
      analysis: null
      decision: null
      result: null

    run: analyze -> decide -> act
```

### State mechanics

- State is initialised from the `state` block at flow start
- Each task can read state (via template interpolation in prompts/commands, or via CEL)
- Each task can write to state (via the `output` field)
- State is a flat or nested JSON object — tasks write to dot-separated paths: `state.evaluation.severity`
- Writes are merged, not replaced (JSON merge patch semantics)
- State is passed between tasks — not shared concurrently. Parallel tasks each receive a snapshot; their outputs are merged after all complete
- Merge conflicts in parallel tasks (two tasks writing to the same key) are resolved by last-finished-wins, with a warning in the event stream

### State for cmd tasks

Command tasks can write to state by outputting a JSON object on stdout when `output` is specified:

```yaml
tasks:
  run-tests:
    type: cmd
    cmd: go test -json ./...
    output: state.test_results
```

#### How cmd tasks READ state

Controlled by `state_input`:

```yaml
tasks:
  my-task:
    type: cmd
    state_input: template     # how state is delivered to this task
    cmd: go test -timeout {{state.test_timeout}} ./...
```

| Value      | Behaviour                                                    |
|------------|--------------------------------------------------------------|
| `template` | Default. `{{state.x}}` resolved in `cmd` string before execution |
| `env`      | `QP_STATE` env var contains full state as JSON string       |
| `file`     | `QP_STATE_FILE` env var points to a temp JSON file          |
| `stdin`    | State JSON piped to the command's stdin                      |

`template` is the simplest and covers most cases. Use `env` or `file` when the task needs to traverse or filter state dynamically (e.g. with `jq`). Use `stdin` for tools that expect JSON input on stdin.

#### How cmd tasks WRITE state

Controlled by `output` and `output_format`:

```yaml
tasks:
  my-task:
    type: cmd
    cmd: go test -json ./...
    output: state.test_results    # where in state to merge the result
    output_format: json           # how to parse stdout
```

| Value    | Behaviour                                                    |
|----------|--------------------------------------------------------------|
| `json`   | Default. Parse entire stdout as one JSON value               |
| `text`   | Store stdout as a raw string at the output path              |
| `ndjson` | Parse each line of stdout as JSON, collect into array        |

If `output` is not set, stdout is not captured into state (just displayed or logged).

#### Example: cmd task with full state access

```bash
#!/bin/bash
# my-analysis.sh — reads state from env, writes JSON to stdout
BRANCH=$(echo $QP_STATE | jq -r '.branch')
FILES=$(echo $QP_STATE | jq -r '.changed_files[]')
echo "{\"branch\": \"$BRANCH\", \"file_count\": $(echo "$FILES" | wc -l)}"
```

```yaml
tasks:
  analyze:
    type: cmd
    state_input: env
    cmd: ./my-analysis.sh
    output: state.analysis
    output_format: json
```

### State access by node type summary

| Node type | Reads state via                    | Writes state via        |
|-----------|------------------------------------|-------------------------|
| `cmd`     | `state_input` (template/env/file/stdin) | stdout → `output` path |
| `llm`     | `{{state.x}}` in prompt templates  | Response → `output` path |
| `mcp`     | `{{state.x}}` in `input` templates| Result → `output` path  |
| `expr`    | `state.x` natively in CEL         | CEL eval → named fields |
| `approve` | `{{state.x}}` in `message`        | Decision → `output` path|

### State access in CEL vs JSONPath

CEL's native access syntax covers most JSONPath patterns without needing a separate query language:

| JSONPath                               | CEL equivalent                                          |
|----------------------------------------|---------------------------------------------------------|
| `$.test_results[0].status`             | `state.test_results[0].status`                          |
| `$.test_results[?(@.status=='fail')]`  | `state.test_results.filter(r, r.status == 'fail')`      |
| `$.deploy.regions[*].name`             | `state.deploy.regions.map(r, r.name)`                   |
| `$.config.database.host`              | `state.config.database.host`                            |
| `$.results[*][?(@.score > 90)]`        | `state.results.filter(r, r.score > 90)`                 |

CEL's `filter`, `map`, `exists`, `all` macros are more expressive than JSONPath for workflow conditions and compose naturally:

```yaml
when: >
  state.test_results
    .filter(r, r.status == 'fail')
    .filter(r, r.module.startsWith('core/'))
    .size() > 3
```

Dynamic key access is supported via bracket notation:

```
state.results[state.current_key].value
```

### State in CEL

State is available in CEL expressions as the `state` variable:

```yaml
run: >
  analyze
  -> when(state.analysis.risk == "high", deep-scan, quick-scan)
```

---

## 10. CEL + LLM Evaluation (Smart Branching)

**Priority: P2**

This is the pattern where an LLM produces a structured evaluation and CEL uses that evaluation to make routing decisions. Not a separate feature — it emerges from combining LLM nodes, shared state, and CEL branching.

### Pattern

1. LLM node receives context (prior task outputs via state)
2. LLM returns structured JSON evaluation
3. CEL reads the evaluation from state
4. `when`/`switch` routes execution based on the evaluation

### Example: Intelligent triage

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
          {
            "severity": "low" | "medium" | "critical",
            "fixable": true | false,
            "reasoning": "brief explanation",
            "suggested_fix": "description if fixable, null otherwise"
          }
        output: state.evaluation
        structured: true
```

### Example: Iterative refinement with LLM-as-judge

```yaml
flows:
  write-and-refine:
    state:
      topic: "{{param.topic}}"
      draft: null
      review: null

    run: >
      write-draft
      -> until(
           review-draft -> when(state.review.pass, skip, revise-draft),
           state.review.pass == true,
           max: 3
         )
      -> publish

    tasks:
      write-draft:
        type: llm
        model: gpt-4o
        prompt: "Write an article about: {{state.topic}}"
        output: state.draft

      review-draft:
        type: llm
        model: gpt-4o
        prompt: |
          Review this draft. Respond with JSON:
          {"pass": true/false, "feedback": "..."}
          
          Draft: {{state.draft}}
        output: state.review
        structured: true

      revise-draft:
        type: llm
        model: gpt-4o
        prompt: |
          Revise this draft based on feedback:
          Feedback: {{state.review.feedback}}
          Draft: {{state.draft}}
        output: state.draft
```

### Key insight

The LLM provides the judgement, CEL provides the deterministic routing logic, state provides the data flow. The branching decision is auditable — you can inspect `state.evaluation` in the event stream and see exactly why the flow took the path it did. This is a significant advantage over LangGraph where the routing logic is buried in Python functions.

---

## 11. MCP Client Node Type

**Priority: P2**

A task type that calls tools on external MCP servers, making qp an orchestrator for any MCP-compatible service.

### Design philosophy

qp delegates capability to external services over MCP rather than importing libraries. Want vector search? Point at a vector store MCP server. Want Jira integration? Jira MCP server. qp stays thin.

### YAML syntax

```yaml
flows:
  research:
    mcp_servers:
      - name: knowledge
        url: "http://localhost:8100/sse"
      - name: github
        url: "http://localhost:8200/sse"
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

      create-ticket:
        type: mcp
        server: jira
        tool: create_issue
        input:
          project: ENG
          summary: "{{state.analysis.title}}"
          description: "{{state.analysis.body}}"
        output: state.ticket
```

### Server declaration

`mcp_servers` at the flow level. All tasks in the flow reference servers by `name`. Servers are connected on flow start and disconnected on flow end.

### Transports

- `http://` or `https://` — SSE transport
- `stdio://command` — stdio transport (launches a subprocess)
- Auth via headers: `auth: { header: "Authorization", value: "Bearer {{env.TOKEN}}" }`

### Error handling

- Connection failure to MCP server is a task error
- Tool invocation errors propagated as task failures
- Timeout from task-level `timeout` field applies to the full MCP call

---

## 12. Cycle Support (until)

**Priority: P2**

A new DAG node type that enables bounded loops — the primitive required for agentic retry/refine patterns.

### Syntax

```
until(subgraph, cel_condition, max: N)
```

- Runs `subgraph`
- Evaluates `cel_condition` against current state
- If true: exits the loop, continues to next node
- If false: runs `subgraph` again
- Hard stop at `max` iterations (required, no default — forces explicit bound)

### Example

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

### Safety constraints

- `max` is **required** — no unbounded loops
- CEL's termination guarantee means the exit condition itself always evaluates
- Static analysis: validate that the subgraph can modify the state fields referenced in the condition (warn if it can't — likely an infinite loop)
- Each iteration emits an `iteration` event on the NDJSON stream:

```jsonl
{"type":"iteration","loop":"until_1","attempt":2,"max":5,"condition_result":false,"ts":"..."}
```

### State across iterations

State accumulates across iterations. Each iteration sees the state from the previous iteration. The `state.attempt` counter is automatically incremented if not explicitly managed.

---

## 13. Expression Node Type

**Priority: P2**

A task type that evaluates a CEL expression and updates state. For lightweight transformations between nodes without shelling out or calling an LLM.

### YAML syntax

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

## 14. Approval Gate Node Type

**Priority: P3**

A task type that pauses execution and waits for human input.

### YAML syntax

```yaml
tasks:
  approve-deploy:
    type: approve
    message: "Deploy {{state.version}} to production?"
    options: [approve, reject, defer]
    output: state.approval_decision
    timeout: 30m
```

### Behaviour by context

| Context       | Behaviour                                                  |
|---------------|------------------------------------------------------------|
| CLI           | Interactive prompt on stdin, or skip with `--approve-all` flag |
| Event stream  | Emits `{"type":"approval_required",...}` and blocks         |
| MCP           | Returns tool result requesting approval, resumes on reply  |
| Web UI (future)| Button in the visualiser                                  |

### Timeout

If `timeout` expires without a decision, the task fails with reason `approval_timeout`. The flow can handle this with `when`:

```yaml
run: >
  approve-deploy 
  -> when(state.approval_decision == "approve", deploy, abort)
```

---

## 15. Flows as MCP Tools (Server)

**Priority: P3**

Flows with `expose: true` are callable via qp's MCP server interface, making every flow a tool that any agent or MCP client can invoke.

### YAML syntax

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

### MCP tool surface

When `qp serve` is running, exposed flows appear as MCP tools:

```json
{
  "name": "bug-fix",
  "description": "Autonomously diagnose and fix a bug",
  "inputSchema": {
    "type": "object",
    "properties": {
      "issue": { "type": "string", "description": "Issue description" },
      "repo": { "type": "string", "default": "." }
    },
    "required": ["issue"]
  }
}
```

### Invocation from Claude Code

```
> use qp: bug-fix with issue "auth tokens expire early"
```

The entire flow executes — multiple LLM calls, tool invocations, retry loops — and returns a single result.

### Distributed composition

Flows can call other qp instances' flows via `type: mcp`:

```yaml
tasks:
  run-qa:
    type: mcp
    server: "http://qa-server:9119/sse"
    tool: full-regression
    input: { branch: "{{branch()}}" }
    output: state.qa_result
```

### Security

- **Opt-in exposure**: nothing is callable by default, `expose: true` is required
- **Auth**: API key or mTLS for remote, open for localhost
- **Parameter validation**: CEL expressions on inputs as a validation layer:
  ```yaml
  params:
    issue:
      type: string
      required: true
      validate: "size(issue) > 10 && size(issue) < 5000"
  ```
- **Execution limits**: max concurrent flows, per-flow timeout ceiling, memory bounds
- **Audit stream**: every MCP invocation logged with caller identity, parameters, and outcome
- **No ambient authority**: a flow can only call MCP servers explicitly listed in its `mcp_servers` block

---

## 16. Declarative MCP Server Builder

**Priority: P1**

Any task or flow with `expose: true` is automatically registered as an MCP tool when `qp serve` is running. This makes qp a zero-code MCP server builder — define commands in YAML, expose them to agents and LLM clients instantly.

### Motivation

Today, exposing internal tooling over MCP requires writing a TypeScript or Python server, handling the JSON-RPC protocol, managing transports, and deploying it somewhere. With qp it's: write a `qp.yaml` with your commands, set `expose: true`, run `qp serve`. Three minutes from nothing to a spec-compliant MCP server.

### YAML syntax

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

  get-logs:
    expose: true
    description: "Fetch recent logs for a service"
    params:
      service: { type: string, required: true }
      lines: { type: int, default: 100 }
    cmd: journalctl -u {{param.service}} -n {{param.lines}} --no-pager

  internal-cleanup:
    # expose defaults to false — invisible to MCP clients
    cmd: ./scripts/cleanup.sh
```

### Serve command

```bash
# Stdio transport (default — for Claude Code, IDE integrations)
$ qp serve
  MCP server listening on stdio
  Exposed tools: health-check, restart-service, get-logs
  Hidden: internal-cleanup

# SSE transport (for remote/HTTP clients)
$ qp serve --transport sse --addr :9119
  MCP server listening on http://localhost:9119/sse
  Exposed tools: health-check, restart-service, get-logs
```

### How it works

On startup, `qp serve` iterates over all tasks and flows with `expose: true` and registers each as an MCP tool using the official Go SDK (`modelcontextprotocol/go-sdk`). Task params become the tool's `inputSchema`, automatically generating a JSON Schema from the param definitions. The SDK handles all protocol framing, transport, and schema advertisement.

```go
for name, task := range config.Tasks {
    if task.Expose {
        mcp.AddTool(server, &mcp.Tool{
            Name:        name,
            Description: task.Description,
        }, makeHandler(name, task))
    }
}
```

### Tool response contract

Every exposed task returns structured output:

```json
{
  "status": "pass",
  "duration_ms": 320,
  "stdout": "...",
  "stderr": "..."
}
```

Errors return structured error information so agents can reason about failures and decide whether to retry.

### What this enables

- **Ops teams**: expose runbooks as MCP tools — restart, deploy, rollback — callable by agents or chat interfaces
- **Platform teams**: wrap existing scripts and CLIs as MCP tools without rewriting them
- **Data teams**: expose data pipelines and queries as tools for LLM consumption
- **Any team**: three minutes from "I have a script" to "Claude can call it"

### Relationship to flows

Simple tasks with `expose: true` are standalone tools — one command, one response. Flows with `expose: true` (section 15) encapsulate entire multi-step workflows behind a single tool interface. Both are registered the same way; the MCP client doesn't know the difference.

### SDK

Uses `github.com/modelcontextprotocol/go-sdk` — the official MCP Go SDK maintained in collaboration with Google. Covers both client (for `type: mcp` nodes) and server (for `qp serve`), stdio and SSE transports, with OAuth support. Replaces the hand-rolled MCP implementation in the current codebase.

---

## 17. Concurrency Control

**Priority: P1**

Per-task and global concurrency limits for `qp serve` mode. Some tasks are safe to run concurrently (health checks, log reads). Others must be serialised (deployments, restarts). qp needs to know the difference.

### Per-task concurrency

```yaml
tasks:
  health-check:
    expose: true
    concurrency: 0          # unlimited (default) — stateless, safe to parallelise
    cmd: curl -s https://api.internal/health

  restart-service:
    expose: true
    concurrency: 1          # mutex — only one restart at a time
    cmd: systemctl restart {{param.service}}

  run-tests:
    expose: true
    concurrency: 3          # bounded — up to 3 concurrent test runs
    cmd: go test ./...

  deploy:
    expose: true
    concurrency: 1
    queue: true             # don't reject when full, queue and wait
    queue_timeout: 5m
    cmd: ./deploy.sh
```

### Concurrency field

| Value | Meaning                                            |
|-------|----------------------------------------------------|
| `0`   | Unlimited (default) — no concurrency restriction   |
| `1`   | Mutex — only one invocation at a time              |
| `N`   | Bounded — up to N concurrent invocations           |

### Queue behaviour

When a task is at its concurrency limit:

| `queue` setting | Behaviour                                                  |
|-----------------|------------------------------------------------------------|
| `false` (default) | Reject immediately with structured error                 |
| `true`           | Block until a slot opens or `queue_timeout` expires       |

The rejection or timeout returns a structured MCP error so agents know to retry or back off.

### Global serve configuration

```yaml
serve:
  max_concurrent: 20            # global ceiling across all tasks
  default_task_concurrency: 0   # default if task doesn't specify
  drain_timeout: 30s            # graceful shutdown window
  request_timeout: 5m           # default per-request timeout
  transport: stdio              # stdio or sse
  addr: ":9119"                 # listen address (sse only)
```

The global `max_concurrent` is a second semaphore above the per-task ones. Even if individual tasks allow high concurrency, the machine is protected from being overwhelmed.

### Request lifecycle

For every incoming MCP tool call:

1. Request arrives via SDK
2. Acquire global semaphore (reject if at `max_concurrent`)
3. Acquire task semaphore (reject or queue based on task config)
4. Create `context.Context` with request timeout
5. Execute task (shell command, flow, LLM call, etc.)
6. Capture output, build structured response
7. Release task semaphore
8. Release global semaphore
9. Return result to MCP client

### Cancellation

If the MCP client disconnects mid-request, the context cancels and the running task process is killed via `exec.CommandContext`. This propagates correctly through DAG execution — parallel siblings are cancelled, downstream tasks are skipped.

### Graceful shutdown

On SIGTERM/SIGINT:

1. Stop accepting new requests
2. Wait for in-flight tasks to complete (up to `drain_timeout`)
3. Kill any remaining tasks
4. Exit

Standard `signal.NotifyContext` pattern in Go.

### Implementation

Per-task concurrency is a `map[string]chan struct{}` — one buffered channel per task, sized to the concurrency limit. Acquire is a channel send, release is a channel receive. The global semaphore is the same pattern. Total implementation is approximately 30 lines of real logic wrapping the existing task execution engine.

```go
type TaskSemaphores struct {
    mu     sync.Mutex
    sems   map[string]chan struct{}
    global chan struct{}
}

func (ts *TaskSemaphores) Acquire(ctx context.Context, task string, limit int) error {
    // Acquire global first
    select {
    case ts.global <- struct{}{}:
    case <-ctx.Done():
        return ctx.Err()
    }
    // Then per-task
    if limit == 0 { return nil }
    sem := ts.getOrCreate(task, limit)
    select {
    case sem <- struct{}{}:
        return nil
    case <-ctx.Done():
        <-ts.global // release global on failure
        return ctx.Err()
    }
}
```

Go's concurrency primitives — goroutines, channels, contexts — map directly to this problem. No thread pools, no executor frameworks, no callback chains.

---

## 18. JSON Schema for qp.yaml

**Priority: P1**

Publish a JSON Schema for the `qp.yaml` configuration file.

### Benefits

- Editor autocompletion in VS Code, JetBrains, Vim (via yaml-language-server)
- Inline validation without running `qp validate`
- Agent-friendly: LLMs can generate valid config by referencing the schema
- Form generation for future UI (qp Studio)

### Distribution

- Hosted at a stable URL (e.g. via GitHub raw or a docs site)
- Referenced in `qp.yaml` via a schema comment:
  ```yaml
  # yaml-language-server: $schema=https://raw.githubusercontent.com/.../qp.schema.json
  ```
- Bundled in the binary: `qp schema` outputs the JSON Schema to stdout

---

## 19. Structured Logging

**Priority: P1**

Structured JSON logging via `github.com/rs/zerolog` across all task execution, flow orchestration, and LLM/MCP interactions. Makes qp auditable and integrable with any observability stack.

### Motivation

In corporate environments, structured logs are a prerequisite for adoption. Teams need to feed execution data into Splunk, Datadog, ELK, or BigQuery. Many deployment contexts (containers, systemd, CI runners) capture process stdout/stderr directly — qp must support structured logging to both streams as well as to files.

### Output targets

```bash
# Default: human-friendly coloured output on stdout
$ qp run release

# Structured JSON logs on stdout (for process capture / piping)
$ qp run release --log-format json

# Structured JSON logs on stderr (stdout stays clean for task output)
$ qp run release --log-format json --log-target stderr

# Structured JSON logs to file
$ qp run release --log-file /var/log/qp/release.jsonl

# Combine: human output on stdout, structured logs to file
$ qp run release --log-file /var/log/qp/release.jsonl

# Combine: structured logs on stderr AND to file
$ qp run release --log-format json --log-target stderr --log-file /var/log/qp/release.jsonl
```

### Flags

| Flag              | Values                  | Default   | Description                                      |
|-------------------|-------------------------|-----------|--------------------------------------------------|
| `--log-format`    | `text`, `json`          | `text`    | Output format. `text` is human-friendly, `json` is zerolog structured JSON |
| `--log-target`    | `stdout`, `stderr`      | `stdout`  | Where console log output is written               |
| `--log-file`      | file path               | none      | Additionally write structured JSON logs to a file (always JSON regardless of `--log-format`) |
| `--log-level`     | `debug`, `info`, `warn`, `error` | `info` | Minimum log level                                |

### Log schema

Every log entry includes:

```jsonl
{
  "level": "info",
  "ts": "2026-03-23T10:00:00.123Z",
  "run_id": "a1b2c3d4",
  "flow": "release",
  "task": "build",
  "msg": "task completed",
  "status": "pass",
  "duration_ms": 320
}
```

### Standard fields

| Field          | Type     | Description                                           |
|----------------|----------|-------------------------------------------------------|
| `run_id`       | `string` | Correlation ID for the entire run — traces a full flow execution across all entries |
| `flow`         | `string` | Flow name (if running a flow)                         |
| `task`         | `string` | Task name                                             |
| `status`       | `string` | pass, fail, error, skipped                            |
| `duration_ms`  | `int`    | Execution time                                        |
| `attempt`      | `int`    | Iteration count (inside `until` loops)                |
| `trigger`      | `string` | How the run was initiated: `cli`, `shell`, `mcp`, `watch` |
| `caller`       | `string` | MCP caller identity (when invoked via MCP server)     |
| `params`       | `object` | Resolved parameters for the run                       |

### Task output capture

All task stdout/stderr is optionally captured and included in structured logs:

```yaml
# qp.yaml — global setting
logging:
  capture_output: true    # include task stdout/stderr in log entries
  max_output_bytes: 10240 # truncate captured output per task (default 10KB)
```

When enabled, `done` log entries include:

```jsonl
{
  "level": "info",
  "run_id": "a1b2c3d4",
  "task": "test",
  "msg": "task completed",
  "status": "fail",
  "duration_ms": 4200,
  "stdout": "...",
  "stderr": "FAIL: TestAuth timeout\n..."
}
```

### Flow audit records

At flow completion, a summary log entry captures the full audit trail:

```jsonl
{
  "level": "info",
  "run_id": "a1b2c3d4",
  "flow": "bug-fix-agent",
  "msg": "flow completed",
  "status": "pass",
  "duration_ms": 34000,
  "params": {"issue": "auth tokens expire early"},
  "final_state": {"tests_pass": true, "attempt": 3, "diagnosis": "..."},
  "resolved_graph": {"nodes": [...], "edges": [...]},
  "trigger": "mcp",
  "caller": "claude-code"
}
```

This single entry is a complete audit record: who invoked it, with what params, what the flow decided at each branch (via final state), and the outcome.

### LLM call logging

LLM node executions log the provider, model, token usage, and optionally the prompt/response:

```jsonl
{
  "level": "info",
  "run_id": "a1b2c3d4",
  "task": "diagnose",
  "msg": "llm call completed",
  "provider": "openai",
  "model": "gpt-4o",
  "input_tokens": 1200,
  "output_tokens": 340,
  "duration_ms": 2100,
  "prompt_hash": "sha256:abc123..."
}
```

Full prompt/response logging is opt-in (may contain sensitive data):

```yaml
logging:
  log_llm_content: true   # include full prompt and response text
```

### Implementation notes

- Use `github.com/rs/zerolog` — zero allocation, structured JSON, supports multiple outputs via `zerolog.MultiLevelWriter`
- `--log-format text` uses zerolog's `ConsoleWriter` for human-friendly output
- `--log-file` always writes JSON regardless of `--log-format` setting
- `run_id` is a short random ID generated at run start (8 hex chars is sufficient)
- The event stream (`--events`) and structured logs are complementary: events are the real-time execution feed, logs are the persistent audit trail. Both can run simultaneously

---

## 20. Context Dump

**Priority: P1**

Full execution context dump for debugging and audit. The event stream tells you what happened in real time. The structured logs give you the audit trail. The context dump gives you everything — the complete picture of a run in one file, so you can understand exactly why a flow did what it did.

### Usage

```bash
# Dump full context after a run completes
$ qp run release --dump-context release-debug.json

# Dump context snapshot at a specific task (writes after that task, continues)
$ qp run release --dump-context-at build context.json

# Dump context on failure only (the one you'll use most)
$ qp run release --dump-context-on-fail debug.json

# Dump to stdout (for piping to another tool or an LLM)
$ qp run release --dump-context -
```

### Flags

| Flag                     | Argument     | Description                                              |
|--------------------------|--------------|----------------------------------------------------------|
| `--dump-context`         | file path or `-` | Dump full context after run completes (pass `-` for stdout) |
| `--dump-context-at`      | `task file`  | Dump context snapshot after a specific task               |
| `--dump-context-on-fail` | file path or `-` | Dump context only if the run fails                       |

### Context schema

```json
{
  "run_id": "a1b2c3d4",
  "flow": "release",
  "timestamp": "2026-03-23T10:01:23Z",
  "trigger": "cli",
  "status": "fail",
  "duration_ms": 34000,

  "params": {
    "env": "prod"
  },

  "environment": {
    "BRANCH": "main",
    "CI": "true",
    "GOVERSION": "1.22.0"
  },

  "config": {
    "file": "qp.yaml",
    "hash": "sha256:abc123..."
  },

  "resolved_graph": {
    "nodes": ["lint", "test", "audit", "build", "canary", "smoke", "deploy"],
    "edges": [
      ["lint", "build"], ["test", "build"], ["audit", "build"],
      ["build", "canary"], ["build", "smoke"],
      ["canary", "deploy"], ["smoke", "deploy"]
    ]
  },

  "cel_evaluations": [
    {
      "location": "when@release:step3",
      "expression": "env(\"ENV\") == \"prod\"",
      "result": true,
      "branch_taken": "if_true"
    },
    {
      "location": "when@release:step5",
      "expression": "branch() == \"main\"",
      "result": true,
      "branch_taken": "if_true"
    }
  ],

  "state_snapshots": {
    "initial": {
      "issue": "auth tokens expire early",
      "diagnosis": null,
      "patch": null,
      "tests_pass": false,
      "attempt": 0
    },
    "after:gather-context": {
      "issue": "auth tokens expire early",
      "code_context": "func RefreshToken(...) { ... }"
    },
    "after:diagnose": {
      "diagnosis": "Token TTL not accounting for clock skew..."
    },
    "after:generate-fix:attempt:1": {
      "attempt": 1,
      "patch": "--- a/auth.go\n+++ b/auth.go\n..."
    },
    "after:run-tests:attempt:1": {
      "tests_pass": false
    },
    "after:generate-fix:attempt:2": {
      "attempt": 2,
      "patch": "--- a/auth.go\n+++ b/auth.go\n..."
    },
    "after:run-tests:attempt:2": {
      "tests_pass": true
    }
  },

  "tasks": {
    "gather-context": {
      "type": "cmd",
      "cmd": "qp context --task test --json",
      "status": "pass",
      "duration_ms": 120,
      "exit_code": 0,
      "stdout": "...",
      "stderr": ""
    },
    "diagnose": {
      "type": "llm",
      "provider": "openai",
      "model": "gpt-4o",
      "status": "pass",
      "duration_ms": 2100,
      "input_tokens": 1200,
      "output_tokens": 340,
      "prompt": "Issue: auth tokens expire early\nRelevant code: ...",
      "response": "Token TTL not accounting for clock skew..."
    },
    "generate-fix:attempt:1": {
      "type": "llm",
      "provider": "openai",
      "model": "gpt-4o",
      "status": "pass",
      "duration_ms": 3400,
      "input_tokens": 1800,
      "output_tokens": 620,
      "prompt": "Diagnosis: Token TTL...\nAttempt: 1\n...",
      "response": "--- a/auth.go\n+++ b/auth.go\n..."
    },
    "run-tests:attempt:1": {
      "type": "cmd",
      "cmd": "qp run test --json",
      "status": "fail",
      "duration_ms": 4200,
      "exit_code": 1,
      "stdout": "...",
      "stderr": "FAIL: TestTokenRefresh — expected 3600, got 3540"
    },
    "generate-fix:attempt:2": {
      "type": "llm",
      "provider": "openai",
      "model": "gpt-4o",
      "status": "pass",
      "duration_ms": 2800,
      "input_tokens": 2200,
      "output_tokens": 580,
      "prompt": "Diagnosis: Token TTL...\nAttempt: 2\nPrevious patch: ...",
      "response": "--- a/auth.go\n+++ b/auth.go\n..."
    },
    "run-tests:attempt:2": {
      "type": "cmd",
      "cmd": "qp run test --json",
      "status": "pass",
      "duration_ms": 3800,
      "exit_code": 0,
      "stdout": "PASS: all tests",
      "stderr": ""
    }
  },

  "mcp_calls": [
    {
      "task": "notify",
      "server": "slack",
      "tool": "send_message",
      "input": {"channel": "#engineering", "text": "..."},
      "output": {"ok": true, "ts": "1234567890.123456"},
      "duration_ms": 340
    }
  ]
}
```

### What each section provides

| Section             | Purpose                                                                 |
|---------------------|-------------------------------------------------------------------------|
| `params`            | What was passed in — reproducing the run                                |
| `environment`       | Machine/env context — diagnosing environment-specific failures          |
| `config`            | Config file hash — verifying the run used the expected config           |
| `resolved_graph`    | The actual DAG that executed after CEL pruning                          |
| `cel_evaluations`   | Every branching decision with expression, result, and which branch was taken |
| `state_snapshots`   | State at every step boundary — trace data flow through the graph        |
| `tasks`             | Full detail per task — exit codes, output, LLM prompts/responses        |
| `mcp_calls`         | External service interactions — what was sent and received              |

### Agent consumption

The context dump is designed to be fed directly to an LLM for analysis:

```bash
# Pipe failed context to an LLM for diagnosis
$ qp run release --dump-context-on-fail - | qp run diagnose-failure --param context=-
```

Or in a flow:

```yaml
tasks:
  analyze-failure:
    type: llm
    model: gpt-4o
    prompt: |
      A qp flow failed. Here is the full execution context:
      {{state.failure_context}}
      
      Explain why it failed and suggest a fix.
    output: state.failure_analysis
```

The context dump is a complete, self-contained record. An LLM reading it has everything it needs — the graph topology, the branching decisions, the state at each step, the full task outputs — to reason about what happened without access to the running system.

### Sensitive data

Context dumps may contain secrets (env vars, LLM prompts with proprietary code). Options for handling:

```yaml
logging:
  context_dump:
    redact_env: ["API_KEY", "SECRET_*"]    # glob patterns for env vars to redact
    include_llm_content: true               # include full prompts/responses (default true)
    include_task_output: true               # include stdout/stderr (default true)
    max_output_bytes: 102400                # max captured output per task (default 100KB)
```

Redacted values appear as `"[REDACTED]"` in the dump.

---

## 21. Variables

**Priority: P0**

Reusable values declared once and referenced throughout the config. Avoids repeating paths, URLs, version strings, and other constants across task definitions.

### Declaration

```yaml
vars:
  registry: gcr.io/myproject
  go_version: "1.22"
  test_timeout: 5m
  deploy_region: us-central1
  app_name: myapp
```

### Usage

Variables are available everywhere via `{{var.name}}` template interpolation:

```yaml
tasks:
  build:
    cmd: docker build --build-arg GO={{var.go_version}} -t {{var.registry}}/{{var.app_name}} .
  test:
    cmd: go test -timeout {{var.test_timeout}} ./...
  deploy:
    cmd: gcloud run deploy {{var.app_name}} --region {{var.deploy_region}}
```

### In CEL

Variables are accessible as `var.name` in CEL expressions:

```yaml
tasks:
  deploy:
    when: var.deploy_region != "us-central1" || branch() == "main"
```

### Environment variable override

Variables can be overridden from the environment for CI flexibility:

```bash
$ QP_VAR_REGISTRY=gcr.io/production qp run deploy
```

Convention: `QP_VAR_` prefix + uppercase variable name.

### CLI override

```bash
$ qp run deploy --var registry=gcr.io/production --var deploy_region=europe-west1
```

CLI overrides take precedence over env vars, which take precedence over YAML defaults.

### Relationship to task `env` and `env_file`

`vars` and `env` are different concepts that complement each other:

| Concept    | Scope         | Resolved by | Accessed via                | Purpose                          |
|------------|---------------|-------------|-----------------------------|----------------------------------|
| `vars`     | qp config    | qp engine  | `{{var.name}}` / `var.name` in CEL | DRY config — reuse values across tasks |
| `env`      | task process  | OS shell    | `$MY_VAR` in the command    | Process environment for the task |
| `env_file` | task process  | qp engine  | `$MY_VAR` in the command    | Load process env vars from a file |

`vars` are resolved by qp at template time — before the command runs. `env` values are passed to the shell process — the command sees them as environment variables. You can bridge the two by interpolating vars into env values:

```yaml
vars:
  registry: gcr.io/myproject

tasks:
  build:
    env:
      DOCKER_BUILDKIT: "1"              # pure process env
      REGISTRY: "{{var.registry}}"      # bridge: qp var → process env
    env_file: .env                      # additional env from file
    cmd: docker build -t $REGISTRY/app .
```

In this example, `{{var.registry}}` is resolved by qp before the command runs, and the resulting value is passed to the shell as the `REGISTRY` environment variable.

---

## 22. Profiles

**Priority: P1**

Named sets of variable overrides and task configuration changes for different environments. Profiles merge on top of defaults — only specify what changes.

### Declaration

```yaml
vars:
  registry: gcr.io/myproject-dev
  replicas: 1
  log_level: debug

profiles:
  staging:
    vars:
      registry: gcr.io/myproject-staging
      replicas: 2
      log_level: info

  prod:
    vars:
      registry: gcr.io/myproject-prod
      replicas: 5
      log_level: warn
    tasks:
      deploy:
        concurrency: 1
        when: branch() == "main"
```

### Usage

```bash
$ qp run deploy                    # uses default vars
$ qp run deploy --profile staging  # merges staging overrides
$ qp run deploy --profile prod     # merges prod overrides + deploy constraints
```

### Merge semantics

Profiles deep-merge on top of the base config:

- `vars` — profile values override base values; unmentioned vars keep their defaults
- `tasks` — profile can add or override task-level fields (`when`, `concurrency`, `timeout`, `env`). Task `cmd` and `run` are not overridable from profiles (prevents invisible behaviour changes)
- `serve` — profile can override serve config (e.g. different ports per environment)

### Profile in CEL

The active profile is available via `profile()` in CEL expressions:

```yaml
tasks:
  deploy:
    run: >
      build
      -> when(profile() == "prod", canary -> smoke, skip)
      -> push
```

### Multiple profiles

Profiles can be stacked. Later profiles override earlier ones:

```bash
$ qp run deploy --profile staging --profile high-memory
```

### Environment-based default

```yaml
profiles:
  _default: "{{env.QP_PROFILE}}"   # auto-select from env var
```

---

## 23. Templates

**Priority: P1**

Reusable task patterns that can be stamped out with different parameters. Eliminates copy-pasting near-identical task definitions across services in a monorepo.

### Declaration

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
      docker:
        cmd: docker build -t {{var.registry}}/{{param.service}} .
      run:
        run: build -> test -> docker
```

### Usage

```yaml
tasks:
  auth:
    use: go-service
    params: { service: auth, port: 8081 }

  payments:
    use: go-service
    params: { service: payments, port: 8082 }

  gateway:
    use: go-service
    params: { service: gateway, port: 8080 }
```

### Generated task names

Template tasks are namespaced under the parent task name:

```bash
$ qp list
  auth:build
  auth:test
  auth:docker
  auth:run
  payments:build
  payments:test
  payments:docker
  payments:run
  gateway:build
  ...
```

```bash
$ qp run auth:test          # run one task
$ qp run auth               # run the template's default (auth:run)
```

### Template override

Individual instances can override template-level settings:

```yaml
tasks:
  payments:
    use: go-service
    params: { service: payments, port: 8082 }
    override:
      test:
        cmd: go test -tags=integration ./internal/payments/...
        timeout: 10m
```

### Templates in flows

Templates work in flows for reusable agent patterns:

```yaml
templates:
  llm-review:
    params:
      focus: { type: string, required: true }
    tasks:
      review:
        type: llm
        model: gpt-4o
        prompt: |
          Review this code focusing on {{param.focus}}:
          {{state.code}}
        output: state.review_{{param.focus}}
        structured: true

flows:
  full-review:
    state:
      code: "{{param.code}}"

    run: par(security, perf, style) -> summarise

    tasks:
      security: { use: llm-review, params: { focus: security } }
      perf: { use: llm-review, params: { focus: performance } }
      style: { use: llm-review, params: { focus: style } }

      summarise:
        type: llm
        model: gpt-4o
        prompt: |
          Combine these reviews into a single summary:
          Security: {{state.review_security}}
          Performance: {{state.review_performance}}
          Style: {{state.review_style}}
        output: state.summary
```

### Composition

Templates can reference other templates:

```yaml
templates:
  deployable-service:
    params:
      service: { type: string, required: true }
    use: go-service
    tasks:
      deploy:
        cmd: kubectl apply -f k8s/{{param.service}}/
      full:
        run: build -> test -> docker -> deploy
```

---

## 24. Task Caching / Skip

**Priority: P0**

Content-addressed caching that skips tasks whose inputs haven't changed since the last successful run. Make's killer feature, applied to arbitrary task graphs.

### How it works

Each task declares its inputs — source files, environment variables, commands. qp hashes these inputs and stores the hash alongside the task's last successful result. On subsequent runs, if the hash matches, the task is skipped.

```yaml
tasks:
  lint:
    cmd: golangci-lint run ./...
    cache:
      paths: ["**/*.go", ".golangci.yml"]
      env: [GOFLAGS]

  build:
    cmd: go build -o bin/app ./cmd/app
    cache:
      paths: ["**/*.go", "go.mod", "go.sum"]
      env: [GOVERSION, CGO_ENABLED]

  test:
    cmd: go test ./...
    cache:
      paths: ["**/*.go", "go.mod", "go.sum", "testdata/**"]
```

### Cache behaviour

```bash
$ qp run par(lint, test) -> build
  ⏭ lint (cached — no changes)
  ✓ test in 1.2s
  ✓ build in 3.4s

$ qp run par(lint, test) -> build
  ⏭ lint (cached)
  ⏭ test (cached)
  ⏭ build (cached)
  nothing to do
```

### Hash composition

The cache key is a SHA-256 of:

- Content hash of all files matching `cache.paths` globs
- Values of env vars listed in `cache.env`
- The `cmd` string itself (so changing the command invalidates cache)
- The task's `vars` and `params` values
- Cache keys of upstream dependencies (so if `test` depends on `lint` and `lint` ran fresh, `test` also runs fresh)

### Cache location

```
~/.cache/qp/<project-hash>/
  lint.hash
  build.hash
  test.hash
```

Project hash is derived from the absolute path to `qp.yaml`, so caches are per-project.

### Cache control

```bash
$ qp run build --no-cache         # force fresh execution
$ qp cache clean                  # clear all cached hashes for this project
$ qp cache clean --all            # clear all qp caches
$ qp cache status                 # show what's cached and what's stale
```

### Disabled in server mode

**Caching is always disabled when running in `qp serve` mode.** MCP tool calls from agents must always execute — returning stale cached results would break agent reasoning and make results unauditable. The `--no-cache` flag is effectively always set during serve.

### Disabled for specific task types

Caching only applies to `cmd` tasks. The following are never cached:

- `llm` nodes (non-deterministic by nature)
- `mcp` nodes (external service state is unknown)
- `expr` nodes (trivially fast, not worth caching)
- `approve` nodes (human input required)

### DAG integration

Cache-skipping propagates through the DAG. In `par(lint, test) -> build`:

- If `lint` is cached and `test` is cached, `build` checks its own cache
- If `lint` is cached but `test` ran fresh, `build` must run fresh (upstream changed)
- If `build`'s own inputs haven't changed AND all upstreams are cached, `build` is skipped

### Event stream

Cached tasks emit a `skipped` event with reason `cached`:

```jsonl
{"type":"skipped","task":"lint","reason":"cached","cache_hash":"abc123...","ts":"..."}
```

---

## 25. Secrets Management

**Priority: P1**

Dedicated handling for sensitive values — API keys, tokens, credentials — with clear separation from regular config and integration with qp's redaction and logging systems.

### Motivation

qp calls LLM APIs (needs `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`), MCP servers (auth tokens), and cloud providers (service account keys). `env_file` works but doesn't distinguish secrets from regular config — secrets need redaction in logs, exclusion from context dumps, and should never end up in version control.

### Declaration

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
    path: .qp-secrets      # key=value file, auto-added to .gitignore
    key: DB_PASSWORD
```

### Usage in tasks

Secrets are available via `{{secret.name}}` interpolation:

```yaml
tasks:
  analyze:
    type: llm
    model: gpt-4o
    # LLM provider automatically uses the relevant secret
    prompt: "..."

  deploy:
    type: cmd
    env:
      DB_PASSWORD: "{{secret.db_password}}"
    cmd: ./deploy.sh
```

### Secrets file

`.qp-secrets` is a simple key=value file for local development:

```
OPENAI_API_KEY=sk-abc123...
DEPLOY_TOKEN=ghp_xyz789...
DB_PASSWORD=hunter2
```

`qp init` adds `.qp-secrets` to `.gitignore` automatically. `qp validate` warns if `.qp-secrets` is not in `.gitignore`.

### Redaction

Secrets are automatically redacted everywhere:

- Structured logs: secret values replaced with `[REDACTED]`
- Context dumps: secret values replaced with `[REDACTED]`
- Event stream: secret values replaced with `[REDACTED]`
- CLI output: secret values replaced with `[REDACTED]`
- `qp plan --json`: secret values replaced with `[REDACTED]`

The redaction is based on the registered secret values, not pattern matching — so it catches secrets even if they appear in unexpected places in task output.

### LLM provider integration

LLM nodes automatically resolve API keys from secrets:

```yaml
secrets:
  anthropic: { from: env, env: ANTHROPIC_API_KEY }
  openai: { from: env, env: OPENAI_API_KEY }

tasks:
  analyze:
    type: llm
    model: gpt-4o        # provider inferred as openai, key resolved from secrets.openai
```

Convention-based: secret names `anthropic`, `openai`, `google` are automatically used by the corresponding LLM provider. Explicit override via `api_key: "{{secret.my_custom_key}}"` if needed.

---

## 26. diff-plan

**Priority: P1**

Show which tasks need to run based on what files have changed in git. Bridges `git diff` to qp's scope/codemap system. The highest-leverage feature for agent-assisted development.

### Usage

```bash
# What tasks are affected by uncommitted changes?
$ qp diff-plan
  lint        — src/auth/token.go changed (scope: auth)
  test        — src/auth/token.go changed (scope: auth)
  build       — upstream dependency: test

# What tasks are affected by changes in a PR?
$ qp diff-plan --base main
  lint        — 4 files changed in scope: auth, payments
  test        — 4 files changed in scope: auth, payments
  build       — upstream dependency: test
  deploy      — upstream dependency: build

# Structured output for agent consumption
$ qp diff-plan --json
{
  "changed_files": ["src/auth/token.go", "src/auth/token_test.go"],
  "affected_scopes": ["auth"],
  "tasks_to_run": [
    {"task": "lint", "reason": "files changed in scope", "files": [...]},
    {"task": "test", "reason": "files changed in scope", "files": [...]},
    {"task": "build", "reason": "upstream dependency: test"}
  ]
}
```

### How it works

1. Compute `git diff` (working tree vs HEAD, or `--base` branch vs HEAD)
2. Map changed files to scopes using the existing scope definitions
3. Find tasks that own those scopes
4. Walk the DAG to find downstream dependents
5. Output the affected tasks and their reasons

### Agent workflow

An agent makes changes, then calls `qp diff-plan --json` to know exactly what to verify:

```
Agent modifies src/auth/token.go
→ qp diff-plan --json
→ {"tasks_to_run": ["lint", "test"]}
→ Agent runs qp run par(lint, test)
→ All pass → Agent commits
```

This closes the feedback loop: the agent doesn't have to guess what to re-check, and it doesn't waste time re-running unrelated tasks.

### Integration with caching

`diff-plan` and caching are complementary but different:

- **Caching** skips tasks whose inputs haven't changed (content-addressed, automatic)
- **diff-plan** tells you which tasks are _likely_ affected by a diff (scope-based, advisory)

An agent might use `diff-plan` to decide what to run, and caching ensures that within that set, only the tasks whose inputs actually changed will execute.

---

## 27. Task Retry

**Priority: P0**

Simple retry for tasks that fail — covers flaky tests, transient network errors, rate-limited API calls. Simpler than `until` loops, which are for agentic retry with state and reasoning.

### YAML syntax

```yaml
tasks:
  test:
    cmd: go test ./...
    retry: 3                  # retry up to 3 times on failure
    retry_delay: 2s           # wait between retries

  call-api:
    cmd: curl -f https://api.example.com/webhook
    retry: 5
    retry_delay: 5s
    retry_backoff: exponential  # 5s, 10s, 20s, 40s, 80s
```

### Fields

| Field              | Type     | Default  | Description                                      |
|--------------------|----------|----------|--------------------------------------------------|
| `retry`            | `int`    | `0`      | Max retry attempts (0 = no retry)                |
| `retry_delay`      | `duration` | `0s`   | Wait between retries                              |
| `retry_backoff`    | `string` | `fixed`  | `fixed` or `exponential`                         |
| `retry_on`         | `list`   | `[any]`  | Retry conditions: `any`, `exit_code:N`, `stderr_contains:pattern` |

### Selective retry

```yaml
tasks:
  deploy:
    cmd: ./deploy.sh
    retry: 3
    retry_delay: 10s
    retry_on:
      - exit_code:1           # retry on exit code 1 (transient)
      - stderr_contains: "rate limit"
    # exit code 2 (config error) won't be retried
```

### Event stream

Each retry attempt emits events:

```jsonl
{"type":"retry","task":"test","attempt":2,"max":3,"reason":"exit_code:1","delay_ms":2000,"ts":"..."}
{"type":"start","task":"test","attempt":2,"ts":"..."}
```

### Relationship to `until`

`retry` is for blind retries of the same command — the command doesn't change between attempts. `until` is for agentic loops where state accumulates and an LLM adjusts its approach based on prior failures. Use `retry` for flaky infrastructure, `until` for intelligent refinement.

---

## 28. Dry Run

**Priority: P0**

Show exactly what would execute — resolved commands with all templates, variables, state, and CEL evaluations expanded — without executing anything.

### Usage

```bash
# Show resolved commands
$ qp run release --dry-run
  [1] lint
      cmd: golangci-lint run ./...
      env: {GOFLAGS: "-mod=vendor"}

  [2] test (parallel with lint)
      cmd: go test -timeout 5m ./...

  [3] build (after: lint, test)
      cmd: docker build --build-arg GO=1.22 -t gcr.io/myproject-prod/app .

  [4] deploy (after: build)
      cmd: gcloud run deploy myapp --region us-central1 --image gcr.io/myproject-prod/app
      ⚠ requires approval

# JSON output for agent consumption
$ qp run release --dry-run --json
```

### What gets resolved

- `{{var.name}}` → actual variable values (including profile overrides)
- `{{param.name}}` → actual parameter values
- `{{state.x}}` → shows as `{{state.x}}` (state is runtime, can't resolve statically)
- CEL `when` conditions → evaluated against available context (env, branch, vars)
- DAG topology → fully resolved after branch pruning
- Cache status → shows which tasks would be skipped

### Profile-aware

```bash
$ qp run release --profile prod --dry-run
  # shows resolved commands with prod variable values

$ qp run release --profile staging --dry-run
  # shows resolved commands with staging variable values, different DAG shape
```

### Validation mode

`--dry-run` also validates:

- All referenced tasks exist
- Template interpolation has no unresolved references
- CEL expressions parse and type-check
- Required params are provided
- Required secrets are available (checks env vars exist, doesn't print values)
- Prerequisites (`requires`) are met

Exits with non-zero if any validation fails.

---

## 29. Harness Engineering Support

**Priority: P1**

Lightweight, built-in support for the harness engineering patterns that make agent-assisted development reliable at scale. These aren't separate products — they're extensions of features qp already has (guards, scopes, codemap, context, CEL), unified under a single architectural framework declared in `qp.yaml`.

### Motivation

The core insight from large-scale agent-assisted development is that the agent isn't the bottleneck — the environment is. When an agent struggles, the fix is almost never "use a better model." It's identifying what constraint, context, or feedback mechanism is missing and making it legible and enforceable. qp is uniquely positioned to provide this because it already owns the task graph, the scope/codemap system, and the structured output contracts.

This feature set targets codebases in the 100-200k LoC range — typical enterprise services and platforms — where architectural drift is the primary risk and scoped context is the primary enabler for productive agent work.

### 29a. Architecture Enforcement

Declare architectural layers and domain boundaries in `qp.yaml`. qp validates import graphs and dependency directions, catching violations before they land.

```yaml
architecture:
  layers: [types, config, repo, service, runtime, ui]
  rules:
    - direction: forward    # layers can only import from layers to their left
    - cross_cutting: providers  # cross-cutting concerns enter through Providers only
  
  domains:
    auth:
      root: src/auth
      layers: [types, config, repo, service]
    payments:
      root: src/payments
      layers: [types, config, repo, service, runtime, ui]
    gateway:
      root: src/gateway
      layers: [types, config, service, runtime, ui]
```

```bash
$ qp arch-check
  ✗ src/ui/dashboard.go imports src/payments/repo — UI cannot import Repo layer
  ✗ src/auth/service/auth.go imports src/payments/service — cross-domain import must go through Providers
  ✓ 847 other imports OK

$ qp arch-check --json    # structured output for agents
```

`arch-check` can run as a guard task:

```yaml
tasks:
  check-architecture:
    cmd: qp arch-check
    guard: true
    cache:
      paths: ["src/**/*.go", "qp.yaml"]
```

Agents get immediate structured feedback when they violate a boundary — no manual code review needed.

#### How enforcement works

The problem is separated into two parts: extracting the dependency graph (language-specific) and enforcing the rules (universal).

**Step 1: Extract imports (language-specific)**

Each language has a small import extractor (~50-80 lines) behind a common interface:

```go
type ImportExtractor interface {
    Extensions() []string
    Extract(filepath string, content []byte) []Import
}

type Import struct {
    Source  string   // file containing the import
    Target string   // what it imports (normalised to repo-relative path)
    Line   int      // for error reporting
}
```

qp doesn't need to fully parse the language — it only needs to find import statements and normalise them to repo-relative paths.

**Step 2: Map to domain and layer (universal)**

Given an import, qp resolves both source and target to a domain and layer using string prefix matching against the declared roots:

```
Import: src/auth/service/auth.go → src/payments/repo/store.go

Source: domain=auth, layer=service
Target: domain=payments, layer=repo

Rule check: service → repo is forward (OK), but cross-domain without Providers (FAIL)
```

This mapping is pure config — no language knowledge needed. The architectural question is always the same regardless of language: "does module A depend on module B, and is that allowed?"

**Step 3: Check rules (universal)**

The rules engine evaluates every extracted import against the declared constraints: layer direction, cross-domain policies, and any custom rules.

#### Built-in language extractors

| Language         | Extensions            | Import resolution                                    |
|------------------|-----------------------|------------------------------------------------------|
| Go               | `.go`                 | Direct path mapping — `import "app/internal/payments/repo"` maps to repo-relative path |
| Python           | `.py`                 | Package-relative — `from auth.repo import UserStore` resolved against project package structure. Relative imports (`from ..repo`) resolved from file location |
| TypeScript/JS    | `.ts`, `.tsx`, `.js`  | Path-relative — `import { x } from '../../payments/repo'` resolved from importing file. Alias paths from `tsconfig.json` supported via optional `tsconfig` config |
| Java             | `.java`               | Convention-based — `import com.myorg.payments.repo.UserStore` mapped to directory structure |

#### Language configuration

Auto-detection by file extension is the default. Explicit configuration for cases that need it:

```yaml
architecture:
  languages:
    go: {}                                          # auto-detect, no special config
    python: { resolve_from: "src" }                 # package root
    typescript: { tsconfig: "tsconfig.json" }       # alias resolution
    java: { source_root: "src/main/java" }          # maven/gradle layout
```

#### Polyglot repos

For repos with multiple languages, qp runs the appropriate extractor per file based on extension. Most polyglot repos have clear language boundaries that map to separate domains (Go backend, TypeScript frontend), which works naturally:

```yaml
architecture:
  layers: [types, config, repo, service, runtime, ui]
  domains:
    api:
      root: backend/src
      languages: [go]
    web:
      root: frontend/src
      languages: [typescript]
  rules:
    - direction: forward
    - cross_domain: deny    # backend and frontend cannot import each other
```

#### Custom language extractors

For languages without a built-in extractor, an external command can provide the import list:

```yaml
architecture:
  languages:
    rust:
      extractor: cmd
      cmd: "qp-extract-rust {{file}}"
      extensions: [".rs"]
```

The external command receives a file path and outputs a JSON array of imports:

```json
[
  {"target": "crate::payments::repo::store", "line": 3},
  {"target": "crate::auth::types::user", "line": 4}
]
```

qp normalises the targets to repo-relative paths and feeds them into the same universal rule engine. This means any language can be supported without a built-in parser — write a small extractor script and plug it in.

#### Structured error output

```bash
$ qp arch-check --json
{
  "violations": [
    {
      "source": {"file": "src/ui/dashboard.go", "line": 8, "domain": "gateway", "layer": "ui"},
      "target": {"file": "src/payments/repo/store.go", "domain": "payments", "layer": "repo"},
      "rule": "layer_direction",
      "message": "UI cannot import Repo layer (forward-only rule)"
    },
    {
      "source": {"file": "src/auth/service/auth.go", "line": 12, "domain": "auth", "layer": "service"},
      "target": {"file": "src/payments/service/pay.go", "domain": "payments", "layer": "service"},
      "rule": "cross_domain",
      "message": "Cross-domain import must go through Providers"
    }
  ],
  "total_imports": 849,
  "violations_count": 2,
  "status": "fail"
}
```

Agents get exact file paths, line numbers, domain/layer classification, and the rule that was violated — enough to fix the issue without further investigation.

### 29b. Tiered Documentation Structure

Structured documentation hierarchy that provides progressive context disclosure — agents get exactly the context they need for a task, not a 1000-page instruction manual.

```yaml
docs:
  architecture: docs/architecture.md
  conventions: docs/conventions.md
  domains:
    auth: docs/domains/auth.md
    payments: docs/domains/payments.md
    gateway: docs/domains/gateway.md
  
  agent_context: tiered
```

#### Progressive context delivery

When an agent requests context for a task, qp assembles a tiered response:

```bash
$ qp context --task fix-auth-bug
  # Returns (in order of priority):
  # 1. Architecture map (high-level, always included)
  # 2. Auth domain doc (scoped to the relevant domain)
  # 3. Auth scope files (from codemap)
  # 4. Conventions doc (coding standards)
  # NOT: payments domain doc, gateway domain doc, etc.
```

This directly prevents the "one big AGENTS.md" anti-pattern. Context is scoped to the task. Irrelevant information is excluded rather than included.

#### Context budgeting

```bash
$ qp context --task fix-auth-bug --budget 8000
  # Returns context trimmed to ~8000 tokens
  # Most relevant files first, least relevant trimmed
  # Budget is approximate (token estimation, not exact)
```

The agent gets enough context to work without crowding out the task itself.

### 29c. Invariant Enforcement

Declarative project invariants expressed in CEL, enforced mechanically as part of the guard pipeline. These are "taste invariants" — rules about code quality, structure, and conventions that would normally be caught in code review.

```yaml
invariants:
  - name: no-large-files
    check: "changed_files().all(f, file_lines(f) < 500)"
    message: "Files must be under 500 lines"
    severity: error

  - name: test-coverage
    check: "state.coverage >= 80.0"
    message: "Test coverage must stay above 80%"
    severity: warn

  - name: no-direct-db-in-ui
    check: >
      changed_files()
        .filter(f, f.startsWith('src/ui/'))
        .all(f, !file_imports(f, 'repo'))
    message: "UI layer cannot import repo layer directly"
    severity: error
    
  - name: structured-logging-only
    check: >
      changed_files()
        .filter(f, f.endsWith('.go'))
        .all(f, !file_contains(f, 'fmt.Print'))
    message: "Use structured logging (zerolog), not fmt.Print"
    severity: warn

  - name: max-function-complexity
    check: "max_cyclomatic(changed_files()) <= 15"
    message: "Function cyclomatic complexity must not exceed 15"
    severity: warn
```

```bash
$ qp invariants
  ✓ no-large-files
  ✗ no-direct-db-in-ui — src/ui/admin.go imports repo
  ⚠ structured-logging-only — src/auth/login.go uses fmt.Println
  ✓ max-function-complexity
  
  1 error, 1 warning

$ qp invariants --json    # structured for agent consumption
```

Invariants run as part of `qp guard` and produce structured errors the agent can act on. They are declared in `qp.yaml`, versioned with the code, and enforced mechanically — not by convention or code review.

#### Built-in CEL functions for invariants

| Function                  | Returns        | Description                                  |
|---------------------------|----------------|----------------------------------------------|
| `changed_files()`        | `list<string>` | Files changed (git diff)                     |
| `file_lines(path)`       | `int`          | Line count of a file                         |
| `file_contains(path, s)` | `bool`         | Whether file contains string                 |
| `file_imports(path, s)`  | `bool`         | Whether file imports a module matching s      |
| `max_cyclomatic(files)`  | `int`          | Max cyclomatic complexity across files        |

### 29d. Quality Scoring

A lightweight, configurable quality report computed from guard tasks, invariants, and architecture checks. Gives both humans and agents a single view of codebase health.

```bash
$ qp quality
  Architecture:  ✓ no violations
  Invariants:    ⚠ 1 warning (structured-logging)
  Lint:          ✓ clean
  Tests:         ✓ 342/342 passed
  Coverage:      ✓ 87% (threshold: 80%)
  Docs:          ✗ 2 domains missing documentation
  Complexity:    ✓ max 12 (threshold: 15)
  
  Score: 8.4/10

$ qp quality --json    # agent-readable
{
  "score": 8.4,
  "max": 10,
  "checks": {
    "architecture": {"status": "pass", "violations": 0},
    "invariants": {"status": "warn", "errors": 0, "warnings": 1},
    "lint": {"status": "pass"},
    "tests": {"status": "pass", "passed": 342, "failed": 0},
    "coverage": {"status": "pass", "value": 87, "threshold": 80},
    "docs": {"status": "fail", "missing": ["payments", "gateway"]},
    "complexity": {"status": "pass", "max": 12, "threshold": 15}
  }
}
```

Agents can run `qp quality --json` before and after their changes and verify they haven't degraded the codebase. The quality definition is configured in `qp.yaml`:

```yaml
quality:
  checks:
    architecture: { weight: 2 }
    invariants: { weight: 2 }
    lint: { task: lint, weight: 1 }
    tests: { task: test, weight: 2 }
    coverage: { threshold: 80, weight: 1 }
    docs: { weight: 1 }
    complexity: { threshold: 15, weight: 1 }
  
  thresholds:
    fail_below: 6.0
    warn_below: 8.0
```

### 29e. Knowledge Accrual

Agent sessions improve the repo's knowledge base over time. When enabled, agents are instructed to propose updates to `qp.yaml` and documentation based on what they learn during task execution.

```yaml
agent:
  accrue_knowledge: true
```

When `true`, `qp init --docs` generates agent guidance that includes:

- Permission to propose new scope definitions when discovering undocumented code areas
- Permission to propose new invariants when finding recurring issues
- Permission to update domain documentation with findings
- Instructions to propose changes via structured output, not modify config directly

When `false`, agents use qp as-is and don't modify the config or documentation.

This creates a flywheel: agents do work → discover gaps in context/constraints → propose improvements → humans approve → next agent session is more productive.

---

## 30. Scaffolding

**Priority: P1**

Generate project structure, documentation stubs, harness infrastructure, and domain skeletons from the architecture declaration in `qp.yaml`. The same config that enforces structure also generates it.

### Project initialisation

```bash
# Scaffold a new project harness from scratch
$ qp init --scaffold
  Created qp.yaml
  Created AGENTS.md
  Created docs/architecture.md
  Created docs/conventions.md
  Created .qp-secrets (added to .gitignore)
  Created .gitignore additions

# Scaffold from a template
$ qp init --scaffold go-service
  Created qp.yaml (with Go-specific tasks, scopes, conventions)
  Created AGENTS.md
  Created docs/architecture.md
  Created docs/conventions.md
  Created .qp-secrets
```

### Built-in scaffold templates

qp ships with a small set of language-agnostic harness templates for common project topologies:

| Template        | Description                                         |
|-----------------|-----------------------------------------------------|
| `default`       | Minimal harness: `qp.yaml`, AGENTS.md, docs stubs  |
| `service`       | Layered service with domain architecture             |
| `monorepo`      | Multi-service monorepo with shared infrastructure    |
| `cli`           | CLI tool with feature-based layout                   |
| `library`       | Library with public API surface documentation        |

Templates are language-agnostic — they generate the harness infrastructure (config, docs, architecture rules, agent guidance) but not application code. Language-specific project generators (for Go, Python, Rust, etc.) can layer on top of a qp harness scaffold.

### Domain scaffolding

Add a new domain to an existing project. Generates directory structure, documentation stubs, scope definitions, and task entries — all conforming to the declared architecture:

```bash
$ qp scaffold domain payments
  Created src/payments/
  Created src/payments/types/
  Created src/payments/config/
  Created src/payments/repo/
  Created src/payments/service/
  Created docs/domains/payments.md
  Updated qp.yaml:
    - Added scope: payments
    - Added tasks: payments:build, payments:test
  Updated AGENTS.md
```

The directory structure is derived from the architecture declaration:

```yaml
architecture:
  layers: [types, config, repo, service, runtime, ui]
  domains:
    payments:
      root: src/payments
      layers: [types, config, repo, service]  # only these subdirs created
```

### Sync mode

Reconcile the project structure against the architecture declaration. Generates any missing directories and doc stubs without touching existing files:

```bash
$ qp scaffold sync
  Created src/gateway/types/      (declared in architecture, missing on disk)
  Created docs/domains/gateway.md (declared in docs, missing on disk)
  OK: 14 directories, 6 doc files already in sync
```

Useful after adding a new domain to `qp.yaml` — run `scaffold sync` to materialise the structure.

### Documentation stubs

Generated doc stubs are structured templates that prompt both humans and agents to fill in the right information:

```markdown
# Payments Domain

## Purpose
<!-- What business capability does this domain provide? -->

## Key Entities
<!-- List the core types/models in this domain -->

## Dependencies  
<!-- Which other domains does this depend on and why? -->

## API Surface
<!-- What does this domain expose to other domains? -->

## Testing Strategy
<!-- How is this domain tested? Key scenarios? -->
```

An agent reading an unfilled stub knows exactly what information is needed. `qp quality` checks for unfilled stubs (sections still containing only HTML comments) as part of the documentation score.

### AGENTS.md generation

`qp init --docs` generates a tiered AGENTS.md that serves as a table of contents, not a monolithic instruction manual:

```markdown
# Agent Guidance

## Quick Reference
- Task runner config: `qp.yaml`
- Run tasks: `qp run <task>`
- Check quality: `qp quality`
- Architecture rules: `qp arch-check`

## Documentation
- Architecture overview: `docs/architecture.md`
- Coding conventions: `docs/conventions.md`
- Domain docs: `docs/domains/<domain>.md`

## Key Constraints
- Architecture layers are enforced: run `qp arch-check` before committing
- Invariants are enforced: run `qp invariants` before committing
- Minimum quality score: 6.0/10

## Context Commands
- `qp context --task <name>` — get scoped context for a task
- `qp diff-plan` — see which tasks are affected by your changes
- `qp quality --json` — check codebase health before and after changes
```

Short, navigable, points to deeper sources. Regenerated from `qp.yaml` whenever the config changes, so it never goes stale.

### External templates

Scaffold templates can be loaded from external repositories:

```bash
$ qp init --scaffold https://github.com/myorg/qp-templates/go-service
```

This allows organisations to standardise their harness across teams — one template repo, every new project starts with the same architecture rules, conventions, and agent guidance.

### Template authoring

A scaffold template is a directory containing:

```
my-template/
  template.yaml          # template metadata and variable declarations
  qp.yaml.tmpl          # qp.yaml template (Go text/template)
  docs/
    architecture.md.tmpl
    conventions.md.tmpl
  agents.md.tmpl
```

Templates use Go `text/template` syntax with variables declared in `template.yaml`:

```yaml
name: go-service
description: "Layered Go service with domain architecture"
vars:
  app_name: { type: string, required: true, prompt: "Application name" }
  domains: { type: list, default: [], prompt: "Initial domains (comma-separated)" }
```

```bash
$ qp init --scaffold go-service
  Application name: myapp
  Initial domains: auth, payments
  
  Created qp.yaml
  Created docs/architecture.md
  ...
```

---

## 31. Future: qp Studio (UI)

**Priority: P4 — not now, but the architecture supports it**

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
- The UI is a weekend project once the foundation is solid

---

## Implementation Order

| Phase  | Features                                     | Competes with       |
|--------|----------------------------------------------|---------------------|
| **P0** | Rename, daemon mode, coloured output, DAG syntax, variables, task caching/skip, task retry, dry run | make, just          |
| **P1** | CEL engine, conditional branching, profiles, templates, event stream, structured logging, context dump, JSON Schema, declarative MCP server builder, concurrency control, secrets management, diff-plan, harness engineering (architecture enforcement, tiered docs, invariants, quality scoring, knowledge accrual), scaffolding | Jenkins, GitHub Actions |
| **P2** | LLM nodes, shared state, smart branching, MCP client, until, expr node | LangGraph, Dagger   |
| **P3** | Approval gates, flows as MCP tools, security | Temporal, Prefect   |
| **P4** | qp Studio                                   | n8n, Retool         |

Each phase is independently useful and validates the next. Ship P0, get users. Ship P1, get teams. Ship P2, get believers.

---

## Node Type Summary

| Type      | Purpose                                | Input          | Output         |
|-----------|-----------------------------------------|----------------|----------------|
| `cmd`     | Run a shell command                     | Command string | stdout → state |
| `llm`     | Call an LLM API                         | Prompt template| Response → state|
| `mcp`     | Call a tool on an external MCP server   | Tool + input   | Result → state |
| `expr`    | Evaluate CEL and update state           | CEL expressions| State mutation |
| `approve` | Human-in-the-loop gate                  | Message + options | Decision → state |

---

## Full Example: Autonomous Bug Fix Agent

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

## Tagline Options

- *qp — from task runner to agent runtime in one file.*
- *qp — Quickly Please.*
- *qp — like make, but it thinks.*
