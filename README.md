# gjules CLI

`gjules` is a command-line interface for interacting with Jules, Google's AI coding agent. It allows you to create sessions, send messages, and manage repositories directly from your terminal.

## Key Features

- **Multi-user Support**: Config & Cache stored per-user in `~/.gjules/users/`.
- **Smart Notifications**: `msg wait` polls status and notifies you on macOS/Linux/Windows with popups and sound.
- **Diff Viewer**: `msg list --git` specifically displays code changes (Git Patch).
- **Session Focus**: `alias use <name>` to set a "current session" context and stop typing IDs.
- **Task Filters**: `sessions --filter=todo` to see only tasks requiring your action.

## Installation

```bash
curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash
```

## Quick Reference

### Configuration & Users
- `gjules user add pro "YOUR_API_KEY"` : Add user.
- `gjules user current` : Show current active user.
- `gjules user switch pro` : Switch current user.

### Repository Management
- `gjules sources --limit=10` : List all available GitHub repos in your org.
- `gjules repo add myrepo <source_name>` : Create a local alias for a repo.
- `gjules repo use myrepo` : Set the default repo for new sessions.

### Messaging & Workflow
- `gjules new "Implement auth module"` : Start a new task.
- `gjules alias use my-task` : Set current task focus.
- `gjules msg wait` : Wait for completion with desktop notification.
- `gjules msg list --git` : View the generated Git Diff.
- `gjules msg approve` : Approve the plan to apply changes.
- `gjules sessions --filter=todo` : List all sessions waiting for your response.

## Feedback
- `gjules feedback --type=bug "my feedback"`
- `gjules feedback --open --type=bug` (Opens GitHub issue)

## Contributing
To run tests:
`go test -v cmd/gjules/main.go cmd/gjules/main_test.go`
