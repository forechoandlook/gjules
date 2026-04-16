package main

import (
	"fmt"
	"os"
)

// --- User Commands ---

func handleUser(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gjules user <add|switch|list|rm|current>")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user add <name> <key>")
			os.Exit(1)
		}
		userAdd(args[1], args[2])
	case "switch", "use":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user switch <name>")
			os.Exit(1)
		}
		userSwitch(args[1])
	case "list":
		userList()
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gjules user rm <name>")
			os.Exit(1)
		}
		userRm(args[1])
	case "current":
		userCurrent()
	default:
		fmt.Fprintf(os.Stderr, "Unknown user command: %s\n", args[0])
		os.Exit(1)
	}
}

func userAdd(name, key string) {
	c := loadConfig()
	c.Users[name] = key
	if c.CurrentUser == "" {
		c.CurrentUser = name
	}
	saveConfig(c)
	fmt.Printf("User %q added.\n", name)
}

func userSwitch(name string) {
	c := loadConfig()
	if _, ok := c.Users[name]; !ok {
		fmt.Fprintf(os.Stderr, "User %q not found.\n", name)
		os.Exit(1)
	}
	c.CurrentUser = name
	saveConfig(c)
	fmt.Printf("Switched to user %q.\n", name)
}

func userList() {
	c := loadConfig()
	for name := range c.Users {
		current := ""
		if name == c.CurrentUser {
			current = "*"
		}
		fmt.Printf("%s %s\n", current, name)
	}
}

func userRm(name string) {
	c := loadConfig()
	delete(c.Users, name)
	if c.CurrentUser == name {
		c.CurrentUser = ""
	}
	saveConfig(c)
	fmt.Printf("User %q removed.\n", name)
}

func userCurrent() {
	c := loadConfig()
	if c.CurrentUser == "" {
		fmt.Println("No current user.")
	} else {
		fmt.Printf("Current user: %s\n", c.CurrentUser)
	}
}
