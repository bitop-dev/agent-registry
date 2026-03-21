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

// ScanProfileTarball reads a .tar.gz and extracts metadata from its profile.yaml.
func ScanProfileTarball(path string) (ProfileRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return ProfileRecord{}, err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return ProfileRecord{}, fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ProfileRecord{}, err
		}
		parts := strings.SplitN(filepath.ToSlash(hdr.Name), "/", 3)
		if len(parts) != 2 || parts[1] != "profile.yaml" {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return ProfileRecord{}, err
		}
		var mf profileManifest
		if err := yaml.Unmarshal(data, &mf); err != nil {
			return ProfileRecord{}, fmt.Errorf("parse profile.yaml: %w", err)
		}
		if mf.Metadata.Name == "" || mf.Metadata.Version == "" {
			return ProfileRecord{}, fmt.Errorf("profile.yaml missing name or version")
		}
		return ProfileRecord{
			Name:        mf.Metadata.Name,
			Version:     mf.Metadata.Version,
			Description: mf.Metadata.Description,
			Path:        path,
		}, nil
	}
	return ProfileRecord{}, fmt.Errorf("profile.yaml not found in tarball")
}
