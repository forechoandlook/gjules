package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// --- Messages ---

func handleMsg(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules msg <list|latest|show|send|approve|wait>")
		os.Exit(1)
	}
	switch args[0] {
	case "show":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjules msg show <sessionAlias> <activityID>")
			os.Exit(1)
		}
		msgShow(args[1], args[2])
	case "list":
		msgList(args[1:])
	case "latest":
		msgLatest(args[1:])
	case "wait":
		_, positional := splitArgs(args[1:])
		target := ""
		if len(positional) >= 1 {
			target = positional[0]
		}
		msgWait(target)
	case "send":
		c := loadConfig()
		_, positional := splitArgs(args[1:])
		target := ""
		text := ""
		if len(positional) >= 2 {
			target = positional[0]
			text = strings.Join(positional[1:], " ")
		} else if len(positional) == 1 {
			target = c.CurrentSession
			text = positional[0]
		} else {
			fmt.Fprintln(os.Stderr, "Usage: gjules msg send [sessionAlias] \"text\"")
			os.Exit(1)
		}
		msgSend(target, text)
	case "approve":
		_, positional := splitArgs(args[1:])
		target := ""
		if len(positional) >= 1 {
			target = positional[0]
		}
		msgApprove(target)
	default:
		fmt.Fprintf(os.Stderr, "Unknown msg command: %s\n", args[0])
		os.Exit(1)
	}
}

func msgList(args []string) {
	flags, positional := splitArgs(args)
	config, sessionID, opts := parseMsgListOptions(flags, positional, 100, "Usage: gjules msg list [sessionAlias] [--fields=...] [--limit=N] [--detail] [--git] [--type=msg|plan|code] [--refresh]")
	_ = config
	filtered, dataTime := listActivities(sessionID, opts)
	if len(filtered) == 0 {
		fmt.Println("No activities found.")
		return
	}

	printActivities(filtered, opts.Fields, opts.Detail, opts.ShowGit, dataTime)
}

func msgLatest(args []string) {
	flags, positional := splitArgs(args)
	count := 1
	remaining := positional
	if len(positional) >= 2 {
		if _, err := fmt.Sscanf(positional[len(positional)-1], "%d", &count); err == nil {
			remaining = positional[:len(positional)-1]
		}
	}

	if count <= 0 {
		fmt.Fprintln(os.Stderr, "Usage: gjules msg latest [sessionAlias] [N] [--fields=...] [--detail] [--git] [--type=msg|plan|code] [--refresh]")
		os.Exit(1)
	}

	config, sessionID, opts := parseMsgListOptions(flags, remaining, count, "Usage: gjules msg latest [sessionAlias] [N] [--fields=...] [--detail] [--git] [--type=msg|plan|code] [--refresh]")
	_ = config
	opts.Limit = count
	filtered, dataTime := listActivities(sessionID, opts)
	if len(filtered) == 0 {
		fmt.Println("No activities found.")
		return
	}
	printActivities(filtered, opts.Fields, opts.Detail, opts.ShowGit, dataTime)
}

type msgListOptions struct {
	Fields     []string
	Limit      int
	Detail     bool
	ShowGit    bool
	Debug      bool
	Refresh    bool
	FilterType string
}

func parseMsgListOptions(flags []string, positional []string, defaultLimit int, usage string) (*Config, string, msgListOptions) {
	fields, _ := parseFields(flags)
	c := loadConfig()
	sessionID := ""
	if len(positional) > 0 {
		sessionID = resolveSessionID(positional[0])
	} else if c.CurrentSession != "" {
		sessionID = c.CurrentSession
	} else {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	opts := msgListOptions{
		Fields:  fields,
		Limit:   defaultLimit,
		Detail:  false,
		ShowGit: false,
		Debug:   false,
		Refresh: false,
	}
	for _, a := range flags {
		if strings.HasPrefix(a, "--limit=") {
			fmt.Sscanf(a, "--limit=%d", &opts.Limit)
		} else if a == "--detail" {
			opts.Detail = true
		} else if a == "--git" {
			opts.ShowGit = true
		} else if a == "--debug" {
			opts.Debug = true
		} else if a == "--refresh" {
			opts.Refresh = true
		} else if strings.HasPrefix(a, "--type=") {
			opts.FilterType = strings.TrimPrefix(a, "--type=")
		}
	}
	return c, sessionID, opts
}

func listActivities(sessionID string, opts msgListOptions) ([]Activity, time.Time) {
	c := loadConfig()
	key := readKey()
	var allActivities []Activity
	dataTime := time.Now()

	cacheFile := ""
	if c.CurrentUser != "" {
		cacheFile = activityCachePath(c.CurrentUser, sessionID)
	}

	if !opts.Refresh && cacheFile != "" {
		if b, err := os.ReadFile(cacheFile); err == nil {
			if err := json.Unmarshal(b, &allActivities); err == nil && len(allActivities) > 0 {
				if opts.Debug {
					fmt.Fprintln(os.Stderr, "Loaded activities from cache.")
				}
				if stat, statErr := os.Stat(cacheFile); statErr == nil {
					dataTime = stat.ModTime()
				}
				return filterActivities(allActivities, opts.FilterType, opts.Limit), dataTime
			}
		}
	}

	pageToken := ""
	for {
		path := fmt.Sprintf("/sessions/%s/activities?pageSize=50", sessionID)
		if pageToken != "" {
			path += "&pageToken=" + url.QueryEscape(pageToken)
		}

		resp, err := do(key, "GET", path)
		if err != nil {
			die(err)
		}
		defer resp.Body.Close()
		checkResp(resp)

		bodyBytes, _ := io.ReadAll(resp.Body)
		var r struct {
			Activities    []Activity `json:"activities"`
			NextPageToken string     `json:"nextPageToken"`
		}
		if err := json.Unmarshal(bodyBytes, &r); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding activities: %v\n", err)
			break
		}

		allActivities = append(allActivities, r.Activities...)
		pageToken = r.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if cacheFile != "" && len(allActivities) > 0 {
		os.MkdirAll(filepath.Dir(cacheFile), 0755)
		if b, err := json.Marshal(allActivities); err == nil {
			os.WriteFile(cacheFile, b, 0600)
		}
	}

	return filterActivities(allActivities, opts.FilterType, opts.Limit), dataTime
}

func filterActivities(allActivities []Activity, filterType string, limit int) []Activity {
	if len(allActivities) == 0 {
		return nil
	}
	var filtered []Activity
	for _, a := range allActivities {
		if filterType != "" {
			isCode := false
			for _, art := range a.Artifacts {
				if art.ChangeSet != nil {
					isCode = true
					break
				}
			}
			isPlan := a.PlanGenerated != nil
			isMsg := a.AgentMessaged != nil || a.UserMessaged != nil

			match := false
			switch filterType {
			case "msg":
				match = isMsg
			case "plan":
				match = isPlan
			case "code":
				match = isCode
			}
			if !match {
				continue
			}
		}
		filtered = append(filtered, a)
	}

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered
}

func printActivities(activities []Activity, fields []string, detail bool, showGit bool, dataTime time.Time) {
	headerFields := fields
	if len(headerFields) == 0 {
		headerFields = []string{"originator", "content"}
	}
	fmt.Println(strings.Join(headerFields, ","))

	for _, a := range activities {
		content := renderActivityContent(a, detail, showGit)
		t, _ := time.Parse(time.RFC3339, a.CreateTime)
		values := map[string]string{
			"id":          a.ID,
			"originator":  a.Originator,
			"description": a.Description,
			"content":     content,
			"created":     t.Local().Format("2006-01-02 15:04:05"),
			"name":        a.Name,
		}

		fmt.Println(csvFields(headerFields, values))
	}
	fmt.Printf("data_time: %s\n", dataTime.Local().Format("2006-01-02 15:04:05"))
}

func renderActivityContent(a Activity, detail bool, showGit bool) string {
	var parts []string

	if a.AgentMessaged != nil {
		parts = append(parts, a.AgentMessaged.AgentMessage)
	}
	if a.UserMessaged != nil {
		parts = append(parts, a.UserMessaged.UserMessage)
	}
	if a.PlanGenerated != nil {
		var titles []string
		for _, s := range a.PlanGenerated.Plan.Steps {
			title := s.Title
			if detail && s.Description != "" {
				title += "\n  - " + s.Description
			}
			titles = append(titles, title)
		}
		sep := "; "
		if detail {
			sep = "\n"
		}
		parts = append(parts, "Plan:\n"+strings.Join(titles, sep))
	}
	if a.PlanApproved != nil {
		parts = append(parts, "Plan Approved: "+a.PlanApproved.PlanID)
	}
	if a.ProgressUpdated != nil {
		content := a.ProgressUpdated.Title
		if a.ProgressUpdated.Description != "" {
			content += ": " + a.ProgressUpdated.Description
		}
		if content != "" {
			parts = append(parts, content)
		}
	}

	for _, art := range a.Artifacts {
		if art.ChangeSet != nil {
			if showGit {
				parts = append(parts, "Code Change:\n"+art.ChangeSet.GitPatch.UnidiffPatch)
			} else {
				parts = append(parts, summarizeChangeSet(art.ChangeSet))
			}
		} else if art.Media != nil {
			parts = append(parts, "Media Artifact")
		}
	}

	content := strings.Join(parts, "\n")
	if content == "" && a.Description != "" {
		content = a.Description
	}

	if !detail {
		content = strings.ReplaceAll(content, "\n", " ")
		if len(content) > 60 {
			content = content[:57] + "..."
		}
	}
	return content
}

func summarizeChangeSet(cs *ChangeSet) string {
	return "ChangeSet"
}

type changeFile struct {
	Path   string
	Status string
}

func parseChangeSetFiles(patch string) []changeFile {
	lines := strings.Split(patch, "\n")
	files := map[string]changeFile{}
	currentPath := ""

	flush := func() {
		if currentPath == "" {
			return
		}
		if _, ok := files[currentPath]; !ok {
			files[currentPath] = changeFile{Path: currentPath, Status: "modified"}
		}
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			parts := strings.Fields(line)
			currentPath = ""
			if len(parts) >= 4 {
				currentPath = strings.TrimPrefix(parts[3], "b/")
			}
		case strings.HasPrefix(line, "rename to "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
			currentPath = path
			files[path] = changeFile{Path: path, Status: "renamed"}
		case strings.HasPrefix(line, "+++ b/"):
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ b/"))
			if path != "" {
				currentPath = path
				entry := files[path]
				if entry.Path == "" {
					entry.Path = path
				}
				if entry.Status == "" {
					entry.Status = "modified"
				}
				files[path] = entry
			}
		case line == "+++ /dev/null":
			if currentPath != "" {
				entry := files[currentPath]
				entry.Path = currentPath
				entry.Status = "deleted"
				files[currentPath] = entry
			}
		case line == "--- /dev/null":
			if currentPath != "" {
				entry := files[currentPath]
				entry.Path = currentPath
				entry.Status = "added"
				files[currentPath] = entry
			}
		case strings.HasPrefix(line, "new file mode "):
			if currentPath != "" {
				entry := files[currentPath]
				entry.Path = currentPath
				entry.Status = "added"
				files[currentPath] = entry
			}
		case strings.HasPrefix(line, "deleted file mode "):
			if currentPath != "" {
				entry := files[currentPath]
				entry.Path = currentPath
				entry.Status = "deleted"
				files[currentPath] = entry
			}
		}
	}
	flush()

	result := make([]changeFile, 0, len(files))
	for _, file := range files {
		result = append(result, file)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func classifyChangeFile(path string) string {
	lower := strings.ToLower(path)

	switch {
	case lower == "go.mod" || lower == "go.sum" || lower == "package.json" || lower == "package-lock.json" || lower == "pnpm-lock.yaml" || lower == "yarn.lock" || lower == "cargo.toml" || lower == "cargo.lock" || lower == "requirements.txt":
		return "deps"
	case strings.Contains(lower, "/testdata/"),
		strings.Contains(lower, "/__tests__/"),
		strings.Contains(lower, "/tests/"),
		strings.Contains(lower, ".test."),
		strings.Contains(lower, "_test."),
		strings.HasSuffix(lower, "_test.go"),
		strings.HasSuffix(lower, ".spec.ts"),
		strings.HasSuffix(lower, ".spec.tsx"),
		strings.HasSuffix(lower, ".spec.js"),
		strings.HasSuffix(lower, ".spec.jsx"):
		return "tests"
	case strings.HasPrefix(lower, ".github/"),
		strings.HasPrefix(lower, ".gitlab/"),
		strings.Contains(lower, "/workflows/"):
		return "ci"
	case strings.HasPrefix(lower, "docs/"),
		strings.HasSuffix(lower, ".md"),
		strings.HasSuffix(lower, ".mdx"),
		strings.HasSuffix(lower, ".rst"),
		strings.HasSuffix(lower, ".adoc"):
		return "docs"
	case strings.HasSuffix(lower, ".json"),
		strings.HasSuffix(lower, ".yaml"),
		strings.HasSuffix(lower, ".yml"),
		strings.HasSuffix(lower, ".toml"),
		strings.HasSuffix(lower, ".ini"),
		strings.HasSuffix(lower, ".conf"),
		strings.HasSuffix(lower, ".env"),
		strings.Contains(lower, "/config/"),
		strings.Contains(lower, "/configs/"):
		return "config"
	case strings.HasSuffix(lower, ".png"),
		strings.HasSuffix(lower, ".jpg"),
		strings.HasSuffix(lower, ".jpeg"),
		strings.HasSuffix(lower, ".gif"),
		strings.HasSuffix(lower, ".svg"),
		strings.HasSuffix(lower, ".webp"):
		return "assets"
	case strings.HasSuffix(lower, ".go"),
		strings.HasSuffix(lower, ".rs"),
		strings.HasSuffix(lower, ".py"),
		strings.HasSuffix(lower, ".js"),
		strings.HasSuffix(lower, ".jsx"),
		strings.HasSuffix(lower, ".ts"),
		strings.HasSuffix(lower, ".tsx"),
		strings.HasSuffix(lower, ".java"),
		strings.HasSuffix(lower, ".kt"),
		strings.HasSuffix(lower, ".c"),
		strings.HasSuffix(lower, ".cc"),
		strings.HasSuffix(lower, ".cpp"),
		strings.HasSuffix(lower, ".h"),
		strings.HasSuffix(lower, ".hpp"),
		strings.HasSuffix(lower, ".rb"),
		strings.HasSuffix(lower, ".php"),
		strings.HasSuffix(lower, ".sh"):
		return "code"
	default:
		return "other"
	}
}

func msgSend(sessionAlias, text string) {
	c := loadConfig()
	sessionID := resolveSessionID(sessionAlias)
	if sessionID == "" {
		sessionID = c.CurrentSession
	}
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "Error: No session specified.")
		os.Exit(1)
	}
	key := readKey()
	body, _ := json.Marshal(map[string]string{"prompt": text})
	resp, err := do(key, "POST", fmt.Sprintf("/sessions/%s:sendMessage", sessionID), strings.NewReader(string(body)))
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	fmt.Printf("Message sent to session %s.\n", sessionID)
}

func msgApprove(sessionAlias string) {
	c := loadConfig()
	sessionID := resolveSessionID(sessionAlias)
	if sessionID == "" {
		sessionID = c.CurrentSession
	}
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "Error: No session specified.")
		os.Exit(1)
	}
	key := readKey()
	resp, err := do(key, "POST", fmt.Sprintf("/sessions/%s:approvePlan", sessionID), strings.NewReader("{}"))
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	fmt.Printf("Plan approved for session %s.\n", sessionID)
}

func msgWait(sessionAlias string) {
	c := loadConfig()
	sessionID := resolveSessionID(sessionAlias)
	if sessionID == "" {
		sessionID = c.CurrentSession
	}
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "Error: No session specified.")
		os.Exit(1)
	}
	key := readKey()
	fmt.Printf("Waiting for session %s...\n", sessionID)
	for {
		resp, err := do(key, "GET", "/sessions/"+sessionID)
		if err != nil {
			die(err)
		}
		defer resp.Body.Close()
		checkResp(resp)
		bodyBytes, _ := io.ReadAll(resp.Body)
		var s struct {
			State string `json:"state"`
		}
		json.Unmarshal(bodyBytes, &s)
		if strings.HasPrefix(s.State, "AWAITING_") || s.State == "COMPLETED" || s.State == "CANCELLED" {
			fmt.Printf("\nSession %s is now %s\n", sessionID, s.State)
			notify("Session " + s.State)
			return
		}
		fmt.Print(".")
		time.Sleep(5 * time.Second)
	}
}

func msgShow(sessionAlias, activityID string) {
	sessionID := resolveSessionID(sessionAlias)
	key := readKey()
	path := fmt.Sprintf("/sessions/%s/activities/%s", sessionID, activityID)
	resp, result, err := doJSON(key, "GET", path, nil)
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
}

func notify(msg string) {
	fmt.Print("\a")
	switch runtime.GOOS {
	case "darwin":
		exec.Command("osascript", "-e", fmt.Sprintf("display notification %q with title \"Gjules Update\"", msg)).Run()
	case "linux":
		exec.Command("notify-send", "Gjules Update", msg).Run()
	case "windows":
		psCommand := fmt.Sprintf("$ws = New-Object -ComObject WScript.Shell; $ws.Popup(%q, 0, \"Gjules Update\", 64)", msg)
		exec.Command("powershell", "-Command", psCommand).Run()
	}
}
