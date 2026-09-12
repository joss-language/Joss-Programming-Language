package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func FuzzExtractPluginZip(f *testing.F) {
	f.Add("package/src/main.joss", "ok")
	f.Add("package/src/../../escape.joss", "bad")
	f.Add("package/joss.yaml", "name: demo")
	f.Fuzz(func(t *testing.T, entryName, content string) {
		if len(entryName) > 512 || strings.IndexByte(entryName, 0) >= 0 {
			t.Skip()
		}
		root := t.TempDir()
		archivePath := filepath.Join(root, "input.zip")
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		entry, err := writer.Create(entryName)
		if err != nil {
			return
		}
		_, _ = entry.Write([]byte(content))
		if writer.Close() != nil {
			return
		}
		if os.WriteFile(archivePath, buffer.Bytes(), 0600) != nil {
			return
		}
		destination := filepath.Join(root, "destination")
		_ = extractPluginZip(archivePath, destination)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || path == archivePath {
				return nil
			}
			clean := filepath.Clean(path)
			cleanDestination := filepath.Clean(destination)
			if !strings.HasPrefix(clean, cleanDestination+string(filepath.Separator)) {
				t.Fatalf("archive wrote outside destination: %s", clean)
			}
			return nil
		})
	})
}
