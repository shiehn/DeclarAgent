package plan

// Plan is the top-level runbook structure.
type Plan struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description,omitempty"`
	Inputs      map[string]Input `yaml:"inputs,omitempty"`
	Steps       []Step           `yaml:"steps"`
}

// Input defines a plan-level input parameter.
type Input struct {
	Required    bool   `yaml:"required,omitempty"`
	Description string `yaml:"description,omitempty"`
	Default     string `yaml:"default,omitempty"`
}

// Step defines a single step in a plan.
type Step struct {
	ID          string            `yaml:"id"`
	Description string            `yaml:"description,omitempty"`
	Run         string            `yaml:"run,omitempty"`
	Action      string            `yaml:"action,omitempty"`
	Params      map[string]string `yaml:"params,omitempty"`
	Outputs     map[string]string `yaml:"outputs,omitempty"`
	Destructive bool              `yaml:"destructive,omitempty"`
}
