package skill

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shiehn/declaragent/internal/plan"
)

func TestRenderMinimal(t *testing.T) {
	var buf bytes.Buffer
	err := Render(&buf, TemplateData{Binary: "declaragent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "declaragent") {
		t.Error("expected binary name in output")
	}
	if !strings.Contains(out, "# DeclarAgent CLI") {
		t.Error("expected heading in output")
	}
	// No plans section when Plans is empty
	if strings.Contains(out, "## Available Plans") {
		t.Error("did not expect plans section with no plans")
	}
}

func TestRenderWithPlans(t *testing.T) {
	var buf bytes.Buffer
	data := TemplateData{
		Binary:   "dag",
		PlansDir: "./plans",
		Plans: []PlanInfo{
			{
				Filename: "deploy.yaml",
				Plan: &plan.Plan{
					Name:        "deploy",
					Description: "Deploy the app",
					Inputs: map[string]plan.Input{
						"env": {Required: true, Description: "Target environment"},
						"tag": {Default: "latest"},
					},
				},
			},
		},
	}
	err := Render(&buf, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "dag") {
		t.Error("expected binary name in output")
	}
	if !strings.Contains(out, "## Available Plans") {
		t.Error("expected plans section")
	}
	if !strings.Contains(out, "deploy") {
		t.Error("expected plan name in output")
	}
	if !strings.Contains(out, "Deploy the app") {
		t.Error("expected plan description in output")
	}
	if !strings.Contains(out, "*(required)*") {
		t.Error("expected required marker")
	}
	if !strings.Contains(out, "latest") {
		t.Error("expected default value")
	}
}

func TestRenderContainsCommands(t *testing.T) {
	var buf bytes.Buffer
	err := Render(&buf, TemplateData{Binary: "mybin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, cmd := range []string{"validate", "explain", "dry-run", "run"} {
		if !strings.Contains(out, "mybin "+cmd) {
			t.Errorf("expected %q command in output", cmd)
		}
	}
}
