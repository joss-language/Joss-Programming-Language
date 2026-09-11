package formatter

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

type Formatter struct {
	tokens           []Token
	astInfo          *ASTInfo
	pos              int
	indent           int
	out              strings.Builder
	tokensOnLine     int
	consecLines      int
	prevTok          Token
	lineEndedByToken bool
}

func FormatSource(src string) (string, error) {
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return "", fmt.Errorf("syntax error: %s", p.Errors()[0])
	}

	astInfo := AnalyzeAST(prog)
	scanner := NewScanner(src)
	tokens := scanner.ScanAll()

	f := &Formatter{
		tokens:  tokens,
		astInfo: astInfo,
		pos:     0,
		indent:  0,
	}

	formatted := f.formatTokens()

	// Safety Guard: verify that formatted code parses cleanly with 0 syntax errors
	pVerify := parser.NewParser(parser.NewLexer(formatted))
	_ = pVerify.ParseProgram()
	if len(pVerify.Errors()) > 0 {
		return "", fmt.Errorf("formatter safety guard: generated invalid syntax: %s", pVerify.Errors()[0])
	}

	return formatted, nil
}

func (f *Formatter) peek() Token {
	if f.pos >= len(f.tokens) {
		return Token{Kind: TokEOF}
	}
	return f.tokens[f.pos]
}

func (f *Formatter) peekAt(offset int) Token {
	idx := f.pos + offset
	if idx >= len(f.tokens) || idx < 0 {
		return Token{Kind: TokEOF}
	}
	return f.tokens[idx]
}

func (f *Formatter) advance() Token {
	if f.pos >= len(f.tokens) {
		return Token{Kind: TokEOF}
	}
	tok := f.tokens[f.pos]
	f.pos++
	return tok
}

func (f *Formatter) writeIndent(curr Token) {
	if f.tokensOnLine == 0 {
		effectiveIndent := f.indent
		if curr.Text == "->" || curr.Text == "?->" {
			effectiveIndent++
		}
		if effectiveIndent > 0 {
			f.out.WriteString(strings.Repeat("    ", effectiveIndent))
		}
	}
}

func (f *Formatter) writeNewline() {
	f.out.WriteRune('\n')
	f.tokensOnLine = 0
	f.consecLines++
}

func (f *Formatter) formatTokens() string {
	var inGenericBrackets int // tracking <T> in types
	var blockDepth int        // tracking nesting inside { ... }
	var ternaryPending int

	for f.pos < len(f.tokens) {
		tok := f.advance()

		if tok.Kind == TokEOF {
			break
		}

		// Handle Newlines from original source
		if tok.Kind == TokNewline {
			if f.lineEndedByToken {
				// The previous token already emitted the newline for this line.
				f.lineEndedByToken = false
				continue
			}
			if f.tokensOnLine > 0 {
				f.writeNewline()
				continue
			}
			// Empty / blank line in source: preserve at most 1 blank line (max 2 consecutive newlines)
			if f.consecLines < 2 && f.out.Len() > 0 {
				f.writeNewline()
			}
			continue
		}

		// Handle Comments
		if tok.Kind == TokCommentLine || tok.Kind == TokCommentBlock {
			f.writeIndent(tok)
			if f.tokensOnLine > 0 {
				f.out.WriteRune(' ')
			}
			f.out.WriteString(tok.Text)
			f.tokensOnLine++
			if tok.Kind == TokCommentLine {
				f.writeNewline()
				f.lineEndedByToken = true
			}
			f.prevTok = tok
			continue
		}

		// Block closing: }
		if tok.Kind == TokDelimiter && tok.Text == "}" {
			if f.indent > 0 {
				f.indent--
			}
			if blockDepth > 0 {
				blockDepth--
			}
			if f.tokensOnLine > 0 {
				f.writeNewline()
			}
			f.writeIndent(tok)
			f.out.WriteString("}")
			f.tokensOnLine++
			f.consecLines = 0

			// If followed by : { (ternary/guard block false branch), stay on same line
			next := f.peek()
			if next.Kind == TokDelimiter && next.Text == ":" && f.peekAt(1).Kind == TokDelimiter && f.peekAt(1).Text == "{" {
				f.out.WriteString(" : {")
				f.advance() // :
				f.advance() // {
				f.indent++
				blockDepth++
				f.writeNewline()
				f.prevTok = Token{Kind: TokDelimiter, Text: "{"}
				f.lineEndedByToken = true
				if ternaryPending > 0 {
					ternaryPending--
				}
				continue
			}

			// If followed by catch, stay on same line or space
			if next.Kind == TokIdent && next.Text == "catch" {
				f.out.WriteRune(' ')
			} else if next.Kind != TokDelimiter || (next.Text != ";" && next.Text != "," && next.Text != ")") {
				f.writeNewline()
				f.lineEndedByToken = true
			}
			f.prevTok = tok
			continue
		}

		// Block or Map opening: {
		if tok.Kind == TokDelimiter && tok.Text == "{" {
			isBlock := f.isBlockOpen(tok)
			f.writeIndent(tok)
			if f.tokensOnLine > 0 {
				f.out.WriteRune(' ')
			}
			f.out.WriteString("{")
			f.tokensOnLine++
			f.prevTok = tok
			if isBlock {
				f.indent++
				blockDepth++
				f.consecLines = 0
				f.writeNewline()
				f.lineEndedByToken = true
			} else {
				// Map or object literal inline
				f.consecLines = 0
			}
			continue
		}

		// Statement terminator: ;
		if tok.Kind == TokDelimiter && tok.Text == ";" {
			f.out.WriteString(";")
			f.tokensOnLine++
			f.consecLines = 0
			f.writeNewline()
			f.prevTok = tok
			f.lineEndedByToken = true
			ternaryPending = 0
			continue
		}

		// Comma: ,
		if tok.Kind == TokDelimiter && tok.Text == "," {
			f.out.WriteString(",")
			f.tokensOnLine++
			f.prevTok = tok
			f.lineEndedByToken = false
			continue
		}

		// Track ternaries
		wasInTernary := ternaryPending > 0
		if tok.Kind == TokDelimiter && tok.Text == "?" {
			ternaryPending++
		}
		if tok.Kind == TokDelimiter && tok.Text == ":" && ternaryPending > 0 {
			ternaryPending--
		}

		// Ensure indentation at start of line
		f.writeIndent(tok)

		// Generic bracket tracking: array<int>, map<string, User>
		if tok.Kind == TokOperator && tok.Text == "<" {
			if f.prevTok.Kind == TokIdent && (f.prevTok.Text == "array" || f.prevTok.Text == "map") {
				inGenericBrackets++
				f.out.WriteString("<")
				f.tokensOnLine++
				f.prevTok = tok
				f.consecLines = 0
				f.lineEndedByToken = false
				continue
			}
		}
		if tok.Kind == TokOperator && tok.Text == ">" && inGenericBrackets > 0 {
			inGenericBrackets--
			f.out.WriteString(">")
			f.tokensOnLine++
			f.prevTok = tok
			f.consecLines = 0
			f.lineEndedByToken = false
			continue
		}

		// Spacing before token (only if not first token on line)
		if f.tokensOnLine > 0 {
			if shouldSpaceBefore(tok, f.prevTok, f.peek(), inGenericBrackets, wasInTernary) {
				f.out.WriteRune(' ')
			}
		}

		f.out.WriteString(tok.Text)
		f.tokensOnLine++
		f.consecLines = 0
		f.prevTok = tok
		f.lineEndedByToken = false
	}

	result := f.out.String()
	trimmed := strings.TrimRight(result, " \t\n\r")
	if len(trimmed) > 0 {
		return trimmed + "\n"
	}
	return ""
}

func (f *Formatter) isBlockOpen(tok Token) bool {
	if f.astInfo != nil {
		if f.astInfo.IsBlock(tok.Line, tok.Col) {
			return true
		}
		if f.astInfo.IsMap(tok.Line, tok.Col) {
			return false
		}
	}
	if f.prevTok.Text == "return" {
		return false
	}
	switch f.prevTok.Text {
	case ")", ":", "?", "do", "try", "=>":
		return true
	}
	return parser.IsDeclarationKeyword(f.prevTok.Text) || f.prevTok.Kind == TokIdent
}

func shouldSpaceBefore(curr, prev, next Token, inGenerics int, inTernary bool) bool {
	if prev.Kind == TokDelimiter {
		if prev.Text == "(" || prev.Text == "[" || prev.Text == "{" {
			return false
		}
		if prev.Text == "," || prev.Text == ";" || prev.Text == ":" || prev.Text == "?" {
			return true
		}
	}

	if curr.Kind == TokDelimiter {
		if curr.Text == "(" {
			if prev.Kind == TokIdent {
				if parser.IsControlKeyword(prev.Text) || prev.Text == "if" {
					return true
				}
				return false
			}
			if prev.Kind == TokVar {
				return false
			}
		}
		if curr.Text == "," || curr.Text == ";" || curr.Text == ")" || curr.Text == "]" {
			return false
		}
		if curr.Text == ":" {
			if next.Kind == TokDelimiter && next.Text == "{" {
				return true
			}
			if inTernary {
				return true
			}
			if prev.Kind == TokVar || prev.Kind == TokNumber {
				return true
			}
			return false
		}
		if curr.Text == "{" {
			return true
		}
		if curr.Text == "?" {
			return true
		}
	}

	// Operators
	if curr.Kind == TokOperator {
		if curr.Text == "->" || curr.Text == "?->" || curr.Text == "::" || curr.Text == "++" || curr.Text == "--" || curr.Text == ".." {
			return false
		}
		if inGenerics > 0 && (curr.Text == "<" || curr.Text == ">") {
			return false
		}
		if curr.Text == "!" {
			return false
		}
		// Type union: User|null, string|int
		if curr.Text == "|" && prev.Kind == TokIdent && next.Kind == TokIdent {
			return false
		}
		return true
	}

	if prev.Kind == TokOperator {
		if prev.Text == "->" || prev.Text == "?->" || prev.Text == "::" || prev.Text == "!" || prev.Text == ".." {
			return false
		}
		if inGenerics > 0 && (prev.Text == "<" || prev.Text == ">") {
			return false
		}
		// Type union: User|null
		if prev.Text == "|" && curr.Kind == TokIdent {
			return false
		}
		return true
	}

	return true
}

func FormatFile(filePath string, write bool) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	formatted, err := FormatSource(string(data))
	if err != nil {
		return false, err
	}

	changed := formatted != string(data)
	if changed && write {
		if err := os.WriteFile(filePath, []byte(formatted), 0644); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func FormatDirectory(root string, write bool, check bool) ([]string, error) {
	var unformatted []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
			changed, formatErr := FormatFile(path, write)
			if formatErr != nil {
				return fmt.Errorf("error formatting %s: %w", path, formatErr)
			}
			if changed {
				unformatted = append(unformatted, path)
			}
		}
		return nil
	})

	return unformatted, err
}
