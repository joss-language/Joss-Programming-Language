package formatter

import (
	"github.com/jossecurity/joss/pkg/parser"
)

type SourcePos struct {
	Line int
	Col  int
}

type ASTInfo struct {
	blocks        map[SourcePos]bool
	maps          map[SourcePos]bool
	blankBefore   map[SourcePos]bool
}

func AnalyzeAST(prog *parser.Program) *ASTInfo {
	info := &ASTInfo{
		blocks:      make(map[SourcePos]bool),
		maps:        make(map[SourcePos]bool),
		blankBefore: make(map[SourcePos]bool),
	}
	if prog == nil {
		return info
	}
	for i, stmt := range prog.Statements {
		if i > 0 {
			switch stmt.(type) {
			case *parser.ClassStatement, *parser.InterfaceStatement, *parser.EnumStatement, *parser.MethodStatement:
				info.blankBefore[tokenPos(stmt)] = true
			}
		}
		info.walkStatement(stmt)
	}
	return info
}

func tokenPos(stmt parser.Statement) SourcePos {
	switch s := stmt.(type) {
	case *parser.ClassStatement:
		tok := s.Token
		if s.StartToken.Type != "" {
			tok = s.StartToken
		}
		return SourcePos{Line: tok.Line, Col: tok.Column}
	case *parser.InterfaceStatement:
		return SourcePos{Line: s.Token.Line, Col: s.Token.Column}
	case *parser.EnumStatement:
		return SourcePos{Line: s.Token.Line, Col: s.Token.Column}
	case *parser.MethodStatement:
		tok := s.Token
		if s.StartToken.Type != "" {
			tok = s.StartToken
		}
		return SourcePos{Line: tok.Line, Col: tok.Column}
	case *parser.InitStatement:
		return SourcePos{Line: s.Token.Line, Col: s.Token.Column}
	case *parser.LetStatement:
		return SourcePos{Line: s.Token.Line, Col: s.Token.Column}
	default:
		return SourcePos{}
	}
}

func (info *ASTInfo) ShouldBlankBefore(line, col int) bool {
	return info.blankBefore[SourcePos{Line: line, Col: col}]
}

func (info *ASTInfo) IsBlock(line, col int) bool {
	return info.blocks[SourcePos{Line: line, Col: col}]
}

func (info *ASTInfo) IsMap(line, col int) bool {
	return info.maps[SourcePos{Line: line, Col: col}]
}

func (info *ASTInfo) walkStatement(stmt parser.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *parser.BlockStatement:
		info.blocks[SourcePos{Line: s.Token.Line, Col: s.Token.Column}] = true
		for _, inner := range s.Statements {
			info.walkStatement(inner)
		}
	case *parser.ClassStatement:
		if s.Body != nil {
			info.blocks[SourcePos{Line: s.Body.Token.Line, Col: s.Body.Token.Column}] = true
			for i, member := range s.Body.Statements {
				if i > 0 {
					switch member.(type) {
					case *parser.MethodStatement, *parser.InitStatement:
						info.blankBefore[tokenPos(member)] = true
					}
				}
				info.walkStatement(member)
			}
		}
	case *parser.InterfaceStatement:
		for _, m := range s.Methods {
			info.walkStatement(m)
		}
	case *parser.MethodStatement:
		if s.Body != nil {
			info.walkStatement(s.Body)
		}
	case *parser.InitStatement:
		if s.Body != nil {
			info.walkStatement(s.Body)
		}
	case *parser.GuardStatement:
		info.walkExpression(s.Condition)
		if s.Body != nil {
			info.walkStatement(s.Body)
		}
	case *parser.WhileStatement:
		info.walkExpression(s.Condition)
		if s.Body != nil {
			info.walkStatement(s.Body)
		}
	case *parser.DoWhileStatement:
		info.walkExpression(s.Condition)
		if s.Body != nil {
			info.walkStatement(s.Body)
		}
	case *parser.ForeachStatement:
		info.walkExpression(s.Iterable)
		if s.Body != nil {
			info.walkStatement(s.Body)
		}
	case *parser.TryCatchStatement:
		if s.TryBlock != nil {
			info.walkStatement(s.TryBlock)
		}
		if s.CatchBlock != nil {
			info.walkStatement(s.CatchBlock)
		}
	case *parser.SelectStatement:
		for _, c := range s.Cases {
			if c.Comm != nil {
				info.walkStatement(c.Comm)
			}
			if c.Body != nil {
				info.walkStatement(c.Body)
			}
		}
	case *parser.DeferStatement:
		info.walkStatement(s.Body)
	case *parser.ExpressionStatement:
		info.walkExpression(s.Expression)
	case *parser.LetStatement:
		info.walkExpression(s.Value)
	case *parser.MultiLetStatement:
		for _, d := range s.Declarations {
			info.walkExpression(d.Value)
		}
	case *parser.ReturnStatement:
		info.walkExpression(s.ReturnValue)
	case *parser.ThrowStatement:
		info.walkExpression(s.Value)
	case *parser.EchoStatement:
		info.walkExpression(s.Value)
	}
}

func (info *ASTInfo) walkExpression(expr parser.Expression) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *parser.BlockExpression:
		info.blocks[SourcePos{Line: e.Token.Line, Col: e.Token.Column}] = true
		if e.Block != nil {
			info.walkStatement(e.Block)
		}
	case *parser.MapLiteral:
		info.maps[SourcePos{Line: e.Token.Line, Col: e.Token.Column}] = true
		for k, v := range e.Pairs {
			info.walkExpression(k)
			info.walkExpression(v)
		}
	case *parser.ArrayLiteral:
		for _, el := range e.Elements {
			info.walkExpression(el)
		}
	case *parser.TernaryExpression:
		info.walkExpression(e.Condition)
		info.walkExpression(e.True)
		info.walkExpression(e.False)
	case *parser.CallExpression:
		info.walkExpression(e.Function)
		for _, arg := range e.Arguments {
			info.walkExpression(arg)
		}
	case *parser.InfixExpression:
		info.walkExpression(e.Left)
		info.walkExpression(e.Right)
	case *parser.PrefixExpression:
		info.walkExpression(e.Right)
	case *parser.PostfixExpression:
		info.walkExpression(e.Left)
	case *parser.IndexExpression:
		info.walkExpression(e.Left)
		info.walkExpression(e.Index)
	case *parser.MemberExpression:
		info.walkExpression(e.Left)
	case *parser.FunctionLiteral:
		if e.Body != nil {
			info.walkStatement(e.Body)
		}
	case *parser.MatchExpression:
		info.walkExpression(e.Subject)
		for _, arm := range e.Arms {
			for _, k := range arm.Keys {
				info.walkExpression(k)
			}
			info.walkExpression(arm.Value)
		}
	case *parser.ReferenceExpression:
		info.walkExpression(e.Target)
	case *parser.NewExpression:
		info.walkExpression(e.Class)
		for _, arg := range e.Arguments {
			info.walkExpression(arg)
		}
	}
}
