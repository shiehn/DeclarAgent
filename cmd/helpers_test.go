package cmd

import "testing"

func TestParseInputs(t *testing.T) {
	raw := []string{"env=prod", "tag=v1.0"}
	m := parseInputs(raw)
	if m["env"] != "prod" {
		t.Errorf("expected env=prod, got %q", m["env"])
	}
	if m["tag"] != "v1.0" {
		t.Errorf("expected tag=v1.0, got %q", m["tag"])
	}
}

func TestParseInputsEmpty(t *testing.T) {
	m := parseInputs(nil)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestParseInputsValueWithEquals(t *testing.T) {
	raw := []string{"query=a=b=c"}
	m := parseInputs(raw)
	if m["query"] != "a=b=c" {
		t.Errorf("expected value with equals signs preserved, got %q", m["query"])
	}
}

func TestParseInputsSkipsMalformed(t *testing.T) {
	raw := []string{"noequals", "good=value"}
	m := parseInputs(raw)
	if len(m) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m))
	}
	if m["good"] != "value" {
		t.Errorf("expected good=value, got %q", m["good"])
	}
}
