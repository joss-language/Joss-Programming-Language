// Package viewtemplate owns the syntax shared by Joss view directives.
// Rendering and lint policy remain in their respective consumers.
package viewtemplate

import (
	"fmt"
	"strings"
	"unicode"
)

// Directive identifies a template directive and its exact source extent.
// Arguments excludes the surrounding parentheses.
type Directive struct {
	Name      string
	Arguments string
	Start     int
	End       int
}

// Scan recognizes @name and @name(...) forms, honoring quoted strings and
// nested parentheses. It deliberately does not interpret Joss expressions.
func Scan(source string) ([]Directive, error) {
	var result []Directive
	for index := 0; index < len(source); index++ {
		if source[index] != '@' {
			continue
		}
		nameStart := index + 1
		nameEnd := nameStart
		for nameEnd < len(source) {
			r := rune(source[nameEnd])
			if r != '_' && !unicode.IsLetter(r) {
				break
			}
			nameEnd++
		}
		if nameEnd == nameStart {
			continue
		}
		directive := Directive{Name: source[nameStart:nameEnd], Start: index, End: nameEnd}
		cursor := nameEnd
		for cursor < len(source) && unicode.IsSpace(rune(source[cursor])) {
			cursor++
		}
		if cursor >= len(source) || source[cursor] != '(' {
			result = append(result, directive)
			index = directive.End - 1
			continue
		}
		argumentStart := cursor + 1
		depth := 1
		quote := byte(0)
		escaped := false
		for cursor = argumentStart; cursor < len(source); cursor++ {
			current := source[cursor]
			if quote != 0 {
				if escaped {
					escaped = false
					continue
				}
				if current == '\\' {
					escaped = true
					continue
				}
				if current == quote {
					quote = 0
				}
				continue
			}
			if current == '\'' || current == '"' {
				quote = current
				continue
			}
			switch current {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					directive.Arguments = source[argumentStart:cursor]
					directive.End = cursor + 1
					result = append(result, directive)
					index = directive.End - 1
					cursor = len(source)
				}
			}
		}
		if depth != 0 {
			return nil, fmt.Errorf("unterminated @%s directive at byte %d", directive.Name, directive.Start)
		}
	}
	return result, nil
}

// RewriteJSON projects @json(expr) to the raw-output expression consumed by
// the existing view compiler. It is shared by runtime and linter.
func RewriteJSON(source string) (string, error) {
	directives, err := Scan(source)
	if err != nil {
		return source, err
	}
	var output strings.Builder
	last := 0
	for _, directive := range directives {
		if directive.Name != "json" || directive.Arguments == "" {
			continue
		}
		output.WriteString(source[last:directive.Start])
		output.WriteString("{{! json_encode(")
		output.WriteString(directive.Arguments)
		output.WriteString(") }}")
		last = directive.End
	}
	if last == 0 {
		return source, nil
	}
	output.WriteString(source[last:])
	return output.String(), nil
}
