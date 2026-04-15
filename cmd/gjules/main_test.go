package main

import (
	"os"
	"testing"
)

func TestResolveSessionID(t *testing.T) {
	// Prepare temp config
	tmpDir, _ := os.MkdirTemp("", "gjules-test")
	defer os.RemoveAll(tmpDir)
	
	originalBase := overriddenBaseDir
	overriddenBaseDir = tmpDir
	defer func() { overriddenBaseDir = originalBase }()

	c := loadConfig()
	c.CurrentUser = "test-user"
	c.SessionAlias["my-alias"] = "12345"
	saveConfig(c)

	// Re-load to ensure user data is merged
	c = loadConfig()

	tests := []struct {
		input    string
		expected string
	}{
		{"my-alias", "sessions/12345"},
		{"67890", "sessions/67890"},
		{"sessions/abc", "sessions/abc"},
	}

	for _, tt := range tests {
		got := resolveSessionID(tt.input)
		if got != tt.expected {
			t.Errorf("resolveSessionID(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestResolveSource(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gjules-test")
	defer os.RemoveAll(tmpDir)
	originalBase := overriddenBaseDir
	overriddenBaseDir = tmpDir
	defer func() { overriddenBaseDir = originalBase }()

	c := loadConfig()
	c.CurrentUser = "test-user"
	c.RepoAlias["gssh"] = "sources/github/forechoandlook/gssh"
	saveConfig(c)

	// Re-load
	c = loadConfig()

	tests := []struct {
		input    string
		expected string
	}{
		{"gssh", "sources/github/forechoandlook/gssh"},
		{"other", "other"},
	}

	for _, tt := range tests {
		got := resolveSource(tt.input)
		if got != tt.expected {
			t.Errorf("resolveSource(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCsvEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello,world", "\"hello,world\""},
		{"quotes \"here\"", "\"quotes \"\"here\"\"\""},
		{"new\nline", "\"new\nline\""},
	}

	for _, tt := range tests {
		got := csvEscape(tt.input)
		if got != tt.expected {
			t.Errorf("csvEscape(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestOrderedKeys(t *testing.T) {
	m := map[string]string{
		"id":      "1",
		"created": "now",
		"title":   "test",
		"unknown": "val",
	}
	got := orderedKeys(m)
	
	// Check predefined order
	order := []string{"id", "title", "created"}
	lastIdx := -1
	for _, ek := range order {
		for i, k := range got {
			if k == ek {
				if i < lastIdx {
					t.Errorf("Key %s out of order", k)
				}
				lastIdx = i
			}
		}
	}
}

func TestParseFields(t *testing.T) {
	args := []string{"--fields=id,title", "my-alias", "--other=val"}
	fields, remaining := parseFields(args)

	if len(fields) != 2 || fields[0] != "id" || fields[1] != "title" {
		t.Errorf("parseFields failed to extract fields: %v", fields)
	}
	if len(remaining) != 2 || remaining[0] != "my-alias" || remaining[1] != "--other=val" {
		t.Errorf("parseFields failed to preserve remaining args: %v", remaining)
	}
}

func TestConfigPersistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gjules-test-config")
	defer os.RemoveAll(tmpDir)
	originalBase := overriddenBaseDir
	overriddenBaseDir = tmpDir
	defer func() { overriddenBaseDir = originalBase }()

	c := loadConfig()
	c.CurrentUser = "test-user"
	c.Users["test-user"] = "key-123"
	c.CurrentRepo = "repo-abc"
	saveConfig(c)

	// Reload and verify
	c2 := loadConfig()
	if c2.CurrentUser != "test-user" || c2.Users["test-user"] != "key-123" || c2.CurrentRepo != "repo-abc" {
		t.Errorf("Config failed to persist. Got: %+v", c2)
	}
}

func TestSplitArgs(t *testing.T) {
	args := []string{"--limit=10", "my-alias", "--detail", "some text"}
	flags, positional := splitArgs(args)

	if len(flags) != 2 || flags[0] != "--limit=10" || flags[1] != "--detail" {
		t.Errorf("splitArgs failed to extract flags: %v", flags)
	}
	if len(positional) != 2 || positional[0] != "my-alias" || positional[1] != "some text" {
		t.Errorf("splitArgs failed to extract positional args: %v", positional)
	}
}
