package plan

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadFile reads and parses a plan YAML file.
func LoadFile(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan file: %w", err)
	}
	return Load(data)
}

// Load parses plan YAML bytes.
func Load(data []byte) (*Plan, error) {
	var p Plan
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	if len(p.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}
	if p.Name == "" {
		return nil, fmt.Errorf("plan has no name")
	}
	return &p, nil
}
