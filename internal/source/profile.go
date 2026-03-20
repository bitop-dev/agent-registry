package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProfileRecord holds the normalized metadata for one profile package.
type ProfileRecord struct {
	Name        string
	Version     string
	Description string
	Path        string
}

type profileManifest struct {
	Metadata struct {
		Name        string `yaml:"name"`
		Version     string `yaml:"version"`
		Description string `yaml:"description"`
	} `yaml:"metadata"`
}

// ScanProfiles walks root and returns one ProfileRecord per directory that
// contains a valid profile.yaml. Directories starting with "." or named
// "registry" are skipped. Missing or invalid manifests are skipped with a log.
func ScanProfiles(root string) ([]ProfileRecord, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []ProfileRecord
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "registry" {
			continue
		}
		profileDir := filepath.Join(root, entry.Name())
		manifestPath := filepath.Join(profileDir, "profile.yaml")
		if _, err := os.Stat(manifestPath); err != nil {
			continue // no profile.yaml — it's a plugin or unknown dir
		}
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, err
		}
		var mf profileManifest
		if err := yaml.Unmarshal(data, &mf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", manifestPath, err)
		}
		if mf.Metadata.Name == "" || mf.Metadata.Version == "" {
			continue // not a valid profile package
		}
		out = append(out, ProfileRecord{
			Name:        mf.Metadata.Name,
			Version:     mf.Metadata.Version,
			Description: mf.Metadata.Description,
			Path:        profileDir,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
