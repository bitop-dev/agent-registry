// Package stats tracks download counts and READMEs for packages and profiles.
// Data is held in memory and periodically flushed to a JSON file.
package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PackageStats struct {
	Downloads    int       `json:"downloads"`
	LastDownload time.Time `json:"lastDownload,omitempty"`
}

type Store struct {
	mu       sync.RWMutex
	packages map[string]*PackageStats // key: "plugin:name" or "profile:name"
	readmes  map[string]string        // key: "plugin:name" or "profile:name"
	path     string
	dirty    bool
}

func NewStore(dataDir string) *Store {
	s := &Store{
		packages: make(map[string]*PackageStats),
		readmes:  make(map[string]string),
		path:     filepath.Join(dataDir, "stats.json"),
	}
	s.load()
	return s
}

func (s *Store) RecordDownload(kind, name string) {
	key := kind + ":" + name
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.packages[key]
	if !ok {
		st = &PackageStats{}
		s.packages[key] = st
	}
	st.Downloads++
	st.LastDownload = time.Now()
	s.dirty = true
}

func (s *Store) GetDownloads(kind, name string) int {
	key := kind + ":" + name
	s.mu.RLock()
	defer s.mu.RUnlock()
	if st, ok := s.packages[key]; ok {
		return st.Downloads
	}
	return 0
}

func (s *Store) SetREADME(kind, name, content string) {
	key := kind + ":" + name
	s.mu.Lock()
	s.readmes[key] = content
	s.dirty = true
	s.mu.Unlock()
}

func (s *Store) GetREADME(kind, name string) string {
	key := kind + ":" + name
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readmes[key]
}

// StartFlush periodically writes stats to disk.
func (s *Store) StartFlush(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.flush()
		}
	}()
}

type persistedData struct {
	Packages map[string]*PackageStats `json:"packages"`
	Readmes  map[string]string        `json:"readmes"`
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var pd persistedData
	if json.Unmarshal(data, &pd) == nil {
		if pd.Packages != nil {
			s.packages = pd.Packages
		}
		if pd.Readmes != nil {
			s.readmes = pd.Readmes
		}
	}
}

func (s *Store) flush() {
	s.mu.RLock()
	if !s.dirty {
		s.mu.RUnlock()
		return
	}
	pd := persistedData{
		Packages: s.packages,
		Readmes:  s.readmes,
	}
	data, err := json.MarshalIndent(pd, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return
	}

	os.MkdirAll(filepath.Dir(s.path), 0o755)
	os.WriteFile(s.path, data, 0o644)

	s.mu.Lock()
	s.dirty = false
	s.mu.Unlock()
}

func (s *Store) Flush() {
	s.flush()
}
