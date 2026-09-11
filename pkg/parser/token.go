package parser

import (
	"fmt"
	"sort"
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

var multiCharOperators = []string{
	// Length 3
	STRICT_EQ, STRICT_NOT_EQ, SPACESHIP, ELLIPSIS, NULL_SAFE_ARROW, NULL_COALESCE_ASSIGN,
	// Length 2
	EQ, NOT_EQ, LTE, GTE, SHIFT_LEFT, SHIFT_RIGHT, AND, OR, INCREMENT, DECREMENT,
	RANGE, ARROW, DOUBLE_COLON, PIPE, NULL_COALESCE, FAT_ARROW,
	PLUS_ASSIGN, MINUS_ASSIGN, ASTERISK_ASSIGN, SLASH_ASSIGN,
}

// MultiCharOperators returns all multi-character operators defined in Joss,
// sorted by length descending so scanners and formatters can match them dynamically.
func MultiCharOperators() []string {
	cp := make([]string, len(multiCharOperators))
	copy(cp, multiCharOperators)
	return cp
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
