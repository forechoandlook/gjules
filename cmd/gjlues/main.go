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
	configFile = ".gjlues_config"
)

// --- Config ---

type Config struct {
	Users         map[string]string `json:"users,omitempty"`
	CurrentUser   string            `json:"currentUser,omitempty"`
	SessionAlias  map[string]string `json:"sessionAlias,omitempty"`  // alias -> full ID
	RepoAlias     map[string]string `json:"repoAlias,omitempty"`     // alias -> source name (e.g. sources/github-org-repo)
	CurrentRepo   string            `json:"currentRepo,omitempty"`   // default repo alias or full source name
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
	if k := os.Getenv("GJLUES_API_KEY"); k != "" {
		return k
	}
	c := loadConfig()
	if c.CurrentUser == "" {
		fmt.Fprintln(os.Stderr, "No current user. Run 'gjlues user add <name> <key>' first.")
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

// --- Sources ---

func sources(args []string) {
	fields, _ := parseFields(args)
	
	key := readKey()
	resp, err := do(key, "GET", "/sources")
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()

	var r struct {
		Sources []struct {
			Name         string `json:"name"`
			ID           string `json:"id"`
			GithubRepo   *struct {
				Owner string `json:"owner"`
				Repo  string `json:"repo"`
				DefaultBranch *struct {
					DisplayName string `json:"displayName"`
				} `json:"defaultBranch"`
			} `json:"githubRepo"`
		} `json:"sources"`
	}
	json.NewDecoder(resp.Body).Decode(&r)

	if len(r.Sources) == 0 {
		fmt.Println("No sources found.")
		return
	}

	c := loadConfig()
	reverseAlias := make(map[string]string)
	for alias, src := range c.RepoAlias {
		reverseAlias[src] = alias
	}

	// Print header
	headerFields := fields
	if len(headerFields) == 0 {
		headerFields = []string{"name", "id", "owner", "repo", "branch", "alias"}
	}
	fmt.Println(strings.Join(headerFields, ","))

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
		alias := reverseAlias[s.Name]
		if alias == "" {
			alias = "-"
		}
		values := map[string]string{
			"name":    s.Name,
			"id":      s.ID,
			"owner":   owner,
			"repo":    repo,
			"branch":  branch,
			"alias":   alias,
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
	
	key := readKey()
	resp, err := do(key, "GET", "/sessions")
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()

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

	if len(r.Sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	c := loadConfig()
	reverseAlias := make(map[string]string)
	for alias, id := range c.SessionAlias {
		reverseAlias[id] = alias
	}

	// Print header
	headerFields := fields
	if len(headerFields) == 0 {
		headerFields = []string{"alias", "id", "state", "title", "created", "name"}
	}
	fmt.Println(strings.Join(headerFields, ","))

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
	}
	if r.NextPageToken != "" {
		fmt.Fprintf(os.Stderr, "# nextPageToken=%s\n", r.NextPageToken)
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

func newSession(prompt, repoAlias string) {
	key := readKey()
	c := loadConfig()
	
	body := map[string]interface{}{"prompt": prompt}
	
	// Add source context if repo specified
	repo := repoAlias
	if repo == "" {
		repo = c.CurrentRepo
	}
	if repo != "" {
		src := resolveSource(repo)
		if !strings.HasPrefix(src, "sources/") {
			src = "sources/" + src
		}
		body["sourceContext"] = map[string]interface{}{
			"source": src,
		}
	}
	
	_, result, err := doJSON(key, "POST", "/sessions", body)
	if err != nil {
		die(err)
	}

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
		fmt.Fprintln(os.Stderr, "Usage: gjlues msg list <sessionAlias> [--fields=id,originator,description,created]")
		os.Exit(1)
	}
	sessionAlias := remaining[0]

	sessionID := resolveSessionID(sessionAlias)
	key := readKey()
	resp, err := do(key, "GET", fmt.Sprintf("/sessions/%s/activities", sessionID))
	if err != nil {
		die(err)
	}
	defer resp.Body.Close()

	var r struct {
		Activities []struct {
			Name        string `json:"name"`
			ID          string `json:"id"`
			Description string `json:"description"`
			Originator  string `json:"originator"`
			CreateTime  string `json:"createTime"`
		} `json:"activities"`
		NextPageToken string `json:"nextPageToken"`
	}
	json.NewDecoder(resp.Body).Decode(&r)

	if len(r.Activities) == 0 {
		fmt.Println("No activities found.")
		return
	}

	// Print header
	headerFields := fields
	if len(headerFields) == 0 {
		headerFields = []string{"id", "originator", "description", "created"}
	}
	fmt.Println(strings.Join(headerFields, ","))

	for _, a := range r.Activities {
		t, _ := time.Parse(time.RFC3339, a.CreateTime)
		values := map[string]string{
			"id":          a.ID,
			"originator":  a.Originator,
			"description": a.Description,
			"created":     t.Local().Format("2006-01-02 15:04:05"),
			"name":        a.Name,
		}
		fmt.Println(csvFields(fields, values))
	}
	if r.NextPageToken != "" {
		fmt.Fprintf(os.Stderr, "# nextPageToken=%s\n", r.NextPageToken)
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
	order := []string{"alias", "id", "state", "title", "created", "name", "originator", "description", "owner", "repo", "branch"}
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
	fmt.Printf("gjlues %s (commit: %s, tag: %s)\n", Version, GitCommit, GitTag)
}

func selfUpdate() {
	repo := "forechoandlook/gjlues"
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
	if goarch == "x86_64" {
		goarch = "amd64"
	}

	// Find the matching asset
	var assetURL string
	for _, a := range rel.Assets {
		if strings.Contains(a.Name, goos) && strings.Contains(a.Name, goarch) {
			assetURL = a.URL
			break
		}
	}
	if assetURL == "" {
		fmt.Fprintf(os.Stderr, "No binary found for %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Fprintf(os.Stderr, "Download manually: https://github.com/%s/releases/latest\n", repo)
		os.Exit(1)
	}

	fmt.Printf("Downloading %s...\n", rel.TagName)

	// Download to temp dir
	tmpDir, err := os.MkdirTemp("", "gjlues-update")
	if err != nil {
		die(err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := tmpDir + "/gjlues.tar.gz"
	if runtime.GOOS == "windows" {
		tmpFile = tmpDir + "/gjlues.zip"
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		die(err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	dlResp, err := client.Get(assetURL)
	if err != nil {
		die(err)
	}
	io.Copy(out, dlResp.Body)
	out.Close()
	dlResp.Body.Close()

	// Extract
	exePath := tmpDir + "/gjlues"
	if runtime.GOOS == "windows" {
		exePath = tmpDir + "/gjlues.exe"
	}
	cmd := exec.Command("tar", "-xzf", tmpFile, "-C", tmpDir)
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", "Expand-Archive", "-Path", tmpFile, "-DestinationPath", tmpDir)
	}
	if err := cmd.Run(); err != nil {
		die(fmt.Errorf("failed to extract: %w", err))
	}

	// Replace current binary
	exe, err := os.Executable()
	if err != nil {
		die(err)
	}
	if err := os.Rename(exePath, exe); err != nil {
		// Fallback: copy
		src, _ := os.Open(exePath)
		defer src.Close()
		dst, _ := os.OpenFile(exe, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
		defer dst.Close()
		io.Copy(dst, src)
	}

	fmt.Printf("Updated to %s!\n", latestVersion)
}

func handleFeedback(args []string) {
	repo := "forechoandlook/gjlues"
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
		fmt.Fprintln(os.Stderr, "Usage: gjlues feedback [--open] --type=bug|docs|feature|other \"description\"")
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
		feedbackDir := filepath.Join(home, ".gjlues")
		os.MkdirAll(feedbackDir, 0755)
		fname := filepath.Join(feedbackDir, "feedback.jsonl")
		f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			die(err)
		}
		f.WriteString(string(recordJSON) + "\n")
		f.Close()
		fmt.Printf("Feedback appended to %s\n", fname)
		fmt.Printf("To submit, run: gjlues feedback --open --type=%s \"%s\"\n", category, desc)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gjlues - Jules CLI

Usage:
  gjlues user add <name> <key>       Add user with API key
  gjlues user use <name>             Switch to user
  gjlues user list                   List all users
  gjlues user rm <name>              Remove user
  gjlues user current                Show current user

  gjlues sources [--fields=...]      List all sources (repos)
  gjlues repo add <alias> <source>   Add repo alias
  gjlues repo list                   List repo aliases
  gjlues repo rm <alias>             Remove repo alias
  gjlues repo use <alias>            Set default repo

  gjlues sessions [--fields=...]     List all sessions
  gjlues alias add <name> <id>       Add session alias
  gjlues alias list                  List session aliases
  gjlues alias rm <name>             Remove session alias
  gjlues new "prompt" [--repo=...]   Create session
  gjlues new "prompt" --repo=<alias> Create session with specific repo

  gjlues msg list <alias> [--fields=...]  List activities
  gjlues msg send <alias> "text"     Send message
  gjlues msg approve <alias>         Approve plan

  gjlues version                     Show version
  gjlues update                      Self-update to latest release
  gjlues feedback --type=bug "msg"   Append to local JSONL (~/.gjlues/feedback.jsonl)
  gjlues feedback --open --type=bug  Open GitHub issue with pre-filled content

Fields:
  sessions: alias,id,state,title,created,name
  sources:  name,id,owner,repo,branch,alias
  msg list: id,originator,description,created,name

Environment:
  GJLUES_API_KEY                     API key (overrides config)

Config:
  ~/.gjlues_config                   Multi-user config with aliases
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
		fmt.Fprintln(os.Stderr, "Usage: gjlues user <add|use|list|rm|current>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues user add <name> <key>")
			os.Exit(1)
		}
		userAdd(args[1], args[2])
	case "use":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues user use <name>")
			os.Exit(1)
		}
		userUse(args[1])
	case "list":
		userList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues user rm <name>")
			os.Exit(1)
		}
		userRm(args[1])
	case "current":
		c := loadConfig()
		if k := os.Getenv("GJLUES_API_KEY"); k != "" {
			fmt.Println("Current user: (env:GJLUES_API_KEY)")
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
		fmt.Fprintln(os.Stderr, "Usage: gjlues repo <add|list|rm|use>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues repo add <alias> <source>")
			os.Exit(1)
		}
		sourceAliasAdd(args[1], args[2])
	case "list":
		sourceAliasList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues repo rm <alias>")
			os.Exit(1)
		}
		sourceAliasRm(args[1])
	case "use":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues repo use <alias>")
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
		fmt.Fprintln(os.Stderr, "Usage: gjlues alias <add|list|rm>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues alias add <name> <sessionID>")
			os.Exit(1)
		}
		sessionAliasAdd(args[1], args[2])
	case "list":
		sessionAliasList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues alias rm <name>")
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
		fmt.Fprintln(os.Stderr, "Usage: gjlues new \"prompt\"")
		fmt.Fprintln(os.Stderr, "       gjlues new \"prompt\" --repo=<alias>")
		os.Exit(1)
	}
	
	// Check for --repo flag
	repo := ""
	var promptParts []string
	for _, a := range args {
		if strings.HasPrefix(a, "--repo=") {
			repo = strings.TrimPrefix(a, "--repo=")
		} else {
			promptParts = append(promptParts, a)
		}
	}
	
	if len(promptParts) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gjlues new \"prompt\"")
		os.Exit(1)
	}
	
	newSession(strings.Join(promptParts, " "), repo)
}

func handleMsg(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjlues msg <list|send|approve>")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		msgList(args[1:])
	case "send":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues msg send <sessionAlias> \"text\"")
			os.Exit(1)
		}
		msgSend(args[1], strings.Join(args[2:], " "))
	case "approve":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjlues msg approve <sessionAlias>")
			os.Exit(1)
		}
		msgApprove(args[1])
	default:
		fmt.Fprintf(os.Stderr, "Unknown msg command: %s\n", args[0])
		os.Exit(1)
	}
}
