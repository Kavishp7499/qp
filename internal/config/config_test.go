package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
project: demo
tasks:
  test:
    desc: Run tests
    cmd: echo test
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Serve.Transport != "stdio" {
		t.Fatalf("Serve.Transport = %q, want stdio", cfg.Serve.Transport)
	}
	if cfg.Serve.Port != 8080 {
		t.Fatalf("Serve.Port = %d, want 8080", cfg.Serve.Port)
	}
	if cfg.Serve.TokenEnv != "QP_MCP_TOKEN" {
		t.Fatalf("Serve.TokenEnv = %q, want QP_MCP_TOKEN", cfg.Serve.TokenEnv)
	}
	if cfg.Watch.DebounceMS != 500 {
		t.Fatalf("Watch.DebounceMS = %d, want 500", cfg.Watch.DebounceMS)
	}
	if cfg.Context.Caps.GitDiffLines != 200 {
		t.Fatalf("Context.Caps.GitDiffLines = %d, want 200", cfg.Context.Caps.GitDiffLines)
	}
}

func TestLoadRejectsUnknownTaskScope(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  check:
    desc: Check the repo
    cmd: echo check
    scope: missing
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want scope validation error")
	}
	if !strings.Contains(err.Error(), `references unknown scope "missing"`) {
		t.Fatalf("Load() error = %v, want unknown scope", err)
	}
}

func TestLoadIncludesMergeTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "backend.yaml"), []byte(`
tasks:
  test:
    desc: Test
    cmd: go test ./...
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  - tasks/backend.yaml
tasks:
  check:
    desc: Check
    steps: [test]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := cfg.Tasks["test"]; !ok {
		t.Fatal("included task test missing")
	}
	if _, ok := cfg.Tasks["check"]; !ok {
		t.Fatal("main task check missing")
	}
}

func TestLoadIncludesRejectsTaskCollisions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "backend.yaml"), []byte(`
tasks:
  check:
    desc: Check from include
    cmd: echo include
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  - tasks/backend.yaml
tasks:
  check:
    desc: Check from root
    cmd: echo root
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want include collision error")
	}
	if !strings.Contains(err.Error(), `task "check" already defined`) {
		t.Fatalf("Load() error = %v, want task collision message", err)
	}
}

func TestLoadRejectsCircularTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  one:
    desc: one
    steps: [two]
  two:
    desc: two
    steps: [one]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want cycle error")
	}
	if !strings.Contains(err.Error(), "circular task dependency") {
		t.Fatalf("Load() error = %v, want cycle message", err)
	}
}

func TestLoadRejectsTaskSelfReference(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  check:
    desc: Check
    steps: [check]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want self-reference cycle error")
	}
	if !strings.Contains(err.Error(), "circular task dependency") {
		t.Fatalf("Load() error = %v, want cycle message", err)
	}
}

func TestLoadRejectsCircularDependencies(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  one:
    desc: one
    cmd: echo one
    needs: [two]
  two:
    desc: two
    cmd: echo two
    needs: [one]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want cycle error")
	}
	if !strings.Contains(err.Error(), "circular task dependency") {
		t.Fatalf("Load() error = %v, want cycle message", err)
	}
}

func TestLoadRejectsTaskWithRunAndNeeds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: build
    run: test -> lint
    needs: [setup]
  test:
    desc: test
    cmd: echo test
  lint:
    desc: lint
    cmd: echo lint
  setup:
    desc: setup
    cmd: echo setup
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want run/needs validation error")
	}
	if !strings.Contains(err.Error(), `run and needs are mutually exclusive`) {
		t.Fatalf("Load() error = %v, want run/needs validation", err)
	}
}

func TestLoadRejectsUnknownRunTaskReference(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: build
    run: test -> missing
  test:
    desc: test
    cmd: echo test
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want unknown run task validation error")
	}
	if !strings.Contains(err.Error(), `references unknown run task "missing"`) {
		t.Fatalf("Load() error = %v, want unknown run task", err)
	}
}

func TestLoadRejectsCircularRunDependencies(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  one:
    desc: one
    run: two
  two:
    desc: two
    run: one
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want cycle error")
	}
	if !strings.Contains(err.Error(), "circular task dependency") {
		t.Fatalf("Load() error = %v, want cycle message", err)
	}
}

func TestLoadRejectsParamWithoutEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  add-feature:
    desc: Add feature
    cmd: make add-feature
    params:
      feature:
        required: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want param validation error")
	}
	if !strings.Contains(err.Error(), `param "feature": env is required`) {
		t.Fatalf("Load() error = %v, want param env validation", err)
	}
}

func TestLoadSupportsCacheBooleanAndMapping(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  cache-bool:
    desc: cache bool
    cmd: echo ok
    cache: true
  cache-map:
    desc: cache map
    cmd: echo ok
    cache:
      paths:
        - "**/*.go"
        - go.mod
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Tasks["cache-bool"].CacheEnabled() {
		t.Fatal("cache-bool cache disabled, want enabled")
	}
	cachePathsTask := cfg.Tasks["cache-map"]
	if !cachePathsTask.CacheEnabled() {
		t.Fatal("cache-map cache disabled, want enabled")
	}
	if got := cachePathsTask.CachePaths(); len(got) != 2 || got[0] != "**/*.go" || got[1] != "go.mod" {
		t.Fatalf("cache paths = %#v, want [\"**/*.go\", \"go.mod\"]", got)
	}
}

func TestLoadResolvesShellVars(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
vars:
  region: us-east-1
  git_sha:
    sh: "echo abc123"
tasks:
  show:
    desc: show
    cmd: echo ok
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Vars["region"] != "us-east-1" {
		t.Fatalf("Vars[region] = %q, want us-east-1", cfg.Vars["region"])
	}
	if cfg.Vars["git_sha"] != "abc123" {
		t.Fatalf("Vars[git_sha] = %q, want abc123", cfg.Vars["git_sha"])
	}
}

func TestLoadFailsWhenShellVarCommandFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
vars:
  broken:
    sh: "exit 42"
tasks:
  show:
    desc: show
    cmd: echo ok
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want shell var failure")
	}
	if !strings.Contains(err.Error(), `vars.broken`) {
		t.Fatalf("Load() error = %v, want vars.broken context", err)
	}
}

func TestLoadRejectsUnknownSafety(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
    safety: spicy
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want safety validation error")
	}
	if !strings.Contains(err.Error(), `unknown safety "spicy"`) {
		t.Fatalf("Load() error = %v, want safety validation", err)
	}
}

func TestLoadRejectsInvalidArchitectureRule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  check:
    desc: Run checks
    cmd: echo ok
architecture:
  layers: [repo, service]
  domains:
    auth:
      root: src/auth
      layers: [repo, service]
  rules:
    - direction: sideways
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want architecture rule validation error")
	}
	if !strings.Contains(err.Error(), `unknown direction "sideways"`) {
		t.Fatalf("Load() error = %v, want architecture rule validation", err)
	}
}

func TestLoadRejectsArchitectureDomainUnknownLayer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  check:
    desc: Run checks
    cmd: echo ok
architecture:
  layers: [repo, service]
  domains:
    auth:
      root: src/auth
      layers: [repo, mystery]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want architecture domain layer validation error")
	}
	if !strings.Contains(err.Error(), `references unknown layer "mystery"`) {
		t.Fatalf("Load() error = %v, want unknown layer validation", err)
	}
}

func TestLoadRejectsInvalidWhenExpression(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
    when: branch() ==
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want invalid when validation error")
	}
	if !strings.Contains(err.Error(), `invalid when expression`) {
		t.Fatalf("Load() error = %v, want invalid when validation", err)
	}
}

func TestLoadRejectsDuplicateParamPositions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: Build
    cmd: echo build
    params:
      target:
        env: TARGET
        position: 1
      profile:
        env: PROFILE
        position: 1
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want duplicate position error")
	}
	if !strings.Contains(err.Error(), `share position 1`) {
		t.Fatalf("Load() error = %v, want duplicate position validation", err)
	}
}

func TestLoadRejectsVariadicParamWithoutPosition(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: Build
    cmd: echo build
    params:
      files:
        env: FILES
        variadic: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want variadic position error")
	}
	if !strings.Contains(err.Error(), `variadic params must also declare a position`) {
		t.Fatalf("Load() error = %v, want variadic position validation", err)
	}
}

func TestLoadRejectsVariadicParamThatIsNotLast(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: Build
    cmd: echo build
    params:
      files:
        env: FILES
        position: 1
        variadic: true
      target:
        env: TARGET
        position: 2
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want variadic ordering error")
	}
	if !strings.Contains(err.Error(), `variadic param must have the highest position`) {
		t.Fatalf("Load() error = %v, want variadic ordering validation", err)
	}
}

func TestLoadRejectsAliasToUnknownTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
aliases:
  t: missing
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want alias validation error")
	}
	if !strings.Contains(err.Error(), `alias "t" references unknown task "missing"`) {
		t.Fatalf("Load() error = %v, want alias validation", err)
	}
}

func TestLoadRejectsAliasConflictWithTaskName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
aliases:
  test: test
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want alias conflict error")
	}
	if !strings.Contains(err.Error(), `alias "test" conflicts with task of the same name`) {
		t.Fatalf("Load() error = %v, want alias conflict validation", err)
	}
}

func TestLoadWithProfileAppliesVarAndTaskOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
vars:
  region: us-east-1
tasks:
  deploy:
    desc: Deploy
    cmd: ./deploy.sh
    timeout: 1m
    env:
      REGION: "{{vars.region}}"
profiles:
  prod:
    vars:
      region: eu-west-1
    tasks:
      deploy:
        when: branch() == "main"
        timeout: 10m
        env:
          REGION: "{{vars.region}}"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadWithProfile(filepath.Join(dir, "qp.yaml"), "prod")
	if err != nil {
		t.Fatalf("LoadWithProfile() error = %v", err)
	}
	if cfg.Vars["region"] != "eu-west-1" {
		t.Fatalf("Vars[region] = %q, want eu-west-1", cfg.Vars["region"])
	}
	task := cfg.Tasks["deploy"]
	if task.Timeout != "10m" {
		t.Fatalf("deploy timeout = %q, want 10m", task.Timeout)
	}
	if task.When != `branch() == "main"` {
		t.Fatalf("deploy when = %q, want profile override", task.When)
	}
}

func TestLoadSupportsProfilesDefaultExpression(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QP_PROFILE", "prod")
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
vars:
  region: us-east-1
tasks:
  deploy:
    desc: Deploy
    cmd: ./deploy.sh
profiles:
  _default: "{{env.QP_PROFILE}}"
  prod:
    vars:
      region: eu-west-1
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Profiles.Default; got != "{{env.QP_PROFILE}}" {
		t.Fatalf("Profiles.Default = %q, want expression", got)
	}
}

func TestApplyProfilesSupportsStacking(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
vars:
  region: us-east-1
  tier: base
tasks:
  deploy:
    desc: Deploy
    cmd: ./deploy.sh
profiles:
  staging:
    vars:
      region: eu-west-1
  high-memory:
    vars:
      tier: high
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := cfg.ApplyProfiles([]string{"staging", "high-memory"}); err != nil {
		t.Fatalf("ApplyProfiles() error = %v", err)
	}
	if got := cfg.Vars["region"]; got != "eu-west-1" {
		t.Fatalf("Vars[region] = %q, want eu-west-1", got)
	}
	if got := cfg.Vars["tier"]; got != "high" {
		t.Fatalf("Vars[tier] = %q, want high", got)
	}
	if got := cfg.ActiveProfile(); got != "high-memory" {
		t.Fatalf("ActiveProfile() = %q, want high-memory", got)
	}
}

func TestLoadAcceptsDefaultAlias(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
default: verify
tasks:
  check:
    desc: Check the repo
    cmd: echo check
aliases:
  verify: check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Default != "verify" {
		t.Fatalf("Default = %q, want verify", cfg.Default)
	}
}

func TestLoadRejectsUnknownDefaultTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
default: missing
tasks:
  check:
    desc: Check the repo
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want default validation error")
	}
	if !strings.Contains(err.Error(), `default task "missing" does not match a task or alias`) {
		t.Fatalf("Load() error = %v, want default validation", err)
	}
}

func TestLoadRejectsReservedParamName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
    params:
      dry-run:
        env: MODE
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want reserved param validation error")
	}
	if !strings.Contains(err.Error(), `task "test" param "dry-run" uses a reserved CLI flag name`) {
		t.Fatalf("Load() error = %v, want reserved param validation", err)
	}
}

func TestLoadRejectsUnknownErrorFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    error_format: nope
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want error_format validation error")
	}
	if !strings.Contains(err.Error(), `task "test": unknown error_format "nope"`) {
		t.Fatalf("Load() error = %v, want error_format validation", err)
	}
}

func TestLoadRejectsInvalidRetryBackoff(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  flaky:
    desc: Flaky
    cmd: echo flaky
    retry: 2
    retry_backoff: quadratic
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want retry_backoff validation error")
	}
	if !strings.Contains(err.Error(), `unknown retry_backoff "quadratic"`) {
		t.Fatalf("Load() error = %v, want retry_backoff validation", err)
	}
}

func TestLoadRejectsInvalidRetryOnCondition(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  flaky:
    desc: Flaky
    cmd: echo flaky
    retry: 2
    retry_on:
      - network
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want retry_on validation error")
	}
	if !strings.Contains(err.Error(), `unknown retry_on condition "network"`) {
		t.Fatalf("Load() error = %v, want retry_on validation", err)
	}
}

func TestLoadResolvesSecretsFromEnvAndFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "env-secret")
	if err := os.WriteFile(filepath.Join(dir, ".qp-secrets"), []byte("DB_PASSWORD=file-secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
secrets:
  openai_key:
    from: env
    env: OPENAI_API_KEY
  db_password:
    from: file
    path: .qp-secrets
    key: DB_PASSWORD
tasks:
  test:
    desc: Test
    cmd: echo ok
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	secrets := cfg.SecretValues()
	if got := secrets["openai_key"]; got != "env-secret" {
		t.Fatalf("secret openai_key = %q, want env-secret", got)
	}
	if got := secrets["db_password"]; got != "file-secret" {
		t.Fatalf("secret db_password = %q, want file-secret", got)
	}
}

func TestLoadExpandsTaskTemplates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
templates:
  greet:
    params:
      name:
        type: string
        required: true
    tasks:
      hello:
        desc: Hello
        cmd: printf "hello {{param.name}}"
      check:
        desc: Check
        steps: [hello]

tasks:
  world:
    desc: World instance
    use: greet
    params:
      name: world
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := cfg.Tasks["world:hello"]; !ok {
		t.Fatal("generated task world:hello missing")
	}
	if _, ok := cfg.Tasks["world:check"]; !ok {
		t.Fatal("generated task world:check missing")
	}
	if got := cfg.Tasks["world:hello"].Cmd; got != `printf "hello world"` {
		t.Fatalf("world:hello cmd = %q, want interpolated template value", got)
	}
	if got := cfg.Tasks["world"].Steps; len(got) != 2 || got[0] != "world:check" || got[1] != "world:hello" {
		t.Fatalf("world steps = %#v, want generated task steps", got)
	}
	if got := cfg.Tasks["world:check"].Steps; len(got) != 1 || got[0] != "world:hello" {
		t.Fatalf("world:check steps = %#v, want namespaced dependency", got)
	}
}

func TestLoadExpandsTaskTemplateOverrides(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
templates:
  svc:
    params:
      service:
        type: string
        required: true
    tasks:
      build:
        desc: Build
        cmd: printf "build {{param.service}}"
      deploy:
        desc: Deploy
        cmd: printf "deploy {{param.service}}"
        when: branch() == "main"

tasks:
  auth:
    desc: Auth service
    use: svc
    params:
      service: auth
    override:
      tasks:
        deploy:
          when: env("DEPLOY") == "1"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	deploy := cfg.Tasks["auth:deploy"]
	if deploy.When != `env("DEPLOY") == "1"` {
		t.Fatalf("auth:deploy when = %q, want override expression", deploy.When)
	}
}

func TestLoadRejectsGroupWithUnknownTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
groups:
  qa:
    desc: Verification tasks
    tasks:
      - test
      - missing
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want group validation error")
	}
	if !strings.Contains(err.Error(), `group "qa" references unknown task "missing"`) {
		t.Fatalf("Load() error = %v, want group validation", err)
	}
}

func TestLoadRejectsUnknownTaskDependency(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: Build the app
    cmd: echo build
    needs:
      - setup
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want dependency validation error")
	}
	if !strings.Contains(err.Error(), `task "build" references unknown dependency "setup"`) {
		t.Fatalf("Load() error = %v, want dependency validation", err)
	}
}

func TestLoadRejectsUnknownDefaultDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
defaults:
  dir: missing
tasks:
  test:
    desc: Run tests
    cmd: echo test
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want defaults.dir validation error")
	}
	if !strings.Contains(err.Error(), `defaults.dir "missing"`) {
		t.Fatalf("Load() error = %v, want defaults.dir validation", err)
	}
}

func TestLoadRejectsCodemapPackageWithoutDesc(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
codemap:
  packages:
    internal/runner: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want codemap validation error")
	}
	if !strings.Contains(err.Error(), `codemap package "internal/runner": desc is required`) {
		t.Fatalf("Load() error = %v, want codemap validation", err)
	}
}

func TestValidateGuardsInIsolation(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: map[string]Task{
			"test": {Desc: "test", Cmd: "echo test"},
		},
		Guards: map[string]Guard{
			"default": {Steps: []string{"test"}},
		},
	}
	if err := validateGuards(cfg, ""); err != nil {
		t.Fatalf("validateGuards() = %v, want nil", err)
	}

	cfg.Guards["broken"] = Guard{Steps: []string{"missing"}}
	if err := validateGuards(cfg, ""); err == nil {
		t.Fatal("validateGuards() = nil, want error for unknown task")
	}
}

func TestValidateAliasesInIsolation(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: map[string]Task{
			"test": {Desc: "test", Cmd: "echo test"},
		},
		Aliases: map[string]string{
			"t": "test",
		},
	}
	if err := validateAliases(cfg, ""); err != nil {
		t.Fatalf("validateAliases() = %v, want nil", err)
	}

	cfg.Aliases["test"] = "test"
	if err := validateAliases(cfg, ""); err == nil {
		t.Fatal("validateAliases() = nil, want conflict error")
	}
}
