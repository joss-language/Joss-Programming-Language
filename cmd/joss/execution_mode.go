package main

import "strings"

type executionMode uint8

const (
	executionModePrevious executionMode = iota
	executionModeNative
)

func executionModeName(mode executionMode) string {
	if mode == executionModeNative {
		return "native"
	}
	return "anterior"
}

// takeExecutionMode removes the shared --native selector from command args.
// The previous Go runtime is the default so old projects keep working while
// native coverage is migrated capability by capability.
func takeExecutionMode(args []string) (executionMode, []string) {
	mode := executionModePrevious
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.EqualFold(arg, "--native") {
			mode = executionModeNative
			continue
		}
		filtered = append(filtered, arg)
	}
	return mode, filtered
}
