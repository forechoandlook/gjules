# gjules

A lightweight CLI for [Jules](https://jules.google) — Google's AI coding agent.

## 安装 skills 

`npx skills add forechoandlook/gjules --skill gjules`

## Install

```bash
curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash
```

## Uninstall

```bash
curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/uninstall.sh | bash
```

## Quick Start

```bash
# Add your API key
gjules user add main "your-api-key-here"

# Set default repo
gjules sources                              # list available repos
gjules repo add myrepo sources/github-...   # create alias
gjules repo use myrepo                      # set default

# Create a session
gjules new "Add unit tests for the auth module"

# Monitor progress
gjules sessions                             # list sessions
gjules msg list <alias>                     # view activities
gjules msg send <alias> "Also add integration tests"
gjules msg approve <alias>                  # approve plan

# Feedback
gjules feedback --type=bug "msg"            # Append to local JSONL (~/.gjules/feedback.jsonl)
gjules feedback --open --type=bug           # Open GitHub issue with pre-filled content
```

## Commands

| Command | Description |
|---|---|
| `gjules user add <name> <key>` | Add user with API key |
| `gjules user use <name>` | Switch user |
| `gjules user list` | List users |
| `gjules user current` | Show current user |
| `gjules user rm <name>` | Remove user |
| `gjules sources [--fields=...]` | List connected repos |
| `gjules repo add <alias> <src>` | Manage repo aliases |
| `gjules repo list/rm/use` | Manage repo aliases |
| `gjules sessions [--fields=...]` | List sessions |
| `gjules alias add <name> <id>` | Manage session aliases |
| `gjules alias list/rm` | Manage session aliases |
| `gjules new "prompt" [--repo=...]` | Create session |
| `gjules msg list <alias> [--fields=...]` | List activities |
| `gjules msg send <alias> "text"` | Send message |
| `gjules msg approve <alias>` | Approve plan |
| `gjules version` | Show version |
| `gjules update` | Self-update to latest release |
| `gjules feedback` | Submit feedback |

## Configuration

- **Env var**: `GJULES_API_KEY` takes priority over config file
- **Config**: `~/.gjules_config` — multi-user setup with aliases

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
git clone https://github.com/forechoandlook/gjules.git
cd gjules
make build
```

## License

MIT
