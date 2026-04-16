# gjules CLI

`gjules` is a command-line interface for interacting with Jules, Google's AI coding agent. It allows you to create sessions, send messages, and manage repositories directly from your terminal.

## Key Features

- **Multi-user Support**: Config & Cache stored per-user in `~/.gjules/users/`.
- **Smart Notifications**: `msg wait` polls status and notifies you on macOS/Linux/Windows with popups and sound.
- **Diff Viewer**: `msg list --git` displays code changes (Git Patch).
- **Session Focus**: `alias use <name>` to set a "current session" context.
- **Code Application**: `sessions apply <id>` checks out a local session branch from the cloud patch base commit, then applies the patch.
- **Task Filters**: `sessions --filter=todo` to see tasks requiring action, with a coarse `next_action` hint.

## Installation

```bash
curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash
```

Release metadata is published with a lightweight `VERSION` asset, and both `install.sh` and `gjules update` read `releases/latest/download/VERSION` instead of parsing the full release JSON.

To update:
```bash
gjules update
```

## Quick Reference

### Repository Management
- `gjules sources --limit=10` : List all available GitHub repos.
- `gjules sources show <id|alias>` : Show detailed repo information (including branch list).
- `gjules repo add myrepo <source_name>` : Create a local alias for a repo.

### Messaging & Workflow
- `gjules new "Implement auth" --auto-pr --branch=dev` : Start a new task with PR automation and custom base branch.
- `gjules sessions show <id>` : View session details (PR URL, Branch name, status).
- `gjules sessions show <id>` now prints a short summary before raw JSON, including inferred `next_action`.
- `gjules sessions apply <id>` : Create or reuse a local branch named after the session id at the cloud patch base commit, then apply the latest cloud patch.
- `gjules sessions apply <id> --dir=/path/to/repo` : Do the same in a specific local git repo.
- `gjules msg list --type=code` : View only activity history related to code changes, with grouped `ChangeSet` summaries by default.
- `gjules msg latest` : Show the latest activity for the current session.
- `gjules msg latest <alias> 5` : Show the latest 5 activities for a specific session.
- `gjules msg latest <alias> 5 --type=code` : Show the latest 5 code-related activities only.
- `gjules msg show <alias> <actID>` : View raw JSON detail of a specific activity.
- `gjules msg approve` : Approve the plan.

### Apply Semantics
- `sessions apply` does not pull a remote branch or clone a repo. It reads `changeSet.gitPatch.baseCommitId`, verifies that commit exists locally, checks out a local branch named after the session id from that base commit, then pipes `unidiffPatch` into local `git apply`.
- If the local branch already exists, it must already point at the same base commit; otherwise `gjules` stops instead of silently reusing the wrong branch.
- If the base commit does not exist in the local repo, `gjules` stops and asks you to fetch the missing history first.
- It first checks `GET /sessions/<id>` for output-level patches. If none are present, it falls back to scanning `GET /sessions/<id>/activities` for the first activity artifact that contains a `changeSet`.
- The target directory must already be a compatible local git working tree. If the patch no longer matches the files on disk, `git apply` will fail instead of silently merging.

## Contributing
To run tests:
`go test -v cmd/gjules/main.go cmd/gjules/main_test.go`
