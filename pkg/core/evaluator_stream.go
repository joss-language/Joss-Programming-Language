package core

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
	runtimeframe "github.com/jossecurity/joss/pkg/runtime/frame"
	"github.com/shopspring/decimal"
)

func (r *Runtime) evaluateInputInfix(left interface{}, expression *parser.InfixExpression) (interface{}, bool) {
	if expression.Operator != ">>" {
		return nil, false
	}
	if _, ok := left.(*Cin); !ok {
		return nil, false
	}
	if nonInteractive, ok := r.Env["NON_INTERACTIVE"]; ok && (nonInteractive == "true" || nonInteractive == "1") {
		fmt.Println("[Cin] Input skipped (NON_INTERACTIVE mode)")
		return nil, true
	}

	identifier, ok := expression.Right.(*parser.Identifier)
	if !ok {
		fmt.Println("Error: cin >> requiere una variable")
		return nil, true
	}
	value := r.readCinInputForIdentifier(identifier)
	if _, resolved := r.assignLocal(identifier, value, false); resolved {
		return left, true
	}
	if reference, exists := r.Variables[identifier.Value].(*VariableReference); exists {
		reference.Set(r, value)
		return left, true
	}
	if expectedType, exists := r.VarTypes[identifier.Value]; exists {
		value = r.coerceToTypedValue(value, expectedType)
		if !r.checkType(value, expectedType) {
			fmt.Printf("Error de Tipado: No se puede asignar valor a '%s' (se espera %s)\n", identifier.Value, expectedType)
			return nil, true
		}
	}
	r.Variables[identifier.Value] = value
	return left, true
}

func (r *Runtime) evaluateOutputInfix(left, right interface{}, operator string) (interface{}, bool) {
	if operator != "<<" {
		return nil, false
	}
	switch stream := left.(type) {
	case *Cout:
		fmt.Print(right)
		return stream, true
	case *Cerr:
		fmt.Fprint(os.Stderr, right)
		return stream, true
	case *Channel:
		stream.Ch <- right
		return stream, true
	default:
		return nil, false
	}
}

func (r *Runtime) readCinInputForIdentifier(identifier *parser.Identifier) interface{} {
	targetType := ""
	if slot, resolved := r.slotForIdentifier(identifier); resolved {
		if slot.TypeName != "" {
			targetType = slot.TypeName
		} else if slot.Value.Kind == runtimeframe.Int {
			targetType = "int"
		} else if slot.Value.Kind == runtimeframe.Float {
			targetType = "float"
		} else if slot.Value.Kind == runtimeframe.String {
			targetType = "string"
		}
	} else if typeName, ok := r.VarTypes[identifier.Value]; ok {
		targetType = typeName
	} else if existing, ok := r.Variables[identifier.Value]; ok {
		switch existing.(type) {
		case int, int64, int32:
			targetType = "int"
		case float64, float32:
			targetType = "float"
		case string:
			targetType = "string"
		}
	}

	var raw string
	if len(r.cinTokens) > 0 {
		raw = r.cinTokens[0]
		r.cinTokens = r.cinTokens[1:]
	} else {
		if r.cinReader == nil {
			r.cinReader = bufio.NewReader(os.Stdin)
		}
		line, err := r.cinReader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return ""
		}
		line = strings.TrimRight(line, "\r\n")

		if targetType == "int" || targetType == "float" || targetType == "decimal" {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				raw = fields[0]
				r.cinTokens = fields[1:]
			} else if len(fields) == 1 {
				raw = fields[0]
			}
		} else {
			raw = line
		}
	}

	trimmed := strings.TrimSpace(raw)
	switch targetType {
	case "int":
		if number, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return number
		}
	case "float":
		if number, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return number
		}
	case "decimal":
		if number, err := decimal.NewFromString(strings.TrimRight(trimmed, "mMdD")); err == nil {
			return number
		}
	case "string":
		return raw
	default:
		if number, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return number
		}
		if number, err := strconv.ParseFloat(trimmed, 64); err == nil && strings.Contains(trimmed, ".") {
			return number
		}
	}
	return raw
}
