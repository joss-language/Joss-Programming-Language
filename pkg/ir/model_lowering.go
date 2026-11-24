package ir

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

func (l *Lowerer) allClassStatements() map[string]*parser.ClassStatement {
	out := make(map[string]*parser.ClassStatement)
	if l.prep == nil {
		return out
	}
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			if cs, ok := stmt.(*parser.ClassStatement); ok && cs.Name != nil {
				out[cs.Name.Value] = cs
			}
		}
	}
	return out
}

func (l *Lowerer) findClassStatement(name string) *parser.ClassStatement {
	if l.prep == nil || name == "" {
		return nil
	}
	for _, unit := range l.prep.Units {
		if unit.Program == nil {
			continue
		}
		for _, stmt := range unit.Program.Statements {
			if cs, ok := stmt.(*parser.ClassStatement); ok && cs.Name != nil && cs.Name.Value == name {
				return cs
			}
		}
	}
	return nil
}

func (l *Lowerer) isModelClass(className string) bool {
	if className == "" || className == "Model" || className == "GranDB" {
		return false
	}
	visited := make(map[string]bool)
	curr := className
	for curr != "" && !visited[curr] {
		visited[curr] = true
		cs := l.findClassStatement(curr)
		if cs == nil || cs.SuperClass == nil {
			return false
		}
		if cs.SuperClass.Value == "Model" {
			return true
		}
		curr = cs.SuperClass.Value
	}
	return false
}

func (l *Lowerer) isModelType(t Type) bool {
	if t.Kind != TypeKindPtr || t.Elem == nil {
		return false
	}
	return t.Elem.Name == "Model" || l.isModelClass(t.Elem.Name)
}

func extractModelLiteralValue(expr parser.Expression) (interface{}, bool) {
	switch v := expr.(type) {
	case nil:
		return nil, true
	case *parser.StringLiteral:
		return v.Value, true
	case *parser.Boolean:
		return v.Value, true
	case *parser.IntegerLiteral:
		return v.Value, true
	case *parser.FloatLiteral:
		return v.Value, true
	case *parser.NullLiteral:
		return nil, true
	case *parser.ArrayLiteral:
		out := make([]interface{}, 0, len(v.Elements))
		for _, el := range v.Elements {
			lv, ok := extractModelLiteralValue(el)
			if !ok {
				return nil, false
			}
			out = append(out, lv)
		}
		return out, true
	case *parser.MapLiteral:
		out := make(map[string]interface{}, len(v.Pairs))
		for k, val := range v.Pairs {
			lk, ok1 := extractModelLiteralValue(k)
			lv, ok2 := extractModelLiteralValue(val)
			if !ok1 || !ok2 {
				return nil, false
			}
			out[fmt.Sprint(lk)] = lv
		}
		return out, true
	default:
		return nil, false
	}
}

func (l *Lowerer) buildModelMetaJSON(className string) string {
	cfg := make(map[string]interface{})
	hierarchy := l.collectClassHierarchy(className)
	for _, cs := range hierarchy {
		if cs == nil || cs.Body == nil || cs.Name == nil || cs.Name.Value == "Model" {
			continue
		}
		for _, stmt := range cs.Body.Statements {
			ls, ok := stmt.(*parser.LetStatement)
			if !ok || ls.Name == nil || ls.Value == nil {
				continue
			}
			prop := cleanVarName(ls.Name.Value)
			switch prop {
			case "table", "primaryKey", "keyType", "incrementing", "timestamps",
				"createdAt", "updatedAt", "deletedAt", "softDeletes",
				"fillable", "guarded", "hidden", "visible", "casts", "globalScopes":
				if lv, ok := extractModelLiteralValue(ls.Value); ok {
					cfg[prop] = lv
				}
			}
		}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func isModelCallbackMethodName(name string, paramCount int) bool {
	switch name {
	case "saving", "creating", "created", "updating", "updated", "saved",
		"deleting", "deleted", "restoring", "restored":
		return true
	}
	if strings.HasPrefix(name, "scope") && len(name) > 5 {
		return true
	}
	if (strings.HasPrefix(name, "get") || strings.HasPrefix(name, "set")) &&
		strings.HasSuffix(name, "Attribute") && len(name) > len("getAttribute") {
		return true
	}
	// Zero-arg methods on Model subclasses can be relation methods (e.g. posts(), user(), roles())
	if paramCount == 0 && name != "Init" && name != "constructor" {
		return true
	}
	return false
}

// emitModelMetadataRegistrations prepends Model_registerClass and Model_registerMethod
// calls to main's entry block for every class extending Model.
func (l *Lowerer) emitModelMetadataRegistrations() {
	mainFn := l.prog.Functions["main"]
	if mainFn == nil || mainFn.EntryBlock == nil {
		return
	}

	allClasses := l.allClassStatements()
	var modelNames []string
	for name := range allClasses {
		if l.isModelClass(name) {
			modelNames = append(modelNames, name)
		}
	}
	if len(modelNames) == 0 {
		return
	}
	sort.Strings(modelNames)

	var initInsts []Instruction
	for _, clsName := range modelNames {
		metaJSON := l.buildModelMetaJSON(clsName)
		initInsts = append(initInsts, &CallInst{
			Dest:   nil,
			Callee: "fn_Model_registerClass",
			Args:   []Value{NewConstString(clsName), NewConstString(metaJSON)},
		})

		cs := allClasses[clsName]
		if cs == nil || cs.Body == nil {
			continue
		}
		for _, stmt := range cs.Body.Statements {
			ms, ok := stmt.(*parser.MethodStatement)
			if !ok || ms.Name == nil || ms.IsStatic {
				continue
			}
			mName := ms.Name.Value
			if !isModelCallbackMethodName(mName, len(ms.Parameters)) {
				continue
			}
			fnName := clsName + "_" + mName
			if irFn := l.prog.Functions[fnName]; irFn != nil {
				fnType := Type{Kind: TypeKindFunc, Return: &irFn.ReturnType}
				for _, p := range irFn.Params {
					fnType.Params = append(fnType.Params, p.Type())
				}
				initInsts = append(initInsts, &CallInst{
					Dest:   nil,
					Callee: "fn_Model_registerMethod",
					Args: []Value{
						NewConstString(clsName),
						NewConstString(mName),
						NewConstFnPtr(fnName, fnType),
						NewConstInt(int64(len(ms.Parameters)), TypeI64),
					},
				})
			}
		}
	}

	// Insert after leading AllocaInsts in main's entry block
	existing := mainFn.EntryBlock.Instructions
	split := 0
	for split < len(existing) {
		if _, isAlloca := existing[split].(*AllocaInst); isAlloca {
			split++
		} else {
			break
		}
	}
	merged := make([]Instruction, 0, len(existing)+len(initInsts))
	merged = append(merged, existing[:split]...)
	merged = append(merged, initInsts...)
	merged = append(merged, existing[split:]...)
	mainFn.EntryBlock.Instructions = merged
}

func (l *Lowerer) lowerModelStaticCall(className, methodName string, rawArgs []parser.Expression) (Value, bool, error) {
	modelPtrType := PtrType(StructType(className, nil))
	lower := strings.ToLower(methodName)

	switch lower {
	case "query":
		dest := l.currentFn.NewTemp(modelPtrType, "model_q")
		l.currBlock.AddInstruction(&CallInst{
			Dest:   dest,
			Callee: "fn_Model_query",
			Args:   []Value{NewConstString(className)},
		})
		return dest, true, nil

	case "all":
		dest := l.currentFn.NewTemp(ArrayType(modelPtrType), "model_all")
		l.currBlock.AddInstruction(&CallInst{
			Dest:   dest,
			Callee: "fn_Model_all",
			Args:   []Value{NewConstString(className)},
		})
		return dest, true, nil

	case "create":
		var attrs ValArg
		if len(rawArgs) > 0 {
			v, err := l.lowerExpression(rawArgs[0])
			if err != nil {
				return nil, true, err
			}
			attrs = v
		} else {
			attrs = NewConstNull(PtrType(TypeVoid))
		}
		dest := l.currentFn.NewTemp(modelPtrType, "model_created")
		l.currBlock.AddInstruction(&CallInst{
			Dest:   dest,
			Callee: "fn_Model_create",
			Args:   []Value{NewConstString(className), attrs},
		})
		return dest, true, nil

	case "firstornew", "firstorcreate", "updateorcreate":
		var searchVal Value = NewConstNull(PtrType(TypeVoid))
		var valuesVal Value = NewConstNull(PtrType(TypeVoid))
		if len(rawArgs) > 0 {
			v, err := l.lowerExpression(rawArgs[0])
			if err != nil {
				return nil, true, err
			}
			searchVal = v
		}
		if len(rawArgs) > 1 {
			v, err := l.lowerExpression(rawArgs[1])
			if err != nil {
				return nil, true, err
			}
			valuesVal = v
		}
		callee := "fn_Model_firstOrNew"
		if lower == "firstorcreate" {
			callee = "fn_Model_firstOrCreate"
		} else if lower == "updateorcreate" {
			callee = "fn_Model_updateOrCreate"
		}
		dest := l.currentFn.NewTemp(modelPtrType, "model_fow")
		l.currBlock.AddInstruction(&CallInst{
			Dest:   dest,
			Callee: callee,
			Args:   []Value{NewConstString(className), searchVal, valuesVal},
		})
		return dest, true, nil

	default:
		// Query-starting static call (User::find, User::where, User::with, User::withTrashed, etc.)
		qTemp := l.currentFn.NewTemp(modelPtrType, "model_q")
		l.currBlock.AddInstruction(&CallInst{
			Dest:   qTemp,
			Callee: "fn_Model_query",
			Args:   []Value{NewConstString(className)},
		})
		return l.lowerModelInstanceCall(qTemp, methodName, rawArgs)
	}
}

type ValArg = Value

func (l *Lowerer) lowerModelInstanceCall(receiver Value, methodName string, rawArgs []parser.Expression) (Value, bool, error) {
	recvType := receiver.Type()
	lower := strings.ToLower(methodName)

	lowerArgs := make([]Value, 0, len(rawArgs))
	for _, a := range rawArgs {
		v, err := l.lowerExpression(a)
		if err != nil {
			return nil, true, err
		}
		lowerArgs = append(lowerArgs, v)
	}

	optStr := func(idx int) Value {
		if idx < len(lowerArgs) {
			return lowerArgs[idx]
		}
		return NewConstString("")
	}
	optBoxed := func(idx int) Value {
		if idx < len(lowerArgs) {
			return l.boxIfMixed(lowerArgs[idx], TypeMixed)
		}
		return NewConstInt(0, TypeMixed)
	}

	switch lower {
	case "where", "orwhere":
		callee := "fn_Model_where"
		if lower == "orwhere" {
			callee = "fn_Model_orWhere"
		}
		col := optStr(0)
		var arg2, arg3, hasArg3 Value
		if len(lowerArgs) >= 3 {
			arg2 = l.boxIfMixed(lowerArgs[1], TypeMixed)
			arg3 = l.boxIfMixed(lowerArgs[2], TypeMixed)
			hasArg3 = NewConstInt(1, TypeI64)
		} else {
			arg2 = optBoxed(1)
			arg3 = NewConstInt(0, TypeMixed)
			hasArg3 = NewConstInt(0, TypeI64)
		}
		dest := l.currentFn.NewTemp(recvType, "model_w")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: callee, Args: []Value{receiver, col, arg2, arg3, hasArg3}})
		return dest, true, nil

	case "wherein":
		arr := Value(NewConstNull(PtrType(TypeVoid)))
		if len(lowerArgs) > 1 {
			arr = lowerArgs[1]
		}
		dest := l.currentFn.NewTemp(recvType, "model_wi")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_whereIn", Args: []Value{receiver, optStr(0), arr}})
		return dest, true, nil

	case "wherenull":
		dest := l.currentFn.NewTemp(recvType, "model_wn")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_whereNull", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "wherenotnull":
		dest := l.currentFn.NewTemp(recvType, "model_wnn")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_whereNotNull", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "orderby":
		dir := Value(NewConstString("asc"))
		if len(lowerArgs) > 1 {
			dir = lowerArgs[1]
		}
		dest := l.currentFn.NewTemp(recvType, "model_ob")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_orderBy", Args: []Value{receiver, optStr(0), dir}})
		return dest, true, nil

	case "orderbyasc":
		dest := l.currentFn.NewTemp(recvType, "model_oba")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_orderByAsc", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "orderbydesc":
		dest := l.currentFn.NewTemp(recvType, "model_obd")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_orderByDesc", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "limit", "take":
		n := Value(NewConstInt(0, TypeI64))
		if len(lowerArgs) > 0 {
			n = lowerArgs[0]
		}
		dest := l.currentFn.NewTemp(recvType, "model_lim")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_limit", Args: []Value{receiver, n}})
		return dest, true, nil

	case "offset", "skip":
		n := Value(NewConstInt(0, TypeI64))
		if len(lowerArgs) > 0 {
			n = lowerArgs[0]
		}
		dest := l.currentFn.NewTemp(recvType, "model_off")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_offset", Args: []Value{receiver, n}})
		return dest, true, nil

	case "withtrashed":
		dest := l.currentFn.NewTemp(recvType, "model_wt")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_withTrashed", Args: []Value{receiver}})
		return dest, true, nil

	case "onlytrashed":
		dest := l.currentFn.NewTemp(recvType, "model_ot")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_onlyTrashed", Args: []Value{receiver}})
		return dest, true, nil

	case "withouttrashed":
		dest := l.currentFn.NewTemp(recvType, "model_wot")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_withoutTrashed", Args: []Value{receiver}})
		return dest, true, nil

	case "withoutglobalscope":
		dest := l.currentFn.NewTemp(recvType, "model_wgs")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_withoutGlobalScope", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "withoutglobalscopes":
		dest := l.currentFn.NewTemp(recvType, "model_wgss")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_withoutGlobalScopes", Args: []Value{receiver}})
		return dest, true, nil

	case "scope":
		hasArg1 := Value(NewConstInt(0, TypeI64))
		arg1 := Value(NewConstInt(0, TypeMixed))
		if len(lowerArgs) > 1 {
			hasArg1 = NewConstInt(1, TypeI64)
			arg1 = l.boxIfMixed(lowerArgs[1], TypeMixed)
		}
		dest := l.currentFn.NewTemp(recvType, "model_sc")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_scope", Args: []Value{receiver, optStr(0), arg1, hasArg1}})
		return dest, true, nil

	case "with":
		dest := l.currentFn.NewTemp(recvType, "model_with")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_with", Args: []Value{receiver, optBoxed(0)}})
		return dest, true, nil

	case "load", "loadmissing":
		callee := "fn_Model_load"
		if lower == "loadmissing" {
			callee = "fn_Model_loadMissing"
		}
		dest := l.currentFn.NewTemp(recvType, "model_load")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: callee, Args: []Value{receiver, optBoxed(0)}})
		return dest, true, nil

	case "get":
		dest := l.currentFn.NewTemp(ArrayType(recvType), "model_rows")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_get", Args: []Value{receiver}})
		return dest, true, nil

	case "first":
		dest := l.currentFn.NewTemp(recvType, "model_first")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_first", Args: []Value{receiver}})
		return dest, true, nil

	case "find":
		dest := l.currentFn.NewTemp(recvType, "model_find")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_find", Args: []Value{receiver, optBoxed(0)}})
		return dest, true, nil

	case "count":
		dest := l.currentFn.NewTemp(TypeI64, "model_cnt")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_count", Args: []Value{receiver}})
		return dest, true, nil

	case "exists":
		dest := l.currentFn.NewTemp(TypeBool, "model_ex")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_exists", Args: []Value{receiver}})
		return dest, true, nil

	case "fill", "forcefill":
		callee := "fn_Model_fill"
		if lower == "forcefill" {
			callee = "fn_Model_forceFill"
		}
		m := Value(NewConstNull(PtrType(TypeVoid)))
		if len(lowerArgs) > 0 {
			m = lowerArgs[0]
		}
		dest := l.currentFn.NewTemp(recvType, "model_fill")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: callee, Args: []Value{receiver, m}})
		return dest, true, nil

	case "save":
		dest := l.currentFn.NewTemp(TypeBool, "model_save")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_save", Args: []Value{receiver}})
		return dest, true, nil

	case "refresh":
		dest := l.currentFn.NewTemp(recvType, "model_ref")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_refresh", Args: []Value{receiver}})
		return dest, true, nil

	case "delete":
		dest := l.currentFn.NewTemp(TypeBool, "model_del")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_delete", Args: []Value{receiver}})
		return dest, true, nil

	case "forcedelete":
		dest := l.currentFn.NewTemp(TypeBool, "model_fdel")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_forceDelete", Args: []Value{receiver}})
		return dest, true, nil

	case "restore":
		dest := l.currentFn.NewTemp(TypeBool, "model_rest")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_restore", Args: []Value{receiver}})
		return dest, true, nil

	case "isdirty":
		dest := l.currentFn.NewTemp(TypeBool, "model_dirty")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_isDirty", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "isclean":
		dest := l.currentFn.NewTemp(TypeBool, "model_clean")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_isClean", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "waschanged":
		dest := l.currentFn.NewTemp(TypeBool, "model_wc")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_wasChanged", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "getoriginal":
		retType := TypeMixed
		if len(lowerArgs) == 0 {
			retType = PtrType(StructType("map", []Type{TypeMixed}))
		}
		dest := l.currentFn.NewTemp(retType, "model_orig")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_getOriginal", Args: []Value{receiver, optStr(0)}})
		return dest, true, nil

	case "getchanges":
		dest := l.currentFn.NewTemp(PtrType(StructType("map", []Type{TypeMixed})), "model_chg")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_getChanges", Args: []Value{receiver}})
		return dest, true, nil

	case "tomap":
		dest := l.currentFn.NewTemp(PtrType(StructType("map", []Type{TypeMixed})), "model_map")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_toMap", Args: []Value{receiver}})
		return dest, true, nil

	case "tojson":
		dest := l.currentFn.NewTemp(TypeString, "model_json")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_toJSON", Args: []Value{receiver}})
		return dest, true, nil

	case "hasone", "hasmany", "belongsto":
		callee := "fn_Model_hasOne"
		if lower == "hasmany" {
			callee = "fn_Model_hasMany"
		} else if lower == "belongsto" {
			callee = "fn_Model_belongsTo"
		}
		relRetType := PtrType(StructType("Model", nil))
		if len(rawArgs) > 0 {
			if sl, ok := rawArgs[0].(*parser.StringLiteral); ok && sl.Value != "" {
				relRetType = PtrType(StructType(sl.Value, nil))
			}
		}
		dest := l.currentFn.NewTemp(relRetType, "model_rel")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: callee, Args: []Value{receiver, optStr(0), optStr(1), optStr(2)}})
		return dest, true, nil

	case "belongstomany":
		relRetType := PtrType(StructType("Model", nil))
		if len(rawArgs) > 0 {
			if sl, ok := rawArgs[0].(*parser.StringLiteral); ok && sl.Value != "" {
				relRetType = PtrType(StructType(sl.Value, nil))
			}
		}
		dest := l.currentFn.NewTemp(relRetType, "model_btm")
		l.currBlock.AddInstruction(&CallInst{
			Dest:   dest,
			Callee: "fn_Model_belongsToMany",
			Args:   []Value{receiver, optStr(0), optStr(1), optStr(2), optStr(3), optStr(4), optStr(5)},
		})
		return dest, true, nil

	case "attach":
		dest := l.currentFn.NewTemp(TypeBool, "model_att")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_attach", Args: []Value{receiver, optBoxed(0)}})
		return dest, true, nil

	case "detach":
		hasIDs := Value(NewConstInt(0, TypeI64))
		if len(lowerArgs) > 0 {
			hasIDs = NewConstInt(1, TypeI64)
		}
		dest := l.currentFn.NewTemp(TypeBool, "model_det")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_detach", Args: []Value{receiver, optBoxed(0), hasIDs}})
		return dest, true, nil

	case "sync":
		dest := l.currentFn.NewTemp(TypeBool, "model_sync")
		l.currBlock.AddInstruction(&CallInst{Dest: dest, Callee: "fn_Model_sync", Args: []Value{receiver, optBoxed(0)}})
		return dest, true, nil
	}

	return nil, false, nil
}
