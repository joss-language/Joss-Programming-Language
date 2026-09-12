package parser

import (
	"fmt"
	"sort"
	"strings"
)

type TokenType string

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers + literals
	IDENT   = "IDENT"   // add, foobar, x, y, ...
	INT     = "INT"     // 1343456
	FLOAT   = "FLOAT"   // 12.34
	DECIMAL = "DECIMAL" // 12.34m, 100m
	STRING  = "STRING"  // "foobar"

	// Operators and delimiters
	ASSIGN               = "="
	PLUS_ASSIGN          = "+="
	MINUS_ASSIGN         = "-="
	ASTERISK_ASSIGN      = "*="
	SLASH_ASSIGN         = "/="
	NULL_COALESCE_ASSIGN = "??="
	PLUS                 = "+"
	MINUS                = "-"
	BANG                 = "!"
	ASTERISK             = "*"
	SLASH                = "/"
	PERCENT              = "%"

	LT            = "<"
	GT            = ">"
	EQ            = "=="
	NOT_EQ        = "!="
	STRICT_EQ     = "==="
	STRICT_NOT_EQ = "!=="
	SPACESHIP     = "<=>"
	LTE           = "<="
	GTE           = ">="
	SHIFT_LEFT    = "<<"
	SHIFT_RIGHT   = ">>"
	AND           = "&&"
	OR            = "||"
	INCREMENT     = "++"
	DECREMENT     = "--"

	COMMA     = ","
	SEMICOLON = ";"
	COLON     = ":"
	QUESTION  = "?"
	NEWLINE   = "NEWLINE"

	LPAREN          = "("
	RPAREN          = ")"
	LBRACE          = "{"
	RBRACE          = "}"
	LBRACKET        = "["
	RBRACKET        = "]"
	DOT             = "."
	RANGE           = ".."
	ELLIPSIS        = "..."
	ARROW           = "->"
	NULL_SAFE_ARROW = "?->"
	DOUBLE_COLON    = "::"
	TYPE_UNION      = "|"
	PIPE            = "|>"
	NULL_COALESCE   = "??"
	FAT_ARROW       = "=>"

	// Keywords
	FUNCTION = "FUNCTION"
	VAR      = "VAR" // $
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	NULL     = "NULL"
	NIL      = "NIL"

	RETURN   = "RETURN"
	PRINT    = "PRINT"
	ECHO     = "ECHO"
	CLASS    = "CLASS"
	INIT     = "INIT"
	NEW      = "NEW"
	FOREACH  = "FOREACH"
	AS       = "AS"
	THIS     = "THIS"
	ISSET    = "ISSET"
	EMPTY    = "EMPTY"
	BREAK    = "BREAK"
	CONTINUE = "CONTINUE"
	// Control Structures
	WHILE      = "WHILE"
	DO         = "DO"
	TRY        = "TRY"
	CATCH      = "CATCH"
	THROW      = "THROW"
	EXTENDS    = "EXTENDS"
	IF         = "IF"
	ELSE       = "ELSE"
	GUARD      = "GUARD"
	MATCH      = "MATCH"
	DEFAULT    = "DEFAULT"
	ASYNC      = "ASYNC"
	DEFER      = "DEFER"
	INTERFACE  = "INTERFACE"
	IMPLEMENTS = "IMPLEMENTS"
	ABSTRACT   = "ABSTRACT"
	ENUM       = "ENUM"
	CASE       = "CASE"
	IS         = "IS"
	INSTANCEOF = "INSTANCEOF"
	SELECT     = "SELECT"
	YIELD      = "YIELD"

	// Modifiers & Visibility
	PUBLIC    = "PUBLIC"
	PRIVATE   = "PRIVATE"
	PROTECTED = "PROTECTED"
	LET       = "LET"
	CONST     = "CONST"
	STATIC    = "STATIC"
	REF       = "REF"
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

var keywords = map[string]TokenType{
	"true":  TRUE,
	"false": FALSE,
	"null":  NULL,
	"nil":   NIL,

	"let":        LET,
	"const":      CONST,
	"return":     RETURN,
	"class":      CLASS,
	"interface":  INTERFACE,
	"implements": IMPLEMENTS,
	"Init":       INIT,
	"new":        NEW,
	"foreach":    FOREACH,
	"as":         AS,
	"func":       FUNCTION,
	"this":       THIS,
	"echo":       ECHO,
	"print":      PRINT,
	"isset":      ISSET,
	"empty":      EMPTY,
	"break":      BREAK,
	"continue":   CONTINUE,
	"while":      WHILE,
	"do":         DO,
	"try":        TRY,
	"catch":      CATCH,
	"throw":      THROW,
	"extends":    EXTENDS,
	"guard":      GUARD,
	"match":      MATCH,
	"default":    DEFAULT,
	"async":      ASYNC,
	"defer":      DEFER,
	"abstract":   ABSTRACT,
	"enum":       ENUM,
	"case":       CASE,
	"is":         IS,
	"instanceof": INSTANCEOF,
	"select":     SELECT,
	"yield":      YIELD,
	"public":     PUBLIC,
	"private":    PRIVATE,
	"protected":  PROTECTED,
	"static":     STATIC,
	"ref":        REF,
}

var removedKeywords = map[string]string{
	"function":  "Use `func` for declarations and closures.",
	"import":    "Source imports were removed; project files and plugins are loaded automatically.",
	"@import":   "Source imports were removed; project files and plugins are loaded automatically.",
	"use":       "Plugin namespaces are loaded from `joss.yaml`; `use` is not part of Joss.",
	"Use":       "Plugin namespaces are loaded from `joss.yaml`; `use` is not part of Joss.",
	"Import":    "Source imports were removed; project files and plugins are loaded automatically.",
	"namespace": "Source namespaces were removed; classes and functions use the project symbol table.",
	"Namespace": "Source namespaces were removed; classes and functions use the project symbol table.",
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	if _, removed := removedKeywords[ident]; removed {
		return ILLEGAL
	}
	return IDENT
}

func removedKeywordMessage(token Token) (string, bool) {
	if token.Type != ILLEGAL {
		return "", false
	}
	suggestion, removed := removedKeywords[token.Literal]
	if !removed {
		return "", false
	}
	return fmt.Sprintf("La sintaxis eliminada `%s` ya no es válida. %s", token.Literal, suggestion), true
}

// KeywordNames exposes the lexer registry to tooling generators without
// requiring another manually maintained keyword list.
func KeywordNames() []string {
	result := make([]string, 0, len(keywords))
	for keyword := range keywords {
		result = append(result, keyword)
	}
	sort.Strings(result)
	return result
}

// SymbolKind describes how source-preserving tooling should classify a symbol.
// Parsing behavior remains owned by the Pratt parser; this metadata only
// centralizes the lexical spelling shared by the lexer and formatter.
type SymbolKind uint8

const (
	SymbolOperator SymbolKind = iota
	SymbolDelimiter
)

// SymbolDefinition is the canonical lexical definition of a punctuation or
// operator token. Keep language symbols here instead of duplicating switches in
// the lexer, formatter and tooling.
type SymbolDefinition struct {
	Literal string
	Token   TokenType
	Kind    SymbolKind
}

var symbolDefinitions = []SymbolDefinition{
	{STRICT_EQ, STRICT_EQ, SymbolOperator}, {STRICT_NOT_EQ, STRICT_NOT_EQ, SymbolOperator},
	{SPACESHIP, SPACESHIP, SymbolOperator}, {ELLIPSIS, ELLIPSIS, SymbolOperator},
	{NULL_SAFE_ARROW, NULL_SAFE_ARROW, SymbolOperator}, {NULL_COALESCE_ASSIGN, NULL_COALESCE_ASSIGN, SymbolOperator},
	{EQ, EQ, SymbolOperator}, {NOT_EQ, NOT_EQ, SymbolOperator}, {LTE, LTE, SymbolOperator}, {GTE, GTE, SymbolOperator},
	{SHIFT_LEFT, SHIFT_LEFT, SymbolOperator}, {SHIFT_RIGHT, SHIFT_RIGHT, SymbolOperator}, {AND, AND, SymbolOperator}, {OR, OR, SymbolOperator},
	{INCREMENT, INCREMENT, SymbolOperator}, {DECREMENT, DECREMENT, SymbolOperator}, {RANGE, RANGE, SymbolOperator},
	{ARROW, ARROW, SymbolOperator}, {DOUBLE_COLON, DOUBLE_COLON, SymbolOperator}, {PIPE, PIPE, SymbolOperator},
	{NULL_COALESCE, NULL_COALESCE, SymbolOperator}, {FAT_ARROW, FAT_ARROW, SymbolOperator},
	{PLUS_ASSIGN, PLUS_ASSIGN, SymbolOperator}, {MINUS_ASSIGN, MINUS_ASSIGN, SymbolOperator},
	{ASTERISK_ASSIGN, ASTERISK_ASSIGN, SymbolOperator}, {SLASH_ASSIGN, SLASH_ASSIGN, SymbolOperator},
	{ASSIGN, ASSIGN, SymbolOperator}, {PLUS, PLUS, SymbolOperator}, {MINUS, MINUS, SymbolOperator},
	{BANG, BANG, SymbolOperator}, {ASTERISK, ASTERISK, SymbolOperator}, {SLASH, SLASH, SymbolOperator},
	{PERCENT, PERCENT, SymbolOperator}, {LT, LT, SymbolOperator}, {GT, GT, SymbolOperator},
	{DOT, DOT, SymbolOperator}, {TYPE_UNION, TYPE_UNION, SymbolOperator},
	{COMMA, COMMA, SymbolDelimiter}, {SEMICOLON, SEMICOLON, SymbolDelimiter}, {COLON, COLON, SymbolDelimiter},
	{QUESTION, QUESTION, SymbolDelimiter}, {LPAREN, LPAREN, SymbolDelimiter}, {RPAREN, RPAREN, SymbolDelimiter},
	{LBRACE, LBRACE, SymbolDelimiter}, {RBRACE, RBRACE, SymbolDelimiter},
	{LBRACKET, LBRACKET, SymbolDelimiter}, {RBRACKET, RBRACKET, SymbolDelimiter},
}

func init() {
	sort.Slice(symbolDefinitions, func(i, j int) bool {
		if len(symbolDefinitions[i].Literal) != len(symbolDefinitions[j].Literal) {
			return len(symbolDefinitions[i].Literal) > len(symbolDefinitions[j].Literal)
		}
		return symbolDefinitions[i].Literal < symbolDefinitions[j].Literal
	})
}

// SymbolDefinitions returns an immutable snapshot ordered longest-first. The
// ordering lets consumers implement maximal-munch scanning without private
// copies of Joss's operator list.
func SymbolDefinitions() []SymbolDefinition {
	result := make([]SymbolDefinition, len(symbolDefinitions))
	copy(result, symbolDefinitions)
	return result
}

// LookupSymbol resolves a complete source symbol to its token definition.
func LookupSymbol(literal string) (SymbolDefinition, bool) {
	for _, definition := range symbolDefinitions {
		if definition.Literal == literal {
			return definition, true
		}
	}
	return SymbolDefinition{}, false
}

// MultiCharOperators returns all multi-character operators defined in Joss,
// sorted by length descending so scanners and formatters can match them dynamically.
func MultiCharOperators() []string {
	result := make([]string, 0)
	for _, definition := range symbolDefinitions {
		if definition.Kind == SymbolOperator && len(definition.Literal) > 1 {
			result = append(result, definition.Literal)
		}
	}
	return result
}

func matchSymbolPrefix(source string) (SymbolDefinition, bool) {
	for _, definition := range symbolDefinitions {
		if strings.HasPrefix(source, definition.Literal) {
			return definition, true
		}
	}
	return SymbolDefinition{}, false
}

// IsControlKeyword returns true if ident is a control flow keyword requiring '(' (e.g. guard, while, foreach, match, catch).
func IsControlKeyword(ident string) bool {
	switch LookupIdent(ident) {
	case WHILE, FOREACH, MATCH, CATCH, GUARD:
		return true
	default:
		return false
	}
}

// IsDeclarationKeyword returns true if ident is a declaration keyword that may introduce a block (e.g. class, interface, enum, func, Init).
func IsDeclarationKeyword(ident string) bool {
	switch LookupIdent(ident) {
	case CLASS, INTERFACE, ENUM, FUNCTION, INIT:
		return true
	default:
		return false
	}
}
