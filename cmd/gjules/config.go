package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var overriddenConfigPath string
var overriddenBaseDir string

const configFile = ".gjules_config"

func baseDir() string {
	if overriddenBaseDir != "" {
		return overriddenBaseDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gjules")
}

func globalConfigPath() string {
	return filepath.Join(baseDir(), "config.json")
}

func userConfigDir(username string) string {
	return filepath.Join(baseDir(), "users", username)
}

func userConfigPath(username string) string {
	return filepath.Join(userConfigDir(username), "data.json")
}

func userActivityCacheDir(username string) string {
	return filepath.Join(userConfigDir(username), "activities")
}

func activityCachePath(username, sessionID string) string {
	// sessionID might contain slashes if it's a full resource name, so we clean it
	safeID := strings.ReplaceAll(sessionID, "/", "_")
	return filepath.Join(userActivityCacheDir(username), safeID+".json")
}

func configPath() string {
	if overriddenConfigPath != "" {
		return overriddenConfigPath
	}
	return globalConfigPath()
}

func loadConfig() *Config {
	c := &Config{
		Users:        make(map[string]string),
		SessionAlias: make(map[string]string),
		RepoAlias:    make(map[string]string),
	}

	// 1. Try migration from legacy config if exists
	legacyPath := filepath.Join(os.Getenv("HOME"), ".gjules_config")
	if _, err := os.Stat(legacyPath); err == nil {
		b, _ := os.ReadFile(legacyPath)
		json.Unmarshal(b, c)
		// Perform migration once
		saveConfig(c)
		os.Rename(legacyPath, legacyPath+".bak")
		return c
	}

	// 2. Load global config
	b, err := os.ReadFile(configPath())
	if err == nil {
		json.Unmarshal(b, c)
	}

	// 3. Load user-specific config
	if c.CurrentUser != "" {
		ub, err := os.ReadFile(userConfigPath(c.CurrentUser))
		if err == nil {
			var uc Config
			json.Unmarshal(ub, &uc)
			c.SessionAlias = uc.SessionAlias
			c.RepoAlias = uc.RepoAlias
			c.CurrentRepo = uc.CurrentRepo
			c.CurrentSession = uc.CurrentSession
			c.SourcesCache = uc.SourcesCache
			c.SessionsCache = uc.SessionsCache
			c.CacheTime = uc.CacheTime
			c.SessCacheTime = uc.SessCacheTime
		}
	}

	// Ensure maps are initialized
	if c.Users == nil {
		c.Users = make(map[string]string)
	}
	if c.SessionAlias == nil {
		c.SessionAlias = make(map[string]string)
	}
	if c.RepoAlias == nil {
		c.RepoAlias = make(map[string]string)
	}
	return c
}

func saveConfig(c *Config) {
	os.MkdirAll(baseDir(), 0755)

	// 1. Save global config
	gc := struct {
		Users       map[string]string `json:"users,omitempty"`
		CurrentUser string            `json:"currentUser,omitempty"`
	}{
		Users:       c.Users,
		CurrentUser: c.CurrentUser,
	}
	gb, _ := json.MarshalIndent(gc, "", "  ")
	os.WriteFile(globalConfigPath(), gb, 0600)

	// 2. Save user config
	if c.CurrentUser != "" {
		os.MkdirAll(userConfigDir(c.CurrentUser), 0755)
		uc := struct {
			SessionAlias   map[string]string `json:"sessionAlias,omitempty"`
			RepoAlias      map[string]string `json:"repoAlias,omitempty"`
			CurrentRepo    string            `json:"currentRepo,omitempty"`
			CurrentSession string            `json:"currentSession,omitempty"`
			SourcesCache   []CachedSource    `json:"sourcesCache,omitempty"`
			SessionsCache  []CachedSession   `json:"sessionsCache,omitempty"`
			CacheTime      time.Time         `json:"cacheTime,omitempty"`
			SessCacheTime  time.Time         `json:"sessCacheTime,omitempty"`
		}{
			SessionAlias:   c.SessionAlias,
			RepoAlias:      c.RepoAlias,
			CurrentRepo:    c.CurrentRepo,
			CurrentSession: c.CurrentSession,
			SourcesCache:   c.SourcesCache,
			SessionsCache:  c.SessionsCache,
			CacheTime:      c.CacheTime,
			SessCacheTime:  c.SessCacheTime,
		}
		ub, _ := json.MarshalIndent(uc, "", "  ")
		os.WriteFile(userConfigPath(c.CurrentUser), ub, 0600)
	}
}
