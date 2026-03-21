package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"sort"

	"github.com/bitop-dev/agent-registry/internal/archive"
	"github.com/bitop-dev/agent-registry/internal/index"
	"github.com/bitop-dev/agent-registry/internal/metrics"
	"github.com/bitop-dev/agent-registry/internal/middleware"
	"github.com/bitop-dev/agent-registry/internal/source"
	"github.com/bitop-dev/agent-registry/internal/stats"
)

// safeNameRe matches valid package names: lowercase letters, digits, hyphens.
// Anything else is rejected to prevent path traversal or injection.
var safeNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$`)

// WorkerRecord represents a registered agent worker.
type WorkerRecord struct {
	URL          string   `json:"url"`
	Profiles     []string `json:"profiles"`
	Capabilities []string `json:"capabilities"`
	RegisteredAt string   `json:"registeredAt"`
	LastHeartbeat string  `json:"lastHeartbeat"`
}

type Server struct {
	SourceName   string
	DataDir      string
	BaseURL      string
	PublishToken string // empty = publish disabled
	Stats        *stats.Store

	mu               sync.RWMutex
	packages         []source.PackageRecord
	artifacts        map[string]archive.Artifact
	profiles         []source.ProfileRecord
	profileArtifacts map[string]archive.Artifact
	workers          []WorkerRecord
}

// New returns an http.Handler with all registry routes registered and
// wrapped in structured-logging middleware.
func New(sourceName string, packages []source.PackageRecord, artifacts map[string]archive.Artifact,
	profiles []source.ProfileRecord, profileArtifacts map[string]archive.Artifact,
	opts ServerOptions) http.Handler {
	s := &Server{
		SourceName:       sourceName,
		DataDir:          opts.DataDir,
		BaseURL:          opts.BaseURL,
		PublishToken:     opts.PublishToken,
		Stats:            stats.NewStore(opts.DataDir),
		packages:         packages,
		artifacts:        artifacts,
		profiles:         profiles,
		profileArtifacts: profileArtifacts,
	}

	// Extract READMEs from existing artifacts
	for _, pkg := range packages {
		if readme := source.ExtractREADME(pkg.Path); readme != "" {
			s.Stats.SetREADME("plugin", pkg.Name, readme)
		}
	}
	for _, prof := range profiles {
		if readme := source.ExtractREADME(prof.Path); readme != "" {
			s.Stats.SetREADME("profile", prof.Name, readme)
		}
	}

	// Start periodic stats flush (every 60s)
	s.Stats.StartFlush(60 * time.Second)

	mux := http.NewServeMux()

	// Marketplace UI (static files embedded in binary)
	if opts.WebFS != nil {
		mux.Handle("/", opts.WebFS)
	}

	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/index.json", s.handleIndex)
	mux.HandleFunc("/v1/search", s.handleSearch)
	mux.HandleFunc("/v1/packages", s.handlePublish) // POST only
	mux.HandleFunc("/v1/packages/", s.handlePackages)
	mux.HandleFunc("/v1/profiles", s.handleProfilePublish) // POST only
	mux.HandleFunc("/v1/profiles/index.json", s.handleProfileIndex)
	mux.HandleFunc("/v1/profiles/", s.handleProfilePackages)
	mux.HandleFunc("/v1/workers", s.handleWorkers) // GET + POST + DELETE
	mux.HandleFunc("/artifacts/", s.handleArtifacts)
	mux.Handle("/metrics", metrics.Handler())

	return middleware.Logging(mux)
}

// ServerOptions holds optional configuration for the HTTP server.
type ServerOptions struct {
	DataDir      string
	BaseURL      string
	PublishToken string       // if empty, publish endpoint is disabled
	WebFS        http.Handler // marketplace UI static files
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "health")
	s.mu.RLock()
	pkgCount := len(s.packages)
	profCount := len(s.profiles)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "packages": pkgCount, "profiles": profCount})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "index")
	s.mu.RLock()
	pkgs := s.packages
	s.mu.RUnlock()
	middleware.AddField(r.Context(), "package_count", len(pkgs))
	metrics.RecordIndexRequest()
	idx := index.BuildSearchIndex(pkgs, s.SourceName)
	// Inject download counts
	for i := range idx.Packages {
		idx.Packages[i].Downloads = s.Stats.GetDownloads("plugin", idx.Packages[i].Name)
	}
	writeJSON(w, http.StatusOK, idx)
}

// handlePublish accepts POST /v1/packages with a raw .tar.gz body.
// Requires Authorization: Bearer <token> if PublishToken is configured.
func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "publish")

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.PublishToken == "" {
		writeError(w, http.StatusForbidden, "publish is disabled on this registry")
		return
	}
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if auth != s.PublishToken {
		middleware.AddField(r.Context(), "error", "unauthorized")
		writeError(w, http.StatusUnauthorized, "invalid publish token")
		return
	}
	if s.DataDir == "" {
		writeError(w, http.StatusInternalServerError, "registry data-dir not configured")
		return
	}

	// Stream body to a temp file.
	tmp, err := os.CreateTemp("", "agent-registry-publish-*.tar.gz")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create temp file")
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r.Body); err != nil {
		tmp.Close()
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	tmp.Close()

	// Extract plugin.yaml from the tarball to get name/version.
	rec, err := source.ScanTarball(tmp.Name())
	if err != nil {
		middleware.AddField(r.Context(), "error", err.Error())
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid plugin tarball: %v", err))
		return
	}
	middleware.AddField(r.Context(), "package", rec.Name)
	middleware.AddField(r.Context(), "version", rec.Version)

	// Store the tarball at data/artifacts/<name>/<version>.tar.gz.
	artifactPath := filepath.Join(s.DataDir, "artifacts", rec.Name, rec.Version+".tar.gz")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create artifact dir")
		return
	}
	if err := os.Rename(tmp.Name(), artifactPath); err != nil {
		// Rename may fail across filesystems; fall back to copy.
		if err2 := copyFile(tmp.Name(), artifactPath); err2 != nil {
			writeError(w, http.StatusInternalServerError, "failed to store artifact")
			return
		}
	}

	checksum, err := archive.SHA256File(artifactPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute checksum")
		return
	}
	art := archive.Artifact{
		Type:   "tar.gz",
		URL:    strings.TrimRight(s.BaseURL, "/") + "/artifacts/" + rec.Name + "/" + rec.Version + ".tar.gz",
		SHA256: checksum,
		Path:   artifactPath,
	}
	rec.Path = artifactPath // published packages reference the artifact path

	// Update in-memory registry under write lock.
	s.mu.Lock()
	// Replace existing record for this name+version or append as new version.
	replaced := false
	for i, p := range s.packages {
		if p.Name == rec.Name && p.Version == rec.Version {
			s.packages[i] = rec
			replaced = true
			break
		}
	}
	if !replaced {
		s.packages = append(s.packages, rec)
	}
	s.artifacts[rec.Name+"@"+rec.Version] = art
	s.mu.Unlock()

	// Extract README
	if readme := source.ExtractREADME(artifactPath); readme != "" {
		s.Stats.SetREADME("plugin", rec.Name, readme)
	}

	metrics.SetPackagesLoaded(len(s.packages))
	middleware.AddField(r.Context(), "replaced", replaced)
	writeJSON(w, http.StatusCreated, index.BuildVersionManifest(rec, art))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func (s *Server) handlePackages(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1/packages/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		middleware.AddField(r.Context(), "endpoint", "package_meta")
		middleware.AddField(r.Context(), "error", "missing package name")
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	name := strings.TrimSuffix(parts[0], ".json")
	if !safeNameRe.MatchString(name) {
		middleware.AddField(r.Context(), "endpoint", "package_meta")
		middleware.AddField(r.Context(), "error", "invalid package name")
		middleware.AddField(r.Context(), "package", name)
		writeError(w, http.StatusBadRequest, "invalid package name")
		return
	}

	middleware.AddField(r.Context(), "package", name)
	metrics.RecordPackageRequest()

	rec, art, err := s.lookup(name)
	if err != nil {
		middleware.AddField(r.Context(), "endpoint", "package_meta")
		middleware.AddField(r.Context(), "error", err.Error())
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	middleware.AddField(r.Context(), "runtime", rec.Runtime)
	middleware.AddField(r.Context(), "category", rec.Category)

	if len(parts) == 1 {
		// Check for detail.json suffix
		if strings.HasSuffix(parts[0], "detail.json") {
			detailName := strings.TrimSuffix(parts[0], ".detail.json")
			if detailName == "" {
				detailName = name
			}
			s.handleDetail(w, "plugin", detailName)
			return
		}
		middleware.AddField(r.Context(), "endpoint", "package_meta")
		allRecs, allArts := s.allVersions(name)
		if len(allRecs) == 0 {
			writeJSON(w, http.StatusOK, index.BuildPackageMetadata(rec, art))
		} else {
			writeJSON(w, http.StatusOK, index.BuildPackageMetadataMulti(allRecs, allArts))
		}
		return
	}

	// Handle /v1/packages/{name}/detail.json
	if len(parts) == 2 && parts[1] == "detail.json" {
		s.handleDetail(w, "plugin", name)
		return
	}

	version := strings.TrimSuffix(parts[1], ".json")
	middleware.AddField(r.Context(), "version", version)
	middleware.AddField(r.Context(), "endpoint", "version_manifest")

	versionRec, versionArt, err := s.lookupVersion(name, version)
	if err != nil {
		middleware.AddField(r.Context(), "error", "version not found")
		writeError(w, http.StatusNotFound, "version not found")
		return
	}
	writeJSON(w, http.StatusOK, index.BuildVersionManifest(versionRec, versionArt))
}

func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "artifact")

	// Path is either /artifacts/{name}/{version}.tar.gz (plugin)
	// or /artifacts/profiles/{name}/{version}.tar.gz (profile).
	trimmed := strings.TrimPrefix(r.URL.Path, "/artifacts/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")

	if len(parts) == 3 && parts[0] == "profiles" {
		// Profile artifact.
		name := parts[1]
		version := strings.TrimSuffix(parts[2], ".tar.gz")
		if !safeNameRe.MatchString(name) {
			middleware.AddField(r.Context(), "error", "invalid profile name")
			writeError(w, http.StatusBadRequest, "invalid profile name")
			return
		}
		middleware.AddField(r.Context(), "profile", name)
		middleware.AddField(r.Context(), "version", version)
		art, err := s.lookupProfileArtifact(name, version)
		if err != nil {
			middleware.AddField(r.Context(), "error", "artifact not found")
			writeError(w, http.StatusNotFound, "artifact not found")
			return
		}
		middleware.AddField(r.Context(), "artifact_path", art.Path)
		metrics.RecordArtifactDownload()
		s.Stats.RecordDownload("profile", name)
		w.Header().Set("Content-Type", "application/gzip")
		http.ServeFile(w, r, art.Path)
		return
	}

	// Plugin artifact.
	if len(parts) != 2 {
		middleware.AddField(r.Context(), "error", "bad artifact path")
		writeError(w, http.StatusNotFound, "artifact not found")
		return
	}
	name := parts[0]
	version := strings.TrimSuffix(parts[1], ".tar.gz")
	if !safeNameRe.MatchString(name) {
		middleware.AddField(r.Context(), "error", "invalid package name")
		middleware.AddField(r.Context(), "package", name)
		writeError(w, http.StatusBadRequest, "invalid package name")
		return
	}
	middleware.AddField(r.Context(), "package", name)
	middleware.AddField(r.Context(), "version", version)
	rec, art, err := s.lookupVersion(name, version)
	if err != nil {
		middleware.AddField(r.Context(), "error", "artifact not found")
		writeError(w, http.StatusNotFound, "artifact not found")
		return
	}
	middleware.AddField(r.Context(), "runtime", rec.Runtime)
	middleware.AddField(r.Context(), "artifact_path", art.Path)
	metrics.RecordArtifactDownload()
	s.Stats.RecordDownload("plugin", name)
	w.Header().Set("Content-Type", "application/gzip")
	http.ServeFile(w, r, art.Path)
}

func (s *Server) handleProfilePublish(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "profile_publish")
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.PublishToken == "" {
		writeError(w, http.StatusForbidden, "publish is disabled")
		return
	}
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if auth != s.PublishToken {
		writeError(w, http.StatusUnauthorized, "invalid publish token")
		return
	}

	tmp, err := os.CreateTemp("", "agent-profile-publish-*.tar.gz")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create temp file")
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r.Body); err != nil {
		tmp.Close()
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	tmp.Close()

	rec, err := source.ScanProfileTarball(tmp.Name())
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid profile tarball: %v", err))
		return
	}
	middleware.AddField(r.Context(), "profile", rec.Name)
	middleware.AddField(r.Context(), "version", rec.Version)

	artifactPath := filepath.Join(s.DataDir, "artifacts", "profiles", rec.Name, rec.Version+".tar.gz")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create artifact dir")
		return
	}
	if err := os.Rename(tmp.Name(), artifactPath); err != nil {
		if err2 := copyFile(tmp.Name(), artifactPath); err2 != nil {
			writeError(w, http.StatusInternalServerError, "failed to store artifact")
			return
		}
	}

	checksum, err := archive.SHA256File(artifactPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute checksum")
		return
	}
	art := archive.Artifact{
		Type:   "tar.gz",
		URL:    strings.TrimRight(s.BaseURL, "/") + "/artifacts/profiles/" + rec.Name + "/" + rec.Version + ".tar.gz",
		SHA256: checksum,
		Path:   artifactPath,
	}
	rec.Path = artifactPath

	s.mu.Lock()
	replaced := false
	for i, p := range s.profiles {
		if p.Name == rec.Name {
			s.profiles[i] = rec
			replaced = true
			break
		}
	}
	if !replaced {
		s.profiles = append(s.profiles, rec)
	}
	s.profileArtifacts[rec.Name+"@"+rec.Version] = art
	s.mu.Unlock()

	// Extract README
	if readme := source.ExtractREADME(artifactPath); readme != "" {
		s.Stats.SetREADME("profile", rec.Name, readme)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"name":    rec.Name,
		"version": rec.Version,
		"type":    "profile",
	})
}

func (s *Server) handleProfileIndex(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "profile_index")
	s.mu.RLock()
	profs := s.profiles
	s.mu.RUnlock()
	middleware.AddField(r.Context(), "profile_count", len(profs))
	idx := index.BuildProfileIndex(profs, s.SourceName)
	// Inject download counts
	for i := range idx.Profiles {
		idx.Profiles[i].Downloads = s.Stats.GetDownloads("profile", idx.Profiles[i].Name)
	}
	writeJSON(w, http.StatusOK, idx)
}

func (s *Server) handleProfilePackages(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1/profiles/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		middleware.AddField(r.Context(), "endpoint", "profile_meta")
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	name := strings.TrimSuffix(parts[0], ".json")
	if !safeNameRe.MatchString(name) {
		middleware.AddField(r.Context(), "endpoint", "profile_meta")
		middleware.AddField(r.Context(), "error", "invalid profile name")
		writeError(w, http.StatusBadRequest, "invalid profile name")
		return
	}
	middleware.AddField(r.Context(), "profile", name)

	// Handle /v1/profiles/{name}/detail.json
	if len(parts) == 2 && parts[1] == "detail.json" {
		s.handleDetail(w, "profile", name)
		return
	}

	middleware.AddField(r.Context(), "endpoint", "profile_meta")
	rec, art, err := s.lookupProfile(name)
	if err != nil {
		middleware.AddField(r.Context(), "error", err.Error())
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, index.BuildProfileMetadata(rec, art))
}

// lookup returns the latest version of a package by name.
func (s *Server) lookup(name string) (source.PackageRecord, archive.Artifact, error) {
	return s.lookupVersion(name, "")
}

// lookupVersion returns a specific version of a package, or the latest if version is empty.
func (s *Server) lookupVersion(name, version string) (source.PackageRecord, archive.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var match *source.PackageRecord
	for i := range s.packages {
		rec := &s.packages[i]
		if rec.Name != name {
			continue
		}
		if version != "" && rec.Version != version {
			continue
		}
		// If no specific version requested, take the first match (latest by convention).
		match = rec
		break
	}
	if match == nil {
		return source.PackageRecord{}, archive.Artifact{}, errors.New("package not found")
	}
	art, ok := s.artifacts[match.Name+"@"+match.Version]
	if !ok {
		return source.PackageRecord{}, archive.Artifact{}, errors.New("artifact missing")
	}
	return *match, art, nil
}

// allVersions returns all versions of a package sorted newest first.
func (s *Server) allVersions(name string) ([]source.PackageRecord, []archive.Artifact) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var recs []source.PackageRecord
	var arts []archive.Artifact
	for _, rec := range s.packages {
		if rec.Name != name {
			continue
		}
		art, ok := s.artifacts[rec.Name+"@"+rec.Version]
		if !ok {
			continue
		}
		recs = append(recs, rec)
		arts = append(arts, art)
	}
	return recs, arts
}

func (s *Server) lookupProfile(name string) (source.ProfileRecord, archive.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rec := range s.profiles {
		if rec.Name != name {
			continue
		}
		art, ok := s.profileArtifacts[rec.Name+"@"+rec.Version]
		if !ok {
			return source.ProfileRecord{}, archive.Artifact{}, errors.New("profile artifact missing")
		}
		return rec, art, nil
	}
	return source.ProfileRecord{}, archive.Artifact{}, errors.New("profile not found")
}

func (s *Server) lookupProfileArtifact(name, version string) (archive.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := name + "@" + version
	art, ok := s.profileArtifacts[key]
	if !ok {
		return archive.Artifact{}, errors.New("profile artifact not found")
	}
	return art, nil
}

func WarmProfileArtifacts(profiles []source.ProfileRecord, dataDir, baseURL string) (map[string]archive.Artifact, error) {
	arts := make(map[string]archive.Artifact, len(profiles))
	for _, rec := range profiles {
		art, err := archive.EnsureDir(rec.Name, rec.Version, rec.Path, "profiles", dataDir, baseURL)
		if err != nil {
			return nil, err
		}
		arts[rec.Name+"@"+rec.Version] = art
	}
	return arts, nil
}

// handleWorkers manages worker registration, listing, and deregistration.
func (s *Server) handleWorkers(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "workers")
	switch r.Method {
	case http.MethodGet:
		// List workers, optionally filtered by capability.
		capability := r.URL.Query().Get("capability")
		profile := r.URL.Query().Get("profile")
		s.mu.RLock()
		var filtered []WorkerRecord
		for _, wr := range s.workers {
			if capability != "" {
				found := false
				for _, c := range wr.Capabilities {
					if strings.EqualFold(c, capability) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if profile != "" {
				found := false
				for _, p := range wr.Profiles {
					if strings.EqualFold(p, profile) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			filtered = append(filtered, wr)
		}
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{"workers": filtered, "count": len(filtered)})

	case http.MethodPost:
		// Register or heartbeat a worker.
		var req struct {
			URL          string   `json:"url"`
			Profiles     []string `json:"profiles"`
			Capabilities []string `json:"capabilities"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if strings.TrimSpace(req.URL) == "" {
			writeError(w, http.StatusBadRequest, "url is required")
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		s.mu.Lock()
		updated := false
		for i, wr := range s.workers {
			if wr.URL == req.URL {
				s.workers[i].Profiles = req.Profiles
				s.workers[i].Capabilities = req.Capabilities
				s.workers[i].LastHeartbeat = now
				updated = true
				break
			}
		}
		if !updated {
			s.workers = append(s.workers, WorkerRecord{
				URL:           req.URL,
				Profiles:      req.Profiles,
				Capabilities:  req.Capabilities,
				RegisteredAt:  now,
				LastHeartbeat: now,
			})
		}
		total := len(s.workers)
		s.mu.Unlock()
		middleware.AddField(r.Context(), "worker_url", req.URL)
		middleware.AddField(r.Context(), "worker_count", total)
		writeJSON(w, http.StatusOK, map[string]any{"registered": true, "url": req.URL})

	case http.MethodDelete:
		// Deregister a worker.
		workerURL := r.URL.Query().Get("url")
		if workerURL == "" {
			writeError(w, http.StatusBadRequest, "url query parameter is required")
			return
		}
		s.mu.Lock()
		removed := false
		for i, wr := range s.workers {
			if wr.URL == workerURL {
				s.workers = append(s.workers[:i], s.workers[i+1:]...)
				removed = true
				break
			}
		}
		s.mu.Unlock()
		if !removed {
			writeError(w, http.StatusNotFound, "worker not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deregistered": true, "url": workerURL})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSearch provides full-text search across plugins and profiles.
// GET /v1/search?q=grafana&type=plugin&category=integration&sort=downloads
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "search")
	q := strings.ToLower(r.URL.Query().Get("q"))
	filterType := r.URL.Query().Get("type")       // "plugin", "profile", or "" (both)
	filterCat := r.URL.Query().Get("category")     // filter by category
	filterRuntime := r.URL.Query().Get("runtime")  // filter by runtime
	sortBy := r.URL.Query().Get("sort")            // "downloads", "name", "" (default: name)

	type searchResult struct {
		Name        string   `json:"name"`
		Type        string   `json:"type"` // "plugin" or "profile"
		Version     string   `json:"version"`
		Description string   `json:"description"`
		Category    string   `json:"category,omitempty"`
		Runtime     string   `json:"runtime,omitempty"`
		Tools       []string `json:"tools,omitempty"`
		Downloads   int      `json:"downloads"`
		// Profile-specific
		Capabilities []string `json:"capabilities,omitempty"`
		Model        string   `json:"model,omitempty"`
		Extends      string   `json:"extends,omitempty"`
	}

	var results []searchResult

	s.mu.RLock()
	if filterType != "profile" {
		for _, pkg := range s.packages {
			if !matchesSearch(q, pkg.Name, pkg.Description, pkg.Keywords) {
				continue
			}
			if filterCat != "" && !strings.EqualFold(pkg.Category, filterCat) {
				continue
			}
			if filterRuntime != "" && !strings.EqualFold(pkg.Runtime, filterRuntime) {
				continue
			}
			results = append(results, searchResult{
				Name:        pkg.Name,
				Type:        "plugin",
				Version:     pkg.Version,
				Description: pkg.Description,
				Category:    pkg.Category,
				Runtime:     pkg.Runtime,
				Tools:       pkg.Tools,
				Downloads:   s.Stats.GetDownloads("plugin", pkg.Name),
			})
		}
	}
	if filterType != "plugin" {
		for _, prof := range s.profiles {
			if !matchesSearch(q, prof.Name, prof.Description, prof.Capabilities) {
				continue
			}
			results = append(results, searchResult{
				Name:         prof.Name,
				Type:         "profile",
				Version:      prof.Version,
				Description:  prof.Description,
				Capabilities: prof.Capabilities,
				Model:        prof.Model,
				Extends:      prof.Extends,
				Downloads:    s.Stats.GetDownloads("profile", prof.Name),
			})
		}
	}
	s.mu.RUnlock()

	// Sort
	switch sortBy {
	case "downloads":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Downloads > results[j].Downloads
		})
	default:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})
	}

	middleware.AddField(r.Context(), "query", q)
	middleware.AddField(r.Context(), "results", len(results))
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "count": len(results), "query": q})
}

func matchesSearch(query string, fields ...interface{}) bool {
	if query == "" {
		return true // empty query matches all
	}
	for _, f := range fields {
		switch v := f.(type) {
		case string:
			if strings.Contains(strings.ToLower(v), query) {
				return true
			}
		case []string:
			for _, s := range v {
				if strings.Contains(strings.ToLower(s), query) {
					return true
				}
			}
		}
	}
	return false
}

// handleDetail returns full info about a plugin or profile including README.
func (s *Server) handleDetail(w http.ResponseWriter, kind, name string) {
	readme := s.Stats.GetREADME(kind, name)
	downloads := s.Stats.GetDownloads(kind, name)

	if kind == "plugin" {
		rec, _, err := s.lookup(name)
		if err != nil {
			writeError(w, http.StatusNotFound, "plugin not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"name":         rec.Name,
			"type":         "plugin",
			"version":      rec.Version,
			"description":  rec.Description,
			"category":     rec.Category,
			"runtime":      rec.Runtime,
			"tools":        rec.Tools,
			"dependencies": rec.Dependencies,
			"downloads":    downloads,
			"readme":       readme,
		})
	} else {
		s.mu.RLock()
		var found *source.ProfileRecord
		for i := range s.profiles {
			if s.profiles[i].Name == name {
				found = &s.profiles[i]
				break
			}
		}
		s.mu.RUnlock()
		if found == nil {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"name":         found.Name,
			"type":         "profile",
			"version":      found.Version,
			"description":  found.Description,
			"capabilities": found.Capabilities,
			"accepts":      found.Accepts,
			"returns":      found.Returns,
			"extends":      found.Extends,
			"mode":         found.Mode,
			"model":        found.Model,
			"provider":     found.Provider,
			"tools":        found.Tools,
			"downloads":    downloads,
			"readme":       readme,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func WarmArtifacts(packages []source.PackageRecord, dataDir, baseURL string) (map[string]archive.Artifact, error) {
	arts := make(map[string]archive.Artifact, len(packages))
	for _, rec := range packages {
		art, err := archive.Ensure(rec, dataDir, baseURL)
		if err != nil {
			return nil, err
		}
		arts[rec.Name+"@"+rec.Version] = art
	}
	return arts, nil
}

func EnsureDataDir(path string) error {
	return os.MkdirAll(path, 0o755)
}


