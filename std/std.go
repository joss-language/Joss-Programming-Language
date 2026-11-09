package std

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/parser"
)

//go:embed database/*.joss storage/*.joss math/*.joss strings/*.joss http/*.joss websocket/*.joss
var stdlibFS embed.FS

var (
	stdlibUnitsOnce sync.Once
	stdlibUnits     []semanticanalyzer.SourceUnit
)

// GetStdlibUnits returns the pre-parsed SourceUnits for all canonical Joss standard library modules.
func GetStdlibUnits() []semanticanalyzer.SourceUnit {
	units := make([]semanticanalyzer.SourceUnit, 0)
	_ = fs.WalkDir(stdlibFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".joss") {
			return nil
		}
		content, readErr := stdlibFS.ReadFile(path)
		if readErr != nil {
			return nil
		}
		normalizedPath := filepath.ToSlash(path)
		virtualPath := "<std/" + normalizedPath + ">"

		l := parser.NewLexer(string(content))
		p := parser.NewParser(l)
		prog := p.ParseProgram()
		if prog != nil {
			units = append(units, semanticanalyzer.SourceUnit{
				Path:    virtualPath,
				Program: prog,
			})
		}
		return nil
	})
	return units
}
