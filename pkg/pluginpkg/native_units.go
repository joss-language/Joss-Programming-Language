package pluginpkg

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
)

// ExtractArchiveSourceUnits extracts Joss AST SourceUnits from a single .jp
// plugin archive (supporting both compiled .jbc bytecode and embedded .joss
// source files) so they can be lowered directly into Joss Native IR.
func ExtractArchiveSourceUnits(archivePath string) ([]semanticanalyzer.SourceUnit, error) {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, err
	}
	arch, err := Read(data)
	if err != nil {
		return nil, err
	}

	pluginLabel := strings.TrimSpace(arch.Metadata.Name)
	if pluginLabel == "" {
		pluginLabel = strings.TrimSuffix(filepath.Base(archivePath), filepath.Ext(archivePath))
	}

	keys := make([]string, 0, len(arch.Files))
	for k := range arch.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var units []semanticanalyzer.SourceUnit
	for _, name := range keys {
		content := arch.Files[name]
		lower := strings.ToLower(name)
		switch {
		case strings.HasSuffix(lower, ".jbc"):
			prog, decErr := bytecode.Decode(content)
			if decErr == nil && prog != nil {
				units = append(units, semanticanalyzer.SourceUnit{
					Path:    "<plugin:" + pluginLabel + "/" + filepath.ToSlash(name) + ">",
					Program: prog,
				})
			}
		case strings.HasSuffix(lower, ".joss"):
			l := parser.NewLexer(string(content))
			p := parser.NewParser(l)
			prog := p.ParseProgram()
			if prog != nil && len(p.Errors()) == 0 {
				units = append(units, semanticanalyzer.SourceUnit{
					Path:    "<plugin:" + pluginLabel + "/" + filepath.ToSlash(name) + ">",
					Program: prog,
				})
			}
		}
	}
	return units, nil
}

// ExtractPluginSourceUnits scans <projectRoot>/plugins for .jp archives and
// extracts their Joss AST units for native AOT lowering.
func ExtractPluginSourceUnits(projectRoot string) []semanticanalyzer.SourceUnit {
	pluginsDir := filepath.Join(projectRoot, "plugins")
	info, err := os.Stat(pluginsDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	var jpFiles []string
	_ = filepath.WalkDir(pluginsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".jp") {
			jpFiles = append(jpFiles, path)
		}
		return nil
	})
	sort.Strings(jpFiles)

	var all []semanticanalyzer.SourceUnit
	for _, jpPath := range jpFiles {
		units, extractErr := ExtractArchiveSourceUnits(jpPath)
		if extractErr == nil && len(units) > 0 {
			all = append(all, units...)
		}
	}
	return all
}
