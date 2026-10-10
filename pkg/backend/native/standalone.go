package native

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/jossecurity/joss/pkg/ir"
)

// StandaloneBuilder produce un ejecutable nativo puro y mínimo compilando
// las funciones lineales de Joss IR sin cargar AST, VFS ni evaluadores.
type StandaloneBuilder struct{}

func NewStandaloneBuilder() *StandaloneBuilder {
	return &StandaloneBuilder{}
}

// GenerateSource expone el código Go generado a partir de Joss IR para trazabilidad y depuración.
func (b *StandaloneBuilder) GenerateSource(prog *ir.Program) string {
	return b.generateNativeRunnerSource(prog)
}

// BuildNativeExecutable compila un ir.Program a un binario nativo en la máquina host.
func (b *StandaloneBuilder) BuildNativeExecutable(prog *ir.Program, outputPath string) error {
	return b.BuildNativeExecutableWithTarget(prog, outputPath, HostTarget())
}

// BuildNativeExecutableWithTarget compila un ir.Program al target específico (cross-compilation).
func (b *StandaloneBuilder) BuildNativeExecutableWithTarget(prog *ir.Program, outputPath string, target Target) error {
	return b.BuildNativeExecutableWithOptions(prog, outputPath, BuildOptions{
		Target: target,
	})
}

// BuildNativeExecutableWithOptions compila un ir.Program usando opciones completas de target, debug y release.
func (b *StandaloneBuilder) BuildNativeExecutableWithOptions(prog *ir.Program, outputPath string, opts BuildOptions) error {
	tempDir, err := os.MkdirTemp("", "joss-native-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	goSource := b.generateNativeRunnerSource(prog)
	srcFile := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(srcFile, []byte(goSource), 0644); err != nil {
		return err
	}

	buildArgs := []string{"build", "-trimpath"}
	if opts.Debug {
		buildArgs = append(buildArgs, "-gcflags=all=-N -l")
	} else {
		buildArgs = append(buildArgs, "-ldflags=-s -w")
	}
	buildArgs = append(buildArgs, "-o", outputPath, srcFile)

	cmd := exec.Command("go", buildArgs...)
	env := append(os.Environ(), "CGO_ENABLED=0")
	if opts.Target.OS != "" {
		env = append(env, "GOOS="+opts.Target.OS)
	}
	if opts.Target.Arch != "" {
		env = append(env, "GOARCH="+opts.Target.Arch)
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		targetDesc := opts.Target.String()
		if targetDesc == "" {
			targetDesc = HostTarget().String()
		}
		return fmt.Errorf("fallo compilación de binario nativo para %s: %v\nSalida: %s", targetDesc, err, string(out))
	}

	if opts.Release {
		optimizeExecutableWithUPX(outputPath, opts.Target.OS)
	}

	return nil
}

func (b *StandaloneBuilder) generateNativeRunnerSource(prog *ir.Program) string {
	var buf bytes.Buffer
	buf.WriteString("package main\n\n")
	buf.WriteString("import (\n")
	buf.WriteString("\t\"fmt\"\n")
	buf.WriteString("\t\"os\"\n")
	buf.WriteString(")\n\n")

	// Runtime ABI mínimo
	buf.WriteString("// Joss Minimal Native Runtime ABI\n")
	buf.WriteString("func joss_print_i64(v int64) { fmt.Println(v) }\n")
	buf.WriteString("func joss_print_f64(v float64) { fmt.Println(v) }\n")
	buf.WriteString("func joss_print_bool(v bool) { fmt.Println(v) }\n")
	buf.WriteString("func joss_print_string(v string) { fmt.Println(v) }\n")
	buf.WriteString("func joss_panic(msg string) { fmt.Fprintf(os.Stderr, \"Panic: %s\\n\", msg); os.Exit(1) }\n")
	buf.WriteString("func joss_str_concat(a, b string) string { return a + b }\n")
	buf.WriteString("type JossObj struct { Class string; Fields map[string]interface{} }\n")
	buf.WriteString("func joss_obj_new(cls string) *JossObj { return &JossObj{Class: cls, Fields: make(map[string]interface{})} }\n")
	buf.WriteString("func joss_obj_set_field_i64(obj *JossObj, f string, v int64) { if obj != nil { obj.Fields[f] = v } }\n")
	buf.WriteString("func joss_obj_get_field_i64(obj *JossObj, f string) int64 { if obj != nil { if v, ok := obj.Fields[f].(int64); ok { return v }; if v, ok := obj.Fields[f].(int); ok { return int64(v) } }; return 0 }\n")
	buf.WriteString("func joss_obj_set_field_str(obj *JossObj, f string, v string) { if obj != nil { obj.Fields[f] = v } }\n")
	buf.WriteString("func joss_obj_get_field_str(obj *JossObj, f string) string { if obj != nil { if v, ok := obj.Fields[f].(string); ok { return v } }; return \"\" }\n")
	buf.WriteString("type JossArr struct { Data []interface{} }\n")
	buf.WriteString("func joss_arr_new(cap int64) *JossArr { return &JossArr{Data: make([]interface{}, 0, cap)} }\n")
	buf.WriteString("func joss_arr_push_i64(arr *JossArr, v int64) { if arr != nil { arr.Data = append(arr.Data, v) } }\n")
	buf.WriteString("func joss_arr_push_str(arr *JossArr, v string) { if arr != nil { arr.Data = append(arr.Data, v) } }\n")
	buf.WriteString("func joss_arr_get_i64(arr *JossArr, idx int64) int64 { if arr != nil && idx >= 0 && int(idx) < len(arr.Data) { if v, ok := arr.Data[idx].(int64); ok { return v }; if v, ok := arr.Data[idx].(int); ok { return int64(v) } }; return 0 }\n")
	buf.WriteString("func joss_arr_get_str(arr *JossArr, idx int64) string { if arr != nil && idx >= 0 && int(idx) < len(arr.Data) { if v, ok := arr.Data[idx].(string); ok { return v } }; return \"\" }\n")
	buf.WriteString("func joss_arr_set_i64(arr *JossArr, idx int64, v int64) { if arr != nil && idx >= 0 { for int64(len(arr.Data)) <= idx { arr.Data = append(arr.Data, int64(0)) }; arr.Data[idx] = v } }\n")
	buf.WriteString("func joss_arr_set_str(arr *JossArr, idx int64, v string) { if arr != nil && idx >= 0 { for int64(len(arr.Data)) <= idx { arr.Data = append(arr.Data, \"\") }; arr.Data[idx] = v } }\n")
	buf.WriteString("type JossMap struct { Data map[string]interface{} }\n")
	buf.WriteString("func joss_map_new() *JossMap { return &JossMap{Data: make(map[string]interface{})} }\n")
	buf.WriteString("func joss_map_set_i64(m *JossMap, k string, v int64) { if m != nil { m.Data[k] = v } }\n")
	buf.WriteString("func joss_map_get_i64(m *JossMap, k string) int64 { if m != nil { if v, ok := m.Data[k].(int64); ok { return v }; if v, ok := m.Data[k].(int); ok { return int64(v) } }; return 0 }\n")
	buf.WriteString("func joss_map_set_str(m *JossMap, k string, v string) { if m != nil { m.Data[k] = v } }\n")
	buf.WriteString("func joss_map_get_str(m *JossMap, k string) string { if m != nil { if v, ok := m.Data[k].(string); ok { return v } }; return \"\" }\n")
	buf.WriteString("func Exception_constructor(this *JossObj, msg string) { if this != nil { this.Fields[\"message\"] = msg } }\n")
	buf.WriteString("func Exception_getMessage(this *JossObj) string { if this != nil { if m, ok := this.Fields[\"message\"].(string); ok { return m } }; return \"\" }\n")
	buf.WriteString("func Exception_getCode(this *JossObj) int64 { if this != nil { if c, ok := this.Fields[\"code\"].(int64); ok { return c } }; return 0 }\n\n")

	// Generar entrypoint Go main() que llama al código Joss compilado
	buf.WriteString("func main() {\n")
	if prog.Functions["main"] != nil {
		buf.WriteString("\tcode := int(joss_user_main())\n")
		buf.WriteString("\tif code != 0 { os.Exit(code) }\n")
	}
	buf.WriteString("}\n\n")

	// Generar funciones traducidas directamente desde la IR
	for _, fn := range prog.Functions {
		b.emitFunctionGo(&buf, fn)
		buf.WriteString("\n")
	}

	return buf.String()
}

func (b *StandaloneBuilder) emitFunctionGo(buf *bytes.Buffer, fn *ir.Function) {
	retGoType := mapGoType(fn.ReturnType)
	paramList := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		paramList[i] = fmt.Sprintf("arg_%s %s", cleanName(p.Name), mapGoType(p.ValueType))
	}

	fnName := fn.Name
	if fnName == "main" {
		fnName = "joss_user_main"
	}

	buf.WriteString(fmt.Sprintf("func %s(%s) %s {\n", fnName, strings.Join(paramList, ", "), retGoType))

	tempTypes := make(map[int]string)
	for _, block := range fn.Blocks {
		for _, inst := range block.Instructions {
			b.registerTempTypes(inst, tempTypes)
		}
	}

	// Declarar temporales al inicio
	for i := 0; i < fn.NextTempID; i++ {
		tType := tempTypes[i]
		if tType == "" {
			tType = "int64"
		}
		buf.WriteString(fmt.Sprintf("\tvar t%d %s\n", i, tType))
		buf.WriteString(fmt.Sprintf("\t_ = t%d\n", i))
	}

	// Identificar cuáles etiquetas son objetivo de algún salto para no emitir etiquetas huérfanas
	usedLabels := make(map[string]bool)
	markUsedLabels(fn.Blocks, usedLabels)

	// Manejar bloques y saltos con labels nativos
	for _, block := range fn.Blocks {
		if usedLabels[block.Label] {
			buf.WriteString(fmt.Sprintf("block_%s:\n", block.Label))
		}
		for _, inst := range block.Instructions {
			if tc, ok := inst.(*ir.TryCatchInst); ok {
				b.emitTryCatchGo(buf, tc, fn, usedLabels)
				continue
			}
			line := b.emitInstructionGo(inst)
			if line != "" {
				buf.WriteString("\t" + line + "\n")
			}
		}
		if block.Terminator != nil {
			termLine := b.emitTerminatorGo(block.Terminator)
			buf.WriteString("\t" + termLine + "\n")
		}
	}

	if fn.ReturnType.Kind != ir.TypeKindVoid {
		switch fn.ReturnType.Kind {
		case ir.TypeKindString:
			buf.WriteString("\treturn \"\"\n")
		case ir.TypeKindPtr, ir.TypeKindStruct, ir.TypeKindArray:
			buf.WriteString("\treturn nil\n")
		default:
			buf.WriteString("\treturn 0\n")
		}
	}
	buf.WriteString("}\n")
}

func (b *StandaloneBuilder) registerTempTypes(inst ir.Instruction, tempTypes map[int]string) {
	switch i := inst.(type) {
	case *ir.AllocaInst:
		tempTypes[i.Dest.ID] = mapGoType(i.AllocType)
	case *ir.LoadInst:
		tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
	case *ir.MoveInst:
		tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
	case *ir.BinaryInst:
		tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
	case *ir.CompareInst:
		tempTypes[i.Dest.ID] = "int64"
	case *ir.UnaryInst:
		tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
	case *ir.CallInst:
		if i.Dest != nil {
			tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
		}
	case *ir.CallRuntimeInst:
		if i.Dest != nil {
			tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
		}
	case *ir.NewObjectInst:
		tempTypes[i.Dest.ID] = "*JossObj"
	case *ir.LoadFieldInst:
		tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
	case *ir.NewArrayInst:
		tempTypes[i.Dest.ID] = "*JossArr"
	case *ir.ArrayGetInst:
		tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
	case *ir.NewMapInst:
		tempTypes[i.Dest.ID] = "*JossMap"
	case *ir.MapGetInst:
		tempTypes[i.Dest.ID] = mapGoType(i.Dest.Type())
	case *ir.TryCatchInst:
		if i.CatchVarPtr != nil {
			if temp, ok := i.CatchVarPtr.(*ir.TempValue); ok {
				tempTypes[temp.ID] = "string"
			}
		}
		for _, tb := range i.TryBlocks {
			for _, tInst := range tb.Instructions {
				b.registerTempTypes(tInst, tempTypes)
			}
		}
		for _, cb := range i.CatchBlocks {
			for _, cInst := range cb.Instructions {
				b.registerTempTypes(cInst, tempTypes)
			}
		}
	}
}

func markUsedLabels(blocks []*ir.BasicBlock, usedLabels map[string]bool) {
	for _, block := range blocks {
		if block.Terminator != nil {
			for _, succ := range block.Terminator.Successors() {
				if succ != nil {
					usedLabels[succ.Label] = true
				}
			}
		}
		for _, inst := range block.Instructions {
			if tc, ok := inst.(*ir.TryCatchInst); ok {
				markUsedLabels(tc.TryBlocks, usedLabels)
				markUsedLabels(tc.CatchBlocks, usedLabels)
			}
		}
	}
}

func (b *StandaloneBuilder) emitTryCatchGo(buf *bytes.Buffer, tc *ir.TryCatchInst, fn *ir.Function, usedLabels map[string]bool) {
	retGoType := mapGoType(fn.ReturnType)
	hasRet := fn.ReturnType.Kind != ir.TypeKindVoid
	id := tc.ID

	buf.WriteString(fmt.Sprintf("\tvar try_caught_%d interface{}\n", id))
	buf.WriteString(fmt.Sprintf("\tvar try_returned_%d bool\n", id))
	if hasRet {
		buf.WriteString(fmt.Sprintf("\tvar try_retval_%d %s\n", id, retGoType))
	}
	buf.WriteString("\tfunc() {\n")
	buf.WriteString(fmt.Sprintf("\t\tdefer func() {\n\t\t\tif r := recover(); r != nil {\n\t\t\t\ttry_caught_%d = r\n\t\t\t}\n\t\t}()\n", id))
	for _, block := range tc.TryBlocks {
		if usedLabels[block.Label] {
			buf.WriteString(fmt.Sprintf("\tblock_%s:\n", block.Label))
		}
		for _, inst := range block.Instructions {
			line := b.emitInstructionGo(inst)
			if line != "" {
				buf.WriteString("\t\t" + line + "\n")
			}
		}
		if block.Terminator != nil {
			if retTerm, ok := block.Terminator.(*ir.ReturnTerminator); ok {
				if retTerm.Val != nil && hasRet {
					buf.WriteString(fmt.Sprintf("\t\ttry_retval_%d = %s\n", id, formatGoVal(retTerm.Val)))
				}
				buf.WriteString(fmt.Sprintf("\t\ttry_returned_%d = true\n", id))
				buf.WriteString("\t\treturn\n")
			} else {
				termLine := b.emitTerminatorGo(block.Terminator)
				if termLine != "" {
					buf.WriteString("\t\t" + termLine + "\n")
				}
			}
		}
	}
	buf.WriteString("\t}()\n")
	if hasRet {
		buf.WriteString(fmt.Sprintf("\tif try_returned_%d { return try_retval_%d }\n", id, id))
	} else {
		buf.WriteString(fmt.Sprintf("\tif try_returned_%d { return }\n", id))
	}
	buf.WriteString(fmt.Sprintf("\tif try_caught_%d != nil {\n", id))
	if tc.CatchVarPtr != nil {
		if temp, ok := tc.CatchVarPtr.(*ir.TempValue); ok {
			buf.WriteString(fmt.Sprintf("\t\tif s, ok := try_caught_%d.(string); ok {\n", id))
			buf.WriteString(fmt.Sprintf("\t\t\tt%d = s\n", temp.ID))
			buf.WriteString(fmt.Sprintf("\t\t} else if obj, ok := try_caught_%d.(*JossObj); ok {\n", id))
			buf.WriteString(fmt.Sprintf("\t\t\tif msg, ok := obj.Fields[\"message\"].(string); ok {\n"))
			buf.WriteString(fmt.Sprintf("\t\t\t\tt%d = msg\n", temp.ID))
			buf.WriteString(fmt.Sprintf("\t\t\t} else {\n\t\t\t\tt%d = obj.Class\n\t\t\t}\n", temp.ID))
			buf.WriteString(fmt.Sprintf("\t\t} else {\n"))
			buf.WriteString(fmt.Sprintf("\t\t\tt%d = fmt.Sprintf(\"%%v\", try_caught_%d)\n", temp.ID, id))
			buf.WriteString("\t\t}\n")
		}
	}
	for _, block := range tc.CatchBlocks {
		if usedLabels[block.Label] {
			buf.WriteString(fmt.Sprintf("\tblock_%s:\n", block.Label))
		}
		for _, inst := range block.Instructions {
			line := b.emitInstructionGo(inst)
			if line != "" {
				buf.WriteString("\t\t" + line + "\n")
			}
		}
		if block.Terminator != nil {
			if retTerm, ok := block.Terminator.(*ir.ReturnTerminator); ok {
				if retTerm.Val != nil && hasRet {
					buf.WriteString(fmt.Sprintf("\t\treturn %s\n", formatGoVal(retTerm.Val)))
				} else {
					buf.WriteString("\t\treturn\n")
				}
			} else {
				termLine := b.emitTerminatorGo(block.Terminator)
				if termLine != "" {
					buf.WriteString("\t\t" + termLine + "\n")
				}
			}
		}
	}
	buf.WriteString("\t}\n")
}

func (b *StandaloneBuilder) emitInstructionGo(inst ir.Instruction) string {
	switch i := inst.(type) {
	case *ir.AllocaInst:
		if i.AllocType.Kind == ir.TypeKindString {
			return fmt.Sprintf("t%d = \"\"", i.Dest.ID)
		}
		if i.AllocType.Kind == ir.TypeKindPtr || i.AllocType.Kind == ir.TypeKindStruct || i.AllocType.Kind == ir.TypeKindArray {
			return fmt.Sprintf("t%d = nil", i.Dest.ID)
		}
		return fmt.Sprintf("t%d = 0", i.Dest.ID)

	case *ir.StoreInst:
		if temp, ok := i.Ptr.(*ir.TempValue); ok {
			return fmt.Sprintf("t%d = %s", temp.ID, formatGoVal(i.Val))
		}
		return ""

	case *ir.LoadInst:
		if temp, ok := i.Ptr.(*ir.TempValue); ok {
			return fmt.Sprintf("t%d = t%d", i.Dest.ID, temp.ID)
		}
		return ""

	case *ir.MoveInst:
		return fmt.Sprintf("t%d = %s", i.Dest.ID, formatGoVal(i.Src))

	case *ir.BinaryInst:
		op := i.Op
		opStr := "+"
		switch op {
		case ir.OpAdd:
			opStr = "+"
		case ir.OpSub:
			opStr = "-"
		case ir.OpMul:
			opStr = "*"
		case ir.OpDiv:
			opStr = "/"
		case ir.OpMod:
			opStr = "%"
		case ir.OpShl:
			opStr = "<<"
		case ir.OpShr:
			opStr = ">>"
		case ir.OpAnd:
			opStr = "&"
		case ir.OpOr:
			opStr = "|"
		case ir.OpXor:
			opStr = "^"
		}
		return fmt.Sprintf("t%d = %s %s %s", i.Dest.ID, formatGoVal(i.Left), opStr, formatGoVal(i.Right))

	case *ir.CompareInst:
		if i.Op == ir.OpSpaceship {
			return fmt.Sprintf("if %s < %s { t%d = -1 } else if %s > %s { t%d = 1 } else { t%d = 0 }", formatGoVal(i.Left), formatGoVal(i.Right), i.Dest.ID, formatGoVal(i.Left), formatGoVal(i.Right), i.Dest.ID, i.Dest.ID)
		}
		cmpStr := "=="
		switch i.Op {
		case ir.OpCmpEq:
			cmpStr = "=="
		case ir.OpCmpNe:
			cmpStr = "!="
		case ir.OpCmpLt:
			cmpStr = "<"
		case ir.OpCmpLe:
			cmpStr = "<="
		case ir.OpCmpGt:
			cmpStr = ">"
		case ir.OpCmpGe:
			cmpStr = ">="
		}
		return fmt.Sprintf("if %s %s %s { t%d = 1 } else { t%d = 0 }", formatGoVal(i.Left), cmpStr, formatGoVal(i.Right), i.Dest.ID, i.Dest.ID)

	case *ir.UnaryInst:
		if i.Op == ir.OpNeg {
			return fmt.Sprintf("t%d = -%s", i.Dest.ID, formatGoVal(i.Val))
		}
		if i.Op == ir.OpNot {
			return fmt.Sprintf("if %s == 0 { t%d = 1 } else { t%d = 0 }", formatGoVal(i.Val), i.Dest.ID, i.Dest.ID)
		}

	case *ir.CallInst:
		callee := i.Callee
		if callee == "main" {
			callee = "joss_user_main"
		}
		args := make([]string, len(i.Args))
		for idx, a := range i.Args {
			args[idx] = formatGoVal(a)
		}
		if i.Dest != nil {
			return fmt.Sprintf("t%d = %s(%s)", i.Dest.ID, callee, strings.Join(args, ", "))
		}
		return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", "))

	case *ir.CallRuntimeInst:
		args := make([]string, len(i.Args))
		for idx, a := range i.Args {
			args[idx] = formatGoVal(a)
		}
		if i.Dest != nil {
			return fmt.Sprintf("t%d = joss_%s(%s)", i.Dest.ID, i.Func, strings.Join(args, ", "))
		}
		return fmt.Sprintf("joss_%s(%s)", i.Func, strings.Join(args, ", "))

	case *ir.NewObjectInst:
		return fmt.Sprintf("t%d = joss_obj_new(%q)", i.Dest.ID, i.ClassName)

	case *ir.LoadFieldInst:
		if i.Dest.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("t%d = joss_obj_get_field_str(%s, %q)", i.Dest.ID, formatGoVal(i.Obj), i.FieldName)
		}
		return fmt.Sprintf("t%d = joss_obj_get_field_i64(%s, %q)", i.Dest.ID, formatGoVal(i.Obj), i.FieldName)

	case *ir.StoreFieldInst:
		if i.Val.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("joss_obj_set_field_str(%s, %q, %s)", formatGoVal(i.Obj), i.FieldName, formatGoVal(i.Val))
		}
		return fmt.Sprintf("joss_obj_set_field_i64(%s, %q, %s)", formatGoVal(i.Obj), i.FieldName, formatGoVal(i.Val))

	case *ir.NewArrayInst:
		lines := []string{fmt.Sprintf("t%d = joss_arr_new(%d)", i.Dest.ID, len(i.Elements))}
		for _, el := range i.Elements {
			if el.Type().Kind == ir.TypeKindString {
				lines = append(lines, fmt.Sprintf("joss_arr_push_str(t%d, %s)", i.Dest.ID, formatGoVal(el)))
			} else {
				lines = append(lines, fmt.Sprintf("joss_arr_push_i64(t%d, %s)", i.Dest.ID, formatGoVal(el)))
			}
		}
		return strings.Join(lines, "\n\t")

	case *ir.ArrayGetInst:
		if i.Dest.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("t%d = joss_arr_get_str(%s, %s)", i.Dest.ID, formatGoVal(i.Array), formatGoVal(i.Index))
		}
		return fmt.Sprintf("t%d = joss_arr_get_i64(%s, %s)", i.Dest.ID, formatGoVal(i.Array), formatGoVal(i.Index))

	case *ir.ArraySetInst:
		if i.Val.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("joss_arr_set_str(%s, %s, %s)", formatGoVal(i.Array), formatGoVal(i.Index), formatGoVal(i.Val))
		}
		return fmt.Sprintf("joss_arr_set_i64(%s, %s, %s)", formatGoVal(i.Array), formatGoVal(i.Index), formatGoVal(i.Val))

	case *ir.NewMapInst:
		lines := []string{fmt.Sprintf("t%d = joss_map_new()", i.Dest.ID)}
		for _, p := range i.Pairs {
			keyVal := formatGoVal(p[0])
			valVal := formatGoVal(p[1])
			if p[1].Type().Kind == ir.TypeKindString {
				lines = append(lines, fmt.Sprintf("joss_map_set_str(t%d, %s, %s)", i.Dest.ID, keyVal, valVal))
			} else {
				lines = append(lines, fmt.Sprintf("joss_map_set_i64(t%d, %s, %s)", i.Dest.ID, keyVal, valVal))
			}
		}
		return strings.Join(lines, "\n\t")

	case *ir.MapGetInst:
		if i.Dest.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("t%d = joss_map_get_str(%s, %s)", i.Dest.ID, formatGoVal(i.Map), formatGoVal(i.Key))
		}
		return fmt.Sprintf("t%d = joss_map_get_i64(%s, %s)", i.Dest.ID, formatGoVal(i.Map), formatGoVal(i.Key))

	case *ir.MapSetInst:
		if i.Val.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("joss_map_set_str(%s, %s, %s)", formatGoVal(i.Map), formatGoVal(i.Key), formatGoVal(i.Val))
		}
		return fmt.Sprintf("joss_map_set_i64(%s, %s, %s)", formatGoVal(i.Map), formatGoVal(i.Key), formatGoVal(i.Val))

	case *ir.ThrowInst:
		return fmt.Sprintf("panic(%s)", formatGoVal(i.Val))
	}

	return ""
}

func (b *StandaloneBuilder) emitTerminatorGo(term ir.Terminator) string {
	switch t := term.(type) {
	case *ir.ReturnTerminator:
		if t.Val != nil {
			return fmt.Sprintf("return %s", formatGoVal(t.Val))
		}
		return "return"
	case *ir.JumpTerminator:
		return fmt.Sprintf("goto block_%s", t.Target.Label)
	case *ir.BranchTerminator:
		return fmt.Sprintf("if %s != 0 { goto block_%s } else { goto block_%s }", formatGoVal(t.Cond), t.TrueBlock.Label, t.FalseBlock.Label)
	case *ir.UnreachableTerminator:
		return "panic(\"unreachable\")"
	}
	return ""
}

func mapGoType(t ir.Type) string {
	switch t.Kind {
	case ir.TypeKindVoid:
		return ""
	case ir.TypeKindString:
		return "string"
	case ir.TypeKindF32:
		return "float32"
	case ir.TypeKindF64:
		return "float64"
	case ir.TypeKindStruct:
		if t.Name == "map" {
			return "*JossMap"
		}
		return "*JossObj"
	case ir.TypeKindArray:
		return "*JossArr"
	case ir.TypeKindPtr:
		if t.Elem != nil && t.Elem.Kind == ir.TypeKindStruct {
			if t.Elem.Name == "map" {
				return "*JossMap"
			}
			return "*JossObj"
		}
		if t.Elem != nil && t.Elem.Kind == ir.TypeKindArray {
			return "*JossArr"
		}
		return "int64"
	default:
		return "int64"
	}
}

func formatGoVal(val ir.Value) string {
	switch v := val.(type) {
	case *ir.TempValue:
		return fmt.Sprintf("t%d", v.ID)
	case *ir.ParamValue:
		return fmt.Sprintf("arg_%s", cleanName(v.Name))
	case *ir.ConstInt:
		return fmt.Sprintf("%d", v.Val)
	case *ir.ConstFloat:
		return strconv.FormatFloat(v.Val, 'g', -1, 64)
	case *ir.ConstBool:
		if v.Val {
			return "1"
		}
		return "0"
	case *ir.ConstString:
		return fmt.Sprintf("%q", v.Val)
	case *ir.ConstNull:
		return "0"
	default:
		return val.String()
	}
}

func cleanName(name string) string {
	return strings.TrimPrefix(name, "$")
}

func optimizeExecutableWithUPX(filePath, targetOS string) {
	upxPath := resolveUPXPath()
	if upxPath != "" {
		fmt.Printf("⚡ Optimizando ejecutable nativo con UPX...\n")
		cmd := exec.Command(upxPath, "--best", "--lzma", filePath)
		_ = cmd.Run()
	}
}

func resolveUPXPath() string {
	if p, err := exec.LookPath("upx"); err == nil {
		return p
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}
	toolsDir := filepath.Join(homeDir, ".joss", "tools")
	exeName := "upx"
	if runtime.GOOS == "windows" {
		exeName = "upx.exe"
	}
	cachedUPX := filepath.Join(toolsDir, exeName)
	if fi, err := os.Stat(cachedUPX); err == nil && !fi.IsDir() {
		return cachedUPX
	}
	if selfExe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(selfExe), exeName)
		if fi, err := os.Stat(sibling); err == nil && !fi.IsDir() {
			return sibling
		}
	}
	return ""
}
