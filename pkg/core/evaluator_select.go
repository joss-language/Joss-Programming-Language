package core

import (
	"fmt"
	"reflect"

	"github.com/jossecurity/joss/pkg/diagnostics"
	"github.com/jossecurity/joss/pkg/parser"
)

func (r *Runtime) executeSelect(ss *parser.SelectStatement) interface{} {
	cases := make([]reflect.SelectCase, 0, len(ss.Cases))
	type caseInfo struct {
		caseStmt *parser.SelectCaseStatement
		assignTo *parser.Identifier
		closed   bool
	}
	caseInfos := make([]caseInfo, 0, len(ss.Cases))
	var activeSends []*Channel
	var closingSignals []chan struct{}
	releaseSenders := func() {
		for _, ch := range activeSends {
			ch.endSend()
		}
		activeSends = nil
	}
	defer releaseSenders()

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
							closing, err := ch.beginSend()
							if err != nil {
								panic(err)
							}
							activeSends = append(activeSends, ch)
							closingSignals = append(closingSignals, closing)
							sendValue := reflect.ValueOf(sendVal)
							if !sendValue.IsValid() {
								sendValue = reflect.Zero(reflect.TypeOf(ch.Ch).Elem())
							}
							cases = append(cases, reflect.SelectCase{
								Dir:  reflect.SelectSend,
								Chan: reflect.ValueOf(ch.Ch),
								Send: sendValue,
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
	for _, closing := range closingSignals {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(closing)})
		caseInfos = append(caseInfos, caseInfo{closed: true})
	}

	var cancelIdx int = -1
	if r.executionContext != nil {
		cancelIdx = len(cases)
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(r.executionContext.Done()),
		})
	}

	var chosen int
	var recvVal reflect.Value
	var recvOK bool

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				if jerr, ok := rec.(*JossError); ok {
					panic(jerr)
				}
				panic(&JossError{
					Type:    "ChannelError",
					Code:    diagnostics.CodeChannelClosed,
					Message: fmt.Sprintf("Error en operación de canal durante select: %v", rec),
					File:    r.CurrentFile,
					Line:    ss.Token.Line,
				})
			}
		}()
		chosen, recvVal, recvOK = reflect.Select(cases)
	}()
	releaseSenders()

	if cancelIdx >= 0 && chosen == cancelIdx {
		r.checkExecutionCancelled()
		return nil
	}

	chosenInfo := caseInfos[chosen]
	if chosenInfo.closed {
		panic(closedChannelError())
	}

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
