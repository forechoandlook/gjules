package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// --- Update Commands ---

func handleUpdate() {
	fmt.Printf("Current version: %s\n", Version)
	fmt.Println("Checking for updates...")
	latest, err := fetchLatestVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}
	current := strings.TrimPrefix(Version, "v")
	latest = strings.TrimPrefix(latest, "v")
	if current == latest {
		fmt.Printf("You are already on the latest version (%s).\n", Version)
		return
	}
	fmt.Printf("New version available: v%s. Updating...\n", latest)
	cmd := exec.Command("bash", "-c", "curl -sSf https://raw.githubusercontent.com/forechoandlook/gjules/main/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("gjules updated successfully!")
}

func fetchLatestVersion() (string, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/forechoandlook/gjules/main/VERSION")
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return "", fmt.Errorf("server returned status %s", resp.Status) }
	body, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(body)), nil
}
