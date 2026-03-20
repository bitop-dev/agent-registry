package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"

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
	SourceName string
	Packages   []source.PackageRecord
	Artifacts  map[string]archive.Artifact
}

// New returns an http.Handler with all registry routes registered and
// wrapped in structured-logging middleware.
func New(sourceName string, packages []source.PackageRecord, artifacts map[string]archive.Artifact) http.Handler {
	s := &Server{SourceName: sourceName, Packages: packages, Artifacts: artifacts}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/index.json", s.handleIndex)
	mux.HandleFunc("/v1/packages/", s.handlePackages)
	mux.HandleFunc("/artifacts/", s.handleArtifacts)
	mux.Handle("/metrics", metrics.Handler())

	// Wrap the entire mux with wide-event logging middleware.
	return middleware.Logging(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "health")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	middleware.AddField(r.Context(), "endpoint", "index")
	middleware.AddField(r.Context(), "package_count", len(s.Packages))
	metrics.RecordIndexRequest()
	writeJSON(w, http.StatusOK, index.BuildSearchIndex(s.Packages, s.SourceName))
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
	for _, rec := range s.Packages {
		if rec.Name != name {
			continue
		}
		art, ok := s.Artifacts[rec.Name+"@"+rec.Version]
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
