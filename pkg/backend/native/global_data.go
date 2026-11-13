package native

import (
	"fmt"
	"sort"

	"github.com/jossecurity/joss/pkg/ir"
)

const (
	// 0x800..0x83F holds the Windows IAT and 0x840..0x8FF is reserved.
	nativeGlobalDataStart uint32 = 0x900
	nativeGlobalDataEnd   uint32 = dataFixedSize
)

// assignNativeGlobalOffsets reserves deterministic pointer-sized slots in the
// standalone runtime data segment after the fixed OS/runtime fields.
func assignNativeGlobalOffsets(program *ir.Program) error {
	if program == nil || len(program.Globals) == 0 {
		return nil
	}
	if nativeGlobalDataStart+uint32(len(program.Globals))*8 > nativeGlobalDataEnd {
		return fmt.Errorf("native backend: %d global values exceed reserved data capacity", len(program.Globals))
	}
	names := make([]string, 0, len(program.Globals))
	for name := range program.Globals {
		names = append(names, name)
	}
	sort.Strings(names)
	for index, name := range names {
		program.Globals[name].DataOffset = nativeGlobalDataStart + uint32(index)*8
	}
	return nil
}

func nativeGlobalOffset(program *ir.Program, name string) (uint32, bool) {
	if program == nil {
		return 0, false
	}
	global, ok := program.Globals[name]
	if !ok || global == nil || global.DataOffset < nativeGlobalDataStart || global.DataOffset+8 > nativeGlobalDataEnd {
		return 0, false
	}
	return global.DataOffset, true
}
