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

	"github.com/ncecere/agent-registry/internal/archive"
	"github.com/ncecere/agent-registry/internal/index"
	"github.com/ncecere/agent-registry/internal/metrics"
	"github.com/ncecere/agent-registry/internal/middleware"
	"github.com/ncecere/agent-registry/internal/source"
)

// safeNameRe matches valid package names: lowercase letters, digits, hyphens.
// Anything else is rejected to prevent path traversal or injection.
var safeNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$`)

type Server struct {
	SourceName   string
	DataDir      string
	BaseURL      string
	PublishToken string // empty = publish disabled

	mu        sync.RWMutex
	packages  []source.PackageRecord
	artifacts map[string]archive.Artifact
}

// New returns an http.Handler with all registry routes registered and
// wrapped in structured-logging middleware.
func New(sourceName string, packages []source.PackageRecord, artifacts map[string]archive.Artifact, opts ServerOptions) http.Handler {
	s := &Server{
		SourceName:   sourceName,
		DataDir:      opts.DataDir,
		BaseURL:      opts.BaseURL,
		PublishToken: opts.PublishToken,
		packages:     packages,
		artifacts:    artifacts,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/index.json", s.handleIndex)
	mux.HandleFunc("/v1/packages", s.handlePublish) // POST only
	mux.HandleFunc("/v1/packages/", s.handlePackages)
	mux.HandleFunc("/artifacts/", s.handleArtifacts)
	mux.Handle("/metrics", metrics.Handler())

	// Wrap the entire mux with wide-event logging middleware.
	return middleware.Logging(mux)
}

// ServerOptions holds optional configuration for the HTTP server.
type ServerOptions struct {
	DataDir      string
	BaseURL      string
	PublishToken string // if empty, publish endpoint is disabled
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "health")
	s.mu.RLock()
	count := len(s.packages)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "packages": count})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "index")
	s.mu.RLock()
	pkgs := s.packages
	s.mu.RUnlock()
	middleware.AddField(r.Context(), "package_count", len(pkgs))
	metrics.RecordIndexRequest()
	writeJSON(w, http.StatusOK, index.BuildSearchIndex(pkgs, s.SourceName))
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
	// Replace existing record for this name+version or append.
	replaced := false
	for i, p := range s.packages {
		if p.Name == rec.Name {
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
		middleware.AddField(r.Context(), "endpoint", "package_meta")
		writeJSON(w, http.StatusOK, index.BuildPackageMetadata(rec, art))
		return
	}

	version := strings.TrimSuffix(parts[1], ".json")
	middleware.AddField(r.Context(), "version", version)
	middleware.AddField(r.Context(), "endpoint", "version_manifest")

	if version != rec.Version {
		middleware.AddField(r.Context(), "error", "version not found")
		writeError(w, http.StatusNotFound, "version not found")
		return
	}
	writeJSON(w, http.StatusOK, index.BuildVersionManifest(rec, art))
}

func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "artifact")

	trimmed := strings.TrimPrefix(r.URL.Path, "/artifacts/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
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

	rec, art, err := s.lookup(name)
	if err != nil || version != rec.Version {
		middleware.AddField(r.Context(), "error", "artifact not found")
		writeError(w, http.StatusNotFound, "artifact not found")
		return
	}

	middleware.AddField(r.Context(), "runtime", rec.Runtime)
	middleware.AddField(r.Context(), "artifact_path", art.Path)
	metrics.RecordArtifactDownload()

	w.Header().Set("Content-Type", "application/gzip")
	http.ServeFile(w, r, art.Path)
}

func (s *Server) lookup(name string) (source.PackageRecord, archive.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rec := range s.packages {
		if rec.Name != name {
			continue
		}
		art, ok := s.artifacts[rec.Name+"@"+rec.Version]
		if !ok {
			return source.PackageRecord{}, archive.Artifact{}, errors.New("artifact missing")
		}
		return rec, art, nil
	}
	return source.PackageRecord{}, archive.Artifact{}, errors.New("package not found")
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

// SHA256File computes the hex SHA256 of a file — exposed for use in publish handler.
// The real implementation is in archive but re-exported here for convenience.
var _ = archive.SHA256File // ensure it's accessible
