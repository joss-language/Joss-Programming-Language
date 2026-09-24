package fixer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/jossecurity/joss/pkg/formatter"
	"github.com/jossecurity/joss/pkg/parser"
)

type FixResult struct {
	File         string `json:"file"`
	Changed      bool   `json:"changed"`
	FixesApplied int    `json:"fixes_applied"`
	Original     string `json:"original,omitempty"`
	Fixed        string `json:"fixed,omitempty"`
}

type Fixer struct {
	dryRun bool
}

func NewFixer(dryRun bool) *Fixer {
	return &Fixer{dryRun: dryRun}
}

var (
	// Fix missing visibility: func name( -> public func name(
	reFuncVisibility  = regexp.MustCompile(`(?m)^([ \t]*)func\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	reClassVisibility = regexp.MustCompile(`(?m)^([ \t]*)class\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	// Fix missing mixed param type: ($x -> (mixed $x, , $y -> , mixed $y
	reUntypedParam = regexp.MustCompile(`([\(,]\s*)\$([a-zA-Z_][a-zA-Z0-9_]*)`)

	// Fix empty ternary false branch: } : {} -> }
	reEmptyTernaryElse = regexp.MustCompile(`\}\s*:\s*\{\s*\}`)

	// Fix deprecated type aliases
	reDeprecatedType = regexp.MustCompile(`\b(integer|double|boolean|dynamic|any|list)\b(\s+\$[a-zA-Z_][a-zA-Z0-9_]*|\s*[\)|,])`)

	// Fix deprecated procedural string helpers to canonical static methods
	reStrContains       = regexp.MustCompile(`\bstr_contains\s*\(`)
	reStrStartsWith     = regexp.MustCompile(`\bstr_starts_with\s*\(`)
	reStrEndsWith       = regexp.MustCompile(`\bstr_ends_with\s*\(`)
	reStrReplace        = regexp.MustCompile(`\bstr_replace\s*\(`)
	reLegacyLetDynamic  = regexp.MustCompile(`(?m)^([ \t]*)let[ \t]+(\$[a-zA-Z_][a-zA-Z0-9_]*)`)
	reLegacyLetTyped    = regexp.MustCompile(`(?m)^([ \t]*)let[ \t]+([a-zA-Z_][a-zA-Z0-9_]*(?:<[^\r\n>]+>)?(?:\|[a-zA-Z_][a-zA-Z0-9_]*)*)[ \t]+(\$[a-zA-Z_][a-zA-Z0-9_]*)`)
	reLegacyConstructor = regexp.MustCompile(`(?m)^([ \t]*)Init[ \t]+constructor[ \t]*(\([^\r\n]*\))[ \t]*\{`)
)

var deprecatedTypeReplacements = map[string]string{
	"integer": "int",
	"double":  "float",
	"boolean": "bool",
	"dynamic": "mixed",
	"any":     "mixed",
	"list":    "array",
}

func (f *Fixer) FixSource(src string) (string, int) {
	applied := 0
	fixed := src
	if reLegacyConstructor.MatchString(fixed) {
		fixed = reLegacyConstructor.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return reLegacyConstructor.ReplaceAllString(match, "${1}public func constructor${2}: void {")
		})
	}

	// Migrate only declaration-shaped `let` lines. Destructuring is left alone
	// because it has no proven one-token equivalent yet.
	if reLegacyLetDynamic.MatchString(fixed) {
		fixed = reLegacyLetDynamic.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return reLegacyLetDynamic.ReplaceAllString(match, "${1}mixed ${2}")
		})
	}
	if reLegacyLetTyped.MatchString(fixed) {
		fixed = reLegacyLetTyped.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return reLegacyLetTyped.ReplaceAllString(match, "${1}${2} ${3}")
		})
	}

	var nilFixes int
	fixed, nilFixes = replaceIdentifierOutsideTrivia(fixed, "nil", "null")
	applied += nilFixes

	// 1. Fix empty ternary false branch `: {}`
	if reEmptyTernaryElse.MatchString(fixed) {
		fixed = reEmptyTernaryElse.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return "}"
		})
	}

	// 2. Fix implicit top-level func visibility
	if reFuncVisibility.MatchString(fixed) {
		fixed = reFuncVisibility.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return reFuncVisibility.ReplaceAllString(match, "${1}public func ${2}(")
		})
	}

	// 3. Fix implicit top-level class visibility
	if reClassVisibility.MatchString(fixed) {
		fixed = reClassVisibility.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return reClassVisibility.ReplaceAllString(match, "${1}public class ${2}")
		})
	}

	// 4. Fix deprecated type aliases
	if reDeprecatedType.MatchString(fixed) {
		fixed = reDeprecatedType.ReplaceAllStringFunc(fixed, func(match string) string {
			for oldT, newT := range deprecatedTypeReplacements {
				if len(match) >= len(oldT) && match[:len(oldT)] == oldT {
					applied++
					return newT + match[len(oldT):]
				}
			}
			return match
		})
	}

	// 5. Fix procedural helpers to canonical Str calls
	if reStrContains.MatchString(fixed) {
		fixed = reStrContains.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return "Str::contains("
		})
	}
	if reStrStartsWith.MatchString(fixed) {
		fixed = reStrStartsWith.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return "Str::startsWith("
		})
	}
	if reStrEndsWith.MatchString(fixed) {
		fixed = reStrEndsWith.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return "Str::endsWith("
		})
	}
	if reStrReplace.MatchString(fixed) {
		fixed = reStrReplace.ReplaceAllStringFunc(fixed, func(match string) string {
			applied++
			return "Str::replace("
		})
	}

	// 6. Format canonically
	formatted, err := formatter.FormatSource(fixed)
	if err == nil && formatted != "" {
		if formatted != fixed {
			applied++
		}
		fixed = formatted
	}

	return fixed, applied
}

// replaceIdentifierOutsideTrivia performs a lexical identifier migration
// without rewriting strings or comments. It intentionally supports only the
// source forms required by safe codemods.
func replaceIdentifierOutsideTrivia(source, oldName, newName string) (string, int) {
	var out strings.Builder
	count := 0
	for i := 0; i < len(source); {
		if source[i] == '"' || source[i] == '\'' || source[i] == '`' {
			quote := source[i]
			start := i
			i++
			for i < len(source) {
				if source[i] == '\\' && quote != '`' && i+1 < len(source) {
					i += 2
					continue
				}
				i++
				if source[i-1] == quote {
					break
				}
			}
			out.WriteString(source[start:i])
			continue
		}
		if i+1 < len(source) && source[i:i+2] == "//" || source[i] == '#' {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				out.WriteString(source[i:])
				break
			}
			end += i
			out.WriteString(source[i:end])
			i = end
			continue
		}
		if i+1 < len(source) && source[i:i+2] == "/*" {
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				out.WriteString(source[i:])
				break
			}
			end += i + 4
			out.WriteString(source[i:end])
			i = end
			continue
		}
		if isIdentifierByte(source[i]) {
			start := i
			for i < len(source) && isIdentifierByte(source[i]) {
				i++
			}
			word := source[start:i]
			if word == oldName {
				word = newName
				count++
			}
			out.WriteString(word)
			continue
		}
		out.WriteByte(source[i])
		i++
	}
	return out.String(), count
}

func isIdentifierByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= utf8RuneSelf && unicode.IsLetter(rune(value)) || value >= '0' && value <= '9'
}

const utf8RuneSelf = 0x80

func (f *Fixer) FixFile(path string) (FixResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FixResult{}, err
	}

	original := string(data)
	fixed, fixesCount := f.FixSource(original)
	changed := fixed != original

	if changed && !f.dryRun {
		if err := os.WriteFile(path, []byte(fixed), 0644); err != nil {
			return FixResult{}, err
		}
	}

	return FixResult{
		File:         path,
		Changed:      changed,
		FixesApplied: fixesCount,
		Original:     original,
		Fixed:        fixed,
	}, nil
}

func (f *Fixer) FixDirectory(dir string) ([]FixResult, error) {
	var results []FixResult

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if parser.IsIgnoredDirectory(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if parser.IsJossSourceFile(path) {
			res, fixErr := f.FixFile(path)
			if fixErr != nil {
				return fmt.Errorf("failed fixing %s: %w", path, fixErr)
			}
			if res.Changed {
				results = append(results, res)
			}
		}
		return nil
	})

	return results, err
}
