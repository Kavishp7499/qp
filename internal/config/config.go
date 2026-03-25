package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	celpkg "github.com/neural-chilli/qp/internal/cel"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Project         string                `yaml:"project"`
	Description     string                `yaml:"description"`
	Default         string                `yaml:"default"`
	Includes        []string              `yaml:"includes"`
	Vars            Vars                  `yaml:"vars"`
	Secrets         map[string]SecretSpec `yaml:"secrets"`
	Templates       Templates             `yaml:"templates"`
	Profiles        Profiles              `yaml:"profiles"`
	Defaults        DefaultsConfig        `yaml:"defaults"`
	EnvFile         string                `yaml:"env_file"`
	Tasks           map[string]Task       `yaml:"tasks"`
	Aliases         map[string]string     `yaml:"aliases"`
	Groups          map[string]Group      `yaml:"groups"`
	Guards          map[string]Guard      `yaml:"guards"`
	Scopes          map[string]Scope      `yaml:"scopes"`
	Prompts         map[string]Prompt     `yaml:"prompts"`
	Agent           AgentConfig           `yaml:"agent"`
	Context         ContextConfig         `yaml:"context"`
	Codemap         CodemapConfig         `yaml:"codemap"`
	Serve           ServeConfig           `yaml:"serve"`
	Watch           WatchConfig           `yaml:"watch"`
	Architecture    ArchitectureConfig    `yaml:"architecture"`
	activeProfiles  []string              `yaml:"-"`
	resolvedSecrets map[string]string     `yaml:"-"`
}

type Profile struct {
	Vars  map[string]string      `yaml:"vars"`
	Tasks map[string]ProfileTask `yaml:"tasks"`
}

type Profiles struct {
	Default string
	Entries map[string]Profile
}

type Templates struct {
	Snippets map[string]string
	Tasks    map[string]TaskTemplate
}

type TaskTemplate struct {
	Params map[string]TemplateParam `yaml:"params"`
	Tasks  map[string]Task          `yaml:"tasks"`
}

type TemplateParam struct {
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  any    `yaml:"default"`
}

type Vars map[string]string

type ProfileTask struct {
	When    string            `yaml:"when"`
	Timeout string            `yaml:"timeout"`
	Env     map[string]string `yaml:"env"`
}

type Task struct {
	Desc            string            `yaml:"desc"`
	Cmd             string            `yaml:"cmd"`
	Steps           []string          `yaml:"steps"`
	Run             string            `yaml:"run"`
	When            string            `yaml:"when"`
	Cache           *TaskCache        `yaml:"cache"`
	Silent          bool              `yaml:"silent"`
	Defer           string            `yaml:"defer"`
	Needs           []string          `yaml:"needs"`
	Parallel        bool              `yaml:"parallel"`
	Params          map[string]Param  `yaml:"params"`
	Env             map[string]string `yaml:"env"`
	Dir             string            `yaml:"dir"`
	Shell           string            `yaml:"shell"`
	ShellArgs       []string          `yaml:"shell_args"`
	Safety          string            `yaml:"safety"`
	ErrorFormat     string            `yaml:"error_format"`
	Retry           int               `yaml:"retry"`
	RetryDelay      string            `yaml:"retry_delay"`
	RetryBackoff    string            `yaml:"retry_backoff"`
	RetryOn         []string          `yaml:"retry_on"`
	Timeout         string            `yaml:"timeout"`
	ContinueOnError bool              `yaml:"continue_on_error"`
	Agent           *bool             `yaml:"agent"`
	Scope           string            `yaml:"scope"`
	Use             string            `yaml:"use"`
	TemplateArgs    map[string]any    `yaml:"-"`
	Override        TaskUseOverride   `yaml:"override"`
}

type SecretSpec struct {
	From string `yaml:"from"`
	Env  string `yaml:"env"`
	Path string `yaml:"path"`
	Key  string `yaml:"key"`
}

type TaskUseOverride struct {
	Tasks map[string]ProfileTask `yaml:"tasks"`
}

type Param struct {
	Desc     string `yaml:"desc"`
	Env      string `yaml:"env"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
	Position int    `yaml:"position,omitempty"`
	Variadic bool   `yaml:"variadic,omitempty"`
}

type TaskCache struct {
	Enabled bool     `yaml:"enabled"`
	Paths   []string `yaml:"paths"`
}

type VarSpec struct {
	Value string
	Sh    string
}

type Guard struct {
	Steps []string `yaml:"steps"`
}

type Group struct {
	Desc  string   `yaml:"desc,omitempty"`
	Tasks []string `yaml:"tasks"`
}

type Scope struct {
	Desc  string   `yaml:"desc,omitempty"`
	Paths []string `yaml:"paths,omitempty"`
}

type Prompt struct {
	Desc     string `yaml:"desc"`
	Template string `yaml:"template"`
}

type AgentConfig struct {
	AccrueKnowledge bool `yaml:"accrue_knowledge"`
}

type DefaultsConfig struct {
	Dir string `yaml:"dir"`
}

type ContextConfig struct {
	FileTree     *bool       `yaml:"file_tree"`
	GitLogLines  int         `yaml:"git_log_lines"`
	GitDiff      bool        `yaml:"git_diff"`
	Todos        bool        `yaml:"todos"`
	Dependencies *bool       `yaml:"dependencies"`
	AgentFiles   []string    `yaml:"agent_files"`
	Files        []string    `yaml:"files"`
	Include      []string    `yaml:"include"`
	Exclude      []string    `yaml:"exclude"`
	Caps         ContextCaps `yaml:"caps"`
}

type ContextCaps struct {
	FileTreeEntries int `yaml:"file_tree_entries"`
	FilesMax        int `yaml:"files_max"`
	FileLines       int `yaml:"file_lines"`
	GitLogLines     int `yaml:"git_log_lines"`
	GitDiffLines    int `yaml:"git_diff_lines"`
	TodosMax        int `yaml:"todos_max"`
	AgentFileLines  int `yaml:"agent_file_lines"`
	DependencyLines int `yaml:"dependency_lines"`
}

type ServeConfig struct {
	Transport string `yaml:"transport"`
	Port      int    `yaml:"port"`
	TokenEnv  string `yaml:"token_env"`
}

type WatchConfig struct {
	DebounceMS int      `yaml:"debounce_ms"`
	Paths      []string `yaml:"paths"`
}

type CodemapConfig struct {
	Packages    map[string]CodemapPackage `yaml:"packages"`
	Conventions []string                  `yaml:"conventions"`
	Glossary    map[string]string         `yaml:"glossary"`
}

type ArchitectureConfig struct {
	Layers  []string                      `yaml:"layers"`
	Domains map[string]ArchitectureDomain `yaml:"domains"`
	Rules   []ArchitectureRule            `yaml:"rules"`
}

type ArchitectureDomain struct {
	Root   string   `yaml:"root"`
	Layers []string `yaml:"layers"`
}

type ArchitectureRule struct {
	Direction    string `yaml:"direction"`
	CrossDomain  string `yaml:"cross_domain"`
	CrossCutting string `yaml:"cross_cutting"`
}

type CodemapPackage struct {
	Desc        string   `yaml:"desc"`
	KeyTypes    []string `yaml:"key_types"`
	EntryPoints []string `yaml:"entry_points"`
	Conventions []string `yaml:"conventions"`
	DependsOn   []string `yaml:"depends_on"`
}

func (s *Scope) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var paths []string
		if err := value.Decode(&paths); err != nil {
			return err
		}
		s.Paths = paths
		return nil
	case yaml.MappingNode:
		type rawScope Scope
		var raw rawScope
		if err := value.Decode(&raw); err != nil {
			return err
		}
		s.Desc = raw.Desc
		s.Paths = raw.Paths
		return nil
	default:
		return fmt.Errorf("scope must be a path list or mapping")
	}
}

func (t Task) AgentEnabled() bool {
	return t.Agent == nil || *t.Agent
}

func (t Task) CacheEnabled() bool {
	return t.Cache != nil && t.Cache.Enabled
}

func (t Task) CachePaths() []string {
	if t.Cache == nil {
		return nil
	}
	return append([]string(nil), t.Cache.Paths...)
}

func (t Task) SafetyLevel() string {
	if t.Safety != "" {
		return t.Safety
	}
	return "safe"
}

func (t Task) Type() string {
	if t.Cmd != "" {
		return "cmd"
	}
	return "pipeline"
}

func (c *Config) ResolveTaskName(name string) (string, bool) {
	if _, ok := c.Tasks[name]; ok {
		return name, true
	}
	target, ok := c.Aliases[name]
	if !ok {
		return "", false
	}
	_, ok = c.Tasks[target]
	return target, ok
}

func Load(path string) (*Config, error) {
	return LoadWithProfiles(path, nil)
}

func LoadWithProfile(path, profile string) (*Config, error) {
	if profile == "" {
		return LoadWithProfiles(path, nil)
	}
	return LoadWithProfiles(path, []string{profile})
}

func LoadWithProfiles(path string, profiles []string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}

	var rawCfg struct {
		Vars map[string]VarSpec `yaml:"vars"`
	}
	if err := yaml.Unmarshal(raw, &rawCfg); err != nil {
		return nil, err
	}
	resolvedVars, err := resolveVars(rawCfg.Vars, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	cfg.Vars = resolvedVars
	resolvedSecrets, err := resolveSecrets(cfg.Secrets, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	cfg.resolvedSecrets = resolvedSecrets
	if err := cfg.mergeIncludedTasks(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if err := cfg.expandTaskTemplates(); err != nil {
		return nil, err
	}

	if len(profiles) > 0 {
		if err := cfg.ApplyProfiles(profiles); err != nil {
			return nil, err
		}
	}

	cfg.applyDefaults()

	if err := cfg.Validate(filepath.Dir(path)); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) expandTaskTemplates() error {
	if len(c.Tasks) == 0 {
		return nil
	}
	if len(c.Templates.Tasks) == 0 {
		return nil
	}
	for _, instanceName := range orderedTaskNames(c.Tasks) {
		instance := c.Tasks[instanceName]
		if strings.TrimSpace(instance.Use) == "" {
			continue
		}
		template, ok := c.Templates.Tasks[instance.Use]
		if !ok {
			return fmt.Errorf("task %q: unknown template %q", instanceName, instance.Use)
		}
		values, err := resolveTemplateArgs(instanceName, template.Params, instance.TemplateArgs)
		if err != nil {
			return err
		}
		templateTaskNames := map[string]bool{}
		for name := range template.Tasks {
			templateTaskNames[name] = true
		}
		generatedNames := make([]string, 0, len(template.Tasks))
		for _, templateTaskName := range orderedTaskNames(template.Tasks) {
			templateTask := template.Tasks[templateTaskName]
			generatedName := instanceName + ":" + templateTaskName
			if _, exists := c.Tasks[generatedName]; exists {
				return fmt.Errorf("task %q: generated task %q already exists", instanceName, generatedName)
			}
			resolvedTask := applyTemplateValues(templateTask, values)
			for i, step := range resolvedTask.Steps {
				if templateTaskNames[step] {
					resolvedTask.Steps[i] = instanceName + ":" + step
				}
			}
			for i, need := range resolvedTask.Needs {
				if templateTaskNames[need] {
					resolvedTask.Needs[i] = instanceName + ":" + need
				}
			}
			if override, ok := instance.Override.Tasks[templateTaskName]; ok {
				if override.When != "" {
					resolvedTask.When = override.When
				}
				if override.Timeout != "" {
					resolvedTask.Timeout = override.Timeout
				}
				if len(override.Env) > 0 {
					if resolvedTask.Env == nil {
						resolvedTask.Env = map[string]string{}
					}
					for key, value := range override.Env {
						resolvedTask.Env[key] = value
					}
				}
			}
			c.Tasks[generatedName] = resolvedTask
			generatedNames = append(generatedNames, generatedName)
		}
		instance.Use = ""
		instance.TemplateArgs = nil
		instance.Override = TaskUseOverride{}
		instance.Cmd = ""
		instance.Run = ""
		instance.Steps = generatedNames
		instance.Parallel = false
		c.Tasks[instanceName] = instance
	}
	return nil
}

func orderedTaskNames[T any](items map[string]T) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func resolveTemplateArgs(instanceName string, defs map[string]TemplateParam, provided map[string]any) (map[string]string, error) {
	values := map[string]string{}
	for name, def := range defs {
		if provided != nil {
			if value, ok := provided[name]; ok {
				values[name] = fmt.Sprint(value)
				continue
			}
		}
		if def.Default != nil {
			values[name] = fmt.Sprint(def.Default)
			continue
		}
		if def.Required {
			return nil, fmt.Errorf("task %q: template param %q is required", instanceName, name)
		}
	}
	for name, value := range provided {
		if _, ok := defs[name]; ok {
			values[name] = fmt.Sprint(value)
		}
	}
	return values, nil
}

func applyTemplateValues(task Task, values map[string]string) Task {
	rewrite := func(input string) string {
		out := input
		for name, value := range values {
			out = strings.ReplaceAll(out, "{{param."+name+"}}", value)
		}
		return out
	}
	task.Desc = rewrite(task.Desc)
	task.Cmd = rewrite(task.Cmd)
	task.Run = rewrite(task.Run)
	task.When = rewrite(task.When)
	task.Defer = rewrite(task.Defer)
	task.Dir = rewrite(task.Dir)
	task.Shell = rewrite(task.Shell)
	task.ErrorFormat = rewrite(task.ErrorFormat)
	task.Timeout = rewrite(task.Timeout)
	task.RetryDelay = rewrite(task.RetryDelay)
	task.RetryBackoff = rewrite(task.RetryBackoff)
	for i, step := range task.Steps {
		task.Steps[i] = rewrite(step)
	}
	for i, need := range task.Needs {
		task.Needs[i] = rewrite(need)
	}
	for i, arg := range task.ShellArgs {
		task.ShellArgs[i] = rewrite(arg)
	}
	for i, cond := range task.RetryOn {
		task.RetryOn[i] = rewrite(cond)
	}
	if len(task.Env) > 0 {
		next := make(map[string]string, len(task.Env))
		for key, value := range task.Env {
			next[rewrite(key)] = rewrite(value)
		}
		task.Env = next
	}
	return task
}

func (c *Config) mergeIncludedTasks(baseDir string) error {
	if len(c.Includes) == 0 {
		return nil
	}
	if c.Tasks == nil {
		c.Tasks = map[string]Task{}
	}
	for _, includePath := range c.Includes {
		targetPath := includePath
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(baseDir, includePath)
		}
		raw, err := os.ReadFile(targetPath)
		if err != nil {
			return fmt.Errorf("include %q: %w", includePath, err)
		}
		var includeCfg struct {
			Tasks map[string]Task `yaml:"tasks"`
		}
		if err := yaml.Unmarshal(raw, &includeCfg); err != nil {
			return fmt.Errorf("include %q: %w", includePath, err)
		}
		for taskName, task := range includeCfg.Tasks {
			if _, exists := c.Tasks[taskName]; exists {
				return fmt.Errorf("include %q: task %q already defined", includePath, taskName)
			}
			c.Tasks[taskName] = task
		}
	}
	return nil
}

func (c *Config) applyProfile(profile string) error {
	profileCfg, ok := c.Profiles.Entries[profile]
	if !ok {
		return fmt.Errorf("unknown profile %q", profile)
	}
	if c.Vars == nil {
		c.Vars = Vars{}
	}
	for name, value := range profileCfg.Vars {
		c.Vars[name] = value
	}
	for taskName, override := range profileCfg.Tasks {
		task, ok := c.Tasks[taskName]
		if !ok {
			return fmt.Errorf("profile %q references unknown task %q", profile, taskName)
		}
		if override.When != "" {
			task.When = override.When
		}
		if override.Timeout != "" {
			task.Timeout = override.Timeout
		}
		if len(override.Env) > 0 {
			if task.Env == nil {
				task.Env = map[string]string{}
			}
			for key, value := range override.Env {
				task.Env[key] = value
			}
		}
		c.Tasks[taskName] = task
	}
	return nil
}

func (c *Config) ApplyProfiles(profiles []string) error {
	if len(profiles) == 0 {
		c.activeProfiles = nil
		return nil
	}
	seen := map[string]bool{}
	resolved := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		profile = strings.TrimSpace(profile)
		if profile == "" || seen[profile] {
			continue
		}
		if err := c.applyProfile(profile); err != nil {
			return err
		}
		seen[profile] = true
		resolved = append(resolved, profile)
	}
	c.activeProfiles = resolved
	return nil
}

func (c *Config) ActiveProfile() string {
	if len(c.activeProfiles) == 0 {
		return ""
	}
	return c.activeProfiles[len(c.activeProfiles)-1]
}

func (c *Config) ActiveProfiles() []string {
	return append([]string(nil), c.activeProfiles...)
}

func (c *Config) SecretValues() map[string]string {
	if len(c.resolvedSecrets) == 0 {
		return nil
	}
	out := make(map[string]string, len(c.resolvedSecrets))
	for key, value := range c.resolvedSecrets {
		out[key] = value
	}
	return out
}

func (c *Config) applyDefaults() {
	if c.Context.GitLogLines == 0 {
		c.Context.GitLogLines = 10
	}
	if c.Context.Caps.FileTreeEntries == 0 {
		c.Context.Caps.FileTreeEntries = 200
	}
	if c.Context.Caps.FilesMax == 0 {
		c.Context.Caps.FilesMax = 5
	}
	if c.Context.Caps.FileLines == 0 {
		c.Context.Caps.FileLines = 100
	}
	if c.Context.Caps.GitLogLines == 0 {
		c.Context.Caps.GitLogLines = 30
	}
	if c.Context.Caps.GitDiffLines == 0 {
		c.Context.Caps.GitDiffLines = 200
	}
	if c.Context.Caps.TodosMax == 0 {
		c.Context.Caps.TodosMax = 20
	}
	if c.Context.Caps.AgentFileLines == 0 {
		c.Context.Caps.AgentFileLines = 500
	}
	if c.Context.Caps.DependencyLines == 0 {
		c.Context.Caps.DependencyLines = 100
	}
	if c.Serve.Transport == "" {
		c.Serve.Transport = "stdio"
	}
	if c.Serve.Port == 0 {
		c.Serve.Port = 8080
	}
	if c.Serve.TokenEnv == "" {
		c.Serve.TokenEnv = "QP_MCP_TOKEN"
	}
	if c.Watch.DebounceMS == 0 {
		c.Watch.DebounceMS = 500
	}
}

func (c *TaskCache) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var enabled bool
		if err := value.Decode(&enabled); err != nil {
			return err
		}
		c.Enabled = enabled
		c.Paths = nil
		return nil
	case yaml.MappingNode:
		type rawTaskCache struct {
			Enabled *bool    `yaml:"enabled"`
			Paths   []string `yaml:"paths"`
		}
		var raw rawTaskCache
		if err := value.Decode(&raw); err != nil {
			return err
		}
		c.Enabled = true
		if raw.Enabled != nil {
			c.Enabled = *raw.Enabled
		}
		c.Paths = raw.Paths
		return nil
	default:
		return fmt.Errorf("task cache must be a boolean or mapping")
	}
}

func (p *Profiles) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("profiles must be a mapping")
	}
	out := Profiles{Entries: map[string]Profile{}}
	for i := 0; i < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		var key string
		if err := keyNode.Decode(&key); err != nil {
			return err
		}
		if key == "_default" {
			var raw string
			if err := valNode.Decode(&raw); err != nil {
				return fmt.Errorf("profiles._default must be a string")
			}
			out.Default = raw
			continue
		}
		var profile Profile
		if err := valNode.Decode(&profile); err != nil {
			return fmt.Errorf("profiles.%s: %w", key, err)
		}
		out.Entries[key] = profile
	}
	*p = out
	return nil
}

func (t *Templates) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("templates must be a mapping")
	}
	out := Templates{
		Snippets: map[string]string{},
		Tasks:    map[string]TaskTemplate{},
	}
	for i := 0; i < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		var key string
		if err := keyNode.Decode(&key); err != nil {
			return err
		}
		if valNode.Kind == yaml.ScalarNode {
			var snippet string
			if err := valNode.Decode(&snippet); err != nil {
				return fmt.Errorf("templates.%s: %w", key, err)
			}
			out.Snippets[key] = snippet
			continue
		}
		if valNode.Kind == yaml.MappingNode {
			var taskTemplate TaskTemplate
			if err := valNode.Decode(&taskTemplate); err != nil {
				return fmt.Errorf("templates.%s: %w", key, err)
			}
			if len(taskTemplate.Tasks) == 0 {
				return fmt.Errorf("templates.%s: task template must define tasks", key)
			}
			out.Tasks[key] = taskTemplate
			continue
		}
		return fmt.Errorf("templates.%s: expected string snippet or mapping template", key)
	}
	*t = out
	return nil
}

func (t *Task) UnmarshalYAML(value *yaml.Node) error {
	type rawTask struct {
		Desc            string            `yaml:"desc"`
		Cmd             string            `yaml:"cmd"`
		Steps           []string          `yaml:"steps"`
		Run             string            `yaml:"run"`
		When            string            `yaml:"when"`
		Cache           *TaskCache        `yaml:"cache"`
		Silent          bool              `yaml:"silent"`
		Defer           string            `yaml:"defer"`
		Needs           []string          `yaml:"needs"`
		Parallel        bool              `yaml:"parallel"`
		Env             map[string]string `yaml:"env"`
		Dir             string            `yaml:"dir"`
		Shell           string            `yaml:"shell"`
		ShellArgs       []string          `yaml:"shell_args"`
		Safety          string            `yaml:"safety"`
		ErrorFormat     string            `yaml:"error_format"`
		Retry           int               `yaml:"retry"`
		RetryDelay      string            `yaml:"retry_delay"`
		RetryBackoff    string            `yaml:"retry_backoff"`
		RetryOn         []string          `yaml:"retry_on"`
		Timeout         string            `yaml:"timeout"`
		ContinueOnError bool              `yaml:"continue_on_error"`
		Agent           *bool             `yaml:"agent"`
		Scope           string            `yaml:"scope"`
		Use             string            `yaml:"use"`
		Override        TaskUseOverride   `yaml:"override"`
	}
	var raw rawTask
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*t = Task{
		Desc:            raw.Desc,
		Cmd:             raw.Cmd,
		Steps:           raw.Steps,
		Run:             raw.Run,
		When:            raw.When,
		Cache:           raw.Cache,
		Silent:          raw.Silent,
		Defer:           raw.Defer,
		Needs:           raw.Needs,
		Parallel:        raw.Parallel,
		Env:             raw.Env,
		Dir:             raw.Dir,
		Shell:           raw.Shell,
		ShellArgs:       raw.ShellArgs,
		Safety:          raw.Safety,
		ErrorFormat:     raw.ErrorFormat,
		Retry:           raw.Retry,
		RetryDelay:      raw.RetryDelay,
		RetryBackoff:    raw.RetryBackoff,
		RetryOn:         raw.RetryOn,
		Timeout:         raw.Timeout,
		ContinueOnError: raw.ContinueOnError,
		Agent:           raw.Agent,
		Scope:           raw.Scope,
		Use:             raw.Use,
		Override:        raw.Override,
	}
	paramsNode := findMappingNodeValue(value, "params")
	if paramsNode == nil {
		return nil
	}
	if strings.TrimSpace(t.Use) != "" {
		var args map[string]any
		if err := paramsNode.Decode(&args); err != nil {
			return fmt.Errorf("params must be a mapping of template argument values when use is set")
		}
		t.TemplateArgs = args
		t.Params = nil
		return nil
	}
	var params map[string]Param
	if err := paramsNode.Decode(&params); err != nil {
		return err
	}
	t.Params = params
	t.TemplateArgs = nil
	return nil
}

func findMappingNodeValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func (v *Vars) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("vars must be a mapping")
	}
	out := make(Vars)
	for i := 0; i < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]

		var key string
		if err := keyNode.Decode(&key); err != nil {
			return err
		}
		switch valNode.Kind {
		case yaml.ScalarNode:
			var staticValue string
			if err := valNode.Decode(&staticValue); err != nil {
				return err
			}
			out[key] = staticValue
		case yaml.MappingNode:
			var spec VarSpec
			if err := valNode.Decode(&spec); err != nil {
				return err
			}
			out[key] = ""
		default:
			return fmt.Errorf("vars.%s: var value must be a string or mapping", key)
		}
	}
	*v = out
	return nil
}

func (v *VarSpec) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var staticValue string
		if err := value.Decode(&staticValue); err != nil {
			return err
		}
		v.Value = staticValue
		v.Sh = ""
		return nil
	case yaml.MappingNode:
		type rawVarSpec struct {
			Sh string `yaml:"sh"`
		}
		var raw rawVarSpec
		if err := value.Decode(&raw); err != nil {
			return err
		}
		if strings.TrimSpace(raw.Sh) == "" {
			return fmt.Errorf("var mapping must define non-empty sh")
		}
		v.Value = ""
		v.Sh = raw.Sh
		return nil
	default:
		return fmt.Errorf("var value must be a string or mapping")
	}
}

func resolveVars(specs map[string]VarSpec, repoRoot string) (Vars, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)

	resolved := make(Vars, len(specs))
	for _, name := range names {
		spec := specs[name]
		if spec.Sh == "" {
			resolved[name] = spec.Value
			continue
		}
		value, err := runVarShell(spec.Sh, repoRoot)
		if err != nil {
			return nil, fmt.Errorf("vars.%s: %w", name, err)
		}
		resolved[name] = value
	}
	return resolved, nil
}

func runVarShell(command, repoRoot string) (string, error) {
	shell := "sh"
	args := []string{"-c", command}
	if runtime.GOOS == "windows" {
		shell = "cmd"
		args = []string{"/C", command}
	}

	cmd := exec.Command(shell, args...)
	cmd.Dir = repoRoot

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return "", fmt.Errorf("shell command %q failed: %s", command, errText)
		}
		return "", fmt.Errorf("shell command %q failed: %w", command, err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func resolveSecrets(specs map[string]SecretSpec, repoRoot string) (map[string]string, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make(map[string]string, len(specs))
	for _, name := range names {
		spec := specs[name]
		switch spec.From {
		case "env":
			if strings.TrimSpace(spec.Env) == "" {
				return nil, fmt.Errorf("secrets.%s: env is required when from=env", name)
			}
			out[name] = os.Getenv(spec.Env)
		case "file":
			if strings.TrimSpace(spec.Path) == "" {
				return nil, fmt.Errorf("secrets.%s: path is required when from=file", name)
			}
			if strings.TrimSpace(spec.Key) == "" {
				return nil, fmt.Errorf("secrets.%s: key is required when from=file", name)
			}
			path := spec.Path
			if !filepath.IsAbs(path) {
				path = filepath.Join(repoRoot, path)
			}
			values, err := parseSecretsFile(path)
			if err != nil {
				return nil, fmt.Errorf("secrets.%s: %w", name, err)
			}
			out[name] = values[spec.Key]
		default:
			return nil, fmt.Errorf("secrets.%s: unknown from %q", name, spec.From)
		}
	}
	return out, nil
}

func parseSecretsFile(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out, nil
}

func (c *Config) Validate(repoRoot string) error {
	celEngine := celpkg.New()

	if len(c.Tasks) == 0 {
		return fmt.Errorf("qp.yaml must define at least one task")
	}

	if c.Defaults.Dir != "" {
		target := filepath.Join(repoRoot, c.Defaults.Dir)
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("defaults.dir %q: %w", c.Defaults.Dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("defaults.dir %q is not a directory", c.Defaults.Dir)
		}
	}

	for name, task := range c.Tasks {
		if task.Desc == "" {
			return fmt.Errorf("task %q: desc is required", name)
		}
		taskTypeCount := 0
		if task.Cmd != "" {
			taskTypeCount++
		}
		if len(task.Steps) > 0 {
			taskTypeCount++
		}
		if task.Run != "" {
			taskTypeCount++
		}
		if taskTypeCount != 1 {
			return fmt.Errorf("task %q: set exactly one of cmd, steps, or run", name)
		}
		if task.Run != "" && len(task.Needs) > 0 {
			return fmt.Errorf("task %q: run and needs are mutually exclusive", name)
		}
		if task.Run != "" {
			runExpr, err := ParseRunExpr(task.Run)
			if err != nil {
				return fmt.Errorf("task %q: invalid run expression: %w", name, err)
			}
			for _, ref := range RunExprRefs(runExpr) {
				if _, ok := c.Tasks[ref]; !ok {
					return fmt.Errorf("task %q references unknown run task %q", name, ref)
				}
			}
		}
		if task.When != "" {
			if err := celEngine.Validate(task.When); err != nil {
				return fmt.Errorf("task %q: invalid when expression: %w", name, err)
			}
		}
		if task.Dir != "" {
			target := filepath.Join(repoRoot, task.Dir)
			info, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("task %q: dir %q: %w", name, task.Dir, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("task %q: dir %q is not a directory", name, task.Dir)
			}
		}
		if task.Scope != "" {
			if _, ok := c.Scopes[task.Scope]; !ok {
				return fmt.Errorf("task %q references unknown scope %q", name, task.Scope)
			}
		}
		if task.Safety != "" {
			switch task.Safety {
			case "safe", "idempotent", "destructive", "external":
			default:
				return fmt.Errorf("task %q: unknown safety %q", name, task.Safety)
			}
		}
		for _, dep := range task.Needs {
			if _, ok := c.Tasks[dep]; !ok {
				return fmt.Errorf("task %q references unknown dependency %q", name, dep)
			}
		}
		if task.ErrorFormat != "" {
			switch task.ErrorFormat {
			case "go_test", "pytest", "tsc", "eslint", "generic":
			default:
				return fmt.Errorf("task %q: unknown error_format %q", name, task.ErrorFormat)
			}
		}
		if task.Retry < 0 {
			return fmt.Errorf("task %q: retry must be greater than or equal to 0", name)
		}
		if task.RetryDelay != "" {
			if _, err := time.ParseDuration(task.RetryDelay); err != nil {
				return fmt.Errorf("task %q: invalid retry_delay %q: %w", name, task.RetryDelay, err)
			}
		}
		if task.RetryBackoff != "" {
			switch task.RetryBackoff {
			case "fixed", "exponential":
			default:
				return fmt.Errorf("task %q: unknown retry_backoff %q", name, task.RetryBackoff)
			}
		}
		for _, cond := range task.RetryOn {
			cond = strings.TrimSpace(cond)
			if cond == "" {
				continue
			}
			if cond == "any" || strings.HasPrefix(cond, "exit_code:") || strings.HasPrefix(cond, "stderr_contains:") {
				continue
			}
			return fmt.Errorf("task %q: unknown retry_on condition %q", name, cond)
		}
		for paramName, param := range task.Params {
			if isReservedParamName(paramName) {
				return fmt.Errorf("task %q param %q uses a reserved CLI flag name", name, paramName)
			}
			if param.Env == "" {
				return fmt.Errorf("task %q param %q: env is required", name, paramName)
			}
			if param.Position < 0 {
				return fmt.Errorf("task %q param %q: position must be greater than or equal to 1", name, paramName)
			}
		}
		if err := validateParamPositions(name, task.Params); err != nil {
			return err
		}
	}

	for name, entry := range c.Codemap.Packages {
		if entry.Desc == "" {
			return fmt.Errorf("codemap package %q: desc is required", name)
		}
	}

	for alias, target := range c.Aliases {
		if _, ok := c.Tasks[alias]; ok {
			return fmt.Errorf("alias %q conflicts with task of the same name", alias)
		}
		if _, ok := c.Tasks[target]; !ok {
			return fmt.Errorf("alias %q references unknown task %q", alias, target)
		}
	}

	if c.Default != "" {
		if _, ok := c.ResolveTaskName(c.Default); !ok {
			return fmt.Errorf("default task %q does not match a task or alias", c.Default)
		}
	}

	for name, guard := range c.Guards {
		if len(guard.Steps) == 0 {
			return fmt.Errorf("guard %q: steps are required", name)
		}
		for _, step := range guard.Steps {
			if _, ok := c.Tasks[step]; !ok {
				return fmt.Errorf("guard %q references unknown task %q", name, step)
			}
		}
	}

	for name, group := range c.Groups {
		if len(group.Tasks) == 0 {
			return fmt.Errorf("group %q: tasks are required", name)
		}
		for _, taskName := range group.Tasks {
			if _, ok := c.Tasks[taskName]; !ok {
				return fmt.Errorf("group %q references unknown task %q", name, taskName)
			}
		}
	}

	for name, prompt := range c.Prompts {
		if prompt.Desc == "" {
			return fmt.Errorf("prompt %q: desc is required", name)
		}
		if prompt.Template == "" {
			return fmt.Errorf("prompt %q: template is required", name)
		}
	}

	if err := c.validateArchitecture(); err != nil {
		return err
	}

	return c.validateCycles()
}

func (c *Config) validateArchitecture() error {
	if len(c.Architecture.Layers) == 0 && len(c.Architecture.Domains) == 0 && len(c.Architecture.Rules) == 0 {
		return nil
	}

	if len(c.Architecture.Domains) == 0 {
		return fmt.Errorf("architecture: domains are required when architecture is configured")
	}

	layerSet := map[string]bool{}
	for _, layer := range c.Architecture.Layers {
		if layer == "" {
			return fmt.Errorf("architecture: layers cannot contain empty values")
		}
		layerSet[layer] = true
	}

	for name, domain := range c.Architecture.Domains {
		if domain.Root == "" {
			return fmt.Errorf("architecture domain %q: root is required", name)
		}
		for _, layer := range domain.Layers {
			if len(layerSet) > 0 && !layerSet[layer] {
				return fmt.Errorf("architecture domain %q references unknown layer %q", name, layer)
			}
		}
	}

	for _, rule := range c.Architecture.Rules {
		if rule.Direction != "" {
			switch rule.Direction {
			case "forward":
			default:
				return fmt.Errorf("architecture rule: unknown direction %q", rule.Direction)
			}
		}
		if rule.CrossDomain != "" {
			switch rule.CrossDomain {
			case "allow", "deny":
			default:
				return fmt.Errorf("architecture rule: unknown cross_domain policy %q", rule.CrossDomain)
			}
		}
		if rule.CrossCutting != "" && !layerSet[rule.CrossCutting] {
			return fmt.Errorf("architecture rule: cross_cutting %q is not declared in architecture.layers", rule.CrossCutting)
		}
	}
	return nil
}

func isReservedParamName(name string) bool {
	switch name {
	case "dry-run", "json", "param":
		return true
	default:
		return false
	}
}

func validateParamPositions(taskName string, params map[string]Param) error {
	positions := map[int]string{}
	var variadicName string
	var variadicPosition int
	maxPosition := 0

	for name, param := range params {
		if param.Position == 0 {
			if param.Variadic {
				return fmt.Errorf("task %q param %q: variadic params must also declare a position", taskName, name)
			}
			continue
		}
		if param.Position < 1 {
			return fmt.Errorf("task %q param %q: position must be greater than or equal to 1", taskName, name)
		}
		if existing, ok := positions[param.Position]; ok {
			return fmt.Errorf("task %q params %q and %q share position %d", taskName, existing, name, param.Position)
		}
		positions[param.Position] = name
		if param.Position > maxPosition {
			maxPosition = param.Position
		}
		if param.Variadic {
			if variadicName != "" {
				return fmt.Errorf("task %q params %q and %q are both variadic", taskName, variadicName, name)
			}
			variadicName = name
			variadicPosition = param.Position
		}
	}

	if variadicName != "" && variadicPosition != maxPosition {
		return fmt.Errorf("task %q param %q: variadic param must have the highest position", taskName, variadicName)
	}
	return nil
}

func (c *Config) validateCycles() error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	stack := []string{}

	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			start := 0
			for i, item := range stack {
				if item == name {
					start = i
					break
				}
			}
			cycle := append(append([]string{}, stack[start:]...), name)
			return fmt.Errorf("circular task dependency: %s", joinArrow(cycle))
		}
		if visited[name] {
			return nil
		}

		task, ok := c.Tasks[name]
		if !ok {
			return fmt.Errorf("task %q references unknown task", name)
		}

		visiting[name] = true
		stack = append(stack, name)
		for _, dep := range task.Needs {
			if _, ok := c.Tasks[dep]; ok {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		for _, step := range task.Steps {
			if _, ok := c.Tasks[step]; ok {
				if err := visit(step); err != nil {
					return err
				}
			}
		}
		if task.Run != "" {
			runExpr, err := ParseRunExpr(task.Run)
			if err != nil {
				return fmt.Errorf("task %q: invalid run expression: %w", name, err)
			}
			for _, ref := range RunExprRefs(runExpr) {
				if _, ok := c.Tasks[ref]; ok {
					if err := visit(ref); err != nil {
						return err
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		visiting[name] = false
		visited[name] = true
		return nil
	}

	names := make([]string, 0, len(c.Tasks))
	for name := range c.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func joinArrow(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, part := range parts[1:] {
		out += " -> " + part
	}
	return out
}
