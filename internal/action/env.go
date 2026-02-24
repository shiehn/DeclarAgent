package action

import (
	"fmt"
	"os"
)

// EnvGet implements env.get action.
type EnvGet struct{}

func (e *EnvGet) Execute(params map[string]string) (map[string]string, error) {
	name := params["name"]
	if name == "" {
		return nil, fmt.Errorf("env.get: missing required param 'name'")
	}
	val, ok := os.LookupEnv(name)
	if !ok {
		return nil, fmt.Errorf("env.get: environment variable %q not set", name)
	}
	return map[string]string{"value": val}, nil
}

func (e *EnvGet) DryRun(params map[string]string) string {
	return fmt.Sprintf("Would read environment variable %q", params["name"])
}
