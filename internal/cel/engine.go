package cel

import (
	"fmt"
	"regexp"
	"sort"

	celgo "github.com/google/cel-go/cel"
)

var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Eval(expression string, vars map[string]any) (any, error) {
	envOpts := make([]celgo.EnvOption, 0, len(vars))
	for _, name := range sortedNames(vars) {
		if !identPattern.MatchString(name) {
			continue
		}
		envOpts = append(envOpts, celgo.Variable(name, celgo.DynType))
	}
	env, err := celgo.NewEnv(envOpts...)
	if err != nil {
		return nil, err
	}

	ast, iss := env.Parse(expression)
	if iss != nil && iss.Err() != nil {
		return nil, iss.Err()
	}
	checked, iss := env.Check(ast)
	if iss != nil && iss.Err() != nil {
		return nil, iss.Err()
	}

	program, err := env.Program(checked)
	if err != nil {
		return nil, err
	}
	out, _, err := program.Eval(vars)
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

func (e *Engine) EvalBool(expression string, vars map[string]any) (bool, error) {
	value, err := e.Eval(expression, vars)
	if err != nil {
		return false, err
	}
	boolean, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("expression %q did not evaluate to bool", expression)
	}
	return boolean, nil
}

func sortedNames(vars map[string]any) []string {
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
