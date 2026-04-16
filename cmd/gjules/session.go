package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// --- Sessions ---

func sessions(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "show":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "Usage: gjules sessions show <id|alias>")
				os.Exit(1)
			}
			sessionShow(args[1])
			return
		case "rm":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "Usage: gjules sessions rm <id|alias>")
				os.Exit(1)
			}
			sessionRm(args[1])
			return
		case "apply":
			sessionApply(args[1:])
			return
		}
	}
	flags, _ := splitArgs(args)
	fields, _ := parseFields(flags)
	if len(fields) == 0 {
		fields = []string{"alias", "id", "state", "title", "created", "name"}
	}

	limit := 20
	refresh := false
	filter := ""
	for _, a := range flags {
		if strings.HasPrefix(a, "--limit=") {
			fmt.Sscanf(a, "--limit=%d", &limit)
		} else if a == "--refresh" {
			refresh = true
		} else if strings.HasPrefix(a, "--filter=") {
			filter = strings.TrimPrefix(a, "--filter=")
		}
	}

	c := loadConfig()
	if !refresh && len(c.SessionsCache) > 0 && time.Since(c.SessCacheTime) < 5*time.Minute {
		printSessions(fields, c.SessionsCache, limit, filter)
		return
	}

	key := readKey()
	pageToken := ""
	var allSessions []CachedSession
	first := true

	for {
		path := "/sessions?pageSize=100"
		if pageToken != "" {
			path += "&pageToken=" + url.QueryEscape(pageToken)
		}

		resp, err := do(key, "GET", path)
		if err != nil {
			die(err)
		}
		defer resp.Body.Close()
		checkResp(resp)

		var r struct {
			Sessions []struct {
				Name       string `json:"name"`
				ID         string `json:"id"`
				Title      string `json:"title"`
				State      string `json:"state"`
				CreateTime string `json:"createTime"`
			} `json:"sessions"`
			NextPageToken string `json:"nextPageToken"`
		}
		json.NewDecoder(resp.Body).Decode(&r)

		for _, s := range r.Sessions {
			allSessions = append(allSessions, CachedSession{
				Name:       s.Name,
				ID:         s.ID,
				Title:      s.Title,
				State:      s.State,
				CreateTime: s.CreateTime,
			})
		}

		pageToken = r.NextPageToken
		if pageToken == "" {
			break
		}
	}

	c.SessionsCache = allSessions
	c.SessCacheTime = time.Now()
	saveConfig(c)

	if first {
		fmt.Println(strings.Join(fields, ","))
		first = false
	}
	printSessions(fields, allSessions, limit, filter)
}

func printSessions(fields []string, sessions []CachedSession, limit int, filter string) {
	c := loadConfig()
	reverseAlias := make(map[string]string)
	for alias, id := range c.SessionAlias {
		reverseAlias[id] = alias
	}

	count := 0
	for _, s := range sessions {
		state := s.State
		isTodo := strings.HasPrefix(state, "AWAITING_")
		isActive := state != "COMPLETED" && state != "CANCELLED"

		match := true
		switch filter {
		case "todo":
			match = isTodo
		case "active":
			match = isActive
		case "done":
			match = state == "COMPLETED"
		}

		if !match {
			continue
		}

		if limit > 0 && count >= limit {
			break
		}
		count++

		t, _ := time.Parse(time.RFC3339, s.CreateTime)
		alias := reverseAlias[s.ID]
		if alias == "" {
			alias = "-"
		}

		displayState := state
		if isTodo {
			displayState = "[!] " + state
		}

		values := map[string]string{
			"alias":   alias,
			"id":      s.ID,
			"state":   displayState,
			"title":   s.Title,
			"created": t.Local().Format("2006-01-02 15:04:05"),
			"name":    s.Name,
		}
		fmt.Println(csvFields(fields, values))
	}
}

func sessionShow(target string) {
	sessionID := resolveSessionID(target)
	key := readKey()
	resp, result, err := doJSON(key, "GET", "/sessions/"+sessionID, nil)
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
}

func sessionRm(target string) {
	sessionID := resolveSessionID(target)
	key := readKey()
	resp, err := do(key, "DELETE", "/sessions/"+sessionID)
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	fmt.Printf("Session %s deleted successfully.\n", sessionID)
}

func sessionApply(args []string) {
	target := ""
	dir := "."
	for _, a := range args {
		if strings.HasPrefix(a, "--dir=") {
			dir = strings.TrimPrefix(a, "--dir=")
		} else if target == "" {
			target = a
		}
	}
	sessionID := resolveSessionID(target)
	key := readKey()
	fmt.Printf("Fetching latest code changes for session %s...\n", sessionID)
	var patch string
	var baseCommitID string
	resp, result, err := doJSON(key, "GET", "/sessions/"+sessionID, nil)
	if err == nil {
		defer resp.Body.Close()
		if outputs, ok := result["outputs"].([]interface{}); ok {
			for _, o := range outputs {
				if outMap, ok := o.(map[string]interface{}); ok {
					if cs, ok := outMap["changeSet"].(map[string]interface{}); ok {
						if gp, ok := cs["gitPatch"].(map[string]interface{}); ok {
							if base, ok := gp["baseCommitId"].(string); ok {
								baseCommitID = strings.TrimSpace(base)
							}
							if p, ok := gp["unidiffPatch"].(string); ok {
								patch = p
								goto applyPatch
							}
						}
					}
				}
			}
		}
	}
	{
		path := fmt.Sprintf("/sessions/%s/activities?pageSize=50", sessionID)
		resp, err := do(key, "GET", path)
		if err == nil {
			defer resp.Body.Close()
			var r struct {
				Activities []Activity `json:"activities"`
			}
			json.NewDecoder(resp.Body).Decode(&r)
			for _, a := range r.Activities {
				for _, art := range a.Artifacts {
					if art.ChangeSet != nil {
						if baseCommitID == "" {
							baseCommitID = strings.TrimSpace(art.ChangeSet.GitPatch.BaseCommitID)
						}
						patch = art.ChangeSet.GitPatch.UnidiffPatch
						goto applyPatch
					}
				}
			}
		}
	}
applyPatch:
	if patch == "" {
		fmt.Fprintln(os.Stderr, "Error: No code changes found.")
		os.Exit(1)
	}
	branchName := sessionApplyBranchName(sessionID)
	if baseCommitID == "" {
		fmt.Fprintln(os.Stderr, "Error: No base commit found in session output.")
		os.Exit(1)
	}
	if err := switchToSessionBranch(dir, branchName, baseCommitID); err != nil {
		fmt.Fprintf(os.Stderr, "Error switching to branch %s at base commit %s: %v\n", branchName, baseCommitID, err)
		os.Exit(1)
	}
	gitArgs := []string{"apply", "-"}
	if dir != "." {
		gitArgs = append([]string{"-C", dir}, gitArgs...)
	}
	cmd := exec.Command("git", gitArgs...)
	cmd.Stdin = strings.NewReader(patch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error applying patch: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Successfully applied code changes to %s on branch %s (base %s).\n", dir, branchName, baseCommitID)
}

func sessionApplyBranchName(sessionID string) string {
	return sessionID
}

func switchToSessionBranch(dir, branchName, baseCommitID string) error {
	if err := execGitSilent(dir, "cat-file", "-e", baseCommitID+"^{commit}"); err != nil {
		return fmt.Errorf("base commit not found locally: %s", baseCommitID)
	}

	currentBranch, err := gitOutput(dir, "branch", "--show-current")
	if err == nil && strings.TrimSpace(currentBranch) == branchName {
		currentCommit, commitErr := gitOutput(dir, "rev-parse", "HEAD")
		if commitErr == nil && strings.TrimSpace(currentCommit) == baseCommitID {
			return nil
		}
		return fmt.Errorf("branch %s already exists at %s, expected base %s", branchName, strings.TrimSpace(currentCommit), baseCommitID)
	}

	existingCommit, err := gitOutput(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branchName)
	if err == nil {
		if strings.TrimSpace(existingCommit) != baseCommitID {
			return fmt.Errorf("branch %s already exists at %s, expected base %s", branchName, strings.TrimSpace(existingCommit), baseCommitID)
		}
		return execGit(dir, "switch", branchName)
	}

	return execGit(dir, "switch", "-c", branchName, baseCommitID)
}

func execGitSilent(dir string, args ...string) error {
	cmd := exec.Command("git", gitDirArgs(dir, args...)...)
	return cmd.Run()
}

func execGit(dir string, args ...string) error {
	cmd := exec.Command("git", gitDirArgs(dir, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", gitDirArgs(dir, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitDirArgs(dir string, args ...string) []string {
	if dir == "." {
		return args
	}
	return append([]string{"-C", dir}, args...)
}

func handleNew(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gjules new \"prompt\" [flags]")
		fmt.Fprintln(os.Stderr, "Flags:")
		fmt.Fprintln(os.Stderr, "  --repo=<alias>      Specify repository")
		fmt.Fprintln(os.Stderr, "  --branch=<name>     Specify starting branch")
		fmt.Fprintln(os.Stderr, "  --auto-pr           Enable automatic PR creation")
		fmt.Fprintln(os.Stderr, "  --require-approval  Require plan approval (default: false)")
		os.Exit(1)
	}

	repo := ""
	branch := ""
	autoPR := false
	requireApproval := false
	repoSet := false
	var promptParts []string
	for _, a := range args {
		if strings.HasPrefix(a, "--repo=") {
			repo = strings.TrimPrefix(a, "--repo=")
			repoSet = true
		} else if strings.HasPrefix(a, "--branch=") {
			branch = strings.TrimPrefix(a, "--branch=")
		} else if a == "--auto-pr" {
			autoPR = true
		} else if a == "--require-approval" {
			requireApproval = true
		} else {
			promptParts = append(promptParts, a)
		}
	}

	if len(promptParts) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gjules new \"prompt\"")
		os.Exit(1)
	}

	newSession(strings.Join(promptParts, " "), repo, branch, autoPR, requireApproval, repoSet)
}

func newSession(prompt, repoAlias, branch string, autoPR, requireApproval, repoSet bool) {
	key := readKey()
	c := loadConfig()

	body := map[string]interface{}{
		"prompt":              prompt,
		"requirePlanApproval": requireApproval,
	}

	if autoPR {
		body["automationMode"] = "AUTO_CREATE_PR"
	}

	repo := repoAlias
	if !repoSet && repo == "" {
		repo = c.CurrentRepo
	}

	if repo != "" {
		src := resolveSource(repo)
		sourceContext := map[string]interface{}{
			"source": src,
		}
		if branch != "" {
			sourceContext["githubRepoContext"] = map[string]interface{}{
				"startingBranch": branch,
			}
		} else {
			sourceContext["githubRepoContext"] = map[string]interface{}{}
		}
		body["sourceContext"] = sourceContext
	}

	resp, result, err := doJSON(key, "POST", "/sessions", body)
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)

	fmt.Printf("Session created!\n")
	if name, ok := result["name"].(string); ok {
		fmt.Printf("  Name:  %s\n", name)
	}
	if id, ok := result["id"].(string); ok {
		fmt.Printf("  ID:    %s\n", id)
	}
	if state, ok := result["state"].(string); ok {
		fmt.Printf("  State: %s\n", state)
	}
	if url, ok := result["url"].(string); ok {
		fmt.Printf("  URL:   %s\n", url)
	}
}
