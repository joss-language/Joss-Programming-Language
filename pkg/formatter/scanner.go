package formatter

import (
	"strings"
	"unicode"

	"github.com/jossecurity/joss/pkg/parser"
)

type TokenKind int

const (
	TokEOF TokenKind = iota
	TokNewline
	TokCommentLine
	TokCommentBlock
	TokIdent
	TokVar
	TokNumber
	TokString
	TokOperator
	TokDelimiter
)

type Token struct {
	Kind TokenKind
	Text string
	Line int
	Col  int
}

type Scanner struct {
	input []rune
	pos   int
	line  int
	col   int
}

func NewScanner(src string) *Scanner {
	// Normalize Windows \r\n to \n
	src = strings.ReplaceAll(src, "\r\n", "\n")
	return &Scanner{
		input: []rune(src),
		pos:   0,
		line:  1,
		col:   1,
	}
}

func (s *Scanner) peek() rune {
	if s.pos >= len(s.input) {
		return 0
	}
	return s.input[s.pos]
}

func (s *Scanner) peekAt(offset int) rune {
	idx := s.pos + offset
	if idx >= len(s.input) || idx < 0 {
		return 0
	}
	return s.input[idx]
}

func (s *Scanner) advance() rune {
	if s.pos >= len(s.input) {
		return 0
	}
	ch := s.input[s.pos]
	s.pos++
	if ch == '\n' {
		s.line++
		s.col = 1
	} else {
		s.col++
	}
	return ch
}

func (s *Scanner) ScanAll() []Token {
	var tokens []Token
	for {
		tok := s.NextToken()
		tokens = append(tokens, tok)
		if tok.Kind == TokEOF {
			break
		}
	}
	return tokens
}

func (s *Scanner) NextToken() Token {
	s.skipSpaces()

	if s.pos >= len(s.input) {
		return Token{Kind: TokEOF, Line: s.line, Col: s.col}
	}

	startLine := s.line
	startCol := s.col
	ch := s.peek()

	// Newline
	if ch == '\n' {
		s.advance()
		return Token{Kind: TokNewline, Text: "\n", Line: startLine, Col: startCol}
	}

	// Line Comment: // or #
	if (ch == '/' && s.peekAt(1) == '/') || ch == '#' {
		var b strings.Builder
		for s.peek() != 0 && s.peek() != '\n' {
			b.WriteRune(s.advance())
		}
		return Token{Kind: TokCommentLine, Text: b.String(), Line: startLine, Col: startCol}
	}

	// Block Comment: /* ... */
	if ch == '/' && s.peekAt(1) == '*' {
		var b strings.Builder
		b.WriteRune(s.advance()) // /
		b.WriteRune(s.advance()) // *
		for s.peek() != 0 {
			if s.peek() == '*' && s.peekAt(1) == '/' {
				b.WriteRune(s.advance()) // *
				b.WriteRune(s.advance()) // /
				break
			}
			b.WriteRune(s.advance())
		}
		return Token{Kind: TokCommentBlock, Text: b.String(), Line: startLine, Col: startCol}
	}

	// Variable: $ident
	if ch == '$' {
		var b strings.Builder
		b.WriteRune(s.advance())
		for isIdentChar(s.peek()) {
			b.WriteRune(s.advance())
		}
		return Token{Kind: TokVar, Text: b.String(), Line: startLine, Col: startCol}
	}

	// String: "..." or '...'
	if ch == '"' || ch == '\'' {
		quote := ch
		var b strings.Builder
		b.WriteRune(s.advance())
		escaped := false
		for s.peek() != 0 {
			c := s.advance()
			b.WriteRune(c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				break
			}
		}
		return Token{Kind: TokString, Text: b.String(), Line: startLine, Col: startCol}
	}

	// Numbers
	if unicode.IsDigit(ch) {
		var b strings.Builder
		for unicode.IsDigit(s.peek()) || s.peek() == '.' {
			if s.peek() == '.' && !unicode.IsDigit(s.peekAt(1)) {
				break // Method call or property or dot operator
			}
			b.WriteRune(s.advance())
		}
		return Token{Kind: TokNumber, Text: b.String(), Line: startLine, Col: startCol}
	}

	// Multi-character operators dynamically retrieved from canonical parser registry
	for _, op := range parser.MultiCharOperators() {
		opRunes := []rune(op)
		if s.pos+len(opRunes) <= len(s.input) {
			match := true
			for i, r := range opRunes {
				if s.input[s.pos+i] != r {
					match = false
					break
				}
			}
			if match {
				for range opRunes {
					s.advance()
				}
				return Token{Kind: TokOperator, Text: op, Line: startLine, Col: startCol}
			}
		}
	}

	// Single-character delimiters and operators
	switch ch {
	case '{', '}', '(', ')', '[', ']', ';', ',', '?', ':':
		s.advance()
		return Token{Kind: TokDelimiter, Text: string(ch), Line: startLine, Col: startCol}
	case '+', '-', '*', '/', '%', '=', '<', '>', '!', '.', '&', '|', '^', '~':
		s.advance()
		return Token{Kind: TokOperator, Text: string(ch), Line: startLine, Col: startCol}
	}

	// Identifiers / Keywords
	if isIdentStart(ch) {
		var b strings.Builder
		for isIdentChar(s.peek()) {
			b.WriteRune(s.advance())
		}
		return Token{Kind: TokIdent, Text: b.String(), Line: startLine, Col: startCol}
	}

	// Unknown character fallback
	s.advance()
	return Token{Kind: TokDelimiter, Text: string(ch), Line: startLine, Col: startCol}
}

func (s *Scanner) skipSpaces() {
	for s.pos < len(s.input) {
		ch := s.input[s.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			s.advance()
		} else {
			break
		}
	}
}

func isIdentStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isIdentChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}
