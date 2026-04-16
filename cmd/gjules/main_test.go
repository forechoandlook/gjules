package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		{"my-alias", "12345"},
		{"67890", "67890"},
		{"sessions/abc", "abc"},
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

func TestSummarizeChangeSet(t *testing.T) {
	cs := &ChangeSet{}
	cs.GitPatch.UnidiffPatch = strings.Join([]string{
		"diff --git a/cmd/app/main.go b/cmd/app/main.go",
		"index 1111111..2222222 100644",
		"--- a/cmd/app/main.go",
		"+++ b/cmd/app/main.go",
		"diff --git a/docs/usage.md b/docs/usage.md",
		"index 3333333..4444444 100644",
		"--- a/docs/usage.md",
		"+++ b/docs/usage.md",
		"diff --git a/cmd/app/main_test.go b/cmd/app/main_test.go",
		"new file mode 100644",
		"--- /dev/null",
		"+++ b/cmd/app/main_test.go",
		"diff --git a/.github/workflows/release.yml b/.github/workflows/release.yml",
		"index 5555555..6666666 100644",
		"--- a/.github/workflows/release.yml",
		"+++ b/.github/workflows/release.yml",
	}, "\n")

	got := summarizeChangeSet(cs)
	want := "ChangeSet[code, tests, docs, ci] (4 files) [new]"
	if got != want {
		t.Fatalf("summarizeChangeSet() = %q; want %q", got, want)
	}
}

func TestRenderActivityContent(t *testing.T) {
	a := Activity{
		Originator: "agent",
		AgentMessaged: &AgentMessaged{
			AgentMessage: "done",
		},
		Artifacts: []Artifact{
			{
				ChangeSet: &ChangeSet{
					GitPatch: struct {
						BaseCommitID string `json:"baseCommitId"`
						UnidiffPatch string `json:"unidiffPatch"`
					}{
						UnidiffPatch: strings.Join([]string{
							"diff --git a/README.md b/README.md",
							"--- a/README.md",
							"+++ b/README.md",
						}, "\n"),
					},
				},
			},
		},
	}

	got := renderActivityContent(a, false, false)
	want := "done ChangeSet[docs] (1 files)"
	if got != want {
		t.Fatalf("renderActivityContent() = %q; want %q", got, want)
	}
}

func TestSwitchToSessionBranchFromBaseCommit(t *testing.T) {
	repo := initTempGitRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "base")
	baseCommit := gitStdout(t, repo, "rev-parse", "HEAD")

	writeRepoFile(t, repo, "README.md", "base\nnext\n")
	runGit(t, repo, "commit", "-am", "next")

	if err := switchToSessionBranch(repo, "3389472899446525136", strings.TrimSpace(baseCommit)); err != nil {
		t.Fatalf("switchToSessionBranch() error = %v", err)
	}

	gotBranch := gitStdout(t, repo, "branch", "--show-current")
	if strings.TrimSpace(gotBranch) != "3389472899446525136" {
		t.Fatalf("branch = %q; want session branch", gotBranch)
	}

	gotHead := gitStdout(t, repo, "rev-parse", "HEAD")
	if strings.TrimSpace(gotHead) != strings.TrimSpace(baseCommit) {
		t.Fatalf("HEAD = %q; want base commit %q", gotHead, baseCommit)
	}
}

func TestSwitchToSessionBranchRejectsMismatchedExistingBranch(t *testing.T) {
	repo := initTempGitRepo(t)
	writeRepoFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "base")
	baseCommit := strings.TrimSpace(gitStdout(t, repo, "rev-parse", "HEAD"))

	writeRepoFile(t, repo, "README.md", "base\nother\n")
	runGit(t, repo, "commit", "-am", "other")
	runGit(t, repo, "switch", "-c", "3389472899446525136")
	otherCommit := strings.TrimSpace(gitStdout(t, repo, "rev-parse", "HEAD"))
	runGit(t, repo, "switch", "main")

	err := switchToSessionBranch(repo, "3389472899446525136", baseCommit)
	if err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), otherCommit) || !strings.Contains(err.Error(), baseCommit) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want int
	}{
		{"0.6.4", "0.6.4", 0},
		{"0.6.5", "0.6.4", 1},
		{"0.6.4", "0.6.5", -1},
		{"v0.7.0", "0.6.9", 1},
	}

	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Fatalf("compareVersions(%q, %q) = %d; want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "test")
	runGit(t, repo, "config", "user.email", "test@example.com")
	return repo
}

func writeRepoFile(t *testing.T, repo, relPath, content string) {
	t.Helper()
	path := filepath.Join(repo, relPath)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func gitStdout(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}
