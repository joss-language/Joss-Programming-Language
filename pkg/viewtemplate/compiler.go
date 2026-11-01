package viewtemplate

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

var (
	reExtends       = regexp.MustCompile(`@extends\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	reSection       = regexp.MustCompile(`@section\s*\(\s*['"]([^'"]+)['"]\s*\)([\s\S]*?)@endsection`)
	reYield         = regexp.MustCompile(`@yield\s*\(\s*['"][^'"]+['"]\s*\)`)
	reInclude       = regexp.MustCompile(`@include\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	reBladeComments = regexp.MustCompile(`\{\{--[\s\S]*?--\}\}`)
	reCsrfPre       = regexp.MustCompile(`\{\{\s*csrf_field\(\)\s*\}\}`)
	reForeachStart  = regexp.MustCompile(`^@foreach\s*\(\s*(.*?)\s+as\s+\$([a-zA-Z0-9_]+)(?:\s*=>\s*\$([a-zA-Z0-9_]+))?\s*\)`)
	reExprRaw       = regexp.MustCompile(`(?s)^\{\{!(.*?)\}\}`)
	reExprEscaped   = regexp.MustCompile(`(?s)^\{\{(.*?)\}\}`)
	reDot           = regexp.MustCompile(`\$([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)`)
)

// ResolveSafeSubpath normalizes a dotted or slashed view name into a safe relative subpath
// and rejects directory traversal attempts.
func ResolveSafeSubpath(viewName string) (string, bool) {
	trimmed := strings.TrimSpace(viewName)
	if trimmed == "" || strings.Contains(trimmed, "..") || strings.Contains(trimmed, "\\") || strings.HasPrefix(trimmed, "/") {
		return "", false
	}
	sub := path.Clean(strings.ReplaceAll(trimmed, ".", "/"))
	if sub == "." || sub == ".." || strings.HasPrefix(sub, "../") || strings.HasPrefix(sub, "/") {
		return "", false
	}
	return sub, true
}

// PreprocessOptions customizes how missing or invalid layouts/includes are handled
// during template expansion.
type PreprocessOptions struct {
	FailOnMissingLayout  bool
	AnnotateIncludeError bool
}

// PreprocessDirectives expands @extends, @section/@endsection, @yield, @include,
// strips {{-- ... --}} comments, rewrites @json(...), and normalizes {{ csrf_field() }}.
// It is the canonical preprocessor shared by the interpreter, linter, and native compiler.
func PreprocessDirectives(viewContent string, readView func(name string) (string, bool), opts PreprocessOptions) (string, error) {
	finalHTML := viewContent

	// 1. Handle @extends and @section -> @yield
	if strings.HasPrefix(strings.TrimSpace(viewContent), "@extends") {
		if m := reExtends.FindStringSubmatch(viewContent); len(m) > 1 {
			layoutName := m[1]
			if _, safe := ResolveSafeSubpath(layoutName); !safe {
				if opts.FailOnMissingLayout {
					return "", fmt.Errorf("Layout '%s' inválido", layoutName)
				}
			} else if readView != nil {
				if layoutContent, ok := readView(layoutName); ok {
					sections := make(map[string]string)
					for _, sm := range reSection.FindAllStringSubmatch(viewContent, -1) {
						sections[sm[1]] = sm[2]
					}
					finalHTML = layoutContent
					for secName, secBody := range sections {
						finalHTML = strings.ReplaceAll(finalHTML, fmt.Sprintf("@yield('%s')", secName), secBody)
						finalHTML = strings.ReplaceAll(finalHTML, fmt.Sprintf("@yield(\"%s\")", secName), secBody)
					}
					finalHTML = reYield.ReplaceAllString(finalHTML, "")
				} else if opts.FailOnMissingLayout {
					return "", fmt.Errorf("Layout '%s' no encontrado", layoutName)
				}
			}
		}
	}

	// 2. Expand @include directives (bounded depth to prevent infinite recursion)
	if readView != nil {
		for depth := 0; depth < 16; depth++ {
			m := reInclude.FindStringSubmatch(finalHTML)
			if m == nil {
				break
			}
			fullMatch := m[0]
			incName := m[1]
			if _, safe := ResolveSafeSubpath(incName); !safe {
				replacement := ""
				if opts.AnnotateIncludeError {
					replacement = fmt.Sprintf("<!-- Error: Include '%s' invalid -->", incName)
				}
				finalHTML = strings.Replace(finalHTML, fullMatch, replacement, 1)
				continue
			}
			incContent, ok := readView(incName)
			if !ok {
				if opts.AnnotateIncludeError {
					incContent = fmt.Sprintf("<!-- Error: Include '%s' not found -->", incName)
				} else {
					incContent = ""
				}
			}
			finalHTML = strings.Replace(finalHTML, fullMatch, incContent, 1)
		}
	}

	// 3. Strip Blade comments {{-- ... --}}
	finalHTML = reBladeComments.ReplaceAllString(finalHTML, "")

	// 4. Shared directive rewrite (@json)
	var err error
	finalHTML, err = RewriteJSON(finalHTML)
	if err != nil {
		return finalHTML, err
	}

	// 5. csrf_field() outputs raw HTML input
	finalHTML = reCsrfPre.ReplaceAllString(finalHTML, `{{! csrf_field() }}`)

	return finalHTML, nil
}

// CompileHTMLToJoss translates a preprocessed .joss.html template into canonical Joss source code.
// Both the interpreter (pkg/core) and native compiler (pkg/ir) consume this single function.
func CompileHTMLToJoss(htmlStr string) (string, error) {
	var jossCode strings.Builder

	escapeJossString := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\r", "\\r")
		s = strings.ReplaceAll(s, "${", "\\${")
		return s
	}

	translateExpr := func(expr string) string {
		for reDot.MatchString(expr) {
			expr = reDot.ReplaceAllString(expr, "$$$1->$2")
		}
		return expr
	}

	var compileRange func(str string) string
	compileRange = func(str string) string {
		var code strings.Builder
		i := 0
		n := len(str)

		flushText := func(start, end int) {
			if start >= end {
				return
			}
			txt := str[start:end]
			code.WriteString(fmt.Sprintf("$__output = $__output . \"%s\";\n", escapeJossString(txt)))
		}

		lastIdx := 0

		for i < n {
			// 1. @foreach
			if strings.HasPrefix(str[i:], "@foreach") {
				if m := reForeachStart.FindStringSubmatch(str[i:]); m != nil {
					flushText(lastIdx, i)

					listExpr := translateExpr(m[1])
					firstVar := m[2]
					secondVar := m[3]
					fullMatchLen := len(m[0])

					depth := 1
					j := i + fullMatchLen
					endIdx := -1
					for j < n {
						if strings.HasPrefix(str[j:], "@foreach") {
							depth++
							j += 8
						} else if strings.HasPrefix(str[j:], "@endforeach") {
							depth--
							if depth == 0 {
								endIdx = j
								break
							}
							j += 11
						} else {
							j++
						}
					}

					if endIdx != -1 {
						innerHTML := str[i+fullMatchLen : endIdx]
						compiledInner := compileRange(innerHTML)

						if secondVar != "" {
							code.WriteString(fmt.Sprintf("foreach (%s as $%s => $%s) {\n", listExpr, firstVar, secondVar))
						} else {
							code.WriteString(fmt.Sprintf("foreach (%s as $%s) {\n", listExpr, firstVar))
						}
						code.WriteString(compiledInner)
						code.WriteString("}\n")

						i = endIdx + 11
						lastIdx = i
						continue
					}
				}
			}

			// 2. Block Ternary: {{ (cond) ? { trueBody } : { falseBody } }} or {{ (cond) ? { trueBody } }}
			if i+4 < n && str[i] == '{' && str[i+1] == '{' {
				k := i + 2
				for k < n && (str[k] == ' ' || str[k] == '\t' || str[k] == '\n' || str[k] == '\r') {
					k++
				}
				if k < n && str[k] == '(' {
					condStart := k + 1
					condEnd := findMatchingRunePair(str, k, '(', ')')
					if condEnd != -1 {
						q := condEnd + 1
						for q < n && (str[q] == ' ' || str[q] == '\t' || str[q] == '\n' || str[q] == '\r') {
							q++
						}
						if q < n && str[q] == '?' {
							tbStart := q + 1
							for tbStart < n && (str[tbStart] == ' ' || str[tbStart] == '\t' || str[tbStart] == '\n' || str[tbStart] == '\r') {
								tbStart++
							}
							if tbStart < n && str[tbStart] == '{' {
								tbEnd := findMatchingRunePair(str, tbStart, '{', '}')
								if tbEnd != -1 {
									col := tbEnd + 1
									for col < n && (str[col] == ' ' || str[col] == '\t' || str[col] == '\n' || str[col] == '\r') {
										col++
									}
									if col < n && str[col] == ':' {
										fbStart := col + 1
										for fbStart < n && (str[fbStart] == ' ' || str[fbStart] == '\t' || str[fbStart] == '\n' || str[fbStart] == '\r') {
											fbStart++
										}
										if fbStart < n && str[fbStart] == '{' {
											fbEnd := findMatchingRunePair(str, fbStart, '{', '}')
											if fbEnd != -1 {
												closeBraces := fbEnd + 1
												for closeBraces < n && (str[closeBraces] == ' ' || str[closeBraces] == '\t' || str[closeBraces] == '\n' || str[closeBraces] == '\r') {
													closeBraces++
												}
												if closeBraces+1 < n && str[closeBraces] == '}' && str[closeBraces+1] == '}' {
													flushText(lastIdx, i)

													condExpr := str[condStart:condEnd]
													trueBody := str[tbStart+1 : tbEnd]
													falseBody := str[fbStart+1 : fbEnd]

													code.WriteString(fmt.Sprintf("(%s) ? {\n", translateExpr(condExpr)))
													code.WriteString(compileRange(trueBody))
													code.WriteString("} : {\n")
													code.WriteString(compileRange(falseBody))
													code.WriteString("};\n")

													i = closeBraces + 2
													lastIdx = i
													continue
												}
											}
										}
									} else if col+1 < n && str[col] == '}' && str[col+1] == '}' {
										flushText(lastIdx, i)

										condExpr := str[condStart:condEnd]
										trueBody := str[tbStart+1 : tbEnd]

										code.WriteString(fmt.Sprintf("(%s) ? {\n", translateExpr(condExpr)))
										code.WriteString(compileRange(trueBody))
										code.WriteString("};\n")

										i = col + 2
										lastIdx = i
										continue
									}
								}
							}
						}
					}
				}
			}

			// 3. Raw Expression: {{! expr }}
			if strings.HasPrefix(str[i:], "{{!") {
				if m := reExprRaw.FindStringSubmatch(str[i:]); m != nil {
					flushText(lastIdx, i)
					expr := strings.TrimSpace(m[1])
					fullMatchLen := len(m[0])
					code.WriteString(fmt.Sprintf("$__output = $__output . (%s);\n", translateExpr(expr)))
					i += fullMatchLen
					lastIdx = i
					continue
				}
			}

			// 4. Escaped Expression: {{ expr }}
			if strings.HasPrefix(str[i:], "{{") {
				if m := reExprEscaped.FindStringSubmatch(str[i:]); m != nil {
					flushText(lastIdx, i)
					expr := strings.TrimSpace(m[1])
					fullMatchLen := len(m[0])
					code.WriteString(fmt.Sprintf("$__output = $__output . html_escape(%s);\n", translateExpr(expr)))
					i += fullMatchLen
					lastIdx = i
					continue
				}
			}

			i++
		}

		flushText(lastIdx, n)
		return code.String()
	}

	jossCode.WriteString("var $__output = \"\";\n")
	jossCode.WriteString(compileRange(htmlStr))
	jossCode.WriteString("return $__output;\n")

	return jossCode.String(), nil
}

func findMatchingRunePair(s string, startPos int, startChar, endChar rune) int {
	depth := 0
	for idx, r := range s[startPos:] {
		if r == startChar {
			depth++
		} else if r == endChar {
			depth--
			if depth == 0 {
				return startPos + idx
			}
		}
	}
	return -1
}
