---
name: gjules
description: "gjules CLI, Google's AI coding agent. 连接 GitHub 的仓库，在云端异步地处理任务，如修复 bugs、添加文档和构建新功能。"
author: zwy
version: 0.1.0
---

install: `curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash`

uninstall: `curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/uninstall.sh | bash`


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

gjules new "prompt" --repo=<alias> #Create session with specific repo
gjules sources [--fields=...]      #List all sources (repos)
# Monitor progress
gjules sessions                             # list sessions
gjules msg list <alias>                     # view activities
gjules msg send <alias> "Also add integration tests"
gjules msg approve <alias>                  # approve plan
gjules feedback --type=bug "msg"   # Append to local JSONL (~/.gjules/feedback.jsonl)
gjules feedback --open --type=bug  # Open GitHub issue with pre-filled content
```

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
