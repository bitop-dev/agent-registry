package source

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractREADME reads a .tar.gz and returns the contents of README.md if present.
func ExtractREADME(tarballPath string) string {
	f, err := os.Open(tarballPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return ""
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ""
		}
		name := filepath.Base(hdr.Name)
		if strings.EqualFold(name, "README.md") || strings.EqualFold(name, "README") {
			data, err := io.ReadAll(io.LimitReader(tr, 100*1024)) // 100KB limit
			if err != nil {
				return ""
			}
			return string(data)
		}
	}
	return ""
}
