// Package metrics tracks server-wide counters and exposes them via HTTP.
// Counters are updated by the logging middleware and by individual handlers
// for business-level events (e.g. artifact downloads).
package metrics

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// Store holds all server metrics as atomic counters.
// All fields are safe for concurrent access.
type Store struct {
	startedAt time.Time

	// HTTP request counters
	RequestsTotal     atomic.Int64
	Requests2xx       atomic.Int64
	Requests4xx       atomic.Int64
	Requests5xx       atomic.Int64
	TotalDurationMS   atomic.Int64

	// Business counters
	ArtifactDownloads atomic.Int64
	IndexRequests     atomic.Int64
	PackageRequests   atomic.Int64

	// Gauge (set at startup, never decremented)
	PackagesLoaded atomic.Int64
}

// Global is the single shared metrics store for the process.
var Global = &Store{startedAt: time.Now()}

// Record updates request counters based on HTTP status code and duration.
// Called by the logging middleware after every request.
func Record(status int, durationMS int64) {
	Global.RequestsTotal.Add(1)
	Global.TotalDurationMS.Add(durationMS)
	switch {
	case status >= 500:
		Global.Requests5xx.Add(1)
	case status >= 400:
		Global.Requests4xx.Add(1)
	default:
		Global.Requests2xx.Add(1)
	}
}

// RecordArtifactDownload increments the artifact download counter.
func RecordArtifactDownload() {
	Global.ArtifactDownloads.Add(1)
}

// RecordIndexRequest increments the index endpoint counter.
func RecordIndexRequest() {
	Global.IndexRequests.Add(1)
}

// RecordPackageRequest increments the package metadata endpoint counter.
func RecordPackageRequest() {
	Global.PackageRequests.Add(1)
}

// SetPackagesLoaded records how many packages were discovered at startup.
func SetPackagesLoaded(n int) {
	Global.PackagesLoaded.Store(int64(n))
}

// snapshot returns a point-in-time read of all metrics.
func snapshot() map[string]any {
	total := Global.RequestsTotal.Load()
	totalMS := Global.TotalDurationMS.Load()

	var avgDurationMS float64
	if total > 0 {
		avgDurationMS = float64(totalMS) / float64(total)
	}

	return map[string]any{
		"uptime_seconds":      time.Since(Global.startedAt).Seconds(),
		"packages_loaded":     Global.PackagesLoaded.Load(),
		"requests_total":      total,
		"requests_2xx":        Global.Requests2xx.Load(),
		"requests_4xx":        Global.Requests4xx.Load(),
		"requests_5xx":        Global.Requests5xx.Load(),
		"avg_duration_ms":     avgDurationMS,
		"artifact_downloads":  Global.ArtifactDownloads.Load(),
		"index_requests":      Global.IndexRequests.Load(),
		"package_requests":    Global.PackageRequests.Load(),
	}
}

// Handler returns an HTTP handler that serves the metrics snapshot as JSON.
// Mount at /metrics.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snapshot())
	})
}
