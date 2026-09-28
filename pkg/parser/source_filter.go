package parser

import (
	"path/filepath"
	"strings"
)

// EnvironmentFileNames contains standard environment file names in priority order.
var EnvironmentFileNames = []string{
	"env.joss",
	".env",
	"joss.env",
	".env.local",
	"env.enc",
	".env.enc",
	"env.json",
	".env.json",
}

// IgnoredDirectoryNames contains directories that should always be ignored when scanning,
// linting, formatting, analyzing or building Joss projects.
var IgnoredDirectoryNames = []string{
	"node_modules",
	".git",
	"vendor",
	"build",
	"dist",
	".cache",
	".system_generated",
	".turbo",
	".next",
	".idea",
	".vscode",
	"tmp",
	"temp",
	"scratch",
	".gemini",
	".codex",
	".agents",
	".github",
}

// StandardAppDomains contains standard framework domain subdirectories that hold executable Joss application classes.
var StandardAppDomains = []string{
	"app/controllers",
	"app/models",
	"app/middleware",
	"app/services",
	"app/contracts",
	"app/interfaces",
	"app/database",
	"app/jobs",
	"app/tasks",
	"app/providers",
}

// IsEnvFile returns true if the file path points to an environment, config or encrypted environment file.
func IsEnvFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".enc" {
		return true
	}

	for _, name := range EnvironmentFileNames {
		if base == name {
			return true
		}
	}

	if strings.HasPrefix(base, ".env.") || strings.HasPrefix(base, "env.") {
		return true
	}

	return false
}

// IsIgnoredSourceFile returns true if the given file should not be processed as executable Joss source code.
func IsIgnoredSourceFile(path string) bool {
	return IsEnvFile(path)
}

// IsIgnoredDirectory returns true if the directory name matches any standard ignored project folder.
func IsIgnoredDirectory(dirName string) bool {
	dirLower := strings.ToLower(filepath.Base(dirName))
	for _, name := range IgnoredDirectoryNames {
		if dirLower == name {
			return true
		}
	}
	return false
}

// IsJossSourceFile returns true if path has a .joss extension and is not an environment/ignored non-code file.
func IsJossSourceFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".joss") && !IsIgnoredSourceFile(path)
}

// IsTestSourceFile returns true if path represents a test file (*_test.joss or within a tests directory).
func IsTestSourceFile(path string) bool {
	clean := filepath.Clean(path)
	base := strings.ToLower(filepath.Base(clean))
	if strings.HasSuffix(base, "_test.joss") {
		return true
	}
	parts := strings.Split(filepath.ToSlash(clean), "/")
	for _, part := range parts {
		if strings.EqualFold(part, "tests") || strings.EqualFold(part, "test") {
			return true
		}
	}
	return false
}
