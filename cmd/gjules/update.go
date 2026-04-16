package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
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

	if err := selfUpdate(latest); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}

	// exec new version so the current process is replaced
	newPath, _ := os.Executable()
	newProc, err := os.StartProcess(newPath, os.Args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err == nil {
		newProc.Wait()
	}
	os.Exit(0)
}

func selfUpdate(latest string) error {
	osName := strings.ToLower(runtime.GOOS)
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		// keep as-is
	}

	assetName := fmt.Sprintf("gjules_%s_%s_%s.tar.gz", latest, osName, arch)
	url := fmt.Sprintf("https://github.com/forechoandlook/gjules/releases/download/v%s/%s", latest, assetName)

	tmpDir, err := os.MkdirTemp("", "gjules-update")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dstPath := tmpDir + "/gjules"
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: status %s", resp.Status)
	}

	// Extract tar.gz in-place to dstPath
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	tr := tar.NewReader(zr)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.Name == "gjules" && hdr.Typeflag == tar.TypeReg {
			f, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("create temp binary: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write binary: %w", err)
			}
			f.Close()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("gjules binary not found in archive")
	}
	resp.Body.Close()

	// Replace current binary
	curPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if err := os.Rename(dstPath, curPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Printf("Updated to v%s at %s\n", latest, curPath)
	return nil
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
