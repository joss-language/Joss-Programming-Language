package parser

import "fmt"

func (p *Parser) parseStatement() Statement {
	if msg, removed := removedKeywordMessage(p.curToken); removed {
		p.addError(p.curToken, msg)
		for p.curToken.Type != SEMICOLON && p.curToken.Type != NEWLINE && p.curToken.Type != EOF {
			p.nextToken()
		}
		return nil
	}
	// Syntax Guard: Intercept unsupported control flow keywords from other languages
	lit := p.curToken.Literal
	if lit == "if" || lit == "else" || lit == "elif" {
		p.addError(p.curToken, fmt.Sprintf("La estructura '%s' no existe en Joss. Para condicionales utiliza expresiones ternarias ($cond ? $val1 : $val2) o 'match ($val) { ... }'.", lit))
		return nil
	}
	if lit == "switch" {
		p.addError(p.curToken, "La estructura 'switch' no existe en Joss. Utiliza 'match ($val) { case $x: ... default: ... }' en su lugar.")
		return nil
	}
	if lit == "for" {
		p.addError(p.curToken, "El bucle 'for' no existe en Joss. Utiliza 'foreach ($array as $item)' o 'while ($cond) { ... }' en su lugar.")
		return nil
	}

	// Modifiers before class, interface, enum, function or property: public, private, protected, static, abstract
	if p.curToken.Type == PUBLIC || p.curToken.Type == PRIVATE || p.curToken.Type == PROTECTED || p.curToken.Type == STATIC || p.curToken.Type == ABSTRACT {
		vis := ""
		isStatic := false
		isAbstract := false

		for {
			if p.curToken.Type == PUBLIC || p.curToken.Type == PRIVATE || p.curToken.Type == PROTECTED {
				vis = p.curToken.Literal
			} else if p.curToken.Type == STATIC {
				isStatic = true
			} else if p.curToken.Type == ABSTRACT {
				isAbstract = true
			}
			if p.peekToken.Type == PUBLIC || p.peekToken.Type == PRIVATE || p.peekToken.Type == PROTECTED || p.peekToken.Type == STATIC || p.peekToken.Type == ABSTRACT {
				p.nextToken()
			} else {
				break
			}
		}

		if isStatic && vis == "" {
			p.addError(p.curToken, "`static` no implica visibilidad; escribe `public static`, `protected static` o `private static`.")
		}

		if p.peekToken.Type == CLASS {
			if vis == "protected" {
				p.addError(p.curToken, "Una clase de proyecto sólo puede ser `public` o `private`; `protected` se reserva para miembros.")
			}
			p.nextToken() // move to CLASS
			classStmt := p.parseClassStatement()
			if classStmt != nil {
				classStmt.Visibility = vis
				classStmt.IsAbstract = isAbstract
			}
			return classStmt
		}

		if p.peekToken.Type == ENUM {
			if vis == "protected" {
				p.addError(p.curToken, "Un enum sólo puede ser `public` o `private`; `protected` se reserva para miembros.")
			}
			p.nextToken() // move to ENUM
			enumStmt := p.parseEnumStatement()
			if enumStmt != nil {
				enumStmt.Visibility = vis
			}
			return enumStmt
		}

		if p.peekToken.Type == INTERFACE {
			if vis == "protected" {
				p.addError(p.curToken, "Una interfaz sólo puede ser `public` o `private`; `protected` se reserva para miembros.")
			}
			p.nextToken() // move to INTERFACE
			ifaceStmt := p.parseInterfaceStatement()
			if ifaceStmt != nil {
				ifaceStmt.Visibility = vis
			}
			return ifaceStmt
		}

		if p.peekToken.Type == FUNCTION {
			if vis == "protected" {
				p.addError(p.curToken, "Una función global sólo puede ser `public` o `private`; `protected` se reserva para miembros.")
			}
			p.nextToken() // move to FUNCTION
			methodStmt := p.parseMethodStatement(isAbstract)
			if methodStmt != nil {
				methodStmt.Visibility = vis
				methodStmt.IsStatic = isStatic
				methodStmt.IsAbstract = isAbstract
			}
			return methodStmt
		}

		if p.peekToken.Type == CONST {
			p.nextToken()
			constant := p.parseConstStatement()
			if declaration, ok := constant.(*LetStatement); ok {
				declaration.Visibility = vis
				declaration.IsStatic = isStatic
			}
			return constant
		}

		// Property: public $x = 1, private string $y
		if p.peekToken.Type == VAR {
			p.nextToken() // move to VAR
			name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
			var value Expression
			if p.peekToken.Type == ASSIGN {
				p.nextToken()
				p.nextToken()
				value = p.parseExpression(LOWEST)
			}
			stmt := &LetStatement{
				Token:      Token{Type: IDENT, Literal: "mixed", Line: p.curToken.Line},
				Name:       name,
				Value:      value,
				Visibility: vis,
				IsStatic:   isStatic,
			}
			if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
				p.nextToken()
			}
			return stmt
		}

		if p.peekToken.Type == IDENT {
			p.nextToken() // move to type (e.g. string)
			typeTok := p.curToken
			letStmt := p.parseLetStatement()
			if ls, ok := letStmt.(*LetStatement); ok {
				ls.Visibility = vis
				ls.IsStatic = isStatic
				return ls
			}
			if mls, ok := letStmt.(*MultiLetStatement); ok {
				mls.Visibility = vis
				mls.IsStatic = isStatic
				return mls
			}
			return &LetStatement{Token: typeTok, Visibility: vis, IsStatic: isStatic}
		}
	}

	if p.curToken.Type == CLASS {
		p.addError(p.curToken, "Las clases requieren visibilidad explícita: `public class`, `protected class` o `private class`.")
		return p.parseClassStatement()
	}
	if p.curToken.Type == ENUM {
		p.addError(p.curToken, "Los enums requieren visibilidad explícita: `public enum` o `private enum`.")
		return p.parseEnumStatement()
	}
	if p.curToken.Type == INTERFACE {
		p.addError(p.curToken, "Las interfaces requieren visibilidad explícita: `public interface` o `private interface`.")
		return p.parseInterfaceStatement()
	}
	if p.curToken.Type == INIT {
		return p.parseInitStatement()
	}
	if p.curToken.Type == FOREACH {
		return p.parseForeachStatement()
	}
	if p.curToken.Type == FUNCTION {
		p.addError(p.curToken, "Las funciones con nombre requieren visibilidad explícita: `public func`, `protected func` o `private func`.")
		return p.parseMethodStatement()
	}
	if p.curToken.Type == ECHO || p.curToken.Type == PRINT {
		return p.parseEchoStatement()
	}

	if p.curToken.Type == WHILE {
		return p.parseWhileStatement()
	}
	if p.curToken.Type == DO {
		return p.parseDoWhileStatement()
	}
	if p.curToken.Type == TRY {
		return p.parseTryCatchStatement()
	}
	if p.curToken.Type == THROW {
		return p.parseThrowStatement()
	}
	if p.curToken.Type == RETURN {
		return p.parseReturnStatement()
	}
	if p.curToken.Type == BREAK {
		return p.parseBreakStatement()
	}
	if p.curToken.Type == CONTINUE {
		return p.parseContinueStatement()
	}
	if p.curToken.Type == DEFER {
		return p.parseDeferStatement()
	}
	if p.curToken.Type == ASYNC {
		return p.parseAsyncStatement()
	}
	if p.curToken.Type == SELECT {
		return p.parseSelectStatement()
	}
	if p.curToken.Type == CONST {
		return p.parseConstStatement()
	}
	// Check for 'let' keyword variable declaration: let int $x = 10, let $x = 10
	if p.curToken.Type == LET || p.curToken.Literal == "let" {
		if p.peekToken.Type == IDENT {
			p.nextToken() // move to type (e.g. int, string)
			return p.parseLetStatement()
		}
		if p.peekToken.Type == VAR {
			typeTok := Token{Type: IDENT, Literal: "mixed", Line: p.curToken.Line}
			p.nextToken() // move to VAR ($)
			if !p.expectPeek(IDENT) {
				return nil
			}
			name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
			var value Expression
			if p.peekToken.Type == ASSIGN {
				p.nextToken()
				p.nextToken()
				value = p.parseExpression(LOWEST)
			}
			stmt := &LetStatement{Token: typeTok, Name: name, Value: value}
			if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
				p.nextToken()
			}
			return stmt
		}
	}
	// Check for typed variable declaration: type $name = value
	if isTypeStart(p.curToken) && (p.peekToken.Type == VAR || isTypeContinuation(p.peekToken.Type)) {
		return p.parseLetStatement()
	}
	// Check for Increment: $i++
	// $ is VAR, i is IDENT, ++ is INCREMENT
	// But parseStatement starts at current token.
	// If current is VAR ($), it might be expression statement or assignment.
	// If current is IDENT (variable name after $), wait.
	// In Joss, variables start with $.
	// So `$i` is `VAR` then `IDENT`.
	// `parseExpressionStatement` handles `$i`.
	// `parseExpression` handles `$i` (Identifier).
	// Then `parseExpressionStatement` checks for semicolon.
	// But `++` is postfix?
	// If we have `$i++`, `parseExpression` reads `$i`.
	// Then `peekToken` is `++`.
	// If `++` is registered as infix (postfix), it works.
	// But `++` is usually a statement or expression.
	// Let's register `++` as a postfix operator in `parser.go`?
	// Or handle it here.

	// If we register `++` as infix with high precedence, `$i ++` becomes `InfixExpression($i, ++, nil)`? No.
	// Postfix is `Left -> Operator`.
	// We don't have postfix support in `parser.go` loop yet.
	// Let's add it to `parseExpression` loop or handle as statement.

	// Simpler: Handle as statement if it appears at top level.
	// But it can be used in expression: `$x = $i++`.
	// So it must be an expression.

	// I need to register INCREMENT as a POSTFIX operator in `parser.go`.
	// `parser.go` loop: `infix := p.infixParseFns[p.peekToken.Type]`
	// I can register `INCREMENT` as infix, and the parse function will return `PostfixExpression`.

	return p.parseExpressionStatement()
}

func (p *Parser) parseConstStatement() Statement {
	constToken := p.curToken
	typeToken := Token{Type: IDENT, Literal: "var", Line: constToken.Line, Column: constToken.Column}
	if p.peekToken.Type == IDENT {
		p.nextToken()
		typeToken = p.parseTypeReference()
	}
	if !p.expectPeek(VAR) || !p.expectPeek(IDENT) {
		return nil
	}
	name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(ASSIGN) {
		p.addError(constToken, "Una constante requiere un inicializador.")
		return nil
	}
	p.nextToken()
	value := p.parseExpression(LOWEST)
	if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
		p.nextToken()
	}
	return &LetStatement{Token: typeToken, Name: name, Value: value, IsConst: true}
}

func (p *Parser) parseReturnStatement() *ReturnStatement {
	stmt := &ReturnStatement{Token: p.curToken}

	p.nextToken()

	if p.curToken.Type == SEMICOLON {
		return stmt
	}

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekToken.Type == SEMICOLON {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseBreakStatement() *BreakStatement {
	stmt := &BreakStatement{Token: p.curToken}

	if p.peekToken.Type == SEMICOLON {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseContinueStatement() *ContinueStatement {
	stmt := &ContinueStatement{Token: p.curToken}

	if p.peekToken.Type == SEMICOLON {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseClassStatement() *ClassStatement {
	stmt := &ClassStatement{Token: p.curToken}

	if !p.expectPeek(IDENT) {
		return nil
	}

	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekToken.Type == EXTENDS {
		p.nextToken()
		p.nextToken()
		stmt.SuperClass = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if p.peekToken.Type == IMPLEMENTS {
		p.nextToken() // consume IMPLEMENTS
		for {
			if !p.expectPeek(IDENT) {
				return nil
			}
			stmt.Interfaces = append(stmt.Interfaces, &Identifier{Token: p.curToken, Value: p.curToken.Literal})
			if p.peekToken.Type == COMMA {
				p.nextToken() // consume COMMA
			} else {
				break
			}
		}
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.Body = p.parseClassBody()

	return stmt
}

func (p *Parser) parseInterfaceStatement() *InterfaceStatement {
	stmt := &InterfaceStatement{Token: p.curToken}

	if !p.expectPeek(IDENT) {
		return nil
	}

	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekToken.Type == EXTENDS {
		p.nextToken() // consume EXTENDS
		for {
			if !p.expectPeek(IDENT) {
				return nil
			}
			stmt.Extends = append(stmt.Extends, &Identifier{Token: p.curToken, Value: p.curToken.Literal})
			if p.peekToken.Type == COMMA {
				p.nextToken() // consume COMMA
			} else {
				break
			}
		}
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.Methods = p.parseInterfaceBody()

	return stmt
}

func (p *Parser) parseInterfaceBody() []*MethodStatement {
	methods := []*MethodStatement{}

	p.nextToken() // move past {

	for p.curToken.Type != RBRACE && p.curToken.Type != EOF {
		if p.curToken.Type == NEWLINE || p.curToken.Type == SEMICOLON {
			p.nextToken()
			continue
		}

		vis := "public"
		if p.curToken.Type == PUBLIC || p.curToken.Type == PRIVATE || p.curToken.Type == PROTECTED {
			vis = p.curToken.Literal
			if vis != "public" {
				p.addError(p.curToken, "Los métodos de una interfaz deben ser `public`.")
			}
			p.nextToken()
		}

		if p.curToken.Type != FUNCTION {
			p.addError(p.curToken, "Una interfaz sólo puede contener declaraciones de métodos (`public func ...`).")
			p.nextToken()
			continue
		}

		stmt := &MethodStatement{Token: p.curToken, Visibility: vis}

		if !p.expectPeek(IDENT) {
			return methods
		}
		stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

		if !p.expectPeek(LPAREN) {
			return methods
		}

		stmt.Parameters = p.parseFunctionParameters()
		stmt.ReturnType = p.parseOptionalReturnType()

		// Interface methods must NOT have a body.
		if p.peekToken.Type == LBRACE {
			p.addError(p.peekToken, "Los métodos de una interfaz no pueden tener cuerpo `{ ... }`.")
			p.nextToken()
			p.parseBlockStatement()
		} else if p.peekToken.Type == SEMICOLON {
			p.nextToken() // consume ;
		}

		methods = append(methods, stmt)
		p.nextToken()
	}

	return methods
}

func (p *Parser) parseClassBody() *BlockStatement {
	block := &BlockStatement{Token: p.curToken}
	block.Statements = []Statement{}

	p.nextToken()

	for p.curToken.Type != RBRACE && p.curToken.Type != EOF {
		if p.curToken.Type == NEWLINE {
			p.nextToken()
			continue
		}

		var stmt Statement
		if p.curToken.Type == PUBLIC || p.curToken.Type == PRIVATE || p.curToken.Type == PROTECTED || p.curToken.Type == STATIC || p.curToken.Type == ABSTRACT {
			vis := ""
			isStatic := false
			isAbstract := false

			for {
				if p.curToken.Type == PUBLIC || p.curToken.Type == PRIVATE || p.curToken.Type == PROTECTED {
					vis = p.curToken.Literal
				} else if p.curToken.Type == STATIC {
					isStatic = true
				} else if p.curToken.Type == ABSTRACT {
					isAbstract = true
				}
				if p.peekToken.Type == PUBLIC || p.peekToken.Type == PRIVATE || p.peekToken.Type == PROTECTED || p.peekToken.Type == STATIC || p.peekToken.Type == ABSTRACT {
					p.nextToken()
				} else {
					break
				}
			}

			if isStatic && vis == "" {
				p.addError(p.curToken, "`static` no implica visibilidad; escribe `public static`, `protected static` o `private static`.")
			}

			if p.peekToken.Type == FUNCTION {
				p.nextToken() // move to FUNCTION
				mStmt := p.parseMethodStatement(isAbstract)
				if mStmt != nil {
					mStmt.Visibility = vis
					mStmt.IsStatic = isStatic
					mStmt.IsAbstract = isAbstract
					stmt = mStmt
				}
			} else if p.peekToken.Type == CONST {
				p.nextToken()
				stmt = p.parseConstStatement()
				if declaration, ok := stmt.(*LetStatement); ok {
					declaration.Visibility = vis
					declaration.IsStatic = isStatic
				}
			} else if p.peekToken.Type == VAR {
				p.nextToken() // move to VAR
				name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
				var value Expression
				if p.peekToken.Type == ASSIGN {
					p.nextToken()
					p.nextToken()
					value = p.parseExpression(LOWEST)
				}
				stmt = &LetStatement{
					Token:      Token{Type: IDENT, Literal: "mixed", Line: p.curToken.Line},
					Name:       name,
					Value:      value,
					Visibility: vis,
					IsStatic:   isStatic,
				}
				if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
					p.nextToken()
				}
			} else if p.peekToken.Type == IDENT {
				p.nextToken() // move to type (e.g. string)
				letStmt := p.parseLetStatement()
				if ls, ok := letStmt.(*LetStatement); ok {
					ls.Visibility = vis
					ls.IsStatic = isStatic
					stmt = ls
				} else if mls, ok := letStmt.(*MultiLetStatement); ok {
					mls.Visibility = vis
					mls.IsStatic = isStatic
					stmt = mls
				}
			}
		} else if p.curToken.Type == FUNCTION {
			p.addError(p.curToken, "Los métodos requieren visibilidad explícita: `public func`, `protected func` o `private func`.")
			stmt = p.parseMethodStatement()
		} else if p.curToken.Type == INIT {
			stmt = p.parseInitStatement()
		} else if p.curToken.Type == CONST {
			p.addError(p.curToken, "Las propiedades constantes requieren visibilidad explícita: `public const`, `protected const` o `private const`.")
			stmt = p.parseConstStatement()
		} else if (p.curToken.Type == LET || p.curToken.Literal == "let") && p.peekToken.Type == IDENT {
			p.addError(p.curToken, "Las propiedades requieren visibilidad explícita antes de `let`.")
			p.nextToken()
			stmt = p.parseLetStatement()
		} else if (p.curToken.Type == LET || p.curToken.Literal == "let") && p.peekToken.Type == VAR {
			p.addError(p.curToken, "Las propiedades requieren visibilidad explícita antes de `let`.")
			typeTok := Token{Type: IDENT, Literal: "mixed", Line: p.curToken.Line}
			p.nextToken()
			name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
			var value Expression
			if p.peekToken.Type == ASSIGN {
				p.nextToken()
				p.nextToken()
				value = p.parseExpression(LOWEST)
			}
			stmt = &LetStatement{
				Token:      typeTok,
				Name:       name,
				Value:      value,
				Visibility: "",
			}
			if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
				p.nextToken()
			}
		} else if isTypeStart(p.curToken) && (p.peekToken.Type == VAR || isTypeContinuation(p.peekToken.Type)) { // Property: string $x
			p.addError(p.curToken, "Las propiedades requieren visibilidad explícita: `public`, `protected` o `private`.")
			stmt = p.parseLetStatement()
		} else if p.curToken.Type == VAR { // Property without type: $x = 10
			p.addError(p.curToken, "Las propiedades requieren visibilidad explícita: `public`, `protected` o `private`.")
			name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
			var value Expression
			if p.peekToken.Type == ASSIGN {
				p.nextToken()
				p.nextToken()
				value = p.parseExpression(LOWEST)
			}
			stmt = &LetStatement{
				Token:      Token{Type: IDENT, Literal: "mixed", Line: p.curToken.Line},
				Name:       name,
				Value:      value,
				Visibility: "",
			}
			if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
				p.nextToken()
			}
		} else {
			// Skip or error? For now skip to avoid infinite loop if unknown
			p.nextToken()
			continue
		}

		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseInitStatement() *InitStatement {
	stmt := &InitStatement{Token: p.curToken}

	if p.peekToken.Type == IDENT {
		p.nextToken()
		stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	} else if p.peekToken.Type == LPAREN {
		stmt.Name = &Identifier{Token: p.curToken, Value: "Init"}
	} else {
		p.expectPeek(IDENT)
		return nil
	}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	// Parse parameters
	stmt.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Token: p.curToken}
	block.Statements = []Statement{}

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

	return block
}

func (p *Parser) parseLetStatement() Statement {
	typeToken := p.parseTypeReference()

	if !p.expectPeek(VAR) {
		return nil
	}

	if !p.expectPeek(IDENT) {
		return nil
	}

	name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	var value Expression

	if p.peekToken.Type == ASSIGN {
		p.nextToken()
		p.nextToken()
		value = p.parseExpression(LOWEST)
	}

	if p.peekToken.Type == COMMA {
		return p.parseMultiLetStatement(typeToken, name, value)
	}

	stmt := &LetStatement{Token: typeToken, Name: name, Value: value}

	if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
		p.nextToken()
	}

	return stmt
}

// parseMultiLetStatement parses: type $a[=val],$b[=val],...
// Call only when a COMMA is detected after the first var in a LetStatement.
func (p *Parser) parseMultiLetStatement(typeToken Token, firstName *Identifier, firstValue Expression) *MultiLetStatement {
	stmt := &MultiLetStatement{TypeToken: typeToken}

	// Add first declaration
	stmt.Declarations = append(stmt.Declarations, SingleDecl{Name: firstName, Value: firstValue})

	// Consume each comma-separated declaration
	for p.peekToken.Type == COMMA {
		p.nextToken() // consume COMMA
		if !p.expectPeek(VAR) {
			break
		}
		if !p.expectPeek(IDENT) {
			break
		}
		name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
		var value Expression
		if p.peekToken.Type == ASSIGN {
			p.nextToken() // consume =
			p.nextToken() // move to value
			value = p.parseExpression(LOWEST)
		}
		stmt.Declarations = append(stmt.Declarations, SingleDecl{Name: name, Value: value})
	}

	if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpressionStatement() *ExpressionStatement {
	stmt := &ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)

	if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseForeachStatement() *ForeachStatement {
	stmt := &ForeachStatement{Token: p.curToken}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	p.nextToken()
	stmt.Iterable = p.parseExpression(LOWEST)

	if !p.expectPeek(AS) {
		return nil
	}

	// Expect variable: $val or $key => $val
	// In parser, VAR is '$', then IDENT
	if !p.expectPeek(VAR) {
		return nil
	}
	if !p.expectPeek(IDENT) {
		return nil
	}
	firstVar := p.curToken.Literal

	if p.peekTokenIs(FAT_ARROW) {
		p.nextToken() // consume =>
		if !p.expectPeek(VAR) {
			return nil
		}
		if !p.expectPeek(IDENT) {
			return nil
		}
		stmt.Key = firstVar
		stmt.Value = p.curToken.Literal
	} else {
		stmt.Value = firstVar
	}

	if !p.expectPeek(RPAREN) {
		return nil
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseDeferStatement() *DeferStatement {
	stmt := &DeferStatement{Token: p.curToken}
	p.nextToken() // consume DEFER

	if p.curTokenIs(LBRACE) {
		stmt.Body = p.parseBlockStatement()
		return stmt
	}

	expr := p.parseExpression(LOWEST)
	if expr != nil {
		stmt.Body = &ExpressionStatement{Token: stmt.Token, Expression: expr}
	}
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseEchoStatement() *EchoStatement {
	stmt := &EchoStatement{Token: p.curToken}

	p.nextToken() // consume ECHO/PRINT

	// Optional parentheses: echo("foo")
	if p.curToken.Type == LPAREN {
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
		if p.peekToken.Type == RPAREN {
			p.nextToken()
		}
	} else {
		stmt.Value = p.parseExpression(LOWEST)
	}

	if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseWhileStatement() *WhileStatement {
	stmt := &WhileStatement{Token: p.curToken}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(RPAREN) {
		return nil
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseDoWhileStatement() *DoWhileStatement {
	stmt := &DoWhileStatement{Token: p.curToken}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	if !p.expectPeek(WHILE) {
		return nil
	}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(RPAREN) {
		return nil
	}

	if p.peekToken.Type == SEMICOLON {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseTryCatchStatement() *TryCatchStatement {
	stmt := &TryCatchStatement{Token: p.curToken}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.TryBlock = p.parseBlockStatement()

	if !p.expectPeek(CATCH) {
		return nil
	}
	stmt.CatchToken = p.curToken

	if !p.expectPeek(LPAREN) {
		return nil
	}

	// Expect variable: $e
	if !p.expectPeek(VAR) {
		return nil
	}
	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.CatchVar = p.curToken.Literal

	if !p.expectPeek(RPAREN) {
		return nil
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.CatchBlock = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseThrowStatement() *ThrowStatement {
	stmt := &ThrowStatement{Token: p.curToken}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	if p.peekToken.Type == SEMICOLON {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseMethodStatement(isAbstract ...bool) *MethodStatement {
	stmt := &MethodStatement{Token: p.curToken}
	if len(isAbstract) > 0 && isAbstract[0] {
		stmt.IsAbstract = true
	}

	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	stmt.Parameters = p.parseFunctionParameters()
	stmt.ReturnType = p.parseOptionalReturnType()

	if stmt.IsAbstract {
		if p.peekToken.Type == LBRACE {
			p.addError(p.peekToken, "Los métodos abstractos no pueden tener cuerpo `{ ... }`.")
			p.nextToken()
			p.parseBlockStatement()
			return stmt
		}
		if p.peekToken.Type == SEMICOLON || p.peekToken.Type == NEWLINE {
			p.nextToken()
		}
		return stmt
	}

	if p.peekToken.Type == SEMICOLON {
		stmt.IsAbstract = true
		p.nextToken()
		return stmt
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseEnumStatement() *EnumStatement {
	stmt := &EnumStatement{Token: p.curToken}

	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekToken.Type == COLON {
		p.nextToken() // consume :
		if !p.expectPeek(IDENT) {
			return nil
		}
		stmt.BackingType = p.curToken
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	stmt.Cases = p.parseEnumBody()

	return stmt
}

func (p *Parser) parseEnumBody() []*EnumCaseStatement {
	cases := []*EnumCaseStatement{}
	p.nextToken() // move past {

	for p.curToken.Type != RBRACE && p.curToken.Type != EOF {
		if p.curToken.Type == NEWLINE || p.curToken.Type == SEMICOLON {
			p.nextToken()
			continue
		}

		if p.curToken.Type != CASE {
			p.addError(p.curToken, "Un enum solo puede contener declaraciones de casos (`case Nombre;` o `case Nombre = valor;`).")
			p.nextToken()
			continue
		}

		caseTok := p.curToken
		if !p.expectPeek(IDENT) {
			p.nextToken()
			continue
		}

		caseStmt := &EnumCaseStatement{
			Token: caseTok,
			Name:  &Identifier{Token: p.curToken, Value: p.curToken.Literal},
		}

		if p.peekToken.Type == ASSIGN {
			p.nextToken() // consume =
			p.nextToken()
			caseStmt.Value = p.parseExpression(LOWEST)
		}

		if p.peekToken.Type == SEMICOLON {
			p.nextToken()
		}

		cases = append(cases, caseStmt)
		p.nextToken()
	}

	return cases
}

func (p *Parser) parseSelectStatement() *SelectStatement {
	stmt := &SelectStatement{Token: p.curToken}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	p.nextToken() // move past {

	for p.curToken.Type != RBRACE && p.curToken.Type != EOF {
		if p.curToken.Type == NEWLINE || p.curToken.Type == SEMICOLON {
			p.nextToken()
			continue
		}

		if p.curToken.Type == CASE {
			caseTok := p.curToken
			p.nextToken()
			comm := p.parseStatement()
			if p.peekToken.Type == COLON {
				p.nextToken()
			}
			p.nextToken()
			block := &BlockStatement{Token: p.curToken, Statements: []Statement{}}
			for p.curToken.Type != CASE && p.curToken.Type != DEFAULT && p.curToken.Type != RBRACE && p.curToken.Type != EOF {
				if p.curToken.Type == NEWLINE || p.curToken.Type == SEMICOLON || p.curToken.Type == COLON {
					p.nextToken()
					continue
				}
				subStmt := p.parseStatement()
				if subStmt != nil {
					block.Statements = append(block.Statements, subStmt)
				}
				p.nextToken()
			}
			stmt.Cases = append(stmt.Cases, &SelectCaseStatement{
				Token: caseTok,
				Comm:  comm,
				Body:  block,
			})
			continue
		}

		if p.curToken.Type == DEFAULT {
			defTok := p.curToken
			if p.peekToken.Type == COLON {
				p.nextToken()
			}
			p.nextToken()
			block := &BlockStatement{Token: p.curToken, Statements: []Statement{}}
			for p.curToken.Type != CASE && p.curToken.Type != DEFAULT && p.curToken.Type != RBRACE && p.curToken.Type != EOF {
				if p.curToken.Type == NEWLINE || p.curToken.Type == SEMICOLON {
					p.nextToken()
					continue
				}
				subStmt := p.parseStatement()
				if subStmt != nil {
					block.Statements = append(block.Statements, subStmt)
				}
				p.nextToken()
			}
			stmt.Cases = append(stmt.Cases, &SelectCaseStatement{
				Token:     defTok,
				IsDefault: true,
				Body:      block,
			})
			continue
		}

		p.addError(p.curToken, "Se esperaba `case` o `default` dentro de `select`.")
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseAsyncStatement() Statement {
	tok := p.curToken
	if p.peekToken.Type == LBRACE {
		p.nextToken() // move to {
		block := p.parseBlockStatement()
		fn := &FunctionLiteral{
			Token:      Token{Type: FUNCTION, Literal: "func", Line: tok.Line},
			Parameters: []*Parameter{},
			Body:       block,
		}
		call := &CallExpression{
			Token:     Token{Type: IDENT, Literal: "async", Line: tok.Line},
			Function:  &Identifier{Token: Token{Type: IDENT, Literal: "async", Line: tok.Line}, Value: "async"},
			Arguments: []Expression{fn},
		}
		return &ExpressionStatement{Token: tok, Expression: call}
	}
	if p.peekToken.Type == LPAREN {
		msg := "'async' requiere la sintaxis de bloque 'async { ... }'; 'async(func() ...)' fue eliminado."
		p.addError(tok, msg)
		return nil
	}
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	call := &CallExpression{
		Token:     Token{Type: IDENT, Literal: "async", Line: tok.Line},
		Function:  &Identifier{Token: Token{Type: IDENT, Literal: "async", Line: tok.Line}, Value: "async"},
		Arguments: []Expression{exp},
	}
	return &ExpressionStatement{Token: tok, Expression: call}
}
