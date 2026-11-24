package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/parser"
)

type executionMode uint8

const (
	executionModeNative executionMode = iota
)

func executionModeName(mode executionMode) string {
	return "native"
}

// Joss runs exclusively on the Joss Native HTTP & execution engine.
func serverBackendForMode(mode executionMode) string {
	return "native"
}

func shouldUseGoHTTPRuntime(mode executionMode, backend string) bool {
	return false
}

func preparedProgramUsesHTTPServer(prepared *semanticanalyzer.PreparedProgram) bool {
	if prepared == nil || prepared.Facts == nil {
		return false
	}
	for _, unit := range prepared.Units {
		// Standard Library definitions include their own Server::start call;
		// only project code that invokes a server selects the HTTP server supervisor.
		if strings.HasPrefix(strings.ReplaceAll(unit.Path, `\`, "/"), "<std/") {
			continue
		}
		if programCallsHTTPServer(prepared, unit.Program) {
			return true
		}
	}
	return false
}

func programCallsHTTPServer(prepared *semanticanalyzer.PreparedProgram, program *parser.Program) bool {
	if program == nil {
		return false
	}
	seen := make(map[uintptr]struct{})
	var visit func(reflect.Value) bool
	visit = func(value reflect.Value) bool {
		if !value.IsValid() {
			return false
		}
		switch value.Kind() {
		case reflect.Interface:
			if value.IsNil() {
				return false
			}
			return visit(value.Elem())
		case reflect.Pointer:
			if value.IsNil() {
				return false
			}
			pointer := value.Pointer()
			if _, exists := seen[pointer]; exists {
				return false
			}
			seen[pointer] = struct{}{}
			if call, ok := value.Interface().(*parser.CallExpression); ok {
				if fact, exists := prepared.ResolvedCall(call); exists && isHTTPServerStartTarget(fact.TargetID) {
					return true
				}
			}
			return visit(value.Elem())
		case reflect.Struct:
			for i := 0; i < value.NumField(); i++ {
				if visit(value.Field(i)) {
					return true
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				if visit(value.Index(i)) {
					return true
				}
			}
		case reflect.Map:
			iter := value.MapRange()
			for iter.Next() {
				if visit(iter.Value()) {
					return true
				}
			}
		}
		return false
	}
	return visit(reflect.ValueOf(program))
}

func isHTTPServerStartTarget(target string) bool {
	return target == "Server::start" || target == "HttpServer::start" || target == "HttpServer::run" ||
		strings.HasSuffix(target, ":Server::start") || strings.HasSuffix(target, ":HttpServer::start") || strings.HasSuffix(target, ":HttpServer::run")
}

// takeExecutionMode strips optional --native selector (now always active) and
// warns if deprecated --anterior / --go flags are supplied.
func takeExecutionMode(args []string) (executionMode, []string) {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.EqualFold(arg, "--native") {
			continue
		}
		if strings.EqualFold(arg, "--anterior") || strings.EqualFold(arg, "--go") {
			fmt.Fprintln(os.Stderr, "[aviso] El motor de ejecución Go fue retirado; Joss ejecuta exclusivamente en modo Joss Native.")
			continue
		}
		filtered = append(filtered, arg)
	}
	return executionModeNative, filtered
}

func isNativeCapabilityGap(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "JOSS-NATIVE-001") || strings.Contains(message, "capacidades AOT no disponibles")
}
