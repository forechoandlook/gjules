# gjules CLI

`gjules` is a command-line interface for interacting with Jules, Google's AI coding agent. It allows you to create sessions, send messages, and manage repositories directly from your terminal.

## Key Features

- **Multi-user Support**: Config & Cache stored per-user in `~/.gjules/users/`.
- **Smart Notifications**: `msg wait` polls status and notifies you on macOS/Linux/Windows with popups and sound.
- **Diff Viewer**: `msg list --git` displays code changes (Git Patch).
- **Session Focus**: `alias use <name>` to set a "current session" context.
- **Code Application**: `sessions apply <id>` to directly apply cloud patches to your local workspace.
- **Task Filters**: `sessions --filter=todo` to see tasks requiring action.

## Installation

```bash
curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash
```

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
- `gjules sessions apply <id>` : **Apply the latest code changes from Jules to your local files.**
- `gjules msg list --type=code` : View only activity history related to code changes.
- `gjules msg show <alias> <actID>` : View raw JSON detail of a specific activity.
- `gjules msg approve` : Approve the plan.

## Contributing
To run tests:
`go test -v cmd/gjules/main.go cmd/gjules/main_test.go`
