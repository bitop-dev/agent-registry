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
	addr := flag.String("addr", "127.0.0.1:9080", "listen address")
	pluginRoot := flag.String("plugin-root", "../agent-plugins", "path to plugin package root")
	dataDir := flag.String("data-dir", "./data", "path to generated registry data")
	publishToken := flag.String("publish-token", "", "bearer token required to publish packages (empty = publish disabled)")
	jsonLog := flag.Bool("json-log", true, "emit logs as JSON (set false for human-readable text)")
	flag.Parse()

	// Configure structured logging for the whole process.
	var logHandler slog.Handler
	if *jsonLog {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(logHandler))
	// Also redirect the stdlib log package to slog so any third-party code
	// that calls log.Printf is captured in the same structured stream.
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

	slog.Info("scanning plugins", "plugin_root", absPluginRoot)
	packages, err := source.Scan(absPluginRoot)
	if err != nil {
		slog.Error("scanning plugins", "plugin_root", absPluginRoot, "error", err)
		os.Exit(1)
	}
	slog.Info("plugins loaded", "count", len(packages))
	for _, pkg := range packages {
		slog.Info("plugin",
			"name", pkg.Name,
			"version", pkg.Version,
			"runtime", pkg.Runtime,
			"category", pkg.Category,
		)
	}
	metrics.SetPackagesLoaded(len(packages))

	if err := httpapi.EnsureDataDir(absDataDir); err != nil {
		slog.Error("creating data-dir", "path", absDataDir, "error", err)
		os.Exit(1)
	}

	baseURL := "http://" + *addr
	slog.Info("warming artifacts", "data_dir", absDataDir)
	artifacts, err := httpapi.WarmArtifacts(packages, absDataDir, baseURL)
	if err != nil {
		slog.Error("warming artifacts", "error", err)
		os.Exit(1)
	}
	slog.Info("artifacts ready", "count", len(artifacts))

	handler := httpapi.New("official", packages, artifacts, httpapi.ServerOptions{
		DataDir:      absDataDir,
		BaseURL:      baseURL,
		PublishToken: *publishToken,
	})

	publishEnabled := *publishToken != ""
	slog.Info("server ready",
		"addr", *addr,
		"base_url", baseURL,
		"plugin_root", absPluginRoot,
		"packages", len(packages),
		"publish_enabled", publishEnabled,
		"startup_ms", time.Since(startedAt).Milliseconds(),
		"endpoints", []string{
			baseURL + "/healthz",
			baseURL + "/v1/index.json",
			baseURL + "/metrics",
		},
	)

	if err := http.ListenAndServe(*addr, handler); err != nil {
		slog.Error("server stopped", "error", fmt.Errorf("listen: %w", err))
		os.Exit(1)
	}
}
