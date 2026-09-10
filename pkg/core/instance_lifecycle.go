package core

import (
	"io"
	"reflect"

	"github.com/jossecurity/joss/pkg/parser"
)

// AutoDestroy safely finalizes an instance by invoking its destructor hook (if any),
// closing any held resources (io.Closer), purging instance fields, and marking it destroyed.
func (i *Instance) AutoDestroy(r *Runtime, skipHook ...bool) {
	if i == nil {
		return
	}
	i.Mu.Lock()
	if i.Destroyed || i.Destroying {
		i.Mu.Unlock()
		return
	}
	i.Destroying = true
	i.Mu.Unlock()

	// 1. If the class defines a destructor method, invoke it safely before zeroing fields
	runHook := len(skipHook) == 0 || !skipHook[0]
	if runHook && r != nil && i.Class != nil {
		destructor := i.findDestructor()
		if destructor != nil {
			func() {
				defer func() {
					_ = recover() // Catch any panic in destructor so it doesn't crash the host or GC
				}()
				forked := r.Fork()
				forked.CallMethodEvaluated(destructor, i, nil)
			}()
		}
	}

	i.Mu.Lock()
	i.Destroyed = true
	i.Destroying = false

	// 2. Clean and close any native resources held in fields
	for k, v := range i.Fields {
		closeFieldResource(v)
		delete(i.Fields, k)
	}
	i.Mu.Unlock()
}

func (i *Instance) findDestructor() *parser.MethodStatement {
	if i.Class == nil || i.Class.Body == nil {
		return nil
	}
	for _, stmt := range i.Class.Body.Statements {
		if m, ok := stmt.(*parser.MethodStatement); ok {
			name := m.Name.Value
			if name == "destructor" || name == "destroy" || name == "__destruct" {
				return m
			}
		}
	}
	return nil
}

func closeFieldResource(val interface{}) {
	if val == nil {
		return
	}
	if closer, ok := val.(io.Closer); ok {
		_ = closer.Close()
		return
	}
	// Check for Channel
	if ch, ok := val.(*Channel); ok && ch != nil && ch.Ch != nil {
		func() {
			defer func() { _ = recover() }()
			close(ch.Ch)
		}()
		return
	}
	// Reflect check for Close() method with 0 arguments
	rv := reflect.ValueOf(val)
	if rv.IsValid() {
		closeMethod := rv.MethodByName("Close")
		if closeMethod.IsValid() && closeMethod.Type().NumIn() == 0 {
			_ = closeMethod.Call(nil)
		}
	}
}
