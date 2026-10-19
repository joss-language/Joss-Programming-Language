package vm

import "fmt"

const StackMax = 256
const MaxFrames = 64

type CallFrame struct {
	Chunk *Chunk
	IP    int
	BP    int // Base pointer in stack
}

type VM struct {
	frames [MaxFrames]CallFrame
	fp     int
	stack  [StackMax]Value
	sp     int
	locals [256]Value
}

func NewVM() *VM {
	return &VM{}
}

func (vm *VM) push(v Value) {
	vm.stack[vm.sp] = v
	vm.sp++
}

func (vm *VM) pop() Value {
	vm.sp--
	return vm.stack[vm.sp]
}

func (vm *VM) Run(chunk *Chunk) (Value, error) {
	vm.fp = 0
	vm.sp = 0
	vm.frames[0] = CallFrame{Chunk: chunk, IP: 0, BP: 0}

	for vm.fp >= 0 {
		frame := &vm.frames[vm.fp]
		if frame.IP >= len(frame.Chunk.Code) {
			break
		}

		op := Opcode(frame.Chunk.Code[frame.IP])
		frame.IP++

		switch op {
		case OpHalt:
			if vm.sp > 0 {
				return vm.pop(), nil
			}
			return NullVal(), nil

		case OpConst:
			idx := frame.Chunk.Code[frame.IP]
			frame.IP++
			vm.push(frame.Chunk.Constants[idx])

		case OpLoadLocal:
			slot := frame.Chunk.Code[frame.IP]
			frame.IP++
			vm.push(vm.locals[frame.BP+int(slot)])

		case OpStoreLocal:
			slot := frame.Chunk.Code[frame.IP]
			frame.IP++
			val := vm.pop()
			vm.locals[frame.BP+int(slot)] = val

		case OpAddInt:
			b := vm.pop()
			a := vm.pop()
			vm.push(IntVal(a.Integer + b.Integer))

		case OpSubInt:
			b := vm.pop()
			a := vm.pop()
			vm.push(IntVal(a.Integer - b.Integer))

		case OpMulInt:
			b := vm.pop()
			a := vm.pop()
			vm.push(IntVal(a.Integer * b.Integer))

		case OpDivFloat:
			b := vm.pop()
			a := vm.pop()
			if b.Integer == 0 {
				return NullVal(), fmt.Errorf("division by zero")
			}
			vm.push(FloatVal(float64(a.Integer) / float64(b.Integer)))

		case OpModInt:
			b := vm.pop()
			a := vm.pop()
			if b.Integer == 0 {
				return NullVal(), fmt.Errorf("modulo by zero")
			}
			vm.push(IntVal(a.Integer % b.Integer))

		case OpNegInt:
			a := vm.pop()
			vm.push(IntVal(-a.Integer))

		case OpCmpLt:
			b := vm.pop()
			a := vm.pop()
			vm.push(BoolVal(a.Integer < b.Integer))

		case OpCmpLe:
			b := vm.pop()
			a := vm.pop()
			vm.push(BoolVal(a.Integer <= b.Integer))

		case OpCmpGt:
			b := vm.pop()
			a := vm.pop()
			vm.push(BoolVal(a.Integer > b.Integer))

		case OpCmpGe:
			b := vm.pop()
			a := vm.pop()
			vm.push(BoolVal(a.Integer >= b.Integer))

		case OpCmpEq:
			b := vm.pop()
			a := vm.pop()
			vm.push(BoolVal(a.Integer == b.Integer))

		case OpCmpNe:
			b := vm.pop()
			a := vm.pop()
			vm.push(BoolVal(a.Integer != b.Integer))

		case OpJump:
			offset := int(int16(uint16(frame.Chunk.Code[frame.IP])<<8 | uint16(frame.Chunk.Code[frame.IP+1])))
			frame.IP += 2
			frame.IP += offset

		case OpJumpIfFalse:
			offset := int(int16(uint16(frame.Chunk.Code[frame.IP])<<8 | uint16(frame.Chunk.Code[frame.IP+1])))
			frame.IP += 2
			cond := vm.pop()
			if !cond.IsTruthy() {
				frame.IP += offset
			}

		case OpJumpIfTrue:
			offset := int(int16(uint16(frame.Chunk.Code[frame.IP])<<8 | uint16(frame.Chunk.Code[frame.IP+1])))
			frame.IP += 2
			cond := vm.pop()
			if cond.IsTruthy() {
				frame.IP += offset
			}

		case OpReturn:
			retVal := NullVal()
			if vm.sp > 0 {
				retVal = vm.pop()
			}
			vm.fp--
			if vm.fp >= 0 {
				vm.push(retVal)
			} else {
				return retVal, nil
			}

		case OpPop:
			vm.pop()

		default:
			return NullVal(), fmt.Errorf("unknown opcode %d", op)
		}
	}

	if vm.sp > 0 {
		return vm.pop(), nil
	}
	return NullVal(), nil
}
