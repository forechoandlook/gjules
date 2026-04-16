package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const baseURL = "https://jules.googleapis.com/v1alpha"

func httpClient() *http.Response {
	// Dummy to satisfy structure if needed, but we use http.Client
	return nil
}

func getHttpClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

func readKey() string {
	if k := os.Getenv("GJULES_API_KEY"); k != "" {
		return k
	}
	c := loadConfig()
	if c.CurrentUser == "" {
		fmt.Fprintln(os.Stderr, "No current user. Run 'gjules user add <name> <key>' first.")
		os.Exit(1)
	}
	key, ok := c.Users[c.CurrentUser]
	if !ok {
		fmt.Fprintf(os.Stderr, "User %q not found in config.\n", c.CurrentUser)
		os.Exit(1)
	}
	return key
}

func do(key, method, path string, body ...io.Reader) (*http.Response, error) {
	var r io.Reader
	if len(body) > 0 {
		r = body[0]
	}
	
	p := path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	
	req, err := http.NewRequest(method, baseURL+p, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	return getHttpClient().Do(req)
}

func doJSON(key, method, path string, body map[string]interface{}) (*http.Response, map[string]interface{}, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = strings.NewReader(string(b))
	}
	
	p := path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	
	req, err := http.NewRequest(method, baseURL+p, r)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("x-goog-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := getHttpClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return resp, result, nil
}

func checkResp(resp *http.Response) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Fprintf(os.Stderr, "Error: server returned status %s\n", resp.Status)
		if b, err := json.MarshalIndent(result, "", "  "); err == nil {
			fmt.Fprintf(os.Stderr, "%s\n", string(b))
		}
		os.Exit(1)
	}
}
