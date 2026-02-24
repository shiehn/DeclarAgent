package action

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// JSONGet implements json.get action.
type JSONGet struct{}

func (j *JSONGet) Execute(params map[string]string) (map[string]string, error) {
	file := params["file"]
	path := params["path"]
	if file == "" {
		return nil, fmt.Errorf("json.get: missing required param 'file'")
	}
	if path == "" {
		return nil, fmt.Errorf("json.get: missing required param 'path'")
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("json.get: %w", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("json.get: %w", err)
	}

	val, err := getPath(obj, strings.Split(path, "."))
	if err != nil {
		return nil, fmt.Errorf("json.get: %w", err)
	}

	return map[string]string{"value": fmt.Sprintf("%v", val)}, nil
}

func (j *JSONGet) DryRun(params map[string]string) string {
	return fmt.Sprintf("Would read %s from %s", params["path"], params["file"])
}

// JSONSet implements json.set action.
type JSONSet struct{}

func (j *JSONSet) Execute(params map[string]string) (map[string]string, error) {
	file := params["file"]
	path := params["path"]
	value := params["value"]
	if file == "" {
		return nil, fmt.Errorf("json.set: missing required param 'file'")
	}
	if path == "" {
		return nil, fmt.Errorf("json.set: missing required param 'path'")
	}

	var obj map[string]any
	data, err := os.ReadFile(file)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("json.set: %w", err)
		}
		obj = map[string]any{}
	} else {
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("json.set: %w", err)
		}
	}

	setPath(obj, strings.Split(path, "."), value)

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("json.set: %w", err)
	}
	if err := os.WriteFile(file, out, 0o644); err != nil {
		return nil, fmt.Errorf("json.set: %w", err)
	}

	return map[string]string{"file": file}, nil
}

func (j *JSONSet) DryRun(params map[string]string) string {
	return fmt.Sprintf("Would set %s = %q in %s", params["path"], params["value"], params["file"])
}

func getPath(obj map[string]any, keys []string) (any, error) {
	current := any(obj)
	for _, k := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("key %q: not an object", k)
		}
		v, ok := m[k]
		if !ok {
			return nil, fmt.Errorf("key %q not found", k)
		}
		current = v
	}
	return current, nil
}

func setPath(obj map[string]any, keys []string, value string) {
	for i := 0; i < len(keys)-1; i++ {
		next, ok := obj[keys[i]]
		if !ok {
			next = map[string]any{}
			obj[keys[i]] = next
		}
		obj = next.(map[string]any)
	}
	obj[keys[len(keys)-1]] = value
}
