package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
)

// ProjectBuildConfig stores user overrides and build configurations.
type ProjectBuildConfig struct {
	Mode         string
	TargetOS     string
	TargetArch   string
	Profile      string
	PruneFiles   bool
	PruneMethods bool
	KeepClasses  []string
	KeepSymbols  []string
	KeepFiles    []string
}

// LoadProjectBuildConfig parses joss.yaml if present for build directives.
func LoadProjectBuildConfig() ProjectBuildConfig {
	cfg := ProjectBuildConfig{
		Mode:         "release",
		TargetOS:     "",
		TargetArch:   "",
		Profile:      "cli",
		PruneFiles:   true,
		PruneMethods: true,
	}

	data, err := os.ReadFile("joss.yaml")
	if err != nil {
		return cfg
	}

	content := string(data)
	if mode := packageManifestValue(content, "build", "mode"); mode != "" {
		cfg.Mode = mode
		if mode == "debug" {
			cfg.PruneFiles = false
			cfg.PruneMethods = false
		}
	}
	if target := packageManifestValue(content, "build", "target"); target != "" {
		parts := strings.Split(target, "-")
		if len(parts) == 2 {
			cfg.TargetOS = parts[0]
			cfg.TargetArch = parts[1]
		}
	}
	if profile := packageManifestValue(content, "build", "profile"); profile != "" {
		cfg.Profile = profile
	}
	if prune := packageManifestValue(content, "build", "prune_methods"); prune != "" {
		cfg.PruneMethods = prune == "true"
	}
	if pruneFiles := packageManifestValue(content, "build", "prune_files"); pruneFiles != "" {
		cfg.PruneFiles = pruneFiles == "true"
	}

	cfg.KeepClasses = parseManifestStringList(content, "keep_classes")
	cfg.KeepSymbols = parseManifestStringList(content, "keep_symbols")
	cfg.KeepFiles = parseManifestStringList(content, "keep_files")

	return cfg
}

func parseManifestStringList(content, key string) []string {
	var result []string
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	active := false
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == key+":" {
			active = true
			continue
		}
		if active {
			if strings.HasPrefix(trimmed, "- ") {
				item := strings.Trim(strings.TrimPrefix(trimmed, "- "), "\"' \t")
				if item != "" {
					result = append(result, item)
				}
			} else if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
				break
			}
		}
	}
	return result
}

// AnalyzeProjectReachability scans all available source units and builds the reachability graph.
func AnalyzeProjectReachability(entrypoint string, keepClasses, keepSymbols []string) (*semanticanalyzer.ReachabilityGraph, []semanticanalyzer.SourceUnit, error) {
	// Discover all project sources including app/
	sourceDirs := []string{}
	if _, err := os.Stat("app"); err == nil {
		sourceDirs = append(sourceDirs, "app")
	}

	units, diags := semanticanalyzer.LoadProject(entrypoint, sourceDirs...)
	if len(diags) > 0 {
		var msgs []string
		for _, d := range diags {
			msgs = append(msgs, fmt.Sprintf("%s: %s", d.File, d.Message))
		}
		return nil, nil, fmt.Errorf("error cargando fuentes del proyecto: %s", strings.Join(msgs, "; "))
	}

	report := core.AnalyzeSourceUnits(units)
	prep := report.Prepared
	if report.HasErrors() {
		fmt.Println("\nJoss static analysis")
		fmt.Println("------------------------------------------------------------")
		errCount := 0
		warnCount := 0
		for _, d := range report.Diagnostics {
			if d.Severity == diagnostics.SeverityError {
				fmt.Println(d.String())
				if d.Suggestion != "" {
					fmt.Printf("  suggestion: %s\n", d.Suggestion)
				}
				errCount++
			} else {
				warnCount++
			}
		}
		fmt.Println("------------------------------------------------------------")
		fmt.Printf("%d error(s), %d warning(s)\n\n", errCount, warnCount)
		return nil, nil, fmt.Errorf("se encontraron errores en el análisis estático")
	}

	routeFiles := []string{}
	if _, err := os.Stat("routes.joss"); err == nil {
		routeFiles = append(routeFiles, "routes.joss")
	}
	if _, err := os.Stat("api.joss"); err == nil {
		routeFiles = append(routeFiles, "api.joss")
	}

	opts := semanticanalyzer.ReachabilityOptions{
		Entrypoint:  entrypoint,
		KeepClasses: keepClasses,
		KeepSymbols: keepSymbols,
		RouteFiles:  routeFiles,
	}

	graph := semanticanalyzer.BuildReachabilityGraph(prep, opts)
	return graph, units, nil
}

// FilterProjectFiles applies file-level and AST-level reachability pruning.
func FilterProjectFiles(allFiles []string, graph *semanticanalyzer.ReachabilityGraph, units []semanticanalyzer.SourceUnit, pruneMethods bool) map[string]semanticanalyzer.SourceUnit {
	unitMap := make(map[string]semanticanalyzer.SourceUnit)
	for _, u := range units {
		cPath := filepath.ToSlash(filepath.Clean(u.Path))
		unitMap[cPath] = u
	}

	filtered := make(map[string]semanticanalyzer.SourceUnit)
	for _, f := range allFiles {
		clean := filepath.ToSlash(filepath.Clean(f))
		if parser.IsJossSourceFile(clean) {
			if graph != nil && !graph.IsFileReachable(clean) {
				// Dead source file: pruned!
				continue
			}
			if u, exists := unitMap[clean]; exists {
				if pruneMethods {
					filtered[clean] = semanticanalyzer.PruneProgramAST(u, graph)
				} else {
					filtered[clean] = u
				}
			}
		}
	}

	return filtered
}
