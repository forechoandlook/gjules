# gjules CLI

`gjules` is a command-line interface for interacting with Jules, Google's AI coding agent. It allows you to create sessions, send messages, and manage repositories directly from your terminal.

## Key Features

- **Multi-user Support**: Config & Cache stored per-user in `~/.gjules/users/`.
- **Smart Notifications**: `msg wait` polls status and notifies you on macOS/Linux/Windows with popups and sound.
- **Diff Viewer**: `msg list --git` specifically displays code changes (Git Patch).
- **Session Focus**: `alias use <id>` to set a "current session" and stop typing IDs.
- **Task Filters**: `sessions --filter=todo` to see only tasks requiring your action.

## Installation

```bash
curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash
```

## Usage

```bash
gjules user add pro "YOUR_API_KEY"
gjules user switch pro

# Setup Repo
gjules sources --limit=10
gjules repo add myrepo sources/github/owner/repo
gjules repo use myrepo

# Workflow
gjules new "Implement a simple health check endpoint"
gjules msg wait # Go grab a coffee, it'll ping you!

gjules msg list --git # Review code changes
gjules msg approve # Done!
```

## Contributing

See `cmd/gjules/main.go` for the core logic. To run tests:
`go test -v cmd/gjules/main.go cmd/gjules/main_test.go`
