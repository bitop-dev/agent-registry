package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/bitop-dev/agent-registry/internal/httpapi"
	"github.com/bitop-dev/agent-registry/internal/source"
)

func TestServerHealthAndIndex(t *testing.T) {
	pluginRoot := filepath.Join("..", "agent-plugins")
	packages, err := source.Scan(pluginRoot)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := source.ScanProfiles(pluginRoot)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "data")
	artifacts, err := httpapi.WarmArtifacts(packages, dataDir, "http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	profileArtifacts, err := httpapi.WarmProfileArtifacts(profiles, dataDir, "http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New("official", packages, artifacts, profiles, profileArtifacts, httpapi.ServerOptions{
		BaseURL: "http://example.test",
		DataDir: dataDir,
	}))
	defer server.Close()

	// Health check includes package and profile counts.
	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected health status: %d", resp.StatusCode)
	}

	// Plugin index.
	resp, err = http.Get(server.URL + "/v1/index.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected index status: %d", resp.StatusCode)
	}
	var payload struct {
		Packages []struct {
			Name string `json:"name"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Packages) == 0 {
		t.Fatal("expected packages in index")
	}

	// Profile index always returns a valid response (may be empty if no standalone profiles).
	resp, err = http.Get(server.URL + "/v1/profiles/index.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected profile index status: %d", resp.StatusCode)
	}
	var profilePayload struct {
		APIVersion string `json:"apiVersion"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profilePayload); err != nil {
		t.Fatal(err)
	}
	if profilePayload.APIVersion == "" {
		t.Fatal("expected apiVersion in profile index")
	}
}
