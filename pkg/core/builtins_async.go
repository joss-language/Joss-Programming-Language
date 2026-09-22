package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) callBuiltinAsync(name string, args []interface{}) (interface{}, bool) {
	switch name {
	case "async":
		if len(args) == 1 {
			future := &Future{
				done: make(chan bool),
			}
			argVal := args[0]
			newR := r.Fork() // Fork BEFORE starting the goroutine to avoid race
			go func() {
				defer func() {
					if p := recover(); p != nil {
						if rp, ok := p.(*ReturnPanic); ok {
							future.result = rp.Value
						} else {
							fmt.Printf("[ASYNC PANIC] %v\n", p)
							future.err = fmt.Errorf("%v", p)
						}
					}
					newR.Free()
					close(future.done)
				}()

				if captured, ok := argVal.(*CapturedFunction); ok {
					future.result = newR.callCapturedFunction(captured, nil)
				} else if fn, ok := argVal.(*parser.FunctionLiteral); ok {
					future.result = newR.executeBlock(fn.Body)
				} else if blk, ok := argVal.(*parser.BlockStatement); ok {
					future.result = newR.executeBlock(blk)
				} else {
					future.result = argVal
				}
			}()
			return future, true
		}
		return nil, true

	case "await":
		if len(args) == 1 {
			if future, ok := args[0].(*Future); ok {
				if r.executionContext != nil {
					select {
					case <-future.done:
					case <-r.executionContext.Done():
						r.checkExecutionCancelled()
					}
				}
				return future.Wait(), true
			}
		}
		return nil, true

	case "make_chan":
		size := 0
		if len(args) > 0 {
			if s, ok := args[0].(int64); ok {
				if s < 0 || int64(int(s)) != s {
					panic(&JossError{Type: "ChannelError", Code: diagnostics.CodeChannelCapacity, Message: "La capacidad del canal debe ser un entero no negativo representable"})
				}
				size = int(s)
			}
		}
		return NewChannel(size), true

	case "close":
		if len(args) == 1 {
			if ch, ok := args[0].(*Channel); ok {
				ch.Close()
				return nil, true
			}
		}
		return nil, true

	case "send":
		if len(args) == 2 {
			if ch, ok := args[0].(*Channel); ok {
				if r.executionContext != nil {
					if err := ch.TrySendContext(r.executionContext, args[1]); err != nil {
						panic(err)
					}
				} else {
					ch.Send(args[1])
				}
				return nil, true
			}
		}
		return nil, true

	case "recv":
		if len(args) == 1 {
			if ch, ok := args[0].(*Channel); ok {
				var val interface{}
				var ok bool
				if r.executionContext != nil {
					select {
					case val, ok = <-ch.Ch:
					case <-r.executionContext.Done():
						r.checkExecutionCancelled()
					}
				} else {
					val, ok = <-ch.Ch
				}
				if !ok {
					return nil, true
				}
				return val, true
			}
		}
		return nil, true
	}

	return nil, false
}
