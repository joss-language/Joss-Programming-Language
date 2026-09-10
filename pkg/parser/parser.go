package parser

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/diagnostics"
)

const (
	_ int = iota
	LOWEST
	ASSIGNMENT  // =
	TERNARY     // ? :
	COALESCE    // ??
	LOGICAL     // && or ||
	EQUALS      // ==
	LESSGREATER // > or <
	PIPE_OP     // |>
	SUM         // +
	SHIFT       // << or >>
	PRODUCT     // *
	MODULO      // %
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index]
)

var precedences = map[TokenType]int{
	ASSIGN:               ASSIGNMENT,
	PLUS_ASSIGN:          ASSIGNMENT,
	MINUS_ASSIGN:         ASSIGNMENT,
	ASTERISK_ASSIGN:      ASSIGNMENT,
	SLASH_ASSIGN:         ASSIGNMENT,
	NULL_COALESCE_ASSIGN: ASSIGNMENT,
	QUESTION:             TERNARY,
	NULL_COALESCE:        COALESCE,
	PIPE:                 PIPE_OP,
	PLUS:                 SUM,
	MINUS:                SUM,
	DOT:                  SUM,
	RANGE:                LESSGREATER,
	SLASH:                PRODUCT,
	ASTERISK:             PRODUCT,
	PERCENT:              MODULO,
	AND:                  LOGICAL,
	OR:                   LOGICAL,
	LT:                   LESSGREATER,
	GT:                   LESSGREATER,
	EQ:                   EQUALS,
	NOT_EQ:               EQUALS,
	STRICT_EQ:            EQUALS,
	STRICT_NOT_EQ:        EQUALS,
	SPACESHIP:            EQUALS,
	LTE:                  LESSGREATER,
	GTE:                  LESSGREATER,
	IS:                   LESSGREATER,
	INSTANCEOF:           LESSGREATER,
	SHIFT_LEFT:           SHIFT,
	SHIFT_RIGHT:          SHIFT,
	LPAREN:               CALL,
	LBRACKET:             INDEX,
	ARROW:                INDEX,
	NULL_SAFE_ARROW:      INDEX,
	DOUBLE_COLON:         INDEX,
	INCREMENT:            INDEX,
	DECREMENT:            INDEX,
}

type (
	prefixParseFn func() Expression
	infixParseFn  func(Expression) Expression
)

type Parser struct {
	l         *Lexer
	curToken  Token
	peekToken Token
	issues    []diagnostics.Diagnostic

	prefixParseFns map[TokenType]prefixParseFn
	infixParseFns  map[TokenType]infixParseFn
}

func NewParser(l *Lexer) *Parser {
	p := &Parser{
		l:      l,
		issues: []diagnostics.Diagnostic{},
	}

	p.prefixParseFns = make(map[TokenType]prefixParseFn)
	p.registerPrefix(IDENT, p.parseIdentifier)
	p.registerPrefix(VAR, p.parseVarExpression) // Handle $name
	p.registerPrefix(INT, p.parseIntegerLiteral)
	p.registerPrefix(FLOAT, p.parseFloatLiteral)
	p.registerPrefix(DECIMAL, p.parseDecimalLiteral)
	p.registerPrefix(STRING, p.parseStringLiteral)
	p.registerPrefix(TRUE, p.parseBoolean)
	p.registerPrefix(FALSE, p.parseBoolean)
	p.registerPrefix(NULL, p.parseNullLiteral)
	p.registerPrefix(NIL, p.parseNullLiteral)
	p.registerPrefix(LPAREN, p.parseGroupedExpression)
	p.registerPrefix(LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(LBRACE, p.parseBraceExpression) // Maps { key: val } or Blocks { stmt; }
	p.registerPrefix(NEW, p.parseNewExpression)
	p.registerPrefix(THIS, p.parseIdentifier)
	p.registerPrefix(ISSET, p.parseIssetExpression)
	p.registerPrefix(EMPTY, p.parseEmptyExpression)
	p.registerPrefix(BANG, p.parsePrefixExpression)
	p.registerPrefix(MINUS, p.parsePrefixExpression)
	p.registerPrefix(FUNCTION, p.parseFunctionLiteral)
	p.registerPrefix(MATCH, p.parseMatchExpression)
	p.registerPrefix(ASYNC, p.parseAsyncExpression)
	p.registerPrefix(REF, p.parseReferenceExpression)
	p.registerPrefix(ELLIPSIS, p.parseSpreadExpression)
	p.registerPrefix(YIELD, p.parseYieldExpression)

	p.infixParseFns = make(map[TokenType]infixParseFn)
	p.registerInfix(PLUS, p.parseInfixExpression)
	p.registerInfix(MINUS, p.parseInfixExpression)
	p.registerInfix(SLASH, p.parseInfixExpression)
	p.registerInfix(ASTERISK, p.parseInfixExpression)
	p.registerInfix(PERCENT, p.parseInfixExpression)
	p.registerInfix(AND, p.parseInfixExpression)
	p.registerInfix(OR, p.parseInfixExpression)
	p.registerInfix(LT, p.parseInfixExpression)
	p.registerInfix(GT, p.parseInfixExpression)
	p.registerInfix(EQ, p.parseInfixExpression)
	p.registerInfix(NOT_EQ, p.parseInfixExpression)
	p.registerInfix(STRICT_EQ, p.parseInfixExpression)
	p.registerInfix(STRICT_NOT_EQ, p.parseInfixExpression)
	p.registerInfix(SPACESHIP, p.parseInfixExpression)
	p.registerInfix(LTE, p.parseInfixExpression)
	p.registerInfix(GTE, p.parseInfixExpression)
	p.registerInfix(IS, p.parseIsExpression)
	p.registerInfix(INSTANCEOF, p.parseIsExpression)
	p.registerInfix(SHIFT_LEFT, p.parseInfixExpression)
	p.registerInfix(SHIFT_RIGHT, p.parseInfixExpression)
	p.registerInfix(PIPE, p.parseInfixExpression)
	p.registerInfix(DOT, p.parseInfixExpression)
	p.registerInfix(LPAREN, p.parseCallExpression)
	p.registerInfix(QUESTION, p.parseTernaryExpression)
	p.registerInfix(NULL_COALESCE, p.parseInfixExpression)
	p.registerInfix(LBRACKET, p.parseIndexExpression)
	p.registerInfix(ARROW, p.parseMemberExpression)
	p.registerInfix(NULL_SAFE_ARROW, p.parseMemberExpression)
	p.registerInfix(DOUBLE_COLON, p.parseMemberExpression)
	p.registerInfix(ASSIGN, p.parseAssignExpression)
	p.registerInfix(PLUS_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(MINUS_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(ASTERISK_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(SLASH_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(NULL_COALESCE_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(INCREMENT, p.parsePostfixExpression)
	p.registerInfix(DECREMENT, p.parsePostfixExpression)
	p.registerInfix(RANGE, p.parseInfixExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *Program {
	program := &Program{}
	program.Statements = []Statement{}

	for p.curToken.Type != EOF {
		if p.curToken.Type == NEWLINE {
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}
	return program
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) noPrefixParseFnError(t TokenType) {
	if msg, removed := removedKeywordMessage(p.curToken); removed {
		p.addError(p.curToken, msg)
		return
	}
	lit := p.curToken.Literal
	if lit == "if" || lit == "else" || lit == "elif" {
		msg := fmt.Sprintf("La estructura '%s' no existe en Joss. Usa ternarias ($cond ? $a : $b) o 'match ($val) { ... }'.", lit)
		p.addError(p.curToken, msg)
		return
	}
	if lit == "switch" {
		msg := "La estructura 'switch' no existe en Joss. Usa 'match ($val) { ... }' en su lugar."
		p.addError(p.curToken, msg)
		return
	}
	if lit == "for" {
		msg := "El bucle 'for' no existe en Joss. Usa 'foreach ($array as $item)' o 'while ($cond) { ... }' en su lugar."
		p.addError(p.curToken, msg)
		return
	}
	msg := fmt.Sprintf("Sintaxis no válida o token '%s' inesperado.", p.curToken.Literal)
	p.addError(p.curToken, msg)
}

func (p *Parser) expectPeek(t TokenType) bool {
	if p.peekToken.Type == t {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t TokenType) {
	msg := fmt.Sprintf("Se esperaba el token %s; se encontró %s.", t, p.peekToken.Type)
	p.addError(p.peekToken, msg)
}

func (p *Parser) Errors() []string {
	errors := make([]string, 0, len(p.issues))
	for _, issue := range p.issues {
		errors = append(errors, issue.Message)
	}
	return errors
}

// Diagnostics returns the parser's canonical structured findings. File is
// intentionally empty because the parser consumes text; project loaders add it.
func (p *Parser) Diagnostics() []diagnostics.Diagnostic {
	result := make([]diagnostics.Diagnostic, len(p.issues))
	copy(result, p.issues)
	return result
}

func (p *Parser) addError(token Token, message string) {
	endColumn := token.Column
	if endColumn > 0 && token.Literal != "" {
		endColumn += len([]rune(token.Literal))
	}
	p.issues = append(p.issues, diagnostics.Diagnostic{
		Code:     "JOSS-PARSE-001",
		Severity: diagnostics.SeverityError,
		Message:  message,
		Range: diagnostics.Range{
			Start: diagnostics.Position{Line: token.Line, Column: token.Column},
			End:   diagnostics.Position{Line: token.Line, Column: endColumn},
		},
	})
}

func (p *Parser) peekTokenIs(t TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) curTokenIs(t TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) registerPrefix(tokenType TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}
