package pluginruntime

import (
	"fmt"
	"math"

	"github.com/jossecurity/joss/pkg/plugincompiler/ir"
)

// HostContext provee acceso a funciones y servicios del runtime principal de Joss.
type HostContext interface {
	CallHostFunction(name string, args []interface{}) (interface{}, error)
}

// Map of host functions to required permissions for PermissionGuard
var hostFunctionPermissions = map[string]string{
	"http_get":          "network.http",
	"http_post":         "network.http",
	"fetch":             "network.http",
	"file_read":         "filesystem.read",
	"file_get_contents": "filesystem.read",
	"file_exists":       "filesystem.read",
	"is_dir":            "filesystem.read",
	"is_file":           "filesystem.read",
	"hive_read_box":     "filesystem.read",
	"view":              "filesystem.read",
	"file_write":        "filesystem.write",
	"file_put_contents": "filesystem.write",
	"unlink":            "filesystem.write",
	"file_delete":       "filesystem.write",
	"mkdir":             "filesystem.write",
	"env_read":          "env.read",
	"env":               "env.read",
	"config":            "env.read",
	"env_write":         "env.write",
	"db_query":          "database.query",
	"db_exec":           "database.exec",
	"run":               "process.exec",
}

// JPBCVM es la maquina virtual para la ejecucion de bytecode multilenguaje JPBC.
type JPBCVM struct {
	Module      *JPBCModule
	Permissions *PermissionGuard
	Host        HostContext
}

// NewJPBCVM crea una nueva instancia de la maquina virtual JPBC.
func NewJPBCVM(module *JPBCModule, permissions *PermissionGuard, host HostContext) *JPBCVM {
	return &JPBCVM{
		Module:      module,
		Permissions: permissions,
		Host:        host,
	}
}

// Execute invoca una funcion por nombre dentro del modulo JPBC pasando argumentos.
func (vm *JPBCVM) Execute(fnName string, args []interface{}) (interface{}, error) {
	if vm.Module == nil {
		return nil, fmt.Errorf("jpbc_vm: modulo no cargado")
	}

	fn, ok := vm.Module.Functions[fnName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrFunctionNotFound, fnName)
	}

	return vm.executeFunction(fn, args)
}

const DefaultMaxInstructions = 1000000

func (vm *JPBCVM) executeFunction(fn *JPBCFunction, args []interface{}) (interface{}, error) {
	var acc interface{} = nil
	registers := make(map[string]interface{})
	variables := make(map[string]interface{})

	// Inicializar argumentos en variables locales $0, $1, ... y registro r0
	for i, arg := range args {
		variables[fmt.Sprintf("$%d", i)] = arg
		variables[fmt.Sprintf("arg%d", i)] = arg
		if i == 0 {
			registers["r0"] = arg
			acc = arg
		}
	}

	instructions := fn.Instructions
	instLen := len(instructions)
	pc := 0
	steps := 0

	for pc < instLen {
		steps++
		if steps > DefaultMaxInstructions {
			return nil, fmt.Errorf("%w: excedido limite de %d pasos en %s", ErrExecutionLimitExceeded, DefaultMaxInstructions, fn.Name)
		}
		inst := instructions[pc]

		switch inst.Op {
		case ir.OpNop:
			// No-op

		case ir.OpConst:
			if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				acc = vm.Module.ConstantPool[inst.ConstIdx]
				registers["r0"] = acc
			}

		case ir.OpLoad:
			if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				keyName := fmt.Sprintf("%v", vm.Module.ConstantPool[inst.ConstIdx])
				if val, exists := variables[keyName]; exists {
					acc = val
					registers["r0"] = acc
				}
			}

		case ir.OpStore:
			if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				keyName := fmt.Sprintf("%v", vm.Module.ConstantPool[inst.ConstIdx])
				variables[keyName] = acc
			}

		case ir.OpGetField:
			if objMap, ok := acc.(map[string]interface{}); ok && inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				fieldName := fmt.Sprintf("%v", vm.Module.ConstantPool[inst.ConstIdx])
				acc = objMap[fieldName]
				registers["r0"] = acc
			}

		case ir.OpSetField:
			if objMap, ok := acc.(map[string]interface{}); ok && inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				fieldName := fmt.Sprintf("%v", vm.Module.ConstantPool[inst.ConstIdx])
				objMap[fieldName] = registers["r0"]
			}

		case ir.OpAdd:
			if len(args) >= 2 {
				acc = addValues(args[0], args[1])
			} else if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				acc = addValues(acc, vm.Module.ConstantPool[inst.ConstIdx])
			}
			registers["r0"] = acc

		case ir.OpSub:
			if len(args) >= 2 {
				acc = subValues(args[0], args[1])
			} else if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				acc = subValues(acc, vm.Module.ConstantPool[inst.ConstIdx])
			}
			registers["r0"] = acc

		case ir.OpMul:
			if len(args) >= 2 {
				acc = mulValues(args[0], args[1])
			} else if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				acc = mulValues(acc, vm.Module.ConstantPool[inst.ConstIdx])
			}
			registers["r0"] = acc

		case ir.OpDiv:
			if len(args) >= 2 {
				var err error
				acc, err = divValues(args[0], args[1])
				if err != nil {
					return nil, err
				}
			} else if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				var err error
				acc, err = divValues(acc, vm.Module.ConstantPool[inst.ConstIdx])
				if err != nil {
					return nil, err
				}
			}
			registers["r0"] = acc

		case ir.OpMod:
			if len(args) >= 2 {
				acc = modValues(args[0], args[1])
			}
			registers["r0"] = acc

		case ir.OpCallStatic, ir.OpCallVirtual:
			if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				targetName := fmt.Sprintf("%v", vm.Module.ConstantPool[inst.ConstIdx])
				if targetFn, exists := vm.Module.Functions[targetName]; exists {
					var err error
					acc, err = vm.executeFunction(targetFn, []interface{}{acc})
					if err != nil {
						return nil, err
					}
					registers["r0"] = acc
				} else if vm.Host != nil {
					// PermissionGuard Enforcement (ALIM Capa 3 / PLMS Specification)
					if reqPerm, ok := hostFunctionPermissions[targetName]; ok {
						if vm.Permissions != nil && !vm.Permissions.HasPermission(reqPerm) {
							return nil, fmt.Errorf("error de permiso denegado: la función de host '%s' requiere el permiso '%s'", targetName, reqPerm)
						}
					}
					var err error
					acc, err = vm.Host.CallHostFunction(targetName, args)
					if err != nil {
						return nil, err
					}
					registers["r0"] = acc
				}
			}

		case ir.OpReturn:
			return acc, nil

		case ir.OpBranch:
			if inst.ConstIdx >= 0 && inst.ConstIdx < instLen {
				pc = inst.ConstIdx
				continue
			}

		case ir.OpBranchIf:
			if isTruthy(acc) && inst.ConstIdx >= 0 && inst.ConstIdx < instLen {
				pc = inst.ConstIdx
				continue
			}

		case ir.OpNewObject:
			obj := make(map[string]interface{})
			if inst.ConstIdx >= 0 && inst.ConstIdx < len(vm.Module.ConstantPool) {
				obj["__type__"] = vm.Module.ConstantPool[inst.ConstIdx]
			}
			acc = obj
			registers["r0"] = acc

		default:
			return nil, fmt.Errorf("jpbc_vm: opcode desconocido o no soportado (%d) en PC: %d", inst.Op, pc)
		}

		pc++
	}

	return acc, nil
}

func isTruthy(val interface{}) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case int, int64, int32:
		return v != 0
	case float64, float32:
		return v != 0.0
	case string:
		return v != "" && v != "0"
	default:
		return true
	}
}

func toFloat(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

func addValues(a, b interface{}) interface{} {
	fA, okA := toFloat(a)
	fB, okB := toFloat(b)
	if okA && okB {
		if math.Floor(fA) == fA && math.Floor(fB) == fB {
			return int64(fA) + int64(fB)
		}
		return fA + fB
	}
	return fmt.Sprintf("%v%v", a, b)
}

func subValues(a, b interface{}) interface{} {
	fA, okA := toFloat(a)
	fB, okB := toFloat(b)
	if okA && okB {
		if math.Floor(fA) == fA && math.Floor(fB) == fB {
			return int64(fA) - int64(fB)
		}
		return fA - fB
	}
	return 0.0
}

func mulValues(a, b interface{}) interface{} {
	fA, okA := toFloat(a)
	fB, okB := toFloat(b)
	if okA && okB {
		if math.Floor(fA) == fA && math.Floor(fB) == fB {
			return int64(fA) * int64(fB)
		}
		return fA * fB
	}
	return 0.0
}

func divValues(a, b interface{}) (interface{}, error) {
	fA, okA := toFloat(a)
	fB, okB := toFloat(b)
	if okA && okB {
		if fB == 0 {
			return nil, fmt.Errorf("jpbc_vm: division por cero")
		}
		return fA / fB, nil
	}
	return 0.0, nil
}

func modValues(a, b interface{}) interface{} {
	fA, okA := toFloat(a)
	fB, okB := toFloat(b)
	if okA && okB && int64(fB) != 0 {
		return int64(fA) % int64(fB)
	}
	return 0
}
