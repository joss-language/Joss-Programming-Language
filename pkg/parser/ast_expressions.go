package parser

import (
	"bytes"
	"strings"

	"github.com/shopspring/decimal"
)

type Identifier struct {
	Token Token // The token.VAR ($)
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

type StringLiteral struct {
	Token Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return sl.Token.Literal }

type CallExpression struct {
	Token     Token      // The '(' token
	Function  Expression // Identifier or FunctionLiteral
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer
	args := []string{}
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}
	out.WriteString(ce.Function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

type TernaryExpression struct {
	Token     Token // ?
	Condition Expression
	True      Expression
	False     Expression
}

func (te *TernaryExpression) expressionNode()      {}
func (te *TernaryExpression) TokenLiteral() string { return te.Token.Literal }
func (te *TernaryExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(te.Condition.String())
	out.WriteString(") ? ")
	if te.True != nil {
		out.WriteString(te.True.String())
	}
	if te.False != nil {
		out.WriteString(" : ")
		out.WriteString(te.False.String())
	}
	return out.String()
}

type InfixExpression struct {
	Token    Token // Operator token, e.g. +
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")
	return out.String()
}

type PrefixExpression struct {
	Token    Token // The prefix token, e.g. !
	Operator string
	Right    Expression
}

// ReferenceExpression marks a mutable argument binding. It is deliberately a
// call-site construct, not a first-class pointer value.
type ReferenceExpression struct {
	Token  Token // REF
	Target Expression
}

func (re *ReferenceExpression) expressionNode()      {}
func (re *ReferenceExpression) TokenLiteral() string { return re.Token.Literal }
func (re *ReferenceExpression) String() string {
	if re.Target == nil {
		return "ref"
	}
	return "ref " + re.Target.String()
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(pe.Operator)
	out.WriteString(pe.Right.String())
	out.WriteString(")")
	return out.String()
}

type PostfixExpression struct {
	Token    Token // The postfix token, e.g. ++
	Operator string
	Left     Expression
}

func (pe *PostfixExpression) expressionNode()      {}
func (pe *PostfixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PostfixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(pe.Left.String())
	out.WriteString(pe.Operator)
	out.WriteString(")")
	return out.String()
}

type NullLiteral struct {
	Token Token
}

func (nl *NullLiteral) expressionNode()      {}
func (nl *NullLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NullLiteral) String() string       { return nl.Token.Literal }

type Boolean struct {
	Token Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) String() string       { return b.Token.Literal }

type IntegerLiteral struct {
	Token Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

type FloatLiteral struct {
	Token Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

type DecimalLiteral struct {
	Token Token
	Value decimal.Decimal
}

func (dl *DecimalLiteral) expressionNode()      {}
func (dl *DecimalLiteral) TokenLiteral() string { return dl.Token.Literal }
func (dl *DecimalLiteral) String() string       { return dl.Token.Literal }

type ArrayLiteral struct {
	Token    Token // '['
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer
	elements := []string{}
	for _, el := range al.Elements {
		elements = append(elements, el.String())
	}
	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")
	return out.String()
}

type MapLiteral struct {
	Token Token // '{'
	Pairs map[Expression]Expression
}

func (ml *MapLiteral) expressionNode()      {}
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Literal }
func (ml *MapLiteral) String() string {
	var out bytes.Buffer
	pairs := []string{}
	for key, value := range ml.Pairs {
		pairs = append(pairs, key.String()+": "+value.String())
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}

type IndexExpression struct {
	Token Token // '['
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("])")
	return out.String()
}

type FunctionLiteral struct {
	Token      Token // FUNCTION
	Parameters []*Parameter
	ReturnType Token // optional type after ':'
	Body       *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	out.WriteString(fl.TokenLiteral() + "(")
	params := []string{}
	for _, p := range fl.Parameters {
		params = append(params, p.String())
	}
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(")")
	if fl.ReturnType.Literal != "" {
		out.WriteString(": " + fl.ReturnType.Literal)
	}
	out.WriteString(" ")
	out.WriteString(fl.Body.String())
	return out.String()
}

type NewExpression struct {
	Token     Token // NEW
	Class     *Identifier
	Arguments []Expression
}

func (ne *NewExpression) expressionNode()      {}
func (ne *NewExpression) TokenLiteral() string { return ne.Token.Literal }
func (ne *NewExpression) String() string {
	var out bytes.Buffer
	out.WriteString("new ")
	out.WriteString(ne.Class.String())
	out.WriteString("(")
	args := []string{}
	for _, a := range ne.Arguments {
		args = append(args, a.String())
	}
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

type MemberExpression struct {
	Token    Token // ARROW, NULL_SAFE_ARROW, DOUBLE_COLON, DOT
	Left     Expression
	Property *Identifier
	NullSafe bool
}

func (me *MemberExpression) expressionNode()      {}
func (me *MemberExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MemberExpression) String() string {
	var out bytes.Buffer
	out.WriteString(me.Left.String())
	if me.NullSafe {
		out.WriteString("?->")
	} else if me.Token.Literal != "" {
		out.WriteString(me.Token.Literal)
	} else {
		out.WriteString("->")
	}
	out.WriteString(me.Property.String())
	return out.String()
}

type AssignExpression struct {
	Token Token // =
	Left  Expression
	Value Expression
}

func (ae *AssignExpression) expressionNode()      {}
func (ae *AssignExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AssignExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ae.Left.String())
	out.WriteString(" = ")
	out.WriteString(ae.Value.String())
	return out.String()
}

type IssetExpression struct {
	Token     Token // ISSET
	Arguments []Expression
}

func (ie *IssetExpression) expressionNode()      {}
func (ie *IssetExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IssetExpression) String() string {
	var out bytes.Buffer
	out.WriteString("isset(")
	args := []string{}
	for _, a := range ie.Arguments {
		args = append(args, a.String())
	}
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

type EmptyExpression struct {
	Token    Token // EMPTY
	Argument Expression
}

func (ee *EmptyExpression) expressionNode()      {}
func (ee *EmptyExpression) TokenLiteral() string { return ee.Token.Literal }
func (ee *EmptyExpression) String() string {
	var out bytes.Buffer
	out.WriteString("empty(")
	out.WriteString(ee.Argument.String())
	out.WriteString(")")
	return out.String()
}

type BlockExpression struct {
	Token Token // {
	Block *BlockStatement
}

func (be *BlockExpression) expressionNode()      {}
func (be *BlockExpression) TokenLiteral() string { return be.Token.Literal }
func (be *BlockExpression) String() string {
	return be.Block.String()
}

type MatchArm struct {
	Keys      []Expression
	IsDefault bool
	Value     Expression
}

type MatchExpression struct {
	Token   Token // The MATCH token
	Subject Expression
	Arms    []MatchArm
}

func (me *MatchExpression) expressionNode()      {}
func (me *MatchExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MatchExpression) String() string {
	var out bytes.Buffer
	out.WriteString("match (")
	out.WriteString(me.Subject.String())
	out.WriteString(") {")
	armsStr := []string{}
	for _, arm := range me.Arms {
		keysStr := []string{}
		for _, k := range arm.Keys {
			keysStr = append(keysStr, k.String())
		}
		armsStr = append(armsStr, strings.Join(keysStr, ", ")+" => "+arm.Value.String())
	}
	out.WriteString(strings.Join(armsStr, ", "))
	out.WriteString("}")
	return out.String()
}

type IsExpression struct {
	Token      Token // IS or INSTANCEOF
	Left       Expression
	TargetType Token // Target type token (e.g. "int", "User", "string|null")
}

func (ie *IsExpression) expressionNode()      {}
func (ie *IsExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IsExpression) String() string {
	return "(" + ie.Left.String() + " " + ie.Token.Literal + " " + ie.TargetType.Literal + ")"
}

type SpreadExpression struct {
	Token      Token // ...
	Expression Expression
}

func (se *SpreadExpression) expressionNode()      {}
func (se *SpreadExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SpreadExpression) String() string {
	return "..." + se.Expression.String()
}

type YieldExpression struct {
	Token Token // YIELD
	Key   Expression
	Value Expression
}

func (ye *YieldExpression) expressionNode()      {}
func (ye *YieldExpression) TokenLiteral() string { return ye.Token.Literal }
func (ye *YieldExpression) String() string {
	if ye.Key != nil {
		return "yield " + ye.Key.String() + " => " + ye.Value.String()
	}
	if ye.Value != nil {
		return "yield " + ye.Value.String()
	}
	return "yield"
}

type NamedArgument struct {
	Token Token // Identifier token
	Name  string
	Value Expression
}

func (na *NamedArgument) expressionNode()      {}
func (na *NamedArgument) TokenLiteral() string { return na.Token.Literal }
func (na *NamedArgument) String() string {
	return na.Name + ": " + na.Value.String()
}
