# gjlues

A lightweight CLI for [Jules](https://jules.google) — Google's AI coding agent.

## Install

```bash
curl -sSf https://raw.githubusercontent.com/forechoandlook/gjlues/main/install.sh | bash
```

## Uninstall

```bash
curl -sSf https://raw.githubusercontent.com/forechoandlook/gjlues/main/uninstall.sh | bash
```

## Quick Start

```bash
# Add your API key
gjlues user add main "your-api-key-here"

# Set default repo
gjlues sources                              # list available repos
gjlues repo add myrepo sources/github-...   # create alias
gjlues repo use myrepo                      # set default

# Create a session
gjlues new "Add unit tests for the auth module"

# Monitor progress
gjlues sessions                             # list sessions
gjlues msg list <alias>                     # view activities
gjlues msg send <alias> "Also add integration tests"
gjlues msg approve <alias>                  # approve plan
```

## Commands

| Command | Description |
|---|---|
| `gjlues user add <name> <key>` | Add user with API key |
| `gjlues user use <name>` | Switch user |
| `gjlues user list` | List users |
| `gjlues user current` | Show current user |
| `gjlues sources [--fields=...]` | List connected repos |
| `gjlues repo add/list/rm/use` | Manage repo aliases |
| `gjlues sessions [--fields=...]` | List sessions |
| `gjlues alias add/list/rm` | Manage session aliases |
| `gjlues new "prompt" [--repo=...]` | Create session |
| `gjlues msg list <alias> [--fields=...]` | List activities |
| `gjlues msg send <alias> "text"` | Send message |
| `gjlues msg approve <alias>` | Approve plan |
| `gjlues version` | Show version |

## Configuration

- **Env var**: `GJLUES_API_KEY` takes priority over config file
- **Config**: `~/.gjlues_config` — multi-user setup with aliases

```json
{
  "users": {"alice": "key1"},
  "currentUser": "alice",
  "sessionAlias": {"test1": "abc123"},
  "repoAlias": {"myrepo": "sources/github-org-repo"},
  "currentRepo": "sources/github-org-repo"
}
```

## Build from source

```bash
git clone https://github.com/forechoandlook/gjlues.git
cd gjlues
make build
```

## License

MIT
