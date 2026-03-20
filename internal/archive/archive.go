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

	"github.com/ncecere/agent-registry/internal/source"
)

type Artifact struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Path   string `json:"-"`
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

func build(rec source.PackageRecord, target string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	gzw := gzip.NewWriter(file)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()
	return filepath.Walk(rec.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if shouldSkip(name, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(rec.Path, path)
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
		header.Name = filepath.ToSlash(filepath.Join(rec.Name, rel))
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

func sha256File(path string) (string, error) {
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
