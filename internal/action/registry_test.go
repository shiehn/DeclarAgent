package action

import "testing"

func TestGetKnownActions(t *testing.T) {
	names := []string{"file.write", "file.append", "json.get", "json.set", "env.get", "http"}
	for _, name := range names {
		a, err := Get(name)
		if err != nil {
			t.Errorf("Get(%q) returned error: %v", name, err)
		}
		if a == nil {
			t.Errorf("Get(%q) returned nil action", name)
		}
	}
}

func TestGetUnknownActionReturnsError(t *testing.T) {
	_, err := Get("nonexistent.action")
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestKnownReturnsTrueForRegistered(t *testing.T) {
	names := []string{"file.write", "file.append", "json.get", "json.set", "env.get", "http"}
	for _, name := range names {
		if !Known(name) {
			t.Errorf("Known(%q) returned false, want true", name)
		}
	}
}

func TestKnownReturnsFalseForUnregistered(t *testing.T) {
	if Known("nonexistent.action") {
		t.Error("Known(\"nonexistent.action\") returned true, want false")
	}
}
