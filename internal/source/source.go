package source

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type PackageRecord struct {
	Name         string
	Version      string
	Description  string
	Category     string
	Runtime      string
	Framework    string
	Path         string
	Keywords     []string
	Tools        []string
	Dependencies []string
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
		Contributes struct {
			Tools []struct {
				ID string `yaml:"id"`
			} `yaml:"tools"`
		} `yaml:"contributes"`
		Requires struct {
			Framework string   `yaml:"framework"`
			Plugins   []string `yaml:"plugins"`
		} `yaml:"requires"`
	} `yaml:"spec"`
}

func Scan(root string) ([]PackageRecord, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			// Empty registry — no plugins yet. Create the directory for future publishes.
			os.MkdirAll(root, 0o755)
			return nil, nil
		}
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
	var tools []string
	for _, t := range mf.Spec.Contributes.Tools {
		if t.ID != "" {
			tools = append(tools, t.ID)
		}
	}
	return PackageRecord{
		Name:         mf.Metadata.Name,
		Version:      mf.Metadata.Version,
		Description:  mf.Metadata.Description,
		Category:     mf.Spec.Category,
		Runtime:      mf.Spec.Runtime.Type,
		Framework:    mf.Spec.Requires.Framework,
		Path:         path,
		Keywords:     unique(baseKeywords),
		Tools:        tools,
		Dependencies: mf.Spec.Requires.Plugins,
	}, nil
}

// ScanTarball reads a plugin .tar.gz and extracts metadata from its plugin.yaml.
// The tarball must contain a top-level directory with a plugin.yaml inside.
func ScanTarball(path string) (PackageRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return PackageRecord{}, err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return PackageRecord{}, fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return PackageRecord{}, err
		}
		// Look for a plugin.yaml at exactly one directory level deep.
		parts := strings.SplitN(filepath.ToSlash(hdr.Name), "/", 3)
		if len(parts) != 2 || parts[1] != "plugin.yaml" {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return PackageRecord{}, err
		}
		var mf manifest
		if err := yaml.Unmarshal(data, &mf); err != nil {
			return PackageRecord{}, fmt.Errorf("parse plugin.yaml: %w", err)
		}
		return toRecord(path, mf)
	}
	return PackageRecord{}, fmt.Errorf("plugin.yaml not found in tarball")
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
