package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

func handleAutoAdd(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gjules auto_add <repo> \"prompt\" [--branch=<name>] [--auto-pr]")
		fmt.Fprintln(os.Stderr, "  repo can be: gssh, forechoandlook/gssh, https://github.com/forechoandlook/gssh")
		os.Exit(1)
	}

	repoArg := args[0]
	var promptParts []string
	branch := ""
	autoPR := false

	for _, a := range args[1:] {
		if strings.HasPrefix(a, "--branch=") {
			branch = strings.TrimPrefix(a, "--branch=")
		} else if a == "--auto-pr" {
			autoPR = true
		} else {
			promptParts = append(promptParts, a)
		}
	}

	if len(promptParts) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gjules auto_add <repo> \"prompt\"")
		os.Exit(1)
	}
	prompt := strings.Join(promptParts, " ")

	sourceName := resolveAutoAddRepo(repoArg)
	if sourceName == "" {
		fmt.Fprintf(os.Stderr, "Error: repo %q not found in sources. Run 'gjules sources --refresh' to update.\n", repoArg)
		os.Exit(1)
	}

	fmt.Printf("Using source: %s\n", sourceName)
	newSession(prompt, sourceName, branch, autoPR, false, true)
}

// resolveAutoAddRepo resolves a loose repo reference to a full source name.
// Accepts: "gssh", "forechoandlook/gssh", "https://github.com/forechoandlook/gssh"
func resolveAutoAddRepo(input string) string {
	// Normalize: strip URL prefix, trailing slashes, .git suffix
	s := strings.TrimSpace(input)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")
	if strings.HasPrefix(s, "https://github.com/") {
		s = strings.TrimPrefix(s, "https://github.com/")
	} else if strings.HasPrefix(s, "http://github.com/") {
		s = strings.TrimPrefix(s, "http://github.com/")
	} else if strings.HasPrefix(s, "github.com/") {
		s = strings.TrimPrefix(s, "github.com/")
	}
	// s is now either "owner/repo" or "repo"
	parts := strings.SplitN(s, "/", 2)
	owner, repo := "", ""
	if len(parts) == 2 {
		owner = parts[0]
		repo = parts[1]
	} else {
		repo = parts[0]
	}

	c := loadConfig()

	// Refresh cache if empty
	if len(c.SourcesCache) == 0 || time.Since(c.CacheTime) > 24*time.Hour {
		fmt.Fprintln(os.Stderr, "Sources cache empty or stale, fetching...")
		key := readKey()
		var allSources []CachedSource
		pageToken := ""
		for {
			path := "/sources?pageSize=100"
			if pageToken != "" {
				path += "&pageToken=" + url.QueryEscape(pageToken)
			}
			resp, err := do(key, "GET", path)
			if err != nil {
				die(err)
			}
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
			resp.Body.Close()
			for _, s := range r.Sources {
				cs := CachedSource{Name: s.Name, ID: s.ID}
				if s.GithubRepo != nil {
					cs.Owner = s.GithubRepo.Owner
					cs.Repo = s.GithubRepo.Repo
					if s.GithubRepo.DefaultBranch != nil {
						cs.Branch = s.GithubRepo.DefaultBranch.DisplayName
					}
				}
				allSources = append(allSources, cs)
			}
			pageToken = r.NextPageToken
			if pageToken == "" {
				break
			}
		}
		c.SourcesCache = allSources
		c.CacheTime = time.Now()
		saveConfig(c)
	}

	// Match against cache
	for _, src := range c.SourcesCache {
		if owner != "" {
			// Match owner/repo
			if strings.EqualFold(src.Owner, owner) && strings.EqualFold(src.Repo, repo) {
				return src.Name
			}
		} else {
			// Match repo name only
			if strings.EqualFold(src.Repo, repo) {
				return src.Name
			}
		}
	}
	return ""
}
