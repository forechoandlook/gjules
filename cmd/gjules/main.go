package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Build-time injected via -ldflags
var (
	Version   = "dev"
	GitCommit = "unknown"
	GitTag    = "unknown"
)

const (
	baseURL    = "https://jules.googleapis.com/v1alpha"
	configFile = ".gjules_config"
)

// --- Config ---

type Config struct {
	Users         map[string]string `json:"users,omitempty"`
	CurrentUser   string            `json:"currentUser,omitempty"`
	SessionAlias  map[string]string `json:"sessionAlias,omitempty"`  // alias -> full ID
	RepoAlias     map[string]string `json:"repoAlias,omitempty"`     // alias -> source name (e.g. sources/github-org-repo)
	CurrentRepo   string            `json:"currentRepo,omitempty"`   // default repo alias or full source name
	
	// Cache
	SourcesCache []CachedSource `json:"sourcesCache,omitempty"`
	CacheTime    time.Time      `json:"cacheTime,omitempty"`
}

type CachedSource struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configFile)
}

func loadConfig() *Config {
	b, err := os.ReadFile(configPath())
	if err != nil {
		return &Config{
			Users:      make(map[string]string),
			SessionAlias:  make(map[string]string),
			RepoAlias:     make(map[string]string),
		}
	}
	var c Config
	json.Unmarshal(b, &c)
	if c.Users == nil {
		c.Users = make(map[string]string)
	}
	if c.SessionAlias == nil {
		c.SessionAlias = make(map[string]string)
	}
	if c.RepoAlias == nil {
		c.RepoAlias = make(map[string]string)
	}
	return &c
}

func saveConfig(c *Config) {
	b, _ := json.MarshalIndent(c, "", "  ")
	os.WriteFile(configPath(), b, 0600)
}

func readKey() string {
	if k := os.Getenv("GJULES_API_KEY"); k != "" {
		return k
	}
	c := loadConfig()
	if c.CurrentUser == "" {
		fmt.Fprintln(os.Stderr, "No current user. Run 'gjules user add <name> <key>' first.")
		os.Exit(1)
	}
	key, ok := c.Users[c.CurrentUser]
	if !ok {
		fmt.Fprintf(os.Stderr, "User %q not found in config.\n", c.CurrentUser)
		os.Exit(1)
	}
	return key
}

func resolveSessionID(aliasOrID string) string {
	c := loadConfig()
	if fullID, ok := c.SessionAlias[aliasOrID]; ok {
		return fullID
	}
	return aliasOrID
}

func resolveSource(aliasOrSource string) string {
	c := loadConfig()
	if source, ok := c.RepoAlias[aliasOrSource]; ok {
		return source
	}
	return aliasOrSource
}

// --- HTTP ---

func httpClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func doJSON(key, method, path string, body map[string]interface{}) (*http.Response, map[string]interface{}, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = strings.NewReader(string(b))
	}
	req, _ := http.NewRequest(method, baseURL+path, r)
	req.Header.Set("x-goog-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return resp, result, nil
}

func do(key, method, path string, body ...io.Reader) (*http.Response, error) {
	var r io.Reader
	if len(body) > 0 {
		r = body[0]
	}
	req, _ := http.NewRequest(method, baseURL+path, r)
	req.Header.Set("x-goog-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	return httpClient().Do(req)
}

// --- Commands ---

func userAdd(name, key string) {
	c := loadConfig()
	c.Users[name] = key
	c.CurrentUser = name
	saveConfig(c)
	fmt.Printf("User %q added and switched.\n", name)
}

func userUse(name string) {
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
	if len(c.Users) == 0 {
		fmt.Println("No users configured.")
		return
	}
	fmt.Printf("%-15s %s\n", "USER", "KEY")
	fmt.Println(strings.Repeat("-", 50))
	for name, key := range c.Users {
		mark := " "
		if name == c.CurrentUser {
			mark = "*"
		}
		fmt.Printf("%s %-14s %s...%s\n", mark, name, key[:6], key[len(key)-4:])
	}
	if c.CurrentUser != "" {
		fmt.Printf("\n(current user: %s)\n", c.CurrentUser)
	}
}

func userRm(name string) {
	c := loadConfig()
	if _, ok := c.Users[name]; !ok {
		fmt.Fprintf(os.Stderr, "User %q not found.\n", name)
		os.Exit(1)
	}
	delete(c.Users, name)
	if c.CurrentUser == name {
		c.CurrentUser = ""
	}
	saveConfig(c)
	fmt.Printf("User %q removed.\n", name)
}

func checkResp(resp *http.Response) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Fprintf(os.Stderr, "Error: server returned status %s\n", resp.Status)
		if b, err := json.MarshalIndent(result, "", "  "); err == nil {
			fmt.Fprintf(os.Stderr, "%s\n", string(b))
		}
		os.Exit(1)
	}
}

// --- Sources ---

func sources(args []string) {
	fields, _ := parseFields(args)
	if len(fields) == 0 {
		fields = []string{"alias", "id", "name", "owner", "repo", "branch"}
	}

	limit := 20
	refresh := false
	for _, a := range args {
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
	first := true

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

	if first {
		fmt.Println(strings.Join(fields, ","))
		first = false
	}
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

func sourceAliasAdd(alias, source string) {
	c := loadConfig()
	// Try to validate by listing sources
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
		fmt.Printf("%-15s %s\n", alias, src)
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
	fields, _ := parseFields(args)
	if len(fields) == 0 {
		fields = []string{"alias", "id", "state", "title", "created", "name"}
	}

	limit := 20 // Default limit for sessions
	for _, a := range args {
		if strings.HasPrefix(a, "--limit=") {
			fmt.Sscanf(a, "--limit=%d", &limit)
		}
	}

	key := readKey()
	pageToken := ""
	count := 0
	first := true

	for {
		pageSize := 100
		if limit > 0 && limit-count < 100 {
			pageSize = limit - count
		}
		path := fmt.Sprintf("/sessions?pageSize=%d", pageSize)
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

		if first {
			fmt.Println(strings.Join(fields, ","))
			first = false
		}

		if len(r.Sessions) == 0 && pageToken == "" {
			fmt.Println("No sessions found.")
			return
		}

		c := loadConfig()
		reverseAlias := make(map[string]string)
		for alias, id := range c.SessionAlias {
			reverseAlias[id] = alias
		}

		for _, s := range r.Sessions {
			t, _ := time.Parse(time.RFC3339, s.CreateTime)
			alias := reverseAlias[s.ID]
			if alias == "" {
				alias = "-"
			}
			values := map[string]string{
				"alias":   alias,
				"id":      s.ID,
				"state":   s.State,
				"title":   s.Title,
				"created": t.Local().Format("2006-01-02 15:04:05"),
				"name":    s.Name,
			}
			fmt.Println(csvFields(fields, values))
			count++
			if limit > 0 && count >= limit {
				return
			}
		}

		pageToken = r.NextPageToken
		if pageToken == "" {
			break
		}
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

func newSession(prompt, repoAlias string, repoSet bool) {
	key := readKey()
	c := loadConfig()
	
	body := map[string]interface{}{"prompt": prompt}
	
	// Add source context if repo specified or use default if not explicitly set to empty
	repo := repoAlias
	if !repoSet && repo == "" {
		repo = c.CurrentRepo
	}
	
	if repo != "" {
		src := resolveSource(repo)
		body["sourceContext"] = map[string]interface{}{
			"source": src,
			"githubRepoContext": map[string]interface{}{},
		}
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

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + s + "\""
	}
	return s
}

func msgList(args []string) {
	fields, remaining := parseFields(args)
	if len(remaining) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules msg list <sessionAlias> [--fields=...] [--limit=N]")
		os.Exit(1)
	}
	sessionAlias := remaining[0]

	limit := 20
	for _, a := range args {
		if strings.HasPrefix(a, "--limit=") {
			fmt.Sscanf(a, "--limit=%d", &limit)
		}
	}

	sessionID := resolveSessionID(sessionAlias)
	key := readKey()
	pageToken := ""
	count := 0
	first := true

	for {
		pageSize := 100
		if limit > 0 && limit-count < 100 {
			pageSize = limit - count
		}
		path := fmt.Sprintf("/sessions/%s/activities?pageSize=%d", sessionID, pageSize)
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
			Activities []struct {
				Name          string `json:"name"`
				ID            string `json:"id"`
				Description   string `json:"description"`
				Originator    string `json:"originator"`
				CreateTime    string `json:"createTime"`
				AgentMessaged *struct {
					Text string `json:"text"`
				} `json:"agentMessage"`
				UserMessaged *struct {
					Prompt string `json:"prompt"`
				} `json:"userMessage"`
				PlanGenerated *struct {
					Plan struct {
						Steps []struct {
							Title string `json:"title"`
						} `json:"steps"`
					} `json:"plan"`
				} `json:"planGenerated"`
				PlanApproved *struct {
					PlanID string `json:"planId"`
				} `json:"planApproved"`
				ProgressUpdated *struct {
					Title       string `json:"title"`
					Description string `json:"description"`
				} `json:"progressUpdated"`
			} `json:"activities"`
			NextPageToken string `json:"nextPageToken"`
		}
		json.Unmarshal(bodyBytes, &r)

		if first {
			headerFields := fields
			if len(headerFields) == 0 {
				headerFields = []string{"id", "originator", "description", "content", "created"}
			}
			fmt.Println(strings.Join(headerFields, ","))
			first = false
		}

		if len(r.Activities) == 0 && pageToken == "" {
			fmt.Println("No activities found.")
			return
		}

		for _, a := range r.Activities {
			content := ""
			if a.AgentMessaged != nil {
				content = a.AgentMessaged.Text
			} else if a.UserMessaged != nil {
				content = a.UserMessaged.Prompt
			} else if a.PlanGenerated != nil {
				var titles []string
				for _, s := range a.PlanGenerated.Plan.Steps {
					titles = append(titles, s.Title)
				}
				content = "Plan: " + strings.Join(titles, "; ")
			} else if a.PlanApproved != nil {
				content = "Plan Approved: " + a.PlanApproved.PlanID
			} else if a.ProgressUpdated != nil {
				content = a.ProgressUpdated.Title
				if a.ProgressUpdated.Description != "" {
					content += ": " + a.ProgressUpdated.Description
				}
			}
			// Clean up content for CSV (remove newlines for preview)
			content = strings.ReplaceAll(content, "\n", " ")
			if len(content) > 50 {
				content = content[:47] + "..."
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
			// Use the requested order of fields
			selectedFields := fields
			if len(selectedFields) == 0 {
				selectedFields = []string{"id", "originator", "description", "content", "created"}
			}
			fmt.Println(csvFields(selectedFields, values))
			count++
			if limit > 0 && count >= limit {
				return
			}
		}

		pageToken = r.NextPageToken
		if pageToken == "" {
			break
		}
	}
}

func msgSend(sessionAlias, text string) {
	sessionID := resolveSessionID(sessionAlias)
	key := readKey()
	body, _ := json.Marshal(map[string]string{"prompt": text})
	resp, err := do(key, "POST", fmt.Sprintf("/sessions/%s:sendMessage", sessionID), strings.NewReader(string(body)))
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	fmt.Println("Message sent.")
}

func msgApprove(sessionAlias string) {
	sessionID := resolveSessionID(sessionAlias)
	key := readKey()
	resp, err := do(key, "POST", fmt.Sprintf("/sessions/%s:approvePlan", sessionID), strings.NewReader("{}"))
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()
	checkResp(resp)
	fmt.Println("Plan approved.")
}

// --- Helpers ---

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
	order := []string{"alias", "id", "state", "title", "created", "name", "originator", "description", "content", "owner", "repo", "branch"}
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

func die(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func version() {
	fmt.Printf("gjules %s (commit: %s, tag: %s)\n", Version, GitCommit, GitTag)
}

func selfUpdate() {
	repo := "forechoandlook/gjules"
	fmt.Println("Checking for updates...")

	// Fetch latest release
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()

	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
		} `json:"assets"`
	}
	json.NewDecoder(resp.Body).Decode(&rel)

	latestVersion := strings.TrimPrefix(rel.TagName, "v")
	if latestVersion == Version {
		fmt.Printf("Already on the latest version: %s\n", Version)
		return
	}
	fmt.Printf("New version available: %s (current: %s)\n", latestVersion, Version)

	// Detect OS and arch
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Find the matching asset
	var assetURL string
	var assetName string
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		// Match OS
		osMatch := strings.Contains(name, strings.ToLower(goos))
		// Match Arch
		archMatch := strings.Contains(name, strings.ToLower(goarch))
		if !archMatch && (goarch == "amd64" || goarch == "x86_64") {
			archMatch = strings.Contains(name, "amd64") || strings.Contains(name, "x86_64")
		}
		if !archMatch && (goarch == "arm64" || goarch == "aarch64") {
			archMatch = strings.Contains(name, "arm64") || strings.Contains(name, "aarch64")
		}

		if osMatch && archMatch && (strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip")) {
			assetURL = a.URL
			assetName = a.Name
			break
		}
	}
	if assetURL == "" {
		fmt.Fprintf(os.Stderr, "No binary found for %s/%s\n", goos, goarch)
		fmt.Fprintf(os.Stderr, "Download manually: https://github.com/%s/releases/latest\n", repo)
		os.Exit(1)
	}

	fmt.Printf("Downloading %s...\n", rel.TagName)

	// Download to temp dir
	tmpDir, err := os.MkdirTemp("", "gjules-update")
	if err != nil {
		die(err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	out, err := os.Create(archivePath)
	if err != nil {
		die(err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	dlResp, err := client.Get(assetURL)
	if err != nil {
		die(err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		die(fmt.Errorf("failed to download: %s", dlResp.Status))
	}
	io.Copy(out, dlResp.Body)
	out.Close()

	// Extract
	binaryName := "gjules"
	if goos == "windows" {
		binaryName = "gjules.exe"
	}

	if strings.HasSuffix(assetName, ".tar.gz") {
		cmd := exec.Command("tar", "-xzf", archivePath, "-C", tmpDir)
		if err := cmd.Run(); err != nil {
			die(fmt.Errorf("failed to extract tar.gz: %w", err))
		}
	} else if strings.HasSuffix(assetName, ".zip") {
		if goos == "windows" {
			cmd := exec.Command("powershell", "-Command", "Expand-Archive", "-Path", archivePath, "-DestinationPath", tmpDir)
			if err := cmd.Run(); err != nil {
				die(fmt.Errorf("failed to extract zip: %w", err))
			}
		} else {
			cmd := exec.Command("unzip", "-q", archivePath, "-d", tmpDir)
			if err := cmd.Run(); err != nil {
				die(fmt.Errorf("failed to extract zip: %w", err))
			}
		}
	}

	extractedBinary := filepath.Join(tmpDir, binaryName)
	if _, err := os.Stat(extractedBinary); err != nil {
		die(fmt.Errorf("binary %s not found in archive", binaryName))
	}

	// Replace current binary
	exe, err := os.Executable()
	if err != nil {
		die(err)
	}

	// On Unix, we should remove the old binary first or move it to a temp location 
	// because we can't overwrite a running binary.
	oldExe := exe + ".old"
	if err := os.Rename(exe, oldExe); err != nil {
		die(fmt.Errorf("failed to move current binary: %w", err))
	}
	defer os.Remove(oldExe)

	if err := os.Rename(extractedBinary, exe); err != nil {
		// If rename fails, try to copy
		src, _ := os.Open(extractedBinary)
		defer src.Close()
		dst, _ := os.OpenFile(exe, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
		defer dst.Close()
		io.Copy(dst, src)
	}

	fmt.Printf("Successfully updated to %s!\n", rel.TagName)
}

func handleFeedback(args []string) {
	repo := "forechoandlook/gjules"
	issuesURL := fmt.Sprintf("https://github.com/%s/issues/new", repo)

	// Parse flags
	openBrowser := false
	category := ""
	var description []string
	for _, a := range args {
		switch {
		case a == "--open":
			openBrowser = true
		case strings.HasPrefix(a, "--type="):
			category = strings.TrimPrefix(a, "--type=")
		default:
			description = append(description, a)
		}
	}

	// Valid categories
	validCategories := map[string]bool{
		"bug": true, "docs": true, "feature": true, "other": true,
	}

	// Require description and type in non-interactive mode
	if len(description) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gjules feedback [--open] --type=bug|docs|feature|other \"description\"")
		os.Exit(1)
	}
	if category == "" {
		fmt.Fprintln(os.Stderr, "Category is required. Use --type=bug|docs|feature|other")
		os.Exit(1)
	}
	if _, ok := validCategories[category]; !ok {
		fmt.Fprintf(os.Stderr, "Invalid type: %s. Valid types: bug, docs, feature, other\n", category)
		os.Exit(1)
	}

	desc := strings.Join(description, " ")

	// Build feedback record
	record := map[string]string{
		"type":        category,
		"description": desc,
		"version":     Version,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"commit":      GitCommit,
		"created_at":  time.Now().UTC().Format(time.RFC3339),
	}
	recordJSON, _ := json.Marshal(record)

	if openBrowser {
		title := fmt.Sprintf("[%s] %s", category, desc)
		body := fmt.Sprintf(
			"## Type\n%s\n\n## Description\n%s\n\n## Environment\n- **Version**: %s\n- **OS**: %s/%s\n- **Commit**: %s\n",
			category, desc, Version, runtime.GOOS, runtime.GOARCH, GitCommit,
		)
		url := fmt.Sprintf("%s?title=%s&body=%s",
			issuesURL,
			url.QueryEscape(title),
			url.QueryEscape(body),
		)
		fmt.Printf("Opening GitHub issue page...\n")
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		}
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open browser. Please visit:\n%s\n", url)
		}
	} else {
		// Append to local JSONL file
		home, _ := os.UserHomeDir()
		feedbackDir := filepath.Join(home, ".gjules")
		os.MkdirAll(feedbackDir, 0755)
		fname := filepath.Join(feedbackDir, "feedback.jsonl")
		f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			die(err)
		}
		f.WriteString(string(recordJSON) + "\n")
		f.Close()
		fmt.Printf("Feedback appended to %s\n", fname)
		fmt.Printf("To submit, run: gjules feedback --open --type=%s \"%s\"\n", category, desc)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gjules - Jules CLI

Usage:
  gjules user add <name> <key>       Add user with API key
  gjules user use <name>             Switch to user
  gjules user list                   List all users
  gjules user rm <name>              Remove user
  gjules user current                Show current user

  gjules sources [--limit=20] [--refresh]  List all sources (repos)
  gjules repo add <alias> <source>   Add repo alias
  gjules repo list                   List repo aliases
  gjules repo rm <alias>             Remove repo alias
  gjules repo use <alias>            Set default repo

  gjules sessions [--limit=20]       List all sessions
  gjules alias add <name> <id>       Add session alias
  gjules alias list                  List session aliases
  gjules alias rm <name>             Remove session alias
  gjules new "prompt" [--repo=...]   Create session
  gjules new "prompt" --repo=<alias> Create session with specific repo

  gjules msg list <alias> [--limit=20]  List activities
  gjules msg send <alias> "text"     Send message
  gjules msg approve <alias>         Approve plan

  gjules version                     Show version
  gjules update                      Self-update to latest release
  gjules feedback --type=bug "msg"   Append to local JSONL (~/.gjules/feedback.jsonl)
  gjules feedback --open --type=bug  Open GitHub issue with pre-filled content

Fields:
  sessions: alias,id,state,title,created,name
  sources:  name,id,owner,repo,branch,alias
  msg list: id,originator,description,content,created,name

Environment:
  GJULES_API_KEY                     API key (overrides config)

Config:
  ~/.gjules_config                   Multi-user config with aliases
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "user":
		handleUser(os.Args[2:])
	case "sources":
		sources(os.Args[2:])
	case "repo":
		handleRepo(os.Args[2:])
	case "sessions":
		sessions(os.Args[2:])
	case "alias":
		handleAlias(os.Args[2:])
	case "new":
		handleNew(os.Args[2:])
	case "msg":
		handleMsg(os.Args[2:])
	case "version", "--version", "-v":
		version()
	case "update":
		selfUpdate()
	case "feedback":
		handleFeedback(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func handleUser(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules user <add|use|list|rm|current>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user add <name> <key>")
			os.Exit(1)
		}
		userAdd(args[1], args[2])
	case "use":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user use <name>")
			os.Exit(1)
		}
		userUse(args[1])
	case "list":
		userList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user rm <name>")
			os.Exit(1)
		}
		userRm(args[1])
	case "current":
		c := loadConfig()
		if k := os.Getenv("GJULES_API_KEY"); k != "" {
			fmt.Println("Current user: (env:GJULES_API_KEY)")
			return
		}
		if c.CurrentUser == "" {
			fmt.Println("Current user: (none)")
		} else {
			fmt.Printf("Current user: %s\n", c.CurrentUser)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown user command: %s\n", args[0])
		os.Exit(1)
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

func handleAlias(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules alias <add|list|rm>")
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
	default:
		fmt.Fprintf(os.Stderr, "Unknown alias command: %s\n", args[0])
		os.Exit(1)
	}
}

func handleNew(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gjules new \"prompt\"")
		fmt.Fprintln(os.Stderr, "       gjules new \"prompt\" --repo=<alias>")
		os.Exit(1)
	}
	
	repo := ""
	repoSet := false
	var promptParts []string
	for _, a := range args {
		if strings.HasPrefix(a, "--repo=") {
			repo = strings.TrimPrefix(a, "--repo=")
			repoSet = true
		} else {
			promptParts = append(promptParts, a)
		}
	}
	
	if len(promptParts) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gjules new \"prompt\"")
		os.Exit(1)
	}
	
	newSession(strings.Join(promptParts, " "), repo, repoSet)
}

func handleMsg(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules msg <list|send|approve>")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		msgList(args[1:])
	case "send":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjules msg send <sessionAlias> \"text\"")
			os.Exit(1)
		}
		msgSend(args[1], strings.Join(args[2:], " "))
	case "approve":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules msg approve <sessionAlias>")
			os.Exit(1)
		}
		msgApprove(args[1])
	default:
		fmt.Fprintf(os.Stderr, "Unknown msg command: %s\n", args[0])
		os.Exit(1)
	}
}
