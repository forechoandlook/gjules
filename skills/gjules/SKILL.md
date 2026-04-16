---
name: gjules
description: "gjules CLI, Google's AI coding agent. 连接 GitHub 的仓库，在云端异步地处理任务，如修复 bugs、添加文档和构建新功能。"
author: zwy
version: 0.6.1
---
install: `curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash`
uninstall: `curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/uninstall.sh | bash`

## Workflow Quick Start
```bash
# 1. User & Auth
gjules user add pro "YOUR_API_KEY"
gjules user switch pro
gjules user current                     # Check current user

# 2. Repo Setup
gjules sources --limit=10               # List available repos from API
gjules repo add myrepo sources/github/owner/repo 
gjules repo use myrepo                  # Set default repo for 'new'
gjules repo list                        # View aliases and current default

# 3. Session Management
gjules new "Implement a health check"   # Starts session on default repo
gjules auto_add <repo> "<prompt>"       # Create session by repo name directly
gjules alias add bug1 <session_id>      # Name your session
gjules alias use bug1                   # Focus on this session
gjules msg wait                         # Wait for Jules to finish (Beep & Notify!)

# 4. Review & Converse
gjules msg list                         # View current session activities
gjules msg list bug1 --git              # View specific session with Diff
gjules msg send "Fix the naming"        # Send message to current session
gjules msg approve                      # Approve plan
```

## Command Reference

### Quick Session Creation
- `gjules auto_add <repo> "<prompt>"` : Create a session by repo name — no alias setup needed.
  - `<repo>` can be any of:
    - `gssh` — repo name only (matches any owner)
    - `forechoandlook/gssh` — owner/repo
    - `https://github.com/forechoandlook/gssh` — full GitHub URL (http/https, trailing slash and `.git` suffix are stripped)
  - Internally: normalizes the input to `owner/repo` or `repo`, searches the local sources cache for a match, refreshes cache from API if empty or stale (>24h), then calls `new` with the resolved source name.
  - Optional: `--branch=<name>`, `--auto-pr`

### Session & Alias Management
- `gjules sessions [--filter=todo|active|done]` : List sessions with status filter and a coarse `next_action` hint.
- `gjules sessions apply [id] [--dir=/path]` : Check out a local branch named after the session id from the cloud patch base commit, then fetch the latest patch and run local `git apply`.
- `gjules alias add <name> <id>` : Create a friendly name for a session.
- `gjules alias list` : Show all session aliases.
- `gjules alias use <name>` : Set the "Current Session" context.

### Messaging (Current or Specific)
*Note: [id] is optional if 'alias use' was called.*
- `gjules msg list [id] [--git] [--detail]` : List logs. `--git` for Diff, `--detail` for Plan descriptions.
- `gjules msg latest [id] [N]` : Show the latest 1 or N logs for quick reading.
- `gjules msg latest [id] [N] [--type=code]` : Pair well with code-review flows when you only want the newest patches.
- `gjules msg send [id] "text"` : Send a message.
- `gjules msg approve [id]` : Approve a plan.
- `gjules msg wait [id]` : Block until Jules is ready for input or finished.

### System
- `gjules version` : Show version.
- `gjules update` : Self-update.

## Configuration
- **Structure**: `~/.gjules/` contains `config.json` (global) and `users/<name>/data.json` (user-specific).
- **Priority**: Environment variable `GJULES_API_KEY` overrides file config.

## Fields Reference
- **sessions**: `alias,id,state,title,created`
- **sources**: `alias,id,owner,repo,branch`
- **msg list**: `originator,content`
