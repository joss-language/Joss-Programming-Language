package core

import (
	"fmt"
	"reflect"

	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) executeSelect(ss *parser.SelectStatement) interface{} {
	cases := make([]reflect.SelectCase, 0, len(ss.Cases))
	type caseInfo struct {
		caseStmt *parser.SelectCaseStatement
		assignTo *parser.Identifier
	}
	caseInfos := make([]caseInfo, 0, len(ss.Cases))

	for _, cs := range ss.Cases {
		if cs.IsDefault {
			cases = append(cases, reflect.SelectCase{
				Dir: reflect.SelectDefault,
			})
			caseInfos = append(caseInfos, caseInfo{caseStmt: cs})
			continue
		}

		var commStmt parser.Statement = cs.Comm
		var assignIdent *parser.Identifier

		if exprStmt, ok := commStmt.(*parser.ExpressionStatement); ok {
			if assign, isAssign := exprStmt.Expression.(*parser.AssignExpression); isAssign {
				if ident, isIdent := assign.Left.(*parser.Identifier); isIdent {
					assignIdent = ident
				}
				if call, isCall := assign.Value.(*parser.CallExpression); isCall {
					commStmt = &parser.ExpressionStatement{Token: exprStmt.Token, Expression: call}
				}
			}
		}

		if exprStmt, ok := commStmt.(*parser.ExpressionStatement); ok {
			if call, ok := exprStmt.Expression.(*parser.CallExpression); ok {
				if ident, ok := call.Function.(*parser.Identifier); ok {
					switch ident.Value {
					case "send":
						if len(call.Arguments) >= 2 {
							chVal := r.evaluateExpression(call.Arguments[0])
							ch, ok := chVal.(*Channel)
							if !ok {
								panic(fmt.Sprintf("SelectError: Se esperaba un Channel para send(), se obtuvo %T", chVal))
							}
							sendVal := r.evaluateExpression(call.Arguments[1])
							cases = append(cases, reflect.SelectCase{
								Dir:  reflect.SelectSend,
								Chan: reflect.ValueOf(ch.Ch),
								Send: reflect.ValueOf(sendVal),
							})
							caseInfos = append(caseInfos, caseInfo{caseStmt: cs})
							continue
						}
					case "recv":
						if len(call.Arguments) >= 1 {
							chVal := r.evaluateExpression(call.Arguments[0])
							ch, ok := chVal.(*Channel)
							if !ok {
								panic(fmt.Sprintf("SelectError: Se esperaba un Channel para recv(), se obtuvo %T", chVal))
							}
							cases = append(cases, reflect.SelectCase{
								Dir:  reflect.SelectRecv,
								Chan: reflect.ValueOf(ch.Ch),
							})
							caseInfos = append(caseInfos, caseInfo{caseStmt: cs, assignTo: assignIdent})
							continue
						}
					}
				}
			}
		}

		panic("SelectError: Las clausulas case de select deben ser de la forma `case send($ch, $val):`, `case recv($ch):` o `case $var = recv($ch):`")
	}

	if len(cases) == 0 {
		return nil
	}

	chosen, recvVal, recvOK := reflect.Select(cases)
	chosenInfo := caseInfos[chosen]

	if chosenInfo.assignTo != nil {
		var val interface{}
		if recvOK && recvVal.IsValid() {
			val = recvVal.Interface()
		} else {
			val = nil
		}
		if _, resolved := r.slotForIdentifier(chosenInfo.assignTo); resolved {
			r.assignLocal(chosenInfo.assignTo, val, false)
		} else {
			r.Variables[chosenInfo.assignTo.Value] = val
		}
	}

	if chosenInfo.caseStmt.Body != nil {
		return r.executeBlock(chosenInfo.caseStmt.Body)
	}

	return nil
}
