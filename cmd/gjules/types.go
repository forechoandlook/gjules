package main

import "time"

// Config represents the global and per-user configuration.
type Config struct {
	// Global fields
	Users       map[string]string `json:"users,omitempty"`
	CurrentUser string            `json:"currentUser,omitempty"`

	// Per-user fields (stored in user directory)
	SessionAlias   map[string]string `json:"sessionAlias,omitempty"`
	RepoAlias      map[string]string `json:"repoAlias,omitempty"`
	CurrentRepo    string            `json:"currentRepo,omitempty"`
	CurrentSession string            `json:"currentSession,omitempty"`
	SourcesCache   []CachedSource    `json:"sourcesCache,omitempty"`
	SessionsCache  []CachedSession   `json:"sessionsCache,omitempty"`
	CacheTime      time.Time         `json:"cacheTime,omitempty"`
	SessCacheTime  time.Time         `json:"sessCacheTime,omitempty"`
}

type CachedSource struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

type CachedSession struct {
	Name       string          `json:"name"`
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	State      string          `json:"state"`
	CreateTime string          `json:"createTime"`
	Source     string          `json:"source,omitempty"`
	Url        string          `json:"url,omitempty"`
	Outputs    []SessionOutput `json:"outputs,omitempty"`
}

type SessionOutput struct {
	PullRequest *PullRequest `json:"pullRequest,omitempty"`
}

type PullRequest struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Activity struct {
	Name            string           `json:"name"`
	ID              string           `json:"id"`
	Description     string           `json:"description"`
	Originator      string           `json:"originator"`
	CreateTime      string           `json:"createTime"`
	AgentMessaged   *AgentMessaged   `json:"agentMessaged"`
	UserMessaged    *UserMessaged    `json:"userMessaged"`
	PlanGenerated   *PlanGenerated   `json:"planGenerated"`
	PlanApproved    *PlanApproved    `json:"planApproved"`
	ProgressUpdated *ProgressUpdated `json:"progressUpdated"`
	Artifacts       []Artifact       `json:"artifacts"`
}

type AgentMessaged struct {
	AgentMessage string `json:"agentMessage"`
}

type UserMessaged struct {
	UserMessage string `json:"userMessage"`
}

type PlanGenerated struct {
	Plan struct {
		ID    string `json:"id"`
		Steps []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"steps"`
	} `json:"plan"`
}

type PlanApproved struct {
	PlanID string `json:"planId"`
}

type ProgressUpdated struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Artifact struct {
	ChangeSet *ChangeSet  `json:"changeSet"`
	Media     interface{} `json:"media"`
}

type ChangeSet struct {
	GitPatch struct {
		BaseCommitID string `json:"baseCommitId"`
		UnidiffPatch string `json:"unidiffPatch"`
	} `json:"gitPatch"`
}
