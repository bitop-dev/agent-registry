package index

import (
	"time"

	"github.com/bitop-dev/agent-registry/internal/archive"
	"github.com/bitop-dev/agent-registry/internal/source"
)

const APIVersion = "agent.registry/v1"

type SearchIndex struct {
	APIVersion  string         `json:"apiVersion"`
	GeneratedAt time.Time      `json:"generatedAt"`
	Packages    []IndexPackage `json:"packages"`
}

type IndexPackage struct {
	Name          string   `json:"name"`
	LatestVersion string   `json:"latestVersion"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Runtime       string   `json:"runtime"`
	Keywords      []string `json:"keywords"`
	Source        string   `json:"source"`
}

type PackageMetadata struct {
	APIVersion  string           `json:"apiVersion"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Versions    []VersionSummary `json:"versions"`
}

type VersionSummary struct {
	Version   string           `json:"version"`
	Framework string           `json:"framework"`
	Runtime   string           `json:"runtime"`
	Artifact  archive.Artifact `json:"artifact"`
}

type VersionManifest struct {
	APIVersion   string            `json:"apiVersion"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Framework    string            `json:"framework"`
	Runtime      string            `json:"runtime"`
	Artifact     archive.Artifact  `json:"artifact"`
	InstallHints map[string]string `json:"installHints,omitempty"`
}

func BuildSearchIndex(packages []source.PackageRecord, sourceName string) SearchIndex {
	out := SearchIndex{APIVersion: APIVersion, GeneratedAt: time.Now().UTC()}
	for _, rec := range packages {
		out.Packages = append(out.Packages, IndexPackage{
			Name:          rec.Name,
			LatestVersion: rec.Version,
			Description:   rec.Description,
			Category:      rec.Category,
			Runtime:       rec.Runtime,
			Keywords:      rec.Keywords,
			Source:        sourceName,
		})
	}
	return out
}

func BuildPackageMetadata(rec source.PackageRecord, art archive.Artifact) PackageMetadata {
	return PackageMetadata{
		APIVersion:  APIVersion,
		Name:        rec.Name,
		Description: rec.Description,
		Versions: []VersionSummary{{
			Version:   rec.Version,
			Framework: rec.Framework,
			Runtime:   rec.Runtime,
			Artifact:  art,
		}},
	}
}

// BuildPackageMetadataMulti builds metadata including all available versions.
func BuildPackageMetadataMulti(recs []source.PackageRecord, arts []archive.Artifact) PackageMetadata {
	if len(recs) == 0 {
		return PackageMetadata{APIVersion: APIVersion}
	}
	meta := PackageMetadata{
		APIVersion:  APIVersion,
		Name:        recs[0].Name,
		Description: recs[0].Description,
	}
	for i, rec := range recs {
		var art archive.Artifact
		if i < len(arts) {
			art = arts[i]
		}
		meta.Versions = append(meta.Versions, VersionSummary{
			Version:   rec.Version,
			Framework: rec.Framework,
			Runtime:   rec.Runtime,
			Artifact:  art,
		})
	}
	return meta
}

// ─── Profile index types ────────────────────────────────────────────────────

type ProfileIndex struct {
	APIVersion  string          `json:"apiVersion"`
	GeneratedAt time.Time       `json:"generatedAt"`
	Profiles    []IndexProfile  `json:"profiles"`
}

type IndexProfile struct {
	Name          string `json:"name"`
	LatestVersion string `json:"latestVersion"`
	Description   string `json:"description"`
	Source        string `json:"source"`
}

type ProfileMetadata struct {
	APIVersion  string                `json:"apiVersion"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Versions    []ProfileVersionEntry `json:"versions"`
}

type ProfileVersionEntry struct {
	Version  string           `json:"version"`
	Artifact archive.Artifact `json:"artifact"`
}

func BuildProfileIndex(profiles []source.ProfileRecord, sourceName string) ProfileIndex {
	out := ProfileIndex{APIVersion: APIVersion, GeneratedAt: time.Now().UTC()}
	for _, rec := range profiles {
		out.Profiles = append(out.Profiles, IndexProfile{
			Name:          rec.Name,
			LatestVersion: rec.Version,
			Description:   rec.Description,
			Source:        sourceName,
		})
	}
	return out
}

func BuildProfileMetadata(rec source.ProfileRecord, art archive.Artifact) ProfileMetadata {
	return ProfileMetadata{
		APIVersion:  APIVersion,
		Name:        rec.Name,
		Description: rec.Description,
		Versions: []ProfileVersionEntry{{
			Version:  rec.Version,
			Artifact: art,
		}},
	}
}

// ────────────────────────────────────────────────────────────────────────────

func BuildVersionManifest(rec source.PackageRecord, art archive.Artifact) VersionManifest {
	return VersionManifest{
		APIVersion:  APIVersion,
		Name:        rec.Name,
		Version:     rec.Version,
		Description: rec.Description,
		Framework:   rec.Framework,
		Runtime:     rec.Runtime,
		Artifact:    art,
		InstallHints: map[string]string{
			"runtime": "Install runtime dependencies separately if this plugin uses external services, CLIs, or MCP servers.",
		},
	}
}
