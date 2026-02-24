package action

import (
	"os"
	"testing"
)

func TestEnvGetExistingVar(t *testing.T) {
	os.Setenv("DECLARAGENT_TEST_VAR", "hello123")
	defer os.Unsetenv("DECLARAGENT_TEST_VAR")

	eg := &EnvGet{}
	out, err := eg.Execute(map[string]string{"name": "DECLARAGENT_TEST_VAR"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["value"] != "hello123" {
		t.Errorf("expected 'hello123', got %q", out["value"])
	}
}

func TestEnvGetMissingVarError(t *testing.T) {
	os.Unsetenv("DECLARAGENT_NONEXISTENT_VAR")
	eg := &EnvGet{}
	_, err := eg.Execute(map[string]string{"name": "DECLARAGENT_NONEXISTENT_VAR"})
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

func TestEnvGetDryRun(t *testing.T) {
	eg := &EnvGet{}
	desc := eg.DryRun(map[string]string{"name": "HOME"})
	if desc == "" {
		t.Fatal("expected non-empty dry run description")
	}
}
