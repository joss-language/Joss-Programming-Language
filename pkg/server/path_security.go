package server

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// resolveSafeSubpath resolves rawRelPath under rootDir, rejecting null bytes,
// backslashes, encoded traversal (%2e%2e), parent-directory escapes, and
// symlinks pointing outside rootDir.
func resolveSafeSubpath(rootDir, rawRelPath string) (string, bool) {
	if rawRelPath == "" || strings.ContainsRune(rawRelPath, '\x00') || strings.ContainsRune(rawRelPath, '\\') {
		return "", false
	}
	unescaped, err := url.PathUnescape(rawRelPath)
	if err != nil {
		return "", false
	}
	if strings.ContainsRune(unescaped, '\x00') || strings.ContainsRune(unescaped, '\\') {
		return "", false
	}
	for _, candidate := range []string{rawRelPath, unescaped} {
		for _, seg := range strings.Split(candidate, "/") {
			if seg == ".." {
				return "", false
			}
		}
	}

	cleaned := strings.TrimPrefix(path.Clean("/"+unescaped), "/")
	if cleaned == "" || cleaned == "." || strings.HasPrefix(cleaned, "..") {
		return "", false
	}

	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", false
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(cleaned)))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	if rootReal, err := filepath.EvalSymlinks(rootAbs); err == nil {
		if targetReal, err := filepath.EvalSymlinks(targetAbs); err == nil {
			realRel, err := filepath.Rel(rootReal, targetReal)
			if err != nil || realRel == "." || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
				return "", false
			}
		}
	}

	return targetAbs, true
}
