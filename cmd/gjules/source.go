package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

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
		printSources(fields, c.SourcesCache, limit, c.CacheTime)
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

	printSources(fields, allSources, limit, c.CacheTime)
}

func printSources(fields []string, sources []CachedSource, limit int, dataTime time.Time) {
	fmt.Println(strings.Join(fields, ","))
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
	fmt.Printf("data_time: %s\n", dataTime.Local().Format("2006-01-02 15:04:05"))
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
	fmt.Printf("data_time: %s\n", time.Now().Local().Format("2006-01-02 15:04:05"))
}
