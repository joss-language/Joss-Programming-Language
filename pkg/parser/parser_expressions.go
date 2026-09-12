package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

func (p *Parser) parseExpression(precedence int) Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for {
		if p.peekTokenIs(SEMICOLON) || p.peekTokenIs(EOF) {
			return leftExp
		}

		if isExpressionContinuation(p.curToken.Type) {
			infix := p.infixParseFns[p.curToken.Type]
			if infix != nil {
				leftExp = infix(leftExp)
				continue
			}
		}

		if p.peekTokenIs(NEWLINE) {
			p.nextToken()
			if !isExpressionContinuation(p.peekToken.Type) {
				return leftExp
			}
			continue
		}

		if p.curToken.Type == NEWLINE {
			if !isExpressionContinuation(p.peekToken.Type) {
				return leftExp
			}
			p.nextToken()
			continue
		}

		if precedence >= p.peekPrecedence() {
			return leftExp
		}

		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

}

func isExpressionContinuation(t TokenType) bool {
	switch t {
	case ARROW, DOUBLE_COLON, DOT, LBRACKET, QUESTION, NULL_COALESCE,
		PLUS, MINUS, SLASH, ASTERISK, PERCENT, AND, OR, EQ, NOT_EQ, STRICT_EQ, STRICT_NOT_EQ, SPACESHIP, LT, GT, LTE, GTE,
		SHIFT_LEFT, SHIFT_RIGHT, PIPE:
		return true
	}
	return false
}

func (p *Parser) parseIdentifier() Expression {
	return &Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseFunctionLiteral() Expression {
	lit := &FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()
	lit.ReturnType = p.parseOptionalReturnType()

	if !p.expectPeek(LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

func (p *Parser) parseOptionalReturnType() Token {
	if p.peekToken.Type != COLON {
		return Token{}
	}
	p.nextToken() // ':'
	p.nextToken()
	if !isTypeStart(p.curToken) {
		p.addError(p.curToken, "Se esperaba un tipo de retorno después de `:`.")
		return Token{}
	}
	return p.parseTypeReference()
}

func (p *Parser) parseVarExpression() Expression {
	// Current token is VAR ($)
	// We expect next to be IDENT or THIS
	if p.peekToken.Type == THIS {
		p.nextToken()
		return &Identifier{Token: p.curToken, Value: "this"}
	}
	if !p.expectPeek(IDENT) {
		return nil
	}
	// Now curToken is IDENT
	return &Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() Expression {
	lit := &IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.addError(p.curToken, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() Expression {
	lit := &FloatLiteral{Token: p.curToken}

	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as float", p.curToken.Literal)
		p.addError(p.curToken, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseDecimalLiteral() Expression {
	lit := &DecimalLiteral{Token: p.curToken}

	raw := strings.TrimRight(p.curToken.Literal, "mM")
	value, err := decimal.NewFromString(raw)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as decimal", p.curToken.Literal)
		p.addError(p.curToken, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() Expression {
	if strings.Contains(p.curToken.Literal, "${") {
		return p.parseInterpolatedString(p.curToken)
	}
	return &StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBoolean() Expression {
	return &Boolean{Token: p.curToken, Value: p.curToken.Type == TRUE}
}

func (p *Parser) parseNullLiteral() Expression {
	return &NullLiteral{Token: p.curToken}
}

func (p *Parser) parseGroupedExpression() Expression {
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parseArrayLiteral() Expression {
	arrayToken := p.curToken

	for p.peekTokenIs(NEWLINE) {
		p.nextToken()
	}

	if p.peekTokenIs(RBRACKET) {
		p.nextToken()
		return &ArrayLiteral{Token: arrayToken, Elements: []Expression{}}
	}

	p.nextToken()
	firstExpr := p.parseExpression(LOWEST)
	if firstExpr == nil {
		return nil
	}

	// Check if this is an associative array (map): [ key => val, ... ]
	if p.peekTokenIs(FAT_ARROW) {
		p.nextToken() // consume =>
		p.nextToken() // start of val
		firstVal := p.parseExpression(LOWEST)
		if firstVal == nil {
			return nil
		}

		mapLit := &MapLiteral{Token: arrayToken, Pairs: make(map[Expression]Expression)}
		mapLit.Pairs[firstExpr] = firstVal

		for !p.peekTokenIs(RBRACKET) && !p.peekTokenIs(EOF) {
			if p.peekTokenIs(NEWLINE) {
				p.nextToken()
				continue
			}
			if p.peekTokenIs(COMMA) {
				p.nextToken()
				for p.peekTokenIs(NEWLINE) {
					p.nextToken()
				}
				if p.peekTokenIs(RBRACKET) {
					break
				}
				p.nextToken()
				key := p.parseExpression(LOWEST)
				if key == nil {
					return nil
				}
				if !p.expectPeek(FAT_ARROW) {
					return nil
				}
				p.nextToken()
				val := p.parseExpression(LOWEST)
				if val == nil {
					return nil
				}
				mapLit.Pairs[key] = val
			} else {
				break
			}
		}

		for p.peekTokenIs(NEWLINE) {
			p.nextToken()
		}

		if !p.expectPeek(RBRACKET) {
			return nil
		}
		return mapLit
	}

	// Otherwise, standard ArrayLiteral
	elements := []Expression{firstExpr}
	for p.peekTokenIs(COMMA) || p.peekTokenIs(NEWLINE) {
		if p.peekTokenIs(NEWLINE) {
			p.nextToken()
			if p.peekTokenIs(RBRACKET) {
				break
			}
			if p.peekTokenIs(COMMA) {
				p.nextToken()
			} else {
				continue
			}
		} else {
			p.nextToken()
		}

		for p.peekTokenIs(NEWLINE) {
			p.nextToken()
		}

		if p.peekTokenIs(RBRACKET) {
			break
		}

		p.nextToken()
		elem := p.parseExpression(LOWEST)
		if elem != nil {
			if p.peekTokenIs(FAT_ARROW) {
				mapLit := &MapLiteral{Token: arrayToken, Pairs: make(map[Expression]Expression)}
				for _, prev := range elements {
					if spread, ok := prev.(*SpreadExpression); ok {
						mapLit.Pairs[spread] = nil
					}
				}
				p.nextToken() // consume =>
				p.nextToken() // move to val
				val := p.parseExpression(LOWEST)
				if val == nil {
					return nil
				}
				mapLit.Pairs[elem] = val

				for !p.peekTokenIs(RBRACKET) && !p.peekTokenIs(EOF) {
					if p.peekTokenIs(NEWLINE) {
						p.nextToken()
						continue
					}
					if p.peekTokenIs(COMMA) {
						p.nextToken()
						for p.peekTokenIs(NEWLINE) {
							p.nextToken()
						}
						if p.peekTokenIs(RBRACKET) {
							break
						}
						p.nextToken()
						key := p.parseExpression(LOWEST)
						if key == nil {
							return nil
						}
						if spread, ok := key.(*SpreadExpression); ok {
							mapLit.Pairs[spread] = nil
							continue
						}
						if !p.expectPeek(FAT_ARROW) {
							return nil
						}
						p.nextToken()
						v := p.parseExpression(LOWEST)
						if v == nil {
							return nil
						}
						mapLit.Pairs[key] = v
					} else {
						break
					}
				}

				for p.peekTokenIs(NEWLINE) {
					p.nextToken()
				}

				if !p.expectPeek(RBRACKET) {
					return nil
				}
				return mapLit
			}
			elements = append(elements, elem)
		}
	}

	for p.peekTokenIs(NEWLINE) {
		p.nextToken()
	}

	if !p.expectPeek(RBRACKET) {
		return nil
	}

	return &ArrayLiteral{Token: arrayToken, Elements: elements}
}

func (p *Parser) parseBraceExpression() Expression {
	block := &BlockStatement{Token: p.curToken, Statements: []Statement{}}
	p.nextToken() // consume LBRACE

	// Check for empty
	if p.curToken.Type == RBRACE {
		// Empty {} -> Map (standard convention in dynamic langs)
		return &MapLiteral{Token: p.curToken, Pairs: make(map[Expression]Expression)}
	}

	// Parse first statement
	// Handle NEWLINEs
	for p.curToken.Type == NEWLINE {
		p.nextToken()
	}
	if p.curToken.Type == RBRACE {
		return &MapLiteral{Token: p.curToken, Pairs: make(map[Expression]Expression)}
	}

	firstStmt := p.parseStatement()

	// If the first statement is NOT an ExpressionStatement, it's definitely a Block.
	// e.g. { return 1; } or { if ... }
	exprStmt, isExpr := firstStmt.(*ExpressionStatement)
	if !isExpr {
		// It's a block. Continue parsing statements.
		if firstStmt != nil {
			block.Statements = append(block.Statements, firstStmt)
		}
		p.nextToken()
		for p.curToken.Type != RBRACE && p.curToken.Type != EOF {
			if p.curToken.Type == NEWLINE {
				p.nextToken()
				continue
			}
			stmt := p.parseStatement()
			if stmt != nil {
				block.Statements = append(block.Statements, stmt)
			}
			p.nextToken()
		}
		return &BlockExpression{Token: block.Token, Block: block}
	}

	if p.peekToken.Type == COLON {
		// It's a Map!
		// Convert firstStmt to Key.
		mapLit := &MapLiteral{Token: block.Token, Pairs: make(map[Expression]Expression)}
		key := exprStmt.Expression

		p.nextToken() // curToken is :
		p.nextToken() // curToken is start of value

		val := p.parseExpression(LOWEST)
		mapLit.Pairs[key] = val

		// Continue parsing map
		for !p.peekTokenIs(RBRACE) {
			if p.peekTokenIs(NEWLINE) {
				p.nextToken()
			}
			if p.peekTokenIs(COMMA) {
				p.nextToken()
			}
			if p.peekTokenIs(NEWLINE) {
				p.nextToken()
			}
			if p.peekTokenIs(RBRACE) {
				break
			}

			p.nextToken() // start of next key
			key := p.parseExpression(LOWEST)

			if p.peekTokenIs(NEWLINE) {
				p.nextToken()
			}

			if !p.expectPeek(COLON) {
				return nil
			}
			p.nextToken()
			val := p.parseExpression(LOWEST)
			mapLit.Pairs[key] = val
		}

		if !p.expectPeek(RBRACE) {
			return nil
		}
		return mapLit
	}

	// Not a map. It's a Block.
	if firstStmt != nil {
		block.Statements = append(block.Statements, firstStmt)
	}
	p.nextToken() // Move past the last token of first statement

	for p.curToken.Type != RBRACE && p.curToken.Type != EOF {
		if p.curToken.Type == NEWLINE {
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return &BlockExpression{Token: block.Token, Block: block}
}

func (p *Parser) parseBlockExpression() *BlockExpression {
	block := p.parseBlockStatement()
	return &BlockExpression{Token: block.Token, Block: block}
}

func isStatementStart(t TokenType) bool {
	switch t {
	case RETURN, VAR, FOREACH, WHILE, DO, TRY, THROW, ECHO, PRINT:
		return true
	}
	return false
}

func (p *Parser) parseExpressionList(end TokenType) []Expression {
	list := []Expression{}

	// Allow newline before first element
	for p.peekToken.Type == NEWLINE {
		p.nextToken()
	}

	if p.peekToken.Type == end {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	for p.peekToken.Type == COMMA || p.peekToken.Type == NEWLINE {
		if p.peekToken.Type == NEWLINE {
			p.nextToken()
			// Check if we hit end after newline
			if p.peekToken.Type == end {
				break
			}
			// If no comma after newline, check if next is comma or continue
			if p.peekToken.Type == COMMA {
				p.nextToken()
			} else {
				// Optional comma if newline present?
				// For now, let's assume comma is required unless we want to support newline as separator
				// Let's enforce comma for now, but skip multiple newlines
				// If we are here, we consumed a NEWLINE.
				// If next is not COMMA, and not END, it might be syntax error or optional comma.
				// Let's check if it's start of expression.
				// If so, we treat newline as separator?
				// Joss syntax usually requires comma.
				// Let's just consume newlines and expect comma.
				continue
			}
		} else {
			// It is COMMA
			p.nextToken()
		}

		// Allow newlines after comma
		for p.peekToken.Type == NEWLINE {
			p.nextToken()
		}

		if p.peekToken.Type == end {
			break
		}

		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseIndexExpression(left Expression) Expression {
	exp := &IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()

	// Check for empty index: []
	if p.curToken.Type == RBRACKET {
		exp.Index = nil
		return exp
	}

	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parsePrefixExpression() Expression {
	expression := &PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseInfixExpression(left Expression) Expression {
	expression := &InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	if expression.Operator == "." {
		if call, ok := expression.Right.(*CallExpression); ok {
			funcName := call.Function.String()
			if isBlueprintMethod(funcName) {
				if ident, isIdent := expression.Left.(*Identifier); isIdent && strings.HasPrefix(ident.Value, "$") {
					msg := fmt.Sprintf("Uso de '.' sospechoso para llamar al método '%s'. El acceso a métodos de objetos o mapas usa '->' (ej. $objeto->%s()).", funcName, funcName)
					p.addError(p.curToken, msg)
				}
			}
		}
	}

	return expression
}

func isBlueprintMethod(name string) bool {
	switch name {
	case "id", "string", "text", "integer", "tinyInteger", "smallInteger", "mediumInteger", "bigInteger",
		"unsignedInteger", "unsignedBigInteger", "float", "double", "decimal", "char", "mediumText",
		"longText", "date", "dateTime", "time", "timestamp", "timestamps", "softDeletes", "boolean",
		"json", "enum", "increments", "bigIncrements", "unique", "nullable", "unsigned", "default",
		"comment", "foreign", "references", "on", "onDelete", "onUpdate", "dropColumn":
		return true
	}
	return false
}

func (p *Parser) parseTernaryExpression(condition Expression) Expression {
	expression := &TernaryExpression{
		Token:     p.curToken,
		Condition: condition,
	}

	p.nextToken() // Consume ?

	if p.curToken.Type == COLON {
		// Elvis Operator: condition ?: falsePart
		// True part is implicitly the condition (evaluated once)
		expression.True = nil // Will handle in Evaluator
		p.nextToken()         // Consume :
		expression.False = p.parseExpression(LOWEST)
		return expression
	}

	// Standard or Single-branch Ternary: condition ? truePart [: falsePart]
	expression.True = p.parseExpression(LOWEST)

	if p.peekToken.Type == COLON {
		p.nextToken() // curToken is now COLON
		p.nextToken() // curToken is now start of false expression
		expression.False = p.parseExpression(LOWEST)
	}

	return expression
}

func (p *Parser) parseCallExpression(function Expression) Expression {
	exp := &CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseCallArguments()
	return exp
}

func (p *Parser) parseCallArguments() []Expression {
	args := []Expression{}

	// Allow newline before first argument
	for p.peekToken.Type == NEWLINE {
		p.nextToken()
	}

	if p.peekToken.Type == RPAREN {
		p.nextToken()
		return args
	}

	p.nextToken()
	args = append(args, p.parseArgumentExpression())

	for p.peekToken.Type == COMMA || p.peekToken.Type == NEWLINE {
		if p.peekToken.Type == NEWLINE {
			p.nextToken()
			continue
		}

		if p.peekToken.Type == COMMA {
			p.nextToken() // consume ','
			// Allow newline after comma
			for p.peekToken.Type == NEWLINE {
				p.nextToken()
			}
			if p.peekToken.Type == RPAREN {
				break
			}
			p.nextToken() // Advance to start of expression
			args = append(args, p.parseArgumentExpression())
		}
	}

	if !p.expectPeek(RPAREN) {
		return nil
	}

	return args
}

func (p *Parser) parseArgumentExpression() Expression {
	if (p.curToken.Type == IDENT || p.curToken.Type == VAR) && p.peekToken.Type == COLON {
		name := strings.TrimPrefix(p.curToken.Literal, "$")
		tok := p.curToken
		p.nextToken() // consume name, curToken is now COLON
		p.nextToken() // consume COLON, curToken is now start of expression
		val := p.parseExpression(LOWEST)
		return &NamedArgument{Token: tok, Name: name, Value: val}
	}
	return p.parseExpression(LOWEST)
}

func (p *Parser) parseFunctionParameters() []*Parameter {
	parameters := []*Parameter{}

	// Allow newline before first parameter
	for p.peekToken.Type == NEWLINE {
		p.nextToken()
	}

	if p.peekToken.Type == RPAREN {
		p.nextToken()
		return parameters
	}

	p.nextToken()

	param := p.parseParameter()
	if param != nil {
		parameters = append(parameters, param)
	}

	for p.peekToken.Type == COMMA || p.peekToken.Type == NEWLINE {
		if p.peekToken.Type == NEWLINE {
			p.nextToken()
			continue
		}
		if p.peekToken.Type == COMMA {
			p.nextToken() // consume ','
			// Allow newlines after comma
			for p.peekToken.Type == NEWLINE {
				p.nextToken()
			}
			if p.peekToken.Type == RPAREN {
				break // trailing comma
			}
			p.nextToken()
			param := p.parseParameter()
			if param != nil {
				parameters = append(parameters, param)
			}
		}
	}

	if !p.expectPeek(RPAREN) {
		return nil
	}

	return parameters
}

func (p *Parser) parseParameter() *Parameter {
	param := &Parameter{}
	if p.curToken.Type == PUBLIC || p.curToken.Type == PROTECTED || p.curToken.Type == PRIVATE {
		param.Visibility = p.curToken
		p.nextToken()
	}
	if p.curToken.Type == CONST {
		param.IsConst = true
		if param.Visibility.Literal == "" {
			param.Visibility = Token{Type: PUBLIC, Literal: "public"}
		}
		p.nextToken()
	}
	if p.curToken.Type == REF {
		param.ByReference = true
		p.nextToken()
	}

	// Optional type: T $value, T|null $value or T? $value.
	if isTypeStart(p.curToken) && (p.peekToken.Type == VAR || isTypeContinuation(p.peekToken.Type)) {
		param.Type = p.parseTypeReference()
		if !p.expectPeek(VAR) {
			return nil
		}
	}

	if p.curToken.Type == VAR {
		if !p.expectPeek(IDENT) {
			return nil
		}
		param.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

		// Optional default value: $code = 200, $name = "Guest"
		if p.peekToken.Type == ASSIGN {
			p.nextToken() // consume '='
			p.nextToken() // move to default expression
			param.DefaultValue = p.parseExpression(LOWEST)
		}
	} else {
		// Fallback for syntax errors, we expect VAR
		return nil
	}

	return param
}

func (p *Parser) parseReferenceExpression() Expression {
	expression := &ReferenceExpression{Token: p.curToken}
	p.nextToken()
	expression.Target = p.parseExpression(PREFIX)
	return expression
}

func (p *Parser) parseNewExpression() Expression {
	exp := &NewExpression{Token: p.curToken}

	if !p.expectPeek(IDENT) {
		return nil
	}
	exp.Class = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	exp.Arguments = p.parseCallArguments()

	return exp
}

func (p *Parser) parseMemberExpression(left Expression) Expression {
	exp := &MemberExpression{
		Token:    p.curToken,
		Left:     left,
		NullSafe: p.curToken.Type == NULL_SAFE_ARROW,
	}

	if isIdentifierOrKeyword(p.peekToken.Type) {
		p.nextToken()
		exp.Property = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
		return exp
	}

	p.expectPeek(IDENT)
	return nil
}

func (p *Parser) parseAssignExpression(left Expression) Expression {
	exp := &AssignExpression{Token: p.curToken, Left: left}

	p.nextToken()
	exp.Value = p.parseExpression(LOWEST)

	return exp
}

func (p *Parser) parseCompoundAssignExpression(left Expression) Expression {
	compoundToken := p.curToken
	var op string
	switch compoundToken.Type {
	case PLUS_ASSIGN:
		op = "+"
	case MINUS_ASSIGN:
		op = "-"
	case ASTERISK_ASSIGN:
		op = "*"
	case SLASH_ASSIGN:
		op = "/"
	case NULL_COALESCE_ASSIGN:
		op = "??"
	default:
		op = "+"
	}

	p.nextToken()
	right := p.parseExpression(LOWEST)

	infix := &InfixExpression{
		Token:    Token{Type: TokenType(op), Literal: op, Line: compoundToken.Line, Column: compoundToken.Column},
		Left:     cloneTargetExpression(left),
		Operator: op,
		Right:    right,
	}

	return &AssignExpression{
		Token: Token{Type: ASSIGN, Literal: "=", Line: compoundToken.Line, Column: compoundToken.Column},
		Left:  left,
		Value: infix,
	}
}

func cloneTargetExpression(expr Expression) Expression {
	switch e := expr.(type) {
	case *Identifier:
		return &Identifier{Token: e.Token, Value: e.Value}
	case *MemberExpression:
		return &MemberExpression{
			Token:    e.Token,
			Left:     cloneTargetExpression(e.Left),
			Property: &Identifier{Token: e.Property.Token, Value: e.Property.Value},
		}
	case *IndexExpression:
		return &IndexExpression{
			Token: e.Token,
			Left:  cloneTargetExpression(e.Left),
			Index: e.Index,
		}
	default:
		return e
	}
}

func (p *Parser) parseIssetExpression() Expression {
	exp := &IssetExpression{Token: p.curToken}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	exp.Arguments = p.parseCallArguments()

	return exp
}

func (p *Parser) parseEmptyExpression() Expression {
	exp := &EmptyExpression{Token: p.curToken}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	p.nextToken()
	exp.Argument = p.parseExpression(LOWEST)

	if !p.expectPeek(RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parsePostfixExpression(left Expression) Expression {
	return &PostfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}
}

func (p *Parser) parseMatchExpression() Expression {
	exp := &MatchExpression{Token: p.curToken}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	p.nextToken()
	exp.Subject = p.parseExpression(LOWEST)

	if !p.expectPeek(RPAREN) {
		return nil
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	p.nextToken()

	exp.Arms = []MatchArm{}

	for !p.curTokenIs(RBRACE) && !p.curTokenIs(EOF) {
		if p.curTokenIs(NEWLINE) {
			p.nextToken()
			continue
		}

		var keys []Expression
		for {
			if p.curTokenIs(NEWLINE) {
				p.nextToken()
				continue
			}

			if p.curTokenIs(DEFAULT) {
				keys = append(keys, &Identifier{Token: p.curToken, Value: "default"})
			} else {
				keyExp := p.parseExpression(LOWEST)
				if keyExp == nil {
					return nil
				}
				keys = append(keys, keyExp)
			}

			if p.peekTokenIs(COMMA) {
				p.nextToken() // curToken is COMMA
				p.nextToken() // curToken is start of next key
				continue
			}
			break
		}

		if !p.expectPeek(FAT_ARROW) {
			return nil
		}

		p.nextToken() // move past FAT_ARROW

		valueExp := p.parseExpression(LOWEST)
		if valueExp == nil {
			return nil
		}

		isDefault := false
		for _, k := range keys {
			if ident, ok := k.(*Identifier); ok && ident.Value == "default" && ident.Token.Type == DEFAULT {
				isDefault = true
				break
			}
		}

		arm := MatchArm{
			Keys:      keys,
			IsDefault: isDefault,
			Value:     valueExp,
		}
		exp.Arms = append(exp.Arms, arm)

		if p.peekTokenIs(COMMA) {
			p.nextToken() // curToken is COMMA
		}

		p.nextToken() // move to next token (NEWLINE, RBRACE, or next arm)
	}

	if p.curTokenIs(RBRACE) {
		return exp
	}

	return nil
}

func isIdentifierOrKeyword(t TokenType) bool {
	if t == IDENT {
		return true
	}
	switch t {
	case FUNCTION, VAR, TRUE, FALSE, RETURN, PRINT, ECHO, CLASS, INIT,
		NEW, FOREACH, AS, THIS, ISSET, EMPTY, BREAK,
		CONTINUE, WHILE, DO, TRY, CATCH, THROW, EXTENDS, IF, ELSE, MATCH, DEFAULT, ASYNC,
		INTERFACE, IMPLEMENTS, ABSTRACT, ENUM, CASE, IS, INSTANCEOF, SELECT, YIELD:
		return true
	}
	return false
}

func (p *Parser) parseAsyncExpression() Expression {
	tok := p.curToken
	if p.peekToken.Type == LBRACE {
		p.nextToken() // move to {
		block := p.parseBlockStatement()
		fn := &FunctionLiteral{
			Token:      Token{Type: FUNCTION, Literal: "func", Line: tok.Line},
			Parameters: []*Parameter{},
			Body:       block,
		}
		return &CallExpression{
			Token:     Token{Type: IDENT, Literal: "async", Line: tok.Line},
			Function:  &Identifier{Token: Token{Type: IDENT, Literal: "async", Line: tok.Line}, Value: "async"},
			Arguments: []Expression{fn},
		}
	}
	if p.peekToken.Type == LPAREN {
		msg := "'async' requiere la sintaxis de bloque 'async { ... }'; 'async(func() ...)' fue eliminado."
		p.addError(tok, msg)
		return nil
	}
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	return &CallExpression{
		Token:     Token{Type: IDENT, Literal: "async", Line: tok.Line},
		Function:  &Identifier{Token: Token{Type: IDENT, Literal: "async", Line: tok.Line}, Value: "async"},
		Arguments: []Expression{exp},
	}
}

func (p *Parser) parseIsExpression(left Expression) Expression {
	expr := &IsExpression{
		Token: p.curToken,
		Left:  left,
	}

	if !isTypeStart(p.peekToken) {
		p.addError(p.peekToken, fmt.Sprintf("Se esperaba un tipo después de `%s`.", expr.Token.Literal))
		return expr
	}

	p.nextToken()
	expr.TargetType = p.parseTypeReference()
	return expr
}

func (p *Parser) parseSpreadExpression() Expression {
	tok := p.curToken
	p.nextToken()
	val := p.parseExpression(CALL)
	return &SpreadExpression{
		Token:      tok,
		Expression: val,
	}
}

func (p *Parser) parseYieldExpression() Expression {
	tok := p.curToken
	if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE || p.peekToken.Type == RBRACE || p.peekToken.Type == EOF {
		return &YieldExpression{Token: tok}
	}
	p.nextToken()
	val := p.parseExpression(LOWEST)
	if p.peekToken.Type == FAT_ARROW {
		p.nextToken() // consume =>
		p.nextToken()
		value := p.parseExpression(LOWEST)
		return &YieldExpression{
			Token: tok,
			Key:   val,
			Value: value,
		}
	}
	return &YieldExpression{
		Token: tok,
		Value: val,
	}
}
