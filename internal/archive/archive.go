package archive

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitop-dev/agent-registry/internal/source"
)

type Artifact struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Path   string `json:"-"`
}

// EnsureDir builds (or reuses) a .tar.gz artifact for any named directory.
// subPath is appended to dataDir/artifacts/ to allow separate namespaces
// (e.g. "profiles" for profile packages vs the default plugin namespace).
func EnsureDir(name, version, srcPath, subPath, dataDir, baseURL string) (Artifact, error) {
	artifactPath := filepath.Join(dataDir, "artifacts", subPath, name, version+".tar.gz")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		if err := buildDir(name, srcPath, artifactPath); err != nil {
			return Artifact{}, err
		}
	}
	checksum, err := SHA256File(artifactPath)
	if err != nil {
		return Artifact{}, err
	}
	urlPath := strings.TrimRight(baseURL, "/") + "/artifacts/" + subPath + "/" + name + "/" + version + ".tar.gz"
	return Artifact{
		Type:   "tar.gz",
		URL:    urlPath,
		SHA256: checksum,
		Path:   artifactPath,
	}, nil
}

func Ensure(rec source.PackageRecord, dataDir, baseURL string) (Artifact, error) {
	artifactPath := filepath.Join(dataDir, "artifacts", rec.Name, rec.Version+".tar.gz")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return Artifact{}, err
	}
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		if err := build(rec, artifactPath); err != nil {
			return Artifact{}, err
		}
	}
	checksum, err := sha256File(artifactPath)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{
		Type:   "tar.gz",
		URL:    strings.TrimRight(baseURL, "/") + "/artifacts/" + rec.Name + "/" + rec.Version + ".tar.gz",
		SHA256: checksum,
		Path:   artifactPath,
	}, nil
}

// buildDir builds a .tar.gz from srcPath, prefixing all entries with name.
// This is the generic form used by both plugin and profile artifact generation.
func buildDir(name, srcPath, target string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	gzw := gzip.NewWriter(file)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()
	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if shouldSkip(info.Name(), info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join(name, rel))
		header.ModTime = time.Unix(0, 0)
		header.AccessTime = time.Unix(0, 0)
		header.ChangeTime = time.Unix(0, 0)
		header.Uid = 0
		header.Gid = 0
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	})
}

func build(rec source.PackageRecord, target string) error {
	return buildDir(rec.Name, rec.Path, target)
}

func sha256File(path string) (string, error) {
	return SHA256File(path)
}

// SHA256File computes the hex-encoded SHA256 checksum of a file.
// Exported for use by the publish handler.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func shouldSkip(name string, isDir bool) bool {
	if name == ".git" || name == "node_modules" || name == ".DS_Store" {
		return true
	}
	if isDir && strings.HasPrefix(name, ".") {
		return true
	}
	return false
}
