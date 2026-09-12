package parser

type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
	line         int  // current line number
	column       int  // one-based column of ch
}

func NewLexer(input string) *Lexer {
	// Strip UTF-8 BOM (\xEF\xBB\xBF) if present
	if len(input) >= 3 && input[0] == 0xEF && input[1] == 0xBB && input[2] == 0xBF {
		input = input[3:]
	}
	l := &Lexer{input: input, line: 1}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition += 1
	l.column++
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	startLine, startColumn := l.line, l.column
	switch l.ch {
	case '#':
		l.skipComment()
		return l.NextToken()
	case '/':
		if l.peekChar() == '/' {
			l.skipComment()
			return l.NextToken()
		}
		if l.peekChar() == '*' {
			l.readChar() // consume '*'
			l.skipBlockComment()
			return l.NextToken()
		}
	case '\n':
		l.readChar()
		return Token{Type: NEWLINE, Literal: "\n", Line: startLine, Column: startColumn}
	case '$':
		l.readChar()
		return Token{Type: VAR, Literal: "$", Line: startLine, Column: startColumn}
	case '"':
		literal := l.readString('"')
		l.readChar()
		return Token{Type: STRING, Literal: literal, Line: startLine, Column: startColumn}
	case '\'':
		literal := l.readString('\'')
		l.readChar()
		return Token{Type: STRING, Literal: literal, Line: startLine, Column: startColumn}
	case 0:
		return Token{Type: EOF, Line: startLine, Column: startColumn}
	}

	if isLetter(l.ch) {
		literal := l.readIdentifier()
		return Token{Type: LookupIdent(literal), Literal: literal, Line: startLine, Column: startColumn}
	}
	if isDigit(l.ch) {
		literal := l.readNumber()
		var tokenType TokenType = INT
		if l.ch == '.' && isDigit(l.peekChar()) {
			l.readChar()
			literal += "." + l.readNumber()
			tokenType = FLOAT
		}
		if l.ch == 'm' || l.ch == 'M' {
			tokenType = DECIMAL
			literal += string(l.ch)
			l.readChar()
		}
		return Token{Type: tokenType, Literal: literal, Line: startLine, Column: startColumn}
	}
	if definition, ok := matchSymbolPrefix(l.input[l.position:]); ok {
		for range len(definition.Literal) {
			l.readChar()
		}
		return Token{Type: definition.Token, Literal: definition.Literal, Line: startLine, Column: startColumn}
	}
	if l.ch > 127 {
		for l.ch > 127 {
			l.readChar()
		}
		return l.NextToken()
	}

	literal := string(l.ch)
	l.readChar()
	return Token{Type: ILLEGAL, Literal: literal, Line: startLine, Column: startColumn}
}

func (l *Lexer) skipComment() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) skipBlockComment() {
	// l.ch is currently '*' — skip until */
	for l.ch != 0 {
		l.readChar()
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // consume '*'
			l.readChar() // consume '/'
			return
		}
	}
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch == '@'
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func (l *Lexer) readString(delimiter byte) string {
	var out []byte
	braceDepth := 0
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		if l.ch == delimiter && braceDepth == 0 {
			break
		}

		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'r':
				out = append(out, '\r')
			case '"':
				out = append(out, '"')
			case '\'':
				out = append(out, '\'')
			case '\\':
				out = append(out, '\\')
			default:
				out = append(out, '\\')
				out = append(out, l.ch)
			}
			continue
		}

		if delimiter == '"' {
			if l.ch == '$' && l.peekChar() == '{' {
				out = append(out, '$', '{')
				l.readChar() // consume '{'
				braceDepth++
				continue
			}
			if braceDepth > 0 {
				if l.ch == '{' {
					braceDepth++
				} else if l.ch == '}' {
					braceDepth--
				} else if l.ch == '"' || l.ch == '\'' {
					innerQuote := l.ch
					out = append(out, innerQuote)
					for {
						l.readChar()
						if l.ch == 0 {
							break
						}
						out = append(out, l.ch)
						if l.ch == '\\' {
							l.readChar()
							if l.ch != 0 {
								out = append(out, l.ch)
							}
						} else if l.ch == innerQuote {
							break
						}
					}
					continue
				}
			}
		}

		out = append(out, l.ch)
	}
	return string(out)
}
