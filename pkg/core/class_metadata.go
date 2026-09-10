package core

import (
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/typesystem"
)

type classFieldInfo struct {
	Declaration  *parser.LetStatement
	OwnerClass   string
	IsConst      bool
	DeclaredType string
	ParsedType   typesystem.Type
}

type classMethodInfo struct {
	Method     *parser.MethodStatement
	OwnerClass string
}

type classMetadata struct {
	Class       *parser.ClassStatement
	SuperClass  string
	Methods     map[string]*classMethodInfo
	Fields      map[string]*classFieldInfo
	FieldOrder  []*classFieldInfo
	Constructor *parser.MethodStatement
}

func (r *Runtime) lookupClassMetadata(className string) *classMetadata {
	if className == "" {
		return nil
	}
	r.planMu.Lock()
	defer r.planMu.Unlock()
	if r.classMetadataCache == nil {
		r.classMetadataCache = make(map[string]*classMetadata)
	}
	if meta, ok := r.classMetadataCache[className]; ok && meta != nil {
		return meta
	}

	classStmt, ok := r.Classes[className]
	if !ok || classStmt == nil {
		return nil
	}

	chain := []*parser.ClassStatement{}
	visited := make(map[string]bool)
	for curr := classStmt; curr != nil && !visited[curr.Name.Value]; {
		visited[curr.Name.Value] = true
		chain = append([]*parser.ClassStatement{curr}, chain...)
		if curr.SuperClass != nil {
			curr = r.Classes[curr.SuperClass.Value]
		} else {
			break
		}
	}

	meta := &classMetadata{
		Class:      classStmt,
		Methods:    make(map[string]*classMethodInfo),
		Fields:     make(map[string]*classFieldInfo),
		FieldOrder: make([]*classFieldInfo, 0),
	}
	if classStmt.SuperClass != nil {
		meta.SuperClass = classStmt.SuperClass.Value
	}

	for _, cls := range chain {
		if cls.Body == nil {
			continue
		}
		for _, stmt := range cls.Body.Statements {
			switch node := stmt.(type) {
			case *parser.LetStatement:
				if node.Name == nil {
					continue
				}
				declaredType := node.Token.Literal
				if declaredType == "var" {
					declaredType = ""
				}
				info := &classFieldInfo{
					Declaration:  node,
					OwnerClass:   cls.Name.Value,
					IsConst:      node.IsConst,
					DeclaredType: declaredType,
					ParsedType:   typesystem.Parse(declaredType),
				}
				meta.Fields[node.Name.Value] = info
				meta.FieldOrder = append(meta.FieldOrder, info)
			case *parser.MethodStatement:
				if node.Name == nil {
					continue
				}
				meta.Methods[node.Name.Value] = &classMethodInfo{
					Method:     node,
					OwnerClass: cls.Name.Value,
				}
				if node.Name.Value == "constructor" || node.Name.Value == "main" || node.Name.Value == "Init" {
					if cls == classStmt {
						meta.Constructor = node
					}
					for _, param := range node.Parameters {
						if param != nil && param.Name != nil && param.Visibility.Literal != "" {
							fieldName := param.Name.Value
							if _, exists := meta.Fields[fieldName]; !exists {
								declaredType := param.Type.Literal
								if declaredType == "var" {
									declaredType = ""
								}
								decl := &parser.LetStatement{
									Token:      param.Type,
									Name:       param.Name,
									Value:      param.DefaultValue,
									Visibility: param.Visibility.Literal,
								}
								info := &classFieldInfo{
									Declaration:  decl,
									OwnerClass:   cls.Name.Value,
									DeclaredType: declaredType,
									ParsedType:   typesystem.Parse(declaredType),
								}
								meta.Fields[fieldName] = info
								meta.FieldOrder = append(meta.FieldOrder, info)
							}
						}
					}
				}
			case *parser.InitStatement:
				if node.Name == nil {
					continue
				}
				if node.Name.Value == "constructor" || node.Name.Value == "main" || node.Name.Value == "Init" {
					method := &parser.MethodStatement{
						Token:      node.Token,
						Name:       node.Name,
						Parameters: node.Parameters,
						Body:       node.Body,
					}
					if cls == classStmt {
						meta.Constructor = method
					}
					meta.Methods[node.Name.Value] = &classMethodInfo{
						Method:     method,
						OwnerClass: cls.Name.Value,
					}
					for _, param := range node.Parameters {
						if param != nil && param.Name != nil && param.Visibility.Literal != "" {
							fieldName := param.Name.Value
							if _, exists := meta.Fields[fieldName]; !exists {
								declaredType := param.Type.Literal
								if declaredType == "var" {
									declaredType = ""
								}
								decl := &parser.LetStatement{
									Token:      param.Type,
									Name:       param.Name,
									Value:      param.DefaultValue,
									Visibility: param.Visibility.Literal,
								}
								info := &classFieldInfo{
									Declaration:  decl,
									OwnerClass:   cls.Name.Value,
									DeclaredType: declaredType,
									ParsedType:   typesystem.Parse(declaredType),
								}
								meta.Fields[fieldName] = info
								meta.FieldOrder = append(meta.FieldOrder, info)
							}
						}
					}
				}
			}
		}
	}

	r.classMetadataCache[className] = meta
	return meta
}
