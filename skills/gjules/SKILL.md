name: gjules
description: "gjules CLI, Google's AI coding agent. 连接 GitHub 的仓库，在云端异步地处理任务，如修复 bugs、添加文档和构建新功能。"
author: zwy
version: 0.2.0
---
install: `curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash`
uninstall: `curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/uninstall.sh | bash`
## Quick Start
```bash
# 1. Add your API key
gjules user add main "your-api-key-here"

# 2. Setup Repo (Only once)
gjules sources --limit=10               # List available repos
gjules repo add myrepo sources/github/owner/repo 
gjules repo use myrepo                  # Set as default

# 3. Start a Task
gjules new "Add unit tests for the auth module"
gjules msg wait                         # Wait for Jules to finish (Beep & Notify!)

# 4. Review & Converse
gjules msg list --git                   # Review code changes (Diff)
gjules msg send "Looks good, but fix the indent"
gjules msg wait                         # Wait again...

# 5. Approve
gjules msg approve                      # One-click approval

# Advanced List & Filter
gjules sessions --filter=todo           # Show only tasks needing your action [!]
gjules sessions --filter=active         # Show all in-progress tasks
gjules alias use my-task                # Set "current session" to avoid typing ID
```
## Configuration
- **Global Config**: `~/.gjules/config.json` (User list & Current user)
- **User Data**: `~/.gjules/users/<username>/data.json` (Aliases, Cache, Current state)
- **Env Var**: `GJULES_API_KEY` takes priority.

## Fields Reference
- **sessions**: `alias,id,state,title,created`
- **sources**: `alias,id,owner,repo,branch`
- **msg list**: `originator,content,created` (Add `--git` for Diff, `--detail` for Plan)
