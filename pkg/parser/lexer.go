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

func (l *Lexer) peekAhead(offset int) byte {
	pos := l.readPosition + offset
	if pos >= len(l.input) || pos < 0 {
		return 0
	}
	return l.input[pos]
}

func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			if l.peekChar() == '=' {
				ch2 := l.ch
				l.readChar()
				literal := string(ch) + string(ch2) + string(l.ch)
				tok = Token{Type: STRICT_EQ, Literal: literal, Line: l.line, Column: l.column - 2}
			} else {
				literal := string(ch) + string(l.ch)
				tok = Token{Type: EQ, Literal: literal, Line: l.line, Column: l.column - 1}
			}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: FAT_ARROW, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(ASSIGN, l.ch)
		}
	case ';':
		tok = l.newToken(SEMICOLON, l.ch)
	case '\n':
		tok = l.newToken(NEWLINE, l.ch)
	case '(':
		tok = l.newToken(LPAREN, l.ch)
	case ')':
		tok = l.newToken(RPAREN, l.ch)
	case ',':
		tok = l.newToken(COMMA, l.ch)
	case ':':
		if l.peekChar() == ':' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: DOUBLE_COLON, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(COLON, l.ch)
		}
	case '?':
		if l.peekChar() == '?' {
			ch := l.ch
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				literal := string(ch) + "?" + string(l.ch)
				tok = Token{Type: NULL_COALESCE_ASSIGN, Literal: literal, Line: l.line, Column: l.column - 2}
			} else {
				literal := string(ch) + string(l.ch)
				tok = Token{Type: NULL_COALESCE, Literal: literal, Line: l.line, Column: l.column - 1}
			}
		} else if l.peekChar() == '-' && l.peekAhead(1) == '>' {
			ch := l.ch
			l.readChar() // '-'
			ch2 := l.ch
			l.readChar() // '>'
			literal := string(ch) + string(ch2) + string(l.ch)
			tok = Token{Type: NULL_SAFE_ARROW, Literal: literal, Line: l.line, Column: l.column - 2}
		} else {
			tok = l.newToken(QUESTION, l.ch)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			if l.peekChar() == '=' {
				ch2 := l.ch
				l.readChar()
				literal := string(ch) + string(ch2) + string(l.ch)
				tok = Token{Type: STRICT_NOT_EQ, Literal: literal, Line: l.line, Column: l.column - 2}
			} else {
				literal := string(ch) + string(l.ch)
				tok = Token{Type: NOT_EQ, Literal: literal, Line: l.line, Column: l.column - 1}
			}
		} else {
			tok = l.newToken(BANG, l.ch)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			if l.peekChar() == '>' {
				ch2 := l.ch
				l.readChar()
				literal := string(ch) + string(ch2) + string(l.ch)
				tok = Token{Type: SPACESHIP, Literal: literal, Line: l.line, Column: l.column - 2}
			} else {
				literal := string(ch) + string(l.ch)
				tok = Token{Type: LTE, Literal: literal, Line: l.line, Column: l.column - 1}
			}
		} else if l.peekChar() == '<' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: SHIFT_LEFT, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(LT, l.ch)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: GTE, Literal: literal, Line: l.line, Column: l.column - 1}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: SHIFT_RIGHT, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(GT, l.ch)
		}
	case '+':
		if l.peekChar() == '+' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: INCREMENT, Literal: literal, Line: l.line, Column: l.column - 1}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: PLUS_ASSIGN, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(PLUS, l.ch)
		}
	case '-':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: ARROW, Literal: literal, Line: l.line, Column: l.column - 1}
		} else if l.peekChar() == '-' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: DECREMENT, Literal: literal, Line: l.line, Column: l.column - 1}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: MINUS_ASSIGN, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(MINUS, l.ch)
		}
	case '*':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: ASTERISK_ASSIGN, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(ASTERISK, l.ch)
		}
	case '#':
		// # is treated as a single-line comment (e.g. shebang or Python-style comments)
		l.skipComment()
		return l.NextToken()
	case '/':
		if l.peekChar() == '/' {
			l.skipComment()
			return l.NextToken()
		} else if l.peekChar() == '*' {
			l.readChar() // consume '*'
			l.skipBlockComment()
			return l.NextToken()
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: SLASH_ASSIGN, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(SLASH, l.ch)
		}
	case '%':
		tok = l.newToken(PERCENT, l.ch)
	case '{':
		tok = l.newToken(LBRACE, l.ch)
	case '}':
		tok = l.newToken(RBRACE, l.ch)
	case '[':
		tok = l.newToken(LBRACKET, l.ch)
	case ']':
		tok = l.newToken(RBRACKET, l.ch)
	case '.':
		if l.peekChar() == '.' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: RANGE, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(DOT, l.ch)
		}
	case '$':
		tok = Token{Type: VAR, Literal: "$", Line: l.line, Column: l.column}
	case '"':
		tok.Type = STRING
		tok.Literal = l.readString('"')
		tok.Line = l.line
		tok.Column = l.column
	case '\'':
		tok.Type = STRING
		tok.Literal = l.readString('\'')
		tok.Line = l.line
		tok.Column = l.column
	case 0:
		tok.Literal = ""
		tok.Type = EOF
		tok.Line = l.line
		tok.Column = l.column
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: AND, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(ILLEGAL, l.ch)
		}
	case '|':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: PIPE, Literal: literal, Line: l.line, Column: l.column - 1}
		} else if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = Token{Type: OR, Literal: literal, Line: l.line, Column: l.column - 1}
		} else {
			tok = l.newToken(TYPE_UNION, l.ch)
		}
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = LookupIdent(tok.Literal)
			tok.Line = l.line
			tok.Column = l.column - len(tok.Literal)
			if tok.Column < 1 {
				tok.Column = 1
			}
			return tok
		} else if isDigit(l.ch) {
			tok.Literal = l.readNumber()
			if l.ch == '.' && isDigit(l.peekChar()) {
				l.readChar()
				tok.Literal += "." + l.readNumber()
				tok.Type = FLOAT
			} else {
				tok.Type = INT
			}
			if l.ch == 'm' || l.ch == 'M' {
				tok.Type = DECIMAL
				tok.Literal += string(l.ch)
				l.readChar()
			}
			tok.Line = l.line
			tok.Column = l.column - len(tok.Literal)
			if tok.Column < 1 {
				tok.Column = 1
			}
			return tok
		} else if l.ch > 127 {
			// Skip multi-byte UTF-8 continuation/lead bytes silently
			for l.ch > 127 {
				l.readChar()
			}
			return l.NextToken()
		} else {
			tok = l.newToken(ILLEGAL, l.ch)
		}
	}

	l.readChar()
	return tok
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

func (l *Lexer) newToken(tokenType TokenType, ch byte) Token {
	return Token{Type: tokenType, Literal: string(ch), Line: l.line, Column: l.column}
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
