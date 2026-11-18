package ir

// lowerJossHostServiceCalls binds host-shaped API calls to their canonical
// Joss implementation when a module provides one. The platform-facing
// CallHostService instruction remains only for services without a Joss owner.
func (l *Lowerer) lowerJossHostServiceCalls() {
	if l.prog == nil {
		return
	}
	var rewrite func(*Function, []*BasicBlock)
	rewrite = func(fn *Function, blocks []*BasicBlock) {
		for _, block := range blocks {
			instructions := make([]Instruction, 0, len(block.Instructions))
			for _, instruction := range block.Instructions {
				if call, ok := instruction.(*CallRuntimeInst); ok {
					owner := "__joss_host_" + call.Func
					if ownerFn := l.prog.Functions[owner]; ownerFn != nil {
						hostCall := &CallHostServiceInst{Dest: call.Dest, Args: call.Args}
						if args, matches := hostServiceOwnerArguments(ownerFn, hostCall); matches {
							instructions = append(instructions, &CallInst{
								Dest: call.Dest, Callee: owner, Args: args,
							})
							continue
						}
					}
				}
				if builtin, ok := instruction.(*CallBuiltinInst); ok && builtin.BuiltinID == "json_decode" {
					owner := l.prog.Functions["__joss_builtin_json_decode"]
					if owner != nil && len(builtin.Args) == len(owner.Params) &&
						builtin.Dest != nil && owner.ReturnType.Equal(builtin.Dest.Type()) &&
						len(builtin.Args) == 1 && hostServiceArgumentMatches(owner.Params[0].Type(), builtin.Args[0].Type()) {
						instructions = append(instructions, &CallInst{
							Dest: builtin.Dest, Callee: owner.Name, Args: builtin.Args,
						})
						continue
					}
				}
				if call, ok := instruction.(*CallHostServiceInst); ok {
					owner := "__joss_host_" + call.ClassName + "_" + call.MethodName
					ownerCall := call
					if call.ClassName == "Request" && call.Recv != nil {
						// Request is a request-scoped facade. Its object receiver is
						// compatibility syntax; canonical Joss owners read the same
						// request context through Request::input.
						copy := *call
						copy.Recv = nil
						ownerCall = &copy
					}
					if fn := l.prog.Functions[owner]; hostServiceOwnerMatches(fn, ownerCall) {
						args, _ := hostServiceOwnerArguments(fn, ownerCall)
						instructions = append(instructions, &CallInst{
							Dest: call.Dest, Callee: owner, Args: args,
						})
						continue
					}
				}
				if nested, ok := instruction.(*TryCatchInst); ok {
					rewrite(fn, nested.TryBlocks)
					rewrite(fn, nested.CatchBlocks)
				}
				instructions = append(instructions, instruction)
			}
			block.Instructions = instructions
		}
	}
	for _, fn := range l.prog.Functions {
		rewrite(fn, fn.Blocks)
	}
}

// hostServiceOwnerMatches prevents a same-named Joss function from silently
// changing a platform service's ABI or value semantics. Instance receivers
// are passed as the first source argument; static services have no receiver.
func hostServiceOwnerMatches(owner *Function, call *CallHostServiceInst) bool {
	_, ok := hostServiceOwnerArguments(owner, call)
	return ok
}

func hostServiceOwnerArguments(owner *Function, call *CallHostServiceInst) ([]Value, bool) {
	if owner == nil || call == nil {
		return nil, false
	}
	args := make([]Value, 0, len(call.Args)+1)
	if call.Recv != nil {
		args = append(args, call.Recv)
	}
	args = append(args, call.Args...)
	if len(args) > len(owner.Params) {
		return nil, false
	}
	for index, arg := range args {
		if arg == nil || !hostServiceArgumentMatches(owner.Params[index].Type(), arg.Type()) {
			return nil, false
		}
	}
	for index := len(args); index < len(owner.Params); index++ {
		if index >= len(owner.DefaultArgs) || owner.DefaultArgs[index] == nil {
			return nil, false
		}
		defaultValue := owner.DefaultArgs[index]
		if !hostServiceArgumentMatches(owner.Params[index].Type(), defaultValue.Type()) {
			return nil, false
		}
		args = append(args, defaultValue)
	}
	if call.Dest == nil {
		return args, owner.ReturnType.Kind == TypeKindVoid
	}
	return args, owner.ReturnType.Equal(call.Dest.Type())
}

func hostServiceArgumentMatches(parameter, argument Type) bool {
	return parameter.Kind == TypeKindMixed || parameter.Equal(argument)
}
