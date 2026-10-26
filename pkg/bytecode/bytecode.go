package bytecode

import (
	"bytes"
	"compress/flate"
	"encoding/gob"
	"fmt"
	"io"
	"sync"

	"github.com/jossecurity/joss/pkg/parser"
)

const MaxProgramSize = 32 << 20

var (
	magicCompressed = []byte{'J', 'O', 'S', 'S', 'B', 'C', '2', 'Z'}
	registerOnce    sync.Once
)

// Encode compiles a parsed Joss program into an optimized, compressed JP v2 bytecode payload.
// It stores the AST, compressed using flate.BestCompression for minimal size on disk and in memory.
func Encode(program *parser.Program) ([]byte, error) {
	if program == nil {
		return nil, fmt.Errorf("bytecode: programa nil")
	}
	registerAST()

	var body bytes.Buffer
	if err := gob.NewEncoder(&body).Encode(program); err != nil {
		return nil, fmt.Errorf("bytecode: no se pudo codificar: %w", err)
	}
	if body.Len() > MaxProgramSize {
		return nil, fmt.Errorf("bytecode: el programa excede %d MiB", MaxProgramSize>>20)
	}

	// Compress gob payload for maximum size optimization
	var compressed bytes.Buffer
	fw, err := flate.NewWriter(&compressed, flate.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("bytecode: error inicializando compresor: %w", err)
	}
	if _, err := fw.Write(body.Bytes()); err != nil {
		_ = fw.Close()
		return nil, fmt.Errorf("bytecode: error comprimiendo payload: %w", err)
	}
	if err := fw.Close(); err != nil {
		return nil, fmt.Errorf("bytecode: error cerrando compresor: %w", err)
	}

	result := make([]byte, 0, len(magicCompressed)+compressed.Len())
	result = append(result, magicCompressed...)
	result = append(result, compressed.Bytes()...)
	return result, nil
}

// Decode restores a precompiled Joss program from the current compressed JP v2 payload.
func Decode(data []byte) (*parser.Program, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("bytecode: payload demasiado corto")
	}

	registerAST()
	var program parser.Program
	var reader io.Reader

	if bytes.Equal(data[:8], magicCompressed) {
		fr := flate.NewReader(bytes.NewReader(data[8:]))
		defer fr.Close()
		uncompressed, err := io.ReadAll(io.LimitReader(fr, MaxProgramSize+1))
		if err != nil {
			return nil, fmt.Errorf("bytecode: error descomprimiendo payload: %w", err)
		}
		if len(uncompressed) > MaxProgramSize {
			return nil, fmt.Errorf("bytecode: payload descomprimido excede %d MiB", MaxProgramSize>>20)
		}
		reader = bytes.NewReader(uncompressed)
	} else {
		return nil, fmt.Errorf("bytecode: cabecera JP v2 invalida")
	}

	if err := gob.NewDecoder(reader).Decode(&program); err != nil {
		return nil, fmt.Errorf("bytecode: payload invalido: %w", err)
	}
	return &program, nil
}

func IsBytecode(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[:8], magicCompressed)
}

func registerAST() {
	registerOnce.Do(func() {
		// Statements stored behind parser.Statement interfaces.
		gob.Register(&parser.LetStatement{})
		gob.Register(&parser.MultiLetStatement{})
		gob.Register(&parser.ExpressionStatement{})
		gob.Register(&parser.ClassStatement{})
		gob.Register(&parser.BlockStatement{})
		gob.Register(&parser.EchoStatement{})
		gob.Register(&parser.InitStatement{})
		gob.Register(&parser.ForeachStatement{})
		gob.Register(&parser.MethodStatement{})
		gob.Register(&parser.WhileStatement{})
		gob.Register(&parser.DoWhileStatement{})
		gob.Register(&parser.TryCatchStatement{})
		gob.Register(&parser.ThrowStatement{})
		gob.Register(&parser.ReturnStatement{})
		gob.Register(&parser.BreakStatement{})
		gob.Register(&parser.ContinueStatement{})

		// Expressions stored behind parser.Expression interfaces.
		gob.Register(&parser.Identifier{})
		gob.Register(&parser.StringLiteral{})
		gob.Register(&parser.CallExpression{})
		gob.Register(&parser.TernaryExpression{})
		gob.Register(&parser.InfixExpression{})
		gob.Register(&parser.PrefixExpression{})
		gob.Register(&parser.PostfixExpression{})
		gob.Register(&parser.Boolean{})
		gob.Register(&parser.NullLiteral{})
		gob.Register(&parser.IntegerLiteral{})
		gob.Register(&parser.FloatLiteral{})
		gob.Register(&parser.ArrayLiteral{})
		gob.Register(&parser.MapLiteral{})
		gob.Register(&parser.IndexExpression{})
		gob.Register(&parser.FunctionLiteral{})
		gob.Register(&parser.NewExpression{})
		gob.Register(&parser.MemberExpression{})
		gob.Register(&parser.AssignExpression{})
		gob.Register(&parser.IssetExpression{})
		gob.Register(&parser.EmptyExpression{})
		gob.Register(&parser.BlockExpression{})
		gob.Register(&parser.MatchExpression{})

		// Auxiliary structs and types within AST nodes
		gob.Register(&parser.Parameter{})
		gob.Register(parser.SingleDecl{})
		gob.Register(parser.MatchArm{})
		gob.Register(parser.Token{})
		gob.Register(parser.TokenType(""))
	})
}
