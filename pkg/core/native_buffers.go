package core

import (
	"fmt"
	"strconv"
	"strings"
)

type jossByteBufferState struct {
	data  []byte
	freed bool
}

func getByteBufferState(instance *Instance) *jossByteBufferState {
	if instance == nil || instance.Fields == nil {
		return nil
	}
	if state, ok := instance.Fields["__buf"].(*jossByteBufferState); ok {
		return state
	}
	return nil
}

func (r *Runtime) executeByteBufferMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "Init", "new":
		capVal := 64
		if len(args) > 0 {
			if n := toInt64(args[0]); n > 0 {
				capVal = int(n)
			}
		}
		target := instance
		if target == nil {
			target = &Instance{Class: r.Classes["ByteBuffer"], Fields: make(map[string]interface{})}
		} else if target.Fields == nil {
			target.Fields = make(map[string]interface{})
		}
		target.Fields["__buf"] = &jossByteBufferState{data: make([]byte, 0, capVal)}
		return target

	case "appendByte":
		state := getByteBufferState(instance)
		if state == nil || state.freed {
			panic(&JossError{Type: "MemoryError", Message: "ByteBuffer: use after free", File: r.CurrentFile})
		}
		if len(args) > 0 {
			v := toInt64(args[0])
			state.data = append(state.data, byte(v&0xFF))
		}
		return nil

	case "appendString":
		state := getByteBufferState(instance)
		if state == nil || state.freed {
			panic(&JossError{Type: "MemoryError", Message: "ByteBuffer: use after free", File: r.CurrentFile})
		}
		if len(args) > 0 {
			state.data = append(state.data, fmt.Sprint(args[0])...)
		}
		return nil

	case "get":
		state := getByteBufferState(instance)
		if state == nil || state.freed {
			panic(&JossError{Type: "MemoryError", Message: "ByteBuffer: use after free", File: r.CurrentFile})
		}
		if len(args) == 0 {
			panic(&JossError{Type: "BoundsError", Message: "ByteBuffer.get: missing index", File: r.CurrentFile})
		}
		idx := toInt64(args[0])
		if idx < 0 || int(idx) >= len(state.data) {
			panic(&JossError{Type: "BoundsError", Message: fmt.Sprintf("ByteBuffer.get: index %d out of bounds (len=%d)", idx, len(state.data)), File: r.CurrentFile})
		}
		return int64(state.data[idx])

	case "set":
		state := getByteBufferState(instance)
		if state == nil || state.freed {
			panic(&JossError{Type: "MemoryError", Message: "ByteBuffer: use after free", File: r.CurrentFile})
		}
		if len(args) < 2 {
			panic(&JossError{Type: "BoundsError", Message: "ByteBuffer.set: missing index or value", File: r.CurrentFile})
		}
		idx := toInt64(args[0])
		val := toInt64(args[1])
		if idx < 0 || int(idx) >= len(state.data) {
			panic(&JossError{Type: "BoundsError", Message: fmt.Sprintf("ByteBuffer.set: index %d out of bounds (len=%d)", idx, len(state.data)), File: r.CurrentFile})
		}
		state.data[idx] = byte(val & 0xFF)
		return nil

	case "len":
		state := getByteBufferState(instance)
		if state == nil || state.freed {
			return int64(0)
		}
		return int64(len(state.data))

	case "toString":
		state := getByteBufferState(instance)
		if state == nil || state.freed {
			return ""
		}
		return string(state.data)

	case "free":
		state := getByteBufferState(instance)
		if state != nil {
			state.data = nil
			state.freed = true
		}
		return nil
	}
	return nil
}

type jossStringBuilderState struct {
	sb    strings.Builder
	freed bool
}

func getStringBuilderState(instance *Instance) *jossStringBuilderState {
	if instance == nil || instance.Fields == nil {
		return nil
	}
	if state, ok := instance.Fields["__sb"].(*jossStringBuilderState); ok {
		return state
	}
	return nil
}

func (r *Runtime) executeStringBuilderMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "Init", "new":
		target := instance
		if target == nil {
			target = &Instance{Class: r.Classes["StringBuilder"], Fields: make(map[string]interface{})}
		} else if target.Fields == nil {
			target.Fields = make(map[string]interface{})
		}
		target.Fields["__sb"] = &jossStringBuilderState{}
		return target

	case "append":
		state := getStringBuilderState(instance)
		if state == nil || state.freed {
			panic(&JossError{Type: "MemoryError", Message: "StringBuilder: use after free", File: r.CurrentFile})
		}
		if len(args) > 0 {
			state.sb.WriteString(fmt.Sprint(args[0]))
		}
		return nil

	case "appendInt":
		state := getStringBuilderState(instance)
		if state == nil || state.freed {
			panic(&JossError{Type: "MemoryError", Message: "StringBuilder: use after free", File: r.CurrentFile})
		}
		if len(args) > 0 {
			v := toInt64(args[0])
			state.sb.WriteString(strconv.FormatInt(v, 10))
		}
		return nil

	case "len":
		state := getStringBuilderState(instance)
		if state == nil || state.freed {
			return int64(0)
		}
		return int64(state.sb.Len())

	case "toString":
		state := getStringBuilderState(instance)
		if state == nil || state.freed {
			return ""
		}
		return state.sb.String()

	case "free":
		state := getStringBuilderState(instance)
		if state != nil {
			state.sb.Reset()
			state.freed = true
		}
		return nil
	}
	return nil
}
