package core

import (
	"fmt"
	"strings"

	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) newBlueprint() *Instance {
	class, ok := r.Classes["Blueprint"]
	if !ok {
		return nil
	}
	return &Instance{Class: class, Fields: map[string]interface{}{
		"_columns":  []map[string]string{},
		"_commands": []schemaCommand{},
	}}
}

func (r *Runtime) runBlueprint(fn interface{}) *Instance {
	blueprint := r.newBlueprint()
	if blueprint == nil {
		return nil
	}
	switch f := fn.(type) {
	case *parser.FunctionLiteral:
		r.Variables["$table"] = blueprint
		if len(f.Parameters) > 0 {
			r.Variables[f.Parameters[0].Name.Value] = blueprint
		}
		r.executeBlock(f.Body)
	case *CapturedFunction:
		r.applyFunction(f, []interface{}{blueprint})
	default:
		return nil
	}
	return blueprint
}

func (r *Runtime) executeBlueprintMethod(instance *Instance, method string, args []interface{}) interface{} {
	cols, _ := instance.Fields["_columns"].([]map[string]string)
	commands, _ := instance.Fields["_commands"].([]schemaCommand)

	// Helper to add column
	addCol := func(name, typeStr string) {
		cols = append(cols, map[string]string{"name": name, "type": typeStr})
	}

	// Helper to modify last column
	modCol := func(modifier string) {
		if len(cols) > 0 {
			lastIdx := len(cols) - 1
			cols[lastIdx]["type"] += "|" + modifier
		}
	}
	addCommand := func(command schemaCommand) {
		commands = append(commands, command)
		instance.Fields["_active_command"] = len(commands) - 1
	}
	modifyCommand := func(key string, value interface{}) {
		index, ok := instance.Fields["_active_command"].(int)
		if ok && index >= 0 && index < len(commands) {
			commands[index][key] = value
		}
	}

	switch method {
	// Numeric Types
	case "id":
		addCol("id", "bigIncrements")
	case "increments":
		if len(args) > 0 {
			addCol(args[0].(string), "increments")
		}
	case "integer":
		if len(args) > 0 {
			addCol(args[0].(string), "integer")
		}
	case "tinyInteger":
		if len(args) > 0 {
			addCol(args[0].(string), "tinyInteger")
		}
	case "smallInteger":
		if len(args) > 0 {
			addCol(args[0].(string), "smallInteger")
		}
	case "mediumInteger":
		if len(args) > 0 {
			addCol(args[0].(string), "mediumInteger")
		}
	case "bigInteger":
		if len(args) > 0 {
			addCol(args[0].(string), "bigInteger")
		}
	case "unsignedInteger":
		if len(args) > 0 {
			addCol(args[0].(string), "integer|unsigned")
		}
	case "unsignedBigInteger":
		if len(args) > 0 {
			addCol(args[0].(string), "bigInteger|unsigned")
		}
	case "float":
		if len(args) > 0 {
			addCol(args[0].(string), "float")
		}
	case "double":
		if len(args) > 0 {
			addCol(args[0].(string), "double")
		}
	case "decimal":
		if len(args) > 0 {
			precision := 8
			scale := 2
			if len(args) >= 2 {
				precision = int(args[1].(int64))
			}
			if len(args) >= 3 {
				scale = int(args[2].(int64))
			}
			addCol(args[0].(string), fmt.Sprintf("decimal(%d,%d)", precision, scale))
		}

	// String Types
	case "char":
		if len(args) > 0 {
			length := 255
			if len(args) >= 2 {
				length = int(args[1].(int64))
			}
			addCol(args[0].(string), fmt.Sprintf("char(%d)", length))
		}
	case "string":
		if len(args) > 0 {
			length := 255
			if len(args) >= 2 {
				length = int(args[1].(int64))
			}
			addCol(args[0].(string), fmt.Sprintf("string(%d)", length))
		}
	case "text":
		if len(args) > 0 {
			addCol(args[0].(string), "text")
		}
	case "mediumText":
		if len(args) > 0 {
			addCol(args[0].(string), "mediumText")
		}
	case "longText":
		if len(args) > 0 {
			addCol(args[0].(string), "longText")
		}

	// Date Types
	case "date":
		if len(args) > 0 {
			addCol(args[0].(string), "date")
		}
	case "dateTime":
		if len(args) > 0 {
			addCol(args[0].(string), "dateTime")
		}
	case "time":
		if len(args) > 0 {
			addCol(args[0].(string), "time")
		}
	case "timestamp":
		if len(args) > 0 {
			addCol(args[0].(string), "timestamp")
		}
	case "timestamps":
		addCol("created_at", "timestamp|nullable")
		addCol("updated_at", "timestamp|nullable")
	case "softDeletes":
		addCol("deleted_at", "timestamp|nullable")

	// Other Types
	case "boolean":
		if len(args) > 0 {
			addCol(args[0].(string), "boolean")
		}
	case "json":
		if len(args) > 0 {
			addCol(args[0].(string), "json")
		}
	case "enum":
		if len(args) >= 2 {
			vals := []string{}
			if list, ok := args[1].([]interface{}); ok {
				for _, v := range list {
					vals = append(vals, fmt.Sprintf("'%v'", v))
				}
			}
			addCol(args[0].(string), fmt.Sprintf("enum(%s)", strings.Join(vals, ",")))
		}

	// Modifiers
	case "nullable":
		modCol("nullable")
	case "unsigned":
		modCol("unsigned")
	case "unique":
		if len(args) > 0 && len(schemaStringList(args[0])) > 0 {
			columns := schemaStringList(args[0])
			name := ""
			if len(args) > 1 {
				name, _ = args[1].(string)
			}
			addCommand(schemaCommand{"type": "uniqueIndex", "columns": columns, "name": name})
		} else {
			modCol("unique")
		}
	case "default":
		if len(args) > 0 {
			val := args[0]
			if s, ok := val.(string); ok {
				modCol(fmt.Sprintf("default('%s')", s))
			} else {
				modCol(fmt.Sprintf("default(%v)", val))
			}
		}
	case "comment":
		if len(args) > 0 {
			modCol(fmt.Sprintf("comment('%s')", args[0].(string)))
		}

	// Table commands
	case "dropColumn":
		if len(args) > 0 {
			columns := []string{}
			for _, arg := range args {
				columns = append(columns, schemaStringList(arg)...)
			}
			addCommand(schemaCommand{"type": "dropColumn", "columns": columns})
		}
	case "renameColumn":
		if len(args) >= 2 {
			from, fromOK := args[0].(string)
			to, toOK := args[1].(string)
			if fromOK && toOK {
				addCommand(schemaCommand{"type": "renameColumn", "from": from, "to": to})
			}
		}
	case "index", "uniqueIndex":
		if len(args) > 0 {
			columns := schemaStringList(args[0])
			name := ""
			if len(args) > 1 {
				name, _ = args[1].(string)
			}
			addCommand(schemaCommand{"type": method, "columns": columns, "name": name})
		}
	case "dropIndex":
		if len(args) > 0 {
			if name, ok := args[0].(string); ok {
				addCommand(schemaCommand{"type": "dropIndex", "name": name})
			}
		}
	case "foreign":
		if len(args) > 0 {
			columns := schemaStringList(args[0])
			name := ""
			if len(args) > 1 {
				name, _ = args[1].(string)
			}
			addCommand(schemaCommand{"type": "foreign", "columns": columns, "name": name})
		}
	case "references":
		if len(args) > 0 {
			modifyCommand("references", schemaStringList(args[0]))
		}
	case "on":
		if len(args) > 0 {
			if table, ok := args[0].(string); ok {
				if prefix := r.dbPrefix(); prefix != "" && !strings.HasPrefix(table, prefix) {
					table = prefix + table
				}
				modifyCommand("table", table)
			}
		}
	case "onDelete", "onUpdate":
		if len(args) > 0 {
			if action, ok := args[0].(string); ok {
				modifyCommand(method, action)
			}
		}
	}

	instance.Fields["_columns"] = cols
	instance.Fields["_commands"] = commands
	return instance
}
