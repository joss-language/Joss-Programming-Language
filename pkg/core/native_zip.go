package core

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Zip Native Class Implementation
// Usage:
//
//	$zip = new Zip()
//	$zip->extract("temp_backup.zip", "temp_extracted/")
func (r *Runtime) executeZipMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "extract":
		if err := r.RequireCapability("fs"); err != nil {
			panic(err)
		}
		if len(args) < 2 {
			fmt.Println("[Zip Error] extract() requires zip path and destination path")
			return false
		}
		zipPath := fmt.Sprintf("%v", args[0])
		destPath := fmt.Sprintf("%v", args[1])

		archive, err := zip.OpenReader(zipPath)
		if err != nil {
			fmt.Printf("[Zip Error] Open failed: %v\n", err)
			return false
		}
		defer archive.Close()

		// Clean dest path and make sure it has trailing slash for prefix check
		destClean := filepath.Clean(destPath)
		const (
			maxSingleExtractedFile int64 = 128 << 20
			maxTotalExtractedBytes int64 = 512 << 20
		)
		var totalExtracted int64

		for _, f := range archive.File {
			if f.Mode()&os.ModeSymlink != 0 {
				fmt.Printf("[Zip Error] Symlink entry rejected in zip: %s\n", f.Name)
				return false
			}
			filePath := filepath.Join(destClean, f.Name)

			// Prevent Zip Slip without relying on unsafe string-prefix checks.
			relative, relErr := filepath.Rel(destClean, filepath.Clean(filePath))
			if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
				fmt.Printf("[Zip Error] Illegal file path in zip: %s\n", f.Name)
				return false
			}

			if f.FileInfo().IsDir() {
				os.MkdirAll(filePath, 0755)
				continue
			}

			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				fmt.Printf("[Zip Error] MkdirAll failed: %v\n", err)
				return false
			}

			dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode().Perm())
			if err != nil {
				fmt.Printf("[Zip Error] OpenFile failed: %v\n", err)
				return false
			}

			fileInArchive, err := f.Open()
			if err != nil {
				dstFile.Close()
				fmt.Printf("[Zip Error] Open in archive failed: %v\n", err)
				return false
			}

			remainingBudget := maxTotalExtractedBytes - totalExtracted
			limit := maxSingleExtractedFile
			if remainingBudget < limit {
				limit = remainingBudget
			}
			written, err := io.Copy(dstFile, io.LimitReader(fileInArchive, limit+1))
			if err != nil || written > limit {
				dstFile.Close()
				fileInArchive.Close()
				_ = os.Remove(filePath)
				fmt.Printf("[Zip Error] Copy failed or size limit exceeded for: %s\n", f.Name)
				return false
			}
			totalExtracted += written

			dstFile.Close()
			fileInArchive.Close()
		}
		return true
	}
	return nil
}
