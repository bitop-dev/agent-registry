package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type PackageRecord struct {
	Name        string
	Version     string
	Description string
	Category    string
	Runtime     string
	Framework   string
	Path        string
	Keywords    []string
}

type manifest struct {
	Metadata struct {
		Name        string `yaml:"name"`
		Version     string `yaml:"version"`
		Description string `yaml:"description"`
	} `yaml:"metadata"`
	Spec struct {
		Category string `yaml:"category"`
		Runtime  struct {
			Type string `yaml:"type"`
		} `yaml:"runtime"`
		Requires struct {
			Framework string `yaml:"framework"`
		} `yaml:"requires"`
	} `yaml:"spec"`
}

func Scan(root string) ([]PackageRecord, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []PackageRecord
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "registry" {
			continue
		}
		pluginDir := filepath.Join(root, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.yaml")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, err
		}
		var mf manifest
		if err := yaml.Unmarshal(data, &mf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", manifestPath, err)
		}
		rec, err := toRecord(pluginDir, mf)
		if err != nil {
			return nil, fmt.Errorf("package %s: %w", entry.Name(), err)
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func toRecord(path string, mf manifest) (PackageRecord, error) {
	if mf.Metadata.Name == "" || mf.Metadata.Version == "" || mf.Spec.Runtime.Type == "" {
		return PackageRecord{}, fmt.Errorf("missing required manifest fields")
	}
	baseKeywords := []string{mf.Metadata.Name, mf.Spec.Category, mf.Spec.Runtime.Type}
	for _, part := range strings.Fields(strings.ToLower(strings.ReplaceAll(mf.Metadata.Description, "/", " "))) {
		baseKeywords = append(baseKeywords, part)
	}
	return PackageRecord{
		Name:        mf.Metadata.Name,
		Version:     mf.Metadata.Version,
		Description: mf.Metadata.Description,
		Category:    mf.Spec.Category,
		Runtime:     mf.Spec.Runtime.Type,
		Framework:   mf.Spec.Requires.Framework,
		Path:        path,
		Keywords:    unique(baseKeywords),
	}, nil
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
