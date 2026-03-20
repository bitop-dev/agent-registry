package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ncecere/agent-registry/internal/httpapi"
	"github.com/ncecere/agent-registry/internal/source"
)

func TestServerHealthAndIndex(t *testing.T) {
	pluginRoot := filepath.Join("..", "agent-plugins")
	packages, err := source.Scan(pluginRoot)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "data")
	artifacts, err := httpapi.WarmArtifacts(packages, dataDir, "http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New("official", packages, artifacts))
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected health status: %d", resp.StatusCode)
	}

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
}
