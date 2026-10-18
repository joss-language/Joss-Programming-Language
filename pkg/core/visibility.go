package core

import (
	"fmt"

	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) declaringClassOfMethod(method *parser.MethodStatement) string {
	if method == nil {
		return ""
	}
	for className, class := range r.Classes {
		if class == nil || class.Body == nil {
			continue
		}
		for _, statement := range class.Body.Statements {
			if candidate, ok := statement.(*parser.MethodStatement); ok && candidate == method {
				return className
			}
			if initStmt, ok := statement.(*parser.InitStatement); ok && initStmt.Body != nil && initStmt.Body == method.Body {
				return className
			}
		}
	}
	if r.PluginRegistry != nil {
		for _, p := range r.PluginRegistry.List() {
			if prog := p.Program(); prog != nil {
				for _, stmt := range prog.Statements {
					if class, ok := stmt.(*parser.ClassStatement); ok && class.Body != nil {
						for _, statement := range class.Body.Statements {
							if candidate, ok := statement.(*parser.MethodStatement); ok && candidate == method {
								return class.Name.Value
							}
							if initStmt, ok := statement.(*parser.InitStatement); ok && initStmt.Body != nil && initStmt.Body == method.Body {
								return class.Name.Value
							}
						}
					}
				}
			}
		}
	}
	return ""
}

func (r *Runtime) requireMemberAccess(visibility, owner, member string, line int) {
	if visibility == "" || visibility == "public" || owner == "" || r.currentClass == owner {
		return
	}
	if visibility == "protected" && r.isSubclassOf(r.currentClass, owner) {
		return
	}
	panic(&JossError{Type: "AccessViolation", Message: fmt.Sprintf("El miembro '%s::%s' es %s y no es accesible desde este contexto", owner, member, visibility), File: r.CurrentFile, Line: line})
}

func (r *Runtime) isSubclassOf(className, parentName string) bool {
	visited := map[string]bool{}
	for className != "" && !visited[className] {
		visited[className] = true
		class := r.Classes[className]
		if class == nil || class.SuperClass == nil {
			return false
		}
		if class.SuperClass.Value == parentName {
			return true
		}
		className = class.SuperClass.Value
	}
	return false
}
