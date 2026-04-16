package main

import (
	"fmt"
	"os"
	"strings"
)

func parseFields(args []string) (fields []string, remaining []string) {
	for _, a := range args {
		if strings.HasPrefix(a, "--fields=") {
			raw := strings.TrimPrefix(a, "--fields=")
			fields = strings.Split(raw, ",")
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
		} else {
			remaining = append(remaining, a)
		}
	}
	return
}

func splitArgs(args []string) (flags []string, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			// Support --key value in addition to --key=value
			if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, a+"="+args[i+1])
				i++
			} else {
				flags = append(flags, a)
			}
		} else {
			positional = append(positional, a)
		}
	}
	return
}

func hasFlag(flags []string, name string) bool {
	for _, f := range flags {
		if f == "--"+name {
			return true
		}
	}
	return false
}

func selectFields(allFields []string, values map[string]string) []string {
	if len(allFields) == 0 {
		// Return all values in order
		result := make([]string, 0, len(values))
		for _, f := range orderedKeys(values) {
			result = append(result, values[f])
		}
		return result
	}
	result := make([]string, len(allFields))
	for i, f := range allFields {
		result[i] = values[f]
	}
	return result
}

func orderedKeys(m map[string]string) []string {
	// Predefined order for common fields
	order := []string{"alias", "id", "state", "title", "source", "created", "name", "originator", "description", "content", "owner", "repo", "branch"}
	seen := make(map[string]bool)
	var result []string
	for _, k := range order {
		if _, ok := m[k]; ok && !seen[k] {
			result = append(result, k)
			seen[k] = true
		}
	}
	return result
}

func csvFields(fields []string, values map[string]string) string {
	selected := selectFields(fields, values)
	for i := range selected {
		selected[i] = csvEscape(selected[i])
	}
	return strings.Join(selected, ",")
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + s + "\""
	}
	return s
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func resolveSessionID(aliasOrID string) string {
	c := loadConfig()
	id := aliasOrID
	if fullID, ok := c.SessionAlias[aliasOrID]; ok {
		id = fullID
	}
	// Important: We return the ID part ONLY for consistency
	id = strings.TrimPrefix(id, "sessions/")
	return id
}

func resolveSource(aliasOrSource string) string {
	c := loadConfig()
	if source, ok := c.RepoAlias[aliasOrSource]; ok {
		return source
	}
	return aliasOrSource
}
