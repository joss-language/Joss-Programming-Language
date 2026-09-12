package parser

import "strings"

type interpolationSegment struct {
	isExpression bool
	text         string
}

// parseInterpolatedString owns the nested-expression boundary for string
// interpolation. The ordinary Pratt parser only receives the extracted
// expression text; segmentation and concatenation stay encapsulated here.
func (p *Parser) parseInterpolatedString(token Token) Expression {
	segments := splitInterpolationSegments(token.Literal)
	if len(segments) == 0 {
		return &StringLiteral{Token: token, Value: ""}
	}
	if !hasExpressionSegment(segments) {
		var combined strings.Builder
		for _, segment := range segments {
			combined.WriteString(segment.text)
		}
		return &StringLiteral{Token: token, Value: combined.String()}
	}
	return p.interpolationExpression(token, segments)
}

func splitInterpolationSegments(source string) []interpolationSegment {
	var segments []interpolationSegment
	start := 0
	for index := 0; index < len(source); {
		if source[index] == '\\' && index+2 < len(source) && source[index+1] == '$' && source[index+2] == '{' {
			segments = appendInterpolationText(segments, source[start:index]+"${")
			index += 3
			start = index
			continue
		}
		if source[index] != '$' || index+1 >= len(source) || source[index+1] != '{' {
			index++
			continue
		}

		segments = appendInterpolationText(segments, source[start:index])
		index += 2
		expressionStart := index
		index = interpolationExpressionEnd(source, index)
		segments = append(segments, interpolationSegment{isExpression: true, text: strings.TrimSpace(source[expressionStart:index])})
		if index < len(source) && source[index] == '}' {
			index++
		}
		start = index
	}
	if start < len(source) {
		segments = appendInterpolationText(segments, source[start:])
	}
	return segments
}

func interpolationExpressionEnd(source string, index int) int {
	depth := 1
	for index < len(source) && depth > 0 {
		switch source[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		case '"', '\'':
			index = interpolationQuotedEnd(source, index, source[index])
		}
		index++
	}
	return index
}

func interpolationQuotedEnd(source string, index int, quote byte) int {
	for index++; index < len(source); index++ {
		if source[index] == '\\' && index+1 < len(source) {
			index++
			continue
		}
		if source[index] == quote {
			return index
		}
	}
	return index
}

func appendInterpolationText(segments []interpolationSegment, text string) []interpolationSegment {
	if text == "" {
		return segments
	}
	if len(segments) > 0 && !segments[len(segments)-1].isExpression {
		segments[len(segments)-1].text += text
		return segments
	}
	return append(segments, interpolationSegment{text: text})
}

func hasExpressionSegment(segments []interpolationSegment) bool {
	for _, segment := range segments {
		if segment.isExpression {
			return true
		}
	}
	return false
}

func (p *Parser) interpolationExpression(token Token, segments []interpolationSegment) Expression {
	var root Expression
	for _, segment := range segments {
		current := p.parseInterpolationSegment(token, segment)
		if root == nil && !segment.isExpression {
			root = current
			continue
		}
		if root == nil {
			root = &StringLiteral{Token: token, Value: ""}
		}
		root = &InfixExpression{
			Token: Token{Type: DOT, Literal: ".", Line: token.Line, Column: token.Column},
			Left:  root, Operator: ".", Right: current,
		}
	}
	return root
}

func (p *Parser) parseInterpolationSegment(token Token, segment interpolationSegment) Expression {
	if !segment.isExpression {
		return &StringLiteral{Token: token, Value: segment.text}
	}
	subParser := NewParser(NewLexer(segment.text))
	expression := subParser.parseExpression(LOWEST)
	if expression == nil {
		return &StringLiteral{Token: token, Value: ""}
	}
	return expression
}
