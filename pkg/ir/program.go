package ir

import (
	"bytes"
	"fmt"
	"strings"
)

// BasicBlock representa una secuencia lineal de instrucciones sin saltos intermedios,
// terminada obligatoriamente por una instrucción Terminator.
type BasicBlock struct {
	ID           int
	Label        string
	Instructions []Instruction
	Terminator   Terminator
	Predecessors []*BasicBlock
}

func NewBasicBlock(id int, label string) *BasicBlock {
	return &BasicBlock{
		ID:           id,
		Label:        label,
		Instructions: make([]Instruction, 0),
		Predecessors: make([]*BasicBlock, 0),
	}
}

func (b *BasicBlock) AddInstruction(inst Instruction) {
	b.Instructions = append(b.Instructions, inst)
}

func (b *BasicBlock) SetTerminator(term Terminator) {
	b.Terminator = term
}

func (b *BasicBlock) Successors() []*BasicBlock {
	if b.Terminator == nil {
		return nil
	}
	return b.Terminator.Successors()
}

// Function representa una función en la IR nativa con un CFG completo.
type Function struct {
	Name        string
	Params      []*ParamValue
	ReturnType  Type
	EntryBlock  *BasicBlock
	Blocks      []*BasicBlock
	NextTempID  int
	NextBlockID int
	Attributes  map[string]string
}

func NewFunction(name string, returnType Type) *Function {
	fn := &Function{
		Name:       name,
		Params:     make([]*ParamValue, 0),
		ReturnType: returnType,
		Blocks:     make([]*BasicBlock, 0),
		Attributes: make(map[string]string),
	}
	entry := fn.NewBlock("entry")
	fn.EntryBlock = entry
	return fn
}

func (fn *Function) AddParam(name string, t Type) *ParamValue {
	p := &ParamValue{
		Index:     len(fn.Params),
		Name:      name,
		ValueType: t,
	}
	fn.Params = append(fn.Params, p)
	return p
}

func (fn *Function) NewBlock(prefix string) *BasicBlock {
	id := fn.NextBlockID
	fn.NextBlockID++
	label := fmt.Sprintf("%s%d", prefix, id)
	if prefix == "entry" && id == 0 {
		label = "entry"
	}
	b := NewBasicBlock(id, label)
	fn.Blocks = append(fn.Blocks, b)
	return b
}

func (fn *Function) NewTemp(t Type, hint string) *TempValue {
	id := fn.NextTempID
	fn.NextTempID++
	return &TempValue{
		ID:        id,
		ValueType: t,
		Hint:      hint,
	}
}

// GlobalVar representa una variable global en el módulo de IR.
type GlobalVar struct {
	Name    string
	Type    Type
	InitVal Value // Opcional, inicializador constante
	IsConst bool
}

// Program representa el módulo completo de IR que será verificado y compilado.
type Program struct {
	Name                string
	Globals             map[string]*GlobalVar
	Functions           map[string]*Function
	Types               map[string]Type
	RuntimeRequirements map[string]bool
}

func NewProgram(name string) *Program {
	return &Program{
		Name:                name,
		Globals:             make(map[string]*GlobalVar),
		Functions:           make(map[string]*Function),
		Types:               make(map[string]Type),
		RuntimeRequirements: make(map[string]bool),
	}
}

func (p *Program) AddFunction(fn *Function) {
	p.Functions[fn.Name] = fn
}

func (p *Program) AddGlobal(name string, t Type, init Value, isConst bool) *GlobalVar {
	g := &GlobalVar{
		Name:    name,
		Type:    t,
		InitVal: init,
		IsConst: isConst,
	}
	p.Globals[name] = g
	return g
}

func (p *Program) RequireRuntime(feature string) {
	p.RuntimeRequirements[feature] = true
}

// Dump produce una representación textual determinista del programa IR.
func (p *Program) Dump() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("; Joss Native IR: %s\n\n", p.Name))

	// Requerimientos del runtime
	if len(p.RuntimeRequirements) > 0 {
		buf.WriteString("; Runtime Requirements:\n")
		// Para determinismo, ordenamos keys
		reqs := make([]string, 0, len(p.RuntimeRequirements))
		for r := range p.RuntimeRequirements {
			reqs = append(reqs, r)
		}
		sortStrings(reqs)
		for _, r := range reqs {
			buf.WriteString(fmt.Sprintf(";   - %s\n", r))
		}
		buf.WriteString("\n")
	}

	// Variables globales
	if len(p.Globals) > 0 {
		buf.WriteString("; Globals:\n")
		gNames := make([]string, 0, len(p.Globals))
		for g := range p.Globals {
			gNames = append(gNames, g)
		}
		sortStrings(gNames)
		for _, gName := range gNames {
			g := p.Globals[gName]
			initStr := "null"
			if g.InitVal != nil {
				initStr = g.InitVal.String()
			}
			constPrefix := "var"
			if g.IsConst {
				constPrefix = "const"
			}
			buf.WriteString(fmt.Sprintf("@%s = %s %s (%s)\n", g.Name, constPrefix, g.Type, initStr))
		}
		buf.WriteString("\n")
	}

	// Funciones
	fnNames := make([]string, 0, len(p.Functions))
	for fn := range p.Functions {
		fnNames = append(fnNames, fn)
	}
	sortStrings(fnNames)
	for _, fnName := range fnNames {
		fn := p.Functions[fnName]
		buf.WriteString(fn.Dump())
		buf.WriteString("\n")
	}

	return buf.String()
}

func (fn *Function) Dump() string {
	var buf bytes.Buffer
	paramStrs := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		paramStrs[i] = fmt.Sprintf("%s: %s", p.String(), p.ValueType)
	}
	buf.WriteString(fmt.Sprintf("func @%s(%s) -> %s {\n", fn.Name, strings.Join(paramStrs, ", "), fn.ReturnType))

	for _, block := range fn.Blocks {
		buf.WriteString(fmt.Sprintf("%s:\n", block.Label))
		for _, inst := range block.Instructions {
			buf.WriteString(fmt.Sprintf("    %s\n", inst.String()))
		}
		if block.Terminator != nil {
			buf.WriteString(fmt.Sprintf("    %s\n", block.Terminator.String()))
		} else {
			buf.WriteString("    <missing_terminator>\n")
		}
	}

	buf.WriteString("}\n")
	return buf.String()
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
