package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bitop-dev/agent-registry/internal/httpapi"
	"github.com/bitop-dev/agent-registry/internal/metrics"
	"github.com/bitop-dev/agent-registry/internal/source"
)

func main() {
	addr         := flag.String("addr", "127.0.0.1:9080", "listen address")
	baseURLFlag  := flag.String("base-url", "", "public base URL for artifact download links (default: http://<addr>)")
	pluginRoot   := flag.String("plugin-root", "../agent-plugins", "path to plugin package root (also scanned for profile packages)")
	dataDir      := flag.String("data-dir", "./data", "path to generated registry data")
	publishToken := flag.String("publish-token", "", "bearer token required to publish packages (empty = publish disabled)")
	jsonLog      := flag.Bool("json-log", true, "emit logs as JSON (set false for human-readable text)")
	flag.Parse()

	// Configure structured logging for the whole process.
	var logHandler slog.Handler
	if *jsonLog {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(logHandler))
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	startedAt := time.Now()

	absPluginRoot, err := filepath.Abs(*pluginRoot)
	if err != nil {
		slog.Error("resolving plugin-root", "error", err)
		os.Exit(1)
	}
	absDataDir, err := filepath.Abs(*dataDir)
	if err != nil {
		slog.Error("resolving data-dir", "error", err)
		os.Exit(1)
	}

	// Scan plugin packages.
	slog.Info("scanning plugins", "plugin_root", absPluginRoot)
	packages, err := source.Scan(absPluginRoot)
	if err != nil {
		slog.Error("scanning plugins", "plugin_root", absPluginRoot, "error", err)
		os.Exit(1)
	}
	slog.Info("plugins loaded", "count", len(packages))
	for _, pkg := range packages {
		slog.Info("plugin", "name", pkg.Name, "version", pkg.Version, "runtime", pkg.Runtime, "category", pkg.Category)
	}

	// Scan profile packages from the same root.
	slog.Info("scanning profiles", "plugin_root", absPluginRoot)
	profiles, err := source.ScanProfiles(absPluginRoot)
	if err != nil {
		slog.Error("scanning profiles", "error", err)
		os.Exit(1)
	}
	slog.Info("profiles loaded", "count", len(profiles))
	for _, p := range profiles {
		slog.Info("profile", "name", p.Name, "version", p.Version)
	}

	metrics.SetPackagesLoaded(len(packages))

	if err := httpapi.EnsureDataDir(absDataDir); err != nil {
		slog.Error("creating data-dir", "path", absDataDir, "error", err)
		os.Exit(1)
	}

	baseURL := *baseURLFlag
	if baseURL == "" {
		baseURL = "http://" + *addr
	}

	// Warm plugin artifacts.
	slog.Info("warming plugin artifacts", "data_dir", absDataDir)
	artifacts, err := httpapi.WarmArtifacts(packages, absDataDir, baseURL)
	if err != nil {
		slog.Error("warming plugin artifacts", "error", err)
		os.Exit(1)
	}
	slog.Info("plugin artifacts ready", "count", len(artifacts))

	// Warm profile artifacts.
	slog.Info("warming profile artifacts", "data_dir", absDataDir)
	profileArtifacts, err := httpapi.WarmProfileArtifacts(profiles, absDataDir, baseURL)
	if err != nil {
		slog.Error("warming profile artifacts", "error", err)
		os.Exit(1)
	}
	slog.Info("profile artifacts ready", "count", len(profileArtifacts))

	handler := httpapi.New("official", packages, artifacts, profiles, profileArtifacts, httpapi.ServerOptions{
		DataDir:      absDataDir,
		BaseURL:      baseURL,
		PublishToken: *publishToken,
	})

	slog.Info("server ready",
		"addr", *addr,
		"base_url", baseURL,
		"plugin_root", absPluginRoot,
		"packages", len(packages),
		"profiles", len(profiles),
		"publish_enabled", *publishToken != "",
		"startup_ms", time.Since(startedAt).Milliseconds(),
		"endpoints", []string{
			baseURL + "/healthz",
			baseURL + "/v1/index.json",
			baseURL + "/v1/profiles/index.json",
			baseURL + "/metrics",
		},
	)

	if err := http.ListenAndServe(*addr, handler); err != nil {
		slog.Error("server stopped", "error", fmt.Errorf("listen: %w", err))
		os.Exit(1)
	}
}
