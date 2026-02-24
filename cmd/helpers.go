package cmd

import "strings"

// parseInputs converts ["key=value", ...] to a map.
func parseInputs(raw []string) map[string]string {
	m := map[string]string{}
	for _, kv := range raw {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
