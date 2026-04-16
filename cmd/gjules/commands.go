package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// --- User Commands ---

func handleUser(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules user <add|switch|list|rm|current>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user add <name> <key>")
			os.Exit(1)
		}
		userAdd(args[1], args[2])
	case "switch", "use":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user switch <name>")
			os.Exit(1)
		}
		userSwitch(args[1])
	case "list":
		userList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user rm <name>")
			os.Exit(1)
		}
		userRm(args[1])
	case "current":
		userCurrent()
	default:
		fmt.Fprintf(os.Stderr, "Unknown user command: %s\n", args[0])
		os.Exit(1)
	}
}

func userAdd(name, key string) {
	c := loadConfig()
	c.Users[name] = key
	if c.CurrentUser == "" {
		c.CurrentUser = name
	}
	saveConfig(c)
	fmt.Printf("User %q added.\n", name)
}

func userSwitch(name string) {
	c := loadConfig()
	if _, ok := c.Users[name]; !ok {
		fmt.Fprintf(os.Stderr, "User %q not found.\n", name)
		os.Exit(1)
	}
	c.CurrentUser = name
	saveConfig(c)
	fmt.Printf("Switched to user %q.\n", name)
}

func userList() {
	c := loadConfig()
	for name := range c.Users {
		current := ""
		if name == c.CurrentUser {
			current = "*"
		}
		fmt.Printf("%s %s\n", current, name)
	}
}

func userRm(name string) {
	c := loadConfig()
	delete(c.Users, name)
	if c.CurrentUser == name {
		c.CurrentUser = ""
	}
	saveConfig(c)
	fmt.Printf("User %q removed.\n", name)
}

func userCurrent() {
	c := loadConfig()
	if c.CurrentUser == "" {
		fmt.Println("No current user.")
	} else {
		fmt.Printf("Current user: %s\n", c.CurrentUser)
	}
}

// --- Sources ---

func sources(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "show":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "Usage: gjules sources show <id|alias>")
				os.Exit(1)
			}
			sourceShow(args[1])
			return
		}
	}
	flags, _ := splitArgs(args)
	fields, _ := parseFields(flags)
	if len(fields) == 0 {
		fields = []string{"alias", "id", "name", "owner", "repo", "branch"}
	}

	limit := 20
	refresh := false
	for _, a := range flags {
		if strings.HasPrefix(a, "--limit=") {
			fmt.Sscanf(a, "--limit=%d", &limit)
		} else if a == "--refresh" {
			refresh = true
		}
	}

	c := loadConfig()
	if !refresh && len(c.SourcesCache) > 0 && time.Since(c.CacheTime) < 24*time.Hour {
		printSources(fields, c.SourcesCache, limit)
		return
	}

	key := readKey()
	pageToken := ""
	var allSources []CachedSource

	for {
		path := "/sources?pageSize=100"
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
			Sources []struct {
				Name       string `json:"name"`
				ID         string `json:"id"`
				GithubRepo *struct {
					Owner         string `json:"owner"`
					Repo          string `json:"repo"`
					DefaultBranch *struct {
						DisplayName string `json:"displayName"`
					} `json:"defaultBranch"`
				} `json:"githubRepo"`
			} `json:"sources"`
			NextPageToken string `json:"nextPageToken"`
		}
		json.NewDecoder(resp.Body).Decode(&r)

		for _, s := range r.Sources {
			owner := ""
			repo := ""
			branch := ""
			if s.GithubRepo != nil {
				owner = s.GithubRepo.Owner
				repo = s.GithubRepo.Repo
				if s.GithubRepo.DefaultBranch != nil {
					branch = s.GithubRepo.DefaultBranch.DisplayName
				}
			}
			allSources = append(allSources, CachedSource{
				Name:   s.Name,
				ID:     s.ID,
				Owner:  owner,
				Repo:   repo,
				Branch: branch,
			})
		}

		pageToken = r.NextPageToken
		if pageToken == "" {
			break
		}
	}

	c.SourcesCache = allSources
	c.CacheTime = time.Now()
	saveConfig(c)

	fmt.Println(strings.Join(fields, ","))
	printSources(fields, allSources, limit)
}

func printSources(fields []string, sources []CachedSource, limit int) {
	c := loadConfig()
	reverseAlias := make(map[string]string)
	for alias, src := range c.RepoAlias {
		reverseAlias[src] = alias
	}

	for i, s := range sources {
		if limit > 0 && i >= limit {
			break
		}
		alias := reverseAlias[s.Name]
		if alias == "" {
			alias = "-"
		}
		values := map[string]string{
			"name":   s.Name,
			"id":     s.ID,
			"owner":  s.Owner,
			"repo":   s.Repo,
			"branch": s.Branch,
			"alias":  alias,
		}
		fmt.Println(csvFields(fields, values))
	}
}

func handleRepo(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules repo <add|list|rm|use>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjules repo add <alias> <source>")
			os.Exit(1)
		}
		sourceAliasAdd(args[1], args[2])
	case "list":
		sourceAliasList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules repo rm <alias>")
			os.Exit(1)
		}
		sourceAliasRm(args[1])
	case "use":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules repo use <alias>")
			os.Exit(1)
		}
		sourceUse(args[1])
	default:
		fmt.Fprintf(os.Stderr, "Unknown repo command: %s\n", args[0])
		os.Exit(1)
	}
}

func sourceAliasAdd(alias, source string) {
	c := loadConfig()
	src := resolveSource(source)
	if !strings.HasPrefix(src, "sources/") {
		src = "sources/" + source
	}
	c.RepoAlias[alias] = src
	saveConfig(c)
	fmt.Printf("Repo alias %q -> %s\n", alias, src)
}

func sourceAliasList() {
	c := loadConfig()
	if len(c.RepoAlias) == 0 {
		fmt.Println("No repo aliases configured.")
		return
	}
	fmt.Printf("%-15s %s\n", "ALIAS", "SOURCE")
	fmt.Println(strings.Repeat("-", 50))
	for alias, src := range c.RepoAlias {
		current := ""
		if src == c.CurrentRepo {
			current = "*"
		}
		fmt.Printf("%s %-15s %s\n", current, alias, src)
	}
}

func sourceAliasRm(alias string) {
	c := loadConfig()
	if _, ok := c.RepoAlias[alias]; !ok {
		fmt.Fprintf(os.Stderr, "Repo alias %q not found.\n", alias)
		os.Exit(1)
	}
	delete(c.RepoAlias, alias)
	saveConfig(c)
	fmt.Printf("Repo alias %q removed.\n", alias)
}

func sourceUse(alias string) {
	c := loadConfig()
	src := resolveSource(alias)
	if !strings.HasPrefix(src, "sources/") {
		src = "sources/" + alias
	}
	c.CurrentRepo = src
	saveConfig(c)
	fmt.Printf("Current repo set to %s\n", src)
}

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

	limit := 20 // Default limit for sessions
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

func sessionAliasAdd(alias, sessionID string) {
	c := loadConfig()
	c.SessionAlias[alias] = sessionID
	saveConfig(c)
	fmt.Printf("Session alias %q -> %s\n", alias, sessionID)
}

func sessionAliasList() {
	c := loadConfig()
	if len(c.SessionAlias) == 0 {
		fmt.Println("No session aliases configured.")
		return
	}
	fmt.Printf("%-15s %s\n", "ALIAS", "SESSION ID")
	fmt.Println(strings.Repeat("-", 50))
	for alias, id := range c.SessionAlias {
		fmt.Printf("%-15s %s\n", alias, id)
	}
}

func sessionAliasRm(alias string) {
	c := loadConfig()
	if _, ok := c.SessionAlias[alias]; !ok {
		fmt.Fprintf(os.Stderr, "Session alias %q not found.\n", alias)
		os.Exit(1)
	}
	delete(c.SessionAlias, alias)
	saveConfig(c)
	fmt.Printf("Session alias %q removed.\n", alias)
}

func sessionUse(alias string) {
	c := loadConfig()
	sessionID := resolveSessionID(alias)
	c.CurrentSession = sessionID
	saveConfig(c)
	fmt.Printf("Current session set to %s\n", sessionID)
}

func handleAlias(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules alias <add|list|rm|use>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjules alias add <name> <sessionID>")
			os.Exit(1)
		}
		sessionAliasAdd(args[1], args[2])
	case "list":
		sessionAliasList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules alias rm <name>")
			os.Exit(1)
		}
		sessionAliasRm(args[1])
	case "use":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules alias use <name>")
			os.Exit(1)
		}
		sessionUse(args[1])
	default:
		fmt.Fprintf(os.Stderr, "Unknown alias command: %s\n", args[0])
		os.Exit(1)
	}
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

// --- Messages ---

func msgList(args []string) {
	flags, positional := splitArgs(args)
	fields, _ := parseFields(flags)
	
	c := loadConfig()
	sessionID := ""
	if len(positional) > 0 {
		sessionID = resolveSessionID(positional[0])
	} else if c.CurrentSession != "" {
		sessionID = c.CurrentSession
	} else {
		fmt.Fprintln(os.Stderr, "Usage: gjules msg list [sessionAlias] [--fields=...] [--limit=N] [--detail] [--git] [--type=msg|plan|code]")
		os.Exit(1)
	}

	limit := 100
	detail := false
	showGit := false
	filterType := ""
	for _, a := range flags {
		if strings.HasPrefix(a, "--limit=") {
			fmt.Sscanf(a, "--limit=%d", &limit)
		} else if a == "--detail" {
			detail = true
		} else if a == "--git" {
			showGit = true
		} else if strings.HasPrefix(a, "--type=") {
			filterType = strings.TrimPrefix(a, "--type=")
		}
	}

	key := readKey()
	pageToken := ""
	var allActivities []Activity

	for {
		path := fmt.Sprintf("/sessions/%s/activities?pageSize=100", sessionID)
		if pageToken != "" {
			path += "&pageToken=" + url.QueryEscape(pageToken)
		}

		resp, err := do(key, "GET", path)
		if err != nil {
			die(err)
		}
		defer resp.Body.Close()
		checkResp(resp)

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			die(err)
		}

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

	if len(allActivities) == 0 {
		fmt.Println("No activities found.")
		return
	}

	// Filtering
	var filtered []Activity
	for _, a := range allActivities {
		if filterType != "" {
			isCode := false
			for _, art := range a.Artifacts {
				if art.ChangeSet != nil { isCode = true; break }
			}
			isPlan := a.PlanGenerated != nil
			isMsg := a.AgentMessaged != nil || a.UserMessaged != nil
			
			match := false
			switch filterType {
			case "msg": match = isMsg
			case "plan": match = isPlan
			case "code": match = isCode
			}
			if !match { continue }
		}
		filtered = append(filtered, a)
	}

	// Apply limit: take the LATEST ones
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	headerFields := fields
	if len(headerFields) == 0 {
		headerFields = []string{"id", "originator", "content", "created"}
	}
	fmt.Println(strings.Join(headerFields, ","))

	for _, a := range filtered {
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
			if detail { sep = "\n" }
			parts = append(parts, "Plan:\n" + strings.Join(titles, sep))
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
					parts = append(parts, "ChangeSet")
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
}

func handleMsg(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules msg <list|show|send|approve|wait>")
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
	case "wait":
		_, positional := splitArgs(args[1:])
		target := ""
		if len(positional) >= 1 {
			target = positional[0]
		}
		msgWait(target)
	case "send":
		// Can be: msg send "text" (uses current session)
		// Or:     msg send <alias> "text"
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

func msgSend(sessionAlias, text string) {
	c := loadConfig()
	sessionID := ""
	if sessionAlias != "" {
		sessionID = resolveSessionID(sessionAlias)
	} else {
		sessionID = c.CurrentSession // Already resolved
	}
	
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "Error: No session specified and no current session set. Use 'gjules alias use <alias>' first.")
		os.Exit(1)
	}

	key := readKey()
	body, _ := json.Marshal(map[string]string{"prompt": text})
	resp, err := do(key, "POST", fmt.Sprintf("/%s:sendMessage", sessionID), strings.NewReader(string(body)))
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	fmt.Printf("Message sent to session %s.\n", sessionID)
}

func msgApprove(sessionAlias string) {
	c := loadConfig()
	sessionID := ""
	if sessionAlias != "" {
		sessionID = resolveSessionID(sessionAlias)
	} else {
		sessionID = c.CurrentSession // Already resolved
	}

	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "Error: No session specified and no current session set. Use 'gjules alias use <alias>' first.")
		os.Exit(1)
	}

	key := readKey()
	resp, err := do(key, "POST", fmt.Sprintf("/%s:approvePlan", sessionID), strings.NewReader("{}"))
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	fmt.Printf("Plan approved for session %s.\n", sessionID)
}

func msgWait(sessionAlias string) {
	c := loadConfig()
	sessionID := ""
	if sessionAlias != "" {
		sessionID = resolveSessionID(sessionAlias)
	} else {
		sessionID = c.CurrentSession // Already resolved
	}

	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "Error: No session specified and no current session set.")
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
			ID    string `json:"id"`
			State string `json:"state"`
		}
		json.Unmarshal(bodyBytes, &s)

		state := s.State
		isTodo := strings.HasPrefix(state, "AWAITING_")
		isDone := state == "COMPLETED" || state == "CANCELLED"

		if isTodo || isDone {
			msg := fmt.Sprintf("Session %s is now %s", sessionID, state)
			fmt.Printf("\n%s\n", msg)
			notify(msg)
			return
		}

		fmt.Print(".")
		time.Sleep(5 * time.Second)
	}
}

func notify(msg string) {
	// Always beep for all platforms
	fmt.Print("\a")

	switch runtime.GOOS {
	case "darwin":
		exec.Command("osascript", "-e", fmt.Sprintf("display notification %q with title \"Gjules Update\"", msg)).Run()
	case "linux":
		// Requires libnotify-bin on most distros
		exec.Command("notify-send", "Gjules Update", msg).Run()
	case "windows":
		// Use PowerShell to show a toast/balloon notification
		psCommand := fmt.Sprintf("$ws = New-Object -ComObject WScript.Shell; $ws.Popup(%q, 0, \"Gjules Update\", 64)", msg)
		exec.Command("powershell", "-Command", psCommand).Run()
	}
}

// --- Detailed Management Functions ---

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

func sourceShow(target string) {
	src := resolveSource(target)
	if !strings.HasPrefix(src, "sources/") {
		src = "sources/" + src
	}
	key := readKey()

	resp, result, err := doJSON(key, "GET", "/"+src, nil)
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)

	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
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

	// 1. Try to get patch from Session Outputs first (more reliable for COMPLETED sessions)
	resp, result, err := doJSON(key, "GET", "/sessions/"+sessionID, nil)
	if err == nil {
		defer resp.Body.Close()
		if outputs, ok := result["outputs"].([]interface{}); ok {
			for _, o := range outputs {
				if outMap, ok := o.(map[string]interface{}); ok {
					if cs, ok := outMap["changeSet"].(map[string]interface{}); ok {
						if gp, ok := cs["gitPatch"].(map[string]interface{}); ok {
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

	// 2. Fallback to activities if not found in outputs
	{
		path := fmt.Sprintf("/sessions/%s/activities?pageSize=50", sessionID)
		resp, err := do(key, "GET", path)
		if err == nil {
			defer resp.Body.Close()
			var r struct {
				Activities []struct {
					Artifacts []struct {
						ChangeSet *struct {
							GitPatch struct {
								UnidiffPatch string `json:"unidiffPatch"`
							} `json:"gitPatch"`
						} `json:"changeSet"`
					} `json:"artifacts"`
				} `json:"activities"`
			}
			json.NewDecoder(resp.Body).Decode(&r)
			for _, a := range r.Activities {
				for _, art := range a.Artifacts {
					if art.ChangeSet != nil {
						patch = art.ChangeSet.GitPatch.UnidiffPatch
						goto applyPatch
					}
				}
			}
		}
	}

applyPatch:
	if patch == "" {
		fmt.Fprintln(os.Stderr, "Error: No code changes (ChangeSet) found in this session.")
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
		fmt.Fprintf(os.Stderr, "Error applying patch to %s: %v\n", dir, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully applied code changes to %s.\n", dir)
}


func handleUpdate() {
	fmt.Printf("Current version: %s\n", Version)
	fmt.Println("Checking for updates...")
	
	latest, err := fetchLatestVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}
	
	// Normalize versions for comparison (remove 'v' prefix if present)
	current := strings.TrimPrefix(Version, "v")
	latest = strings.TrimPrefix(latest, "v")
	
	if current == latest {
		fmt.Printf("You are already on the latest version (%s).\n", Version)
		return
	}
	
	fmt.Printf("New version available: v%s. Updating...\n", latest)
	cmd := exec.Command("bash", "-c", "curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("gjules updated successfully!")
}

func fetchLatestVersion() (string, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/forechoandlook/gjules/main/VERSION")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("server returned status %s", resp.Status)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(body)), nil
}
