package native

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	buildArgs := []string{"build"}
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
	buf.WriteString("func joss_panic(msg string) { fmt.Fprintf(os.Stderr, \"Panic: %s\\n\", msg); os.Exit(1) }\n\n")

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
			}
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
	for _, block := range fn.Blocks {
		if block.Terminator != nil {
			for _, succ := range block.Terminator.Successors() {
				if succ != nil {
					usedLabels[succ.Label] = true
				}
			}
		}
	}

	// Manejar bloques y saltos con labels nativos
	for _, block := range fn.Blocks {
		if usedLabels[block.Label] {
			buf.WriteString(fmt.Sprintf("block_%s:\n", block.Label))
		}
		for _, inst := range block.Instructions {
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
		buf.WriteString("\treturn 0\n")
	}
	buf.WriteString("}\n")
}

func (b *StandaloneBuilder) emitInstructionGo(inst ir.Instruction) string {
	switch i := inst.(type) {
	case *ir.AllocaInst:
		if i.AllocType.Kind == ir.TypeKindString {
			return fmt.Sprintf("t%d = \"\"", i.Dest.ID)
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
