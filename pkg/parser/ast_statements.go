package parser

import (
	"bytes"
	"strings"
)

// LetStatement: string $x = "foo"
type LetStatement struct {
	Token      Token // The token.IDENT (e.g. string, int, public, private)
	Name       *Identifier
	Value      Expression
	IsConst    bool
	Visibility string // "public", "private", "protected"
	IsStatic   bool
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out bytes.Buffer
	if ls.Visibility != "" {
		out.WriteString(ls.Visibility + " ")
	}
	if ls.IsStatic {
		out.WriteString("static ")
	}
	if ls.IsConst {
		out.WriteString("const ")
	}
	if !ls.IsConst || ls.Token.Literal != "var" {
		out.WriteString(ls.Token.Literal + " ")
	}
	out.WriteString(ls.Name.String())
	out.WriteString(" = ")
	if ls.Value != nil {
		out.WriteString(ls.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

// SingleDecl is one element in a multi-declaration: $name [= value]
type SingleDecl struct {
	Name  *Identifier
	Value Expression // nil if no initializer (zero-value will be used)
}

// MultiLetStatement: int $a,$b  or  string $x="hi",$y
type MultiLetStatement struct {
	TypeToken    Token        // the type keyword token (e.g. "int")
	Declarations []SingleDecl // one per variable
	Visibility   string
	IsStatic     bool
}

func (mls *MultiLetStatement) statementNode()       {}
func (mls *MultiLetStatement) TokenLiteral() string { return mls.TypeToken.Literal }
func (mls *MultiLetStatement) String() string {
	var parts []string
	for _, d := range mls.Declarations {
		s := d.Name.String()
		if d.Value != nil {
			s += " = " + d.Value.String()
		}
		parts = append(parts, s)
	}
	return mls.TypeToken.Literal + " " + strings.Join(parts, ", ") + ";"
}

type ExpressionStatement struct {
	Token      Token // The first token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

type ClassStatement struct {
	Token      Token // CLASS
	Name       *Identifier
	SuperClass *Identifier
	Interfaces []*Identifier // interfaces implemented via 'implements'
	Body       *BlockStatement
	Visibility string // "public", etc.
	IsAbstract bool
}

func (cs *ClassStatement) statementNode()       {}
func (cs *ClassStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ClassStatement) String() string {
	var out bytes.Buffer
	if cs.Visibility != "" {
		out.WriteString(cs.Visibility + " ")
	}
	if cs.IsAbstract {
		out.WriteString("abstract ")
	}
	out.WriteString("class ")
	out.WriteString(cs.Name.String())
	if cs.SuperClass != nil {
		out.WriteString(" extends ")
		out.WriteString(cs.SuperClass.String())
	}
	if len(cs.Interfaces) > 0 {
		out.WriteString(" implements ")
		for i, iface := range cs.Interfaces {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(iface.String())
		}
	}
	out.WriteString(" ")
	out.WriteString(cs.Body.String())
	return out.String()
}

type InterfaceStatement struct {
	Token      Token // INTERFACE
	Name       *Identifier
	Extends    []*Identifier      // extended interfaces
	Methods    []*MethodStatement // prototypes without bodies
	Visibility string             // "public", "private"
}

func (is *InterfaceStatement) statementNode()       {}
func (is *InterfaceStatement) TokenLiteral() string { return is.Token.Literal }
func (is *InterfaceStatement) String() string {
	var out bytes.Buffer
	if is.Visibility != "" {
		out.WriteString(is.Visibility + " ")
	}
	out.WriteString("interface ")
	out.WriteString(is.Name.String())
	if len(is.Extends) > 0 {
		out.WriteString(" extends ")
		for i, ext := range is.Extends {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(ext.String())
		}
	}
	out.WriteString(" {\n")
	for _, m := range is.Methods {
		out.WriteString("  " + m.String() + "\n")
	}
	out.WriteString("}")
	return out.String()
}

type EnumCaseStatement struct {
	Token Token // CASE
	Name  *Identifier
	Value Expression // optional backing value
}

func (ecs *EnumCaseStatement) statementNode()       {}
func (ecs *EnumCaseStatement) TokenLiteral() string { return ecs.Token.Literal }
func (ecs *EnumCaseStatement) String() string {
	if ecs.Value != nil {
		return "case " + ecs.Name.String() + " = " + ecs.Value.String() + ";"
	}
	return "case " + ecs.Name.String() + ";"
}

type EnumStatement struct {
	Token       Token // ENUM
	Name        *Identifier
	BackingType Token // optional backing type e.g. string or int
	Cases       []*EnumCaseStatement
	Visibility  string // "public", "private"
}

func (es *EnumStatement) statementNode()       {}
func (es *EnumStatement) TokenLiteral() string { return es.Token.Literal }
func (es *EnumStatement) String() string {
	var out bytes.Buffer
	if es.Visibility != "" {
		out.WriteString(es.Visibility + " ")
	}
	out.WriteString("enum ")
	out.WriteString(es.Name.String())
	if es.BackingType.Literal != "" {
		out.WriteString(": " + es.BackingType.Literal)
	}
	out.WriteString(" {\n")
	for _, c := range es.Cases {
		out.WriteString("  " + c.String() + "\n")
	}
	out.WriteString("}")
	return out.String()
}

type SelectCaseStatement struct {
	Token     Token // CASE or DEFAULT
	IsDefault bool
	Comm      Statement
	Body      *BlockStatement
}

func (scs *SelectCaseStatement) statementNode()       {}
func (scs *SelectCaseStatement) TokenLiteral() string { return scs.Token.Literal }
func (scs *SelectCaseStatement) String() string {
	if scs.IsDefault {
		return "default:\n" + scs.Body.String()
	}
	s := "case "
	if scs.Comm != nil {
		s += scs.Comm.String()
	}
	s += ":\n" + scs.Body.String()
	return s
}

type SelectStatement struct {
	Token Token // SELECT
	Cases []*SelectCaseStatement
}

func (ss *SelectStatement) statementNode()       {}
func (ss *SelectStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *SelectStatement) String() string {
	var out bytes.Buffer
	out.WriteString("select {\n")
	for _, c := range ss.Cases {
		out.WriteString("  " + c.String() + "\n")
	}
	out.WriteString("}")
	return out.String()
}

type BlockStatement struct {
	Token      Token // {
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

type EchoStatement struct {
	Token Token // 'echo' or 'print'
	Value Expression
}

func (es *EchoStatement) statementNode()       {}
func (es *EchoStatement) TokenLiteral() string { return es.Token.Literal }
func (es *EchoStatement) String() string {
	var out bytes.Buffer
	out.WriteString(es.Token.Literal + " ")
	if es.Value != nil {
		out.WriteString(es.Value.String())
	}
	return out.String()
}

type InitStatement struct {
	Token      Token       // INIT
	Name       *Identifier // main
	Parameters []*Parameter
	Body       *BlockStatement
}

func (is *InitStatement) statementNode()       {}
func (is *InitStatement) TokenLiteral() string { return is.Token.Literal }
func (is *InitStatement) String() string {
	var out bytes.Buffer
	out.WriteString("Init ")
	out.WriteString(is.Name.String())
	out.WriteString("(")
	params := []string{}
	for _, p := range is.Parameters {
		params = append(params, p.String())
	}
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(is.Body.String())
	return out.String()
}

type ForeachStatement struct {
	Token    Token // 'foreach'
	Iterable Expression
	Key      string // Optional, e.g. "key" in "as $key => $val"
	Value    string // The variable name, e.g. "val" in "as $val"
	Body     *BlockStatement
}

func (fs *ForeachStatement) statementNode()       {}
func (fs *ForeachStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForeachStatement) String() string {
	var out bytes.Buffer
	out.WriteString("foreach (")
	out.WriteString(fs.Iterable.String())
	out.WriteString(" as ")
	if fs.Key != "" {
		out.WriteString("$" + fs.Key + " => ")
	}
	out.WriteString("$" + fs.Value)
	out.WriteString(") ")
	out.WriteString(fs.Body.String())
	return out.String()
}

type DeferStatement struct {
	Token Token // 'defer'
	Body  Statement
}

func (ds *DeferStatement) statementNode()       {}
func (ds *DeferStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DeferStatement) String() string {
	var out bytes.Buffer
	out.WriteString("defer ")
	if ds.Body != nil {
		out.WriteString(ds.Body.String())
	}
	return out.String()
}

type MethodStatement struct {
	Token      Token // FUNCTION
	Name       *Identifier
	Parameters []*Parameter
	ReturnType Token // optional type after ':'
	Body       *BlockStatement
	Visibility string // "public", "private", "protected"
	IsStatic   bool
	IsAbstract bool
}

func (ms *MethodStatement) statementNode()       {}
func (ms *MethodStatement) TokenLiteral() string { return ms.Token.Literal }
func (ms *MethodStatement) String() string {
	var out bytes.Buffer
	if ms.Visibility != "" {
		out.WriteString(ms.Visibility + " ")
	}
	if ms.IsStatic {
		out.WriteString("static ")
	}
	if ms.IsAbstract {
		out.WriteString("abstract ")
	}
	out.WriteString(ms.TokenLiteral() + " ")
	out.WriteString(ms.Name.String())
	out.WriteString("(")
	params := []string{}
	for _, p := range ms.Parameters {
		params = append(params, p.String())
	}
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(")")
	if ms.ReturnType.Literal != "" {
		out.WriteString(": " + ms.ReturnType.Literal)
	}
	if ms.Body != nil {
		out.WriteString(" ")
		out.WriteString(ms.Body.String())
	} else {
		out.WriteString(";")
	}
	return out.String()
}

type WhileStatement struct {
	Token     Token // WHILE
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("while (")
	out.WriteString(ws.Condition.String())
	out.WriteString(") ")
	out.WriteString(ws.Body.String())
	return out.String()
}

type DoWhileStatement struct {
	Token     Token // DO
	Condition Expression
	Body      *BlockStatement
}

func (dws *DoWhileStatement) statementNode()       {}
func (dws *DoWhileStatement) TokenLiteral() string { return dws.Token.Literal }
func (dws *DoWhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("do ")
	out.WriteString(dws.Body.String())
	out.WriteString(" while (")
	out.WriteString(dws.Condition.String())
	out.WriteString(");")
	return out.String()
}

type TryCatchStatement struct {
	Token      Token // TRY
	TryBlock   *BlockStatement
	CatchToken Token  // CATCH
	CatchVar   string // The variable name for the error, e.g. "e"
	CatchBlock *BlockStatement
}

func (tcs *TryCatchStatement) statementNode()       {}
func (tcs *TryCatchStatement) TokenLiteral() string { return tcs.Token.Literal }
func (tcs *TryCatchStatement) String() string {
	var out bytes.Buffer
	out.WriteString("try ")
	out.WriteString(tcs.TryBlock.String())
	out.WriteString(" catch ($")
	out.WriteString(tcs.CatchVar)
	out.WriteString(") ")
	out.WriteString(tcs.CatchBlock.String())
	return out.String()
}

type ThrowStatement struct {
	Token Token // THROW
	Value Expression
}

func (ts *ThrowStatement) statementNode()       {}
func (ts *ThrowStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *ThrowStatement) String() string {
	var out bytes.Buffer
	out.WriteString("throw ")
	if ts.Value != nil {
		out.WriteString(ts.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

type ReturnStatement struct {
	Token       Token // 'return'
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer
	out.WriteString(rs.TokenLiteral() + " ")
	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}
	out.WriteString(";")
	return out.String()
}

// Control Flow: Break
type BreakStatement struct {
	Token Token // 'break'
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) String() string       { return bs.Token.Literal + ";" }

// Control Flow: Continue
type ContinueStatement struct {
	Token Token // 'continue'
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) String() string       { return cs.Token.Literal + ";" }
