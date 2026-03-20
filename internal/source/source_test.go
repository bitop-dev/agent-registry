package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsPluginPackages(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "send-email"), "send-email", "0.1.0", "integration", "http")
	writeManifest(t, filepath.Join(root, "github-cli"), "github-cli", "0.1.0", "integration", "command")
	packages, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}
	if packages[0].Name != "github-cli" || packages[1].Name != "send-email" {
		t.Fatalf("unexpected package order: %#v", packages)
	}
}

func writeManifest(t *testing.T, dir, name, version, category, runtime string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "apiVersion: agent/v1\nkind: Plugin\nmetadata:\n  name: " + name + "\n  version: " + version + "\n  description: example plugin\nspec:\n  category: " + category + "\n  runtime:\n    type: " + runtime + "\n  requires:\n    framework: \">=0.1.0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
