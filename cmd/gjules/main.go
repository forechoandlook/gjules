package main

import (
	"fmt"
	"os"
)

// Build-time injected via -ldflags
var (
	Version   = "v0.6.1"
	GitCommit = "unknown"
	GitTag    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "user":
		handleUser(os.Args[2:])
	case "sources":
		sources(os.Args[2:])
	case "repo":
		handleRepo(os.Args[2:])
	case "sessions":
		sessions(os.Args[2:])
	case "alias":
		handleAlias(os.Args[2:])
	case "new":
		handleNew(os.Args[2:])
	case "msg":
		handleMsg(os.Args[2:])
	case "version":
		fmt.Printf("gjules %s (commit: %s, tag: %s)\n", Version, GitCommit, GitTag)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gjules - Jules CLI

Usage:
  gjules user add <name> <key>       Add user with API key
  gjules user switch <name>          Switch current user
  gjules user list                   List all users
  gjules user rm <name>              Remove user
  gjules user current                Show current user

  gjules sources [--limit=20] [--refresh]  List all sources (repos)
  gjules repo add <alias> <source>   Add repo alias
  gjules repo list                   List repo aliases
  gjules repo rm <alias>             Remove repo alias
  gjules repo use <alias>            Set default repo

  gjules sessions [--limit=20] [--refresh] [--filter=todo|active|done] List sessions
  gjules alias add <name> <id>       Add session alias
  gjules alias list                  List session aliases
  gjules alias rm <name>             Remove session alias
  gjules alias use <name>            Set current session
  gjules new "prompt" [--repo=...]   Create session
  gjules new "prompt" --repo=<alias> Create session with specific repo

  gjules msg list [alias] [--limit=20] [--detail] [--git] List activities
  gjules msg send [alias] "text"     Send message
  gjules msg approve [alias]         Approve plan
  gjules msg wait [alias]            Wait for session to be ready

  gjules version                     Show version

Fields:
  sessions: alias,id,state,title,created,name
  sources:  name,id,owner,repo,branch,alias
  msg list: originator,content,created

Environment:
  GJULES_API_KEY                     API key (overrides config)

Config:
  Global: ~/.gjules/config.json
  User:   ~/.gjules/users/<username>/data.json
`)
}
