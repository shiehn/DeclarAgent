package action

import "fmt"

// Action is the interface for built-in actions.
type Action interface {
	Execute(params map[string]string) (outputs map[string]string, err error)
	DryRun(params map[string]string) string
}

var registry = map[string]Action{}

func init() {
	registry["file.write"] = &FileWrite{}
	registry["file.append"] = &FileAppend{}
	registry["json.get"] = &JSONGet{}
	registry["json.set"] = &JSONSet{}
	registry["env.get"] = &EnvGet{}
	registry["http"] = NewHTTPAction()
}

// Get returns an action by name.
func Get(name string) (Action, error) {
	a, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown action %q", name)
	}
	return a, nil
}

// Known returns true if the action name is registered.
func Known(name string) bool {
	_, ok := registry[name]
	return ok
}
