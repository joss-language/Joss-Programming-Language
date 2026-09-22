package parser

import "strings"

func isTypeStart(token Token) bool {
	return token.Type == IDENT || token.Type == NULL || token.Type == NIL
}

func isTypeContinuation(tokenType TokenType) bool {
	return tokenType == TYPE_UNION || tokenType == QUESTION || tokenType == LT
}

// parseTypeReference consumes a source type from curToken. Union types use
// A|B and nullable shorthand uses T?. Generic collections use array<T> and
// map<K, V>. The AST stores normalized spellings like T|null and array<T>.
func (p *Parser) parseTypeReference() Token {
	token := p.curToken
	first := p.parseSingleType()
	parts := []string{first}
	for p.peekToken.Type == TYPE_UNION {
		p.nextToken()
		if !isTypeStart(p.peekToken) {
			p.addError(p.peekToken, "Se esperaba un tipo después de `|`.")
			break
		}
		p.nextToken()
		parts = append(parts, p.parseSingleType())
	}
	if p.peekToken.Type == QUESTION {
		p.nextToken()
		parts = append(parts, "null")
	}
	token.Literal = strings.Join(parts, "|")
	return token
}

func (p *Parser) parseSingleType() string {
	name := p.curToken.Literal
	if p.peekToken.Type == LT {
		p.nextToken() // consume '<'
		genericParts := []string{}
		for p.peekToken.Type != GT && p.peekToken.Type != SHIFT_RIGHT && p.peekToken.Type != EOF {
			p.nextToken()
			if isTypeStart(p.curToken) {
				genericParts = append(genericParts, p.parseTypeReference().Literal)
			}
			if p.peekToken.Type == COMMA {
				p.nextToken() // consume ','
			}
		}
		if p.peekToken.Type == GT {
			p.nextToken() // consume '>'
		} else if p.peekToken.Type == SHIFT_RIGHT {
			p.curToken = Token{Type: GT, Literal: ">", Line: p.peekToken.Line, Column: p.peekToken.Column}
			p.peekToken = Token{Type: GT, Literal: ">", Line: p.peekToken.Line, Column: p.peekToken.Column + 1}
		} else {
			p.addError(p.peekToken, "Se esperaba `>` para cerrar el tipo genérico.")
		}
		name = name + "<" + strings.Join(genericParts, ", ") + ">"
	}
	return name
}
