package core

import (
	"fmt"
	"sync"

	"github.com/jossecurity/joss/pkg/parser"
	runtimeframe "github.com/jossecurity/joss/pkg/runtime/frame"
	runtimeplan "github.com/jossecurity/joss/pkg/runtime/plan"
	"github.com/jossecurity/joss/pkg/typesystem"
)

// executionFrame is the hybrid fast-path frame. Source locals use compact
// slots; unresolved host/plugin/dynamic bindings continue through Runtime maps.
type executionFrame struct {
	plan         *runtimeplan.Callable
	slots        []runtimeframe.Slot
	allowDynamic bool
	defers       []*parser.DeferStatement
}

var executionFramePool = sync.Pool{New: func() interface{} { return &executionFrame{} }}

func acquireExecutionFrame(compiled *runtimeplan.Callable, allowDynamic bool) *executionFrame {
	frame := executionFramePool.Get().(*executionFrame)
	frame.plan = compiled
	frame.allowDynamic = allowDynamic
	frame.defers = nil
	if cap(frame.slots) < len(compiled.Slots) {
		frame.slots = make([]runtimeframe.Slot, len(compiled.Slots))
	} else {
		frame.slots = frame.slots[:len(compiled.Slots)]
	}
	for index, info := range compiled.Slots {
		typeName := info.TypeName
		valueType := info.Type
		if typeName == "var" {
			typeName = ""
			valueType = typesystem.Type{Kind: typesystem.Unknown}
		}
		frame.slots[index] = runtimeframe.Slot{
			Name: info.Name, TypeName: typeName, Type: valueType,
			Constant: info.Constant, Inferred: info.Inferred, ByReference: info.ByReference,
		}
	}
	return frame
}

func releaseExecutionFrame(frame *executionFrame) {
	if frame == nil {
		return
	}
	for index := range frame.slots {
		frame.slots[index].Clear()
	}
	frame.slots = frame.slots[:0]
	frame.defers = nil
	frame.plan = nil
	frame.allowDynamic = false
	executionFramePool.Put(frame)
}

func (r *Runtime) planForMethod(method *parser.MethodStatement) *runtimeplan.Callable {
	if method == nil {
		return nil
	}
	r.planMu.Lock()
	defer r.planMu.Unlock()
	if r.callablePlans == nil {
		r.callablePlans = make(map[*parser.MethodStatement]*runtimeplan.Callable)
	}
	if compiled := r.callablePlans[method]; compiled != nil {
		return compiled
	}
	owner := r.declaringClassOfMethod(method)
	compiled := runtimeplan.CompileMethod(method, true)
	compiled.Owner = owner
	r.callablePlans[method] = compiled
	return compiled
}

func (r *Runtime) planForFunction(function *parser.FunctionLiteral) *runtimeplan.Callable {
	if function == nil {
		return nil
	}
	r.planMu.Lock()
	defer r.planMu.Unlock()
	if r.functionPlans == nil {
		r.functionPlans = make(map[*parser.FunctionLiteral]*runtimeplan.Callable)
	}
	if compiled := r.functionPlans[function]; compiled != nil {
		return compiled
	}
	compiled := runtimeplan.CompileFunction(function)
	r.functionPlans[function] = compiled
	return compiled
}

func (r *Runtime) slotForIdentifier(identifier *parser.Identifier) (*runtimeframe.Slot, bool) {
	if r.currentFrame == nil || r.currentFrame.plan == nil || identifier == nil {
		return nil, false
	}
	index, exists := r.currentFrame.plan.IdentifierSlots[identifier]
	if !exists || index < 0 || index >= len(r.currentFrame.slots) {
		return nil, false
	}
	return &r.currentFrame.slots[index], true
}

func (r *Runtime) localValue(identifier *parser.Identifier) (interface{}, bool, bool) {
	slot, resolved := r.slotForIdentifier(identifier)
	if !resolved {
		return nil, false, false
	}
	if !slot.Initialized {
		return nil, true, false
	}
	value := slot.Value.Interface()
	if reference, ok := value.(*VariableReference); ok {
		return reference.Get(), true, true
	}
	return value, true, true
}

func (r *Runtime) assignLocal(identifier *parser.Identifier, value interface{}, initializing bool) (interface{}, bool) {
	slot, resolved := r.slotForIdentifier(identifier)
	if !resolved {
		return nil, false
	}
	return r.assignSlot(slot, value, initializing), true
}

func (r *Runtime) assignSlot(slot *runtimeframe.Slot, value interface{}, initializing bool) interface{} {
	if slot == nil {
		return value
	}
	if slot.Initialized && slot.ByReference && !initializing {
		if reference, ok := slot.Value.Interface().(*VariableReference); ok {
			return reference.Set(r, value)
		}
	}
	if slot.Constant && slot.Initialized && !initializing {
		panic(&JossError{Type: "ConstantAssignment", Message: fmt.Sprintf("La constante '%s' no puede reasignarse", slot.Name), File: r.CurrentFile})
	}
	if slot.TypeName != "" && slot.Type.Kind != typesystem.Mixed && slot.Type.Kind != typesystem.Unknown {
		value = r.coerceToParsedType(value, slot.Type)
		if !r.checkParsedType(value, slot.Type) {
			panic(&JossError{Type: "TypeError", Message: fmt.Sprintf("Error de Tipado: La variable '%s' requiere %s", slot.Name, slot.Type.String()), File: r.CurrentFile})
		}
	} else if slot.Inferred && value != nil {
		inferred := runtimeTypeOf(value)
		if inferred.IsKnown() {
			slot.Type = inferred
			slot.TypeName = inferred.String()
		}
	}
	slot.Set(value)
	return value
}

func (r *Runtime) bindSlot(index int, value interface{}) {
	if r.currentFrame == nil || index < 0 || index >= len(r.currentFrame.slots) {
		return
	}
	r.currentFrame.slots[index].Set(value)
}

func (r *Runtime) localBindingExists(identifier *parser.Identifier) (exists bool, initialized bool) {
	slot, exists := r.slotForIdentifier(identifier)
	return exists, exists && slot.Initialized
}

func (r *Runtime) sourceMapVisible(name string) bool {
	return r.currentFrame == nil || r.currentFrame.allowDynamic || r.HostGlobals[name]
}
