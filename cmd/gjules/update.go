package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// --- Update Commands ---

func handleUpdate() {
	fmt.Printf("Current version: %s\n", Version)
	fmt.Println("Checking for updates...")
	latest, err := fetchLatestReleaseVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}
	current := strings.TrimPrefix(Version, "v")
	latest = strings.TrimPrefix(latest, "v")
	switch compareVersions(current, latest) {
	case 0:
		fmt.Printf("You are already on the latest version (%s).\n", Version)
		return
	case 1:
		fmt.Printf("Current version (%s) is newer than the latest published release (v%s).\n", Version, latest)
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

func fetchLatestReleaseVersion() (string, error) {
	version, err := fetchLatestReleaseVersionFromAsset()
	if err == nil {
		return version, nil
	}
	return fetchLatestReleaseVersionFromAPI()
}

func fetchLatestReleaseVersionFromAsset() (string, error) {
	resp, err := http.Get("https://github.com/forechoandlook/gjules/releases/latest/download/VERSION")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("server returned status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", fmt.Errorf("latest release version file was empty")
	}
	return version, nil
}

func fetchLatestReleaseVersionFromAPI() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/forechoandlook/gjules/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("server returned status %s", resp.Status)
	}
	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.TagName) == "" {
		return "", fmt.Errorf("latest release did not include tag_name")
	}
	return strings.TrimSpace(result.TagName), nil
}

func compareVersions(a, b string) int {
	aa := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bb := strings.Split(strings.TrimPrefix(b, "v"), ".")
	for len(aa) < 3 {
		aa = append(aa, "0")
	}
	for len(bb) < 3 {
		bb = append(bb, "0")
	}
	for i := 0; i < 3; i++ {
		ai, _ := strconv.Atoi(aa[i])
		bi, _ := strconv.Atoi(bb[i])
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}
