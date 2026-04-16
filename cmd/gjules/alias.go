package main

import (
	"fmt"
	"os"
	"strings"
)

// --- Alias Commands ---

func handleAlias(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules alias <add|list|rm|use>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjules alias add <name> <sessionID>")
			os.Exit(1)
		}
		sessionAliasAdd(args[1], args[2])
	case "list":
		sessionAliasList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules alias rm <name>")
			os.Exit(1)
		}
		sessionAliasRm(args[1])
	case "use":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules alias use <name>")
			os.Exit(1)
		}
		sessionUse(args[1])
	default:
		fmt.Fprintf(os.Stderr, "Unknown alias command: %s\n", args[0])
		os.Exit(1)
	}
}

func sessionAliasAdd(alias, sessionID string) {
	c := loadConfig()
	c.SessionAlias[alias] = sessionID
	saveConfig(c)
	fmt.Printf("Session alias %q -> %s\n", alias, sessionID)
}

func sessionAliasList() {
	c := loadConfig()
	if len(c.SessionAlias) == 0 {
		fmt.Println("No session aliases configured.")
		return
	}
	fmt.Printf("%-15s %s\n", "ALIAS", "SESSION ID")
	fmt.Println(strings.Repeat("-", 50))
	for alias, id := range c.SessionAlias {
		fmt.Printf("%-15s %s\n", alias, id)
	}
}

func sessionAliasRm(alias string) {
	c := loadConfig()
	if _, ok := c.SessionAlias[alias]; !ok {
		fmt.Fprintf(os.Stderr, "Session alias %q not found.\n", alias)
		os.Exit(1)
	}
	delete(c.SessionAlias, alias)
	saveConfig(c)
	fmt.Printf("Session alias %q removed.\n", alias)
}

func sessionUse(alias string) {
	c := loadConfig()
	sessionID := resolveSessionID(alias)
	c.CurrentSession = sessionID
	saveConfig(c)
	fmt.Printf("Current session set to %s\n", sessionID)
}
