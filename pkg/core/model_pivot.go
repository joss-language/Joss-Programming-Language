package core

import (
	"fmt"
	"sort"
	"strings"
)

type pivotTarget struct {
	id    interface{}
	attrs map[string]interface{}
}

func (r *Runtime) executePivotMutation(relation *modelRelation, operation string, args []interface{}) interface{} {
	if relation == nil || relation.parent == nil || relation.parent.model == nil {
		panic(&JossError{Type: "InvalidRelation", Message: "Relación pivot incompleta"})
	}
	parentID := relation.parent.Fields[relation.localKey]
	if parentID == nil {
		panic(&JossError{Type: "InvalidRelation", Message: "La relación pivot requiere una clave padre persistida"})
	}
	createdTransaction := r.activeTx == nil
	if createdTransaction {
		tx, err := r.GetDB().Begin()
		if err != nil {
			panic(&JossError{Type: "TransactionError", Message: err.Error()})
		}
		r.activeTx = tx
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = tx.Rollback()
				r.activeTx = nil
				panic(recovered)
			}
		}()
	}
	var result interface{}
	switch operation {
	case "attach":
		if len(args) == 0 {
			panic(&JossError{Type: "InvalidRelation", Message: "attach() requiere al menos un ID"})
		}
		targets := pivotTargets(args[0], optionalPivot(args, 1))
		result = r.attachPivotTargets(relation, parentID, targets)
	case "detach":
		if len(args) == 0 {
			result = r.detachPivotTargets(relation, parentID, nil, true)
		} else {
			targets := pivotTargets(args[0], nil)
			if len(targets) == 0 {
				result = int64(0)
			} else {
				result = r.detachPivotTargets(relation, parentID, targets, false)
			}
		}
	case "sync":
		if len(args) == 0 {
			panic(&JossError{Type: "InvalidRelation", Message: "sync() requiere IDs deseados"})
		}
		result = r.syncPivotTargets(relation, parentID, pivotTargets(args[0], nil))
	}
	if createdTransaction {
		tx := r.activeTx
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			r.activeTx = nil
			panic(&JossError{Type: "TransactionError", Message: err.Error()})
		}
		r.activeTx = nil
	}
	return result
}

func optionalPivot(args []interface{}, index int) map[string]interface{} {
	if len(args) > index {
		if value, ok := args[index].(map[string]interface{}); ok {
			return value
		}
	}
	return nil
}

func pivotTargets(value interface{}, common map[string]interface{}) []pivotTarget {
	copyAttrs := func(source map[string]interface{}) map[string]interface{} {
		result := map[string]interface{}{}
		for key, item := range source {
			result[key] = item
		}
		return result
	}
	switch typed := value.(type) {
	case []interface{}:
		result := make([]pivotTarget, 0, len(typed))
		for _, item := range typed {
			result = append(result, pivotTargets(item, common)...)
		}
		return result
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := make([]pivotTarget, 0, len(keys))
		for _, key := range keys {
			attrs := copyAttrs(common)
			if extra, ok := typed[key].(map[string]interface{}); ok {
				for name, item := range extra {
					attrs[name] = item
				}
			}
			result = append(result, pivotTarget{id: key, attrs: attrs})
		}
		return result
	case nil:
		return nil
	default:
		return []pivotTarget{{id: typed, attrs: copyAttrs(common)}}
	}
}

func (r *Runtime) attachPivotTargets(relation *modelRelation, parentID interface{}, targets []pivotTarget) int64 {
	var affected int64
	for _, target := range targets {
		if target.id == nil {
			panic(&JossError{Type: "InvalidRelation", Message: "attach() no acepta ID null"})
		}
		var exists int
		check := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ? AND %s = ?", r.pivotTable(relation), quoteIdentifier(relation.foreignPivotKey), quoteIdentifier(relation.relatedPivotKey))
		if err := r.databaseExecutor().QueryRow(check, parentID, target.id).Scan(&exists); err != nil {
			panic(&JossError{Type: "RelationError", Message: err.Error()})
		}
		if exists > 0 {
			continue
		}
		row := map[string]interface{}{relation.foreignPivotKey: parentID, relation.relatedPivotKey: target.id}
		for name, value := range target.attrs {
			if name == relation.foreignPivotKey || name == relation.relatedPivotKey {
				panic(&JossError{Type: "InvalidRelation", Message: "La metadata pivot no puede reemplazar claves"})
			}
			row[name] = value
		}
		result := r.insertFromMap(r.pivotTable(relation), row, false)
		if result == true {
			affected++
		}
	}
	return affected
}

func (r *Runtime) detachPivotTargets(relation *modelRelation, parentID interface{}, targets []pivotTarget, all bool) int64 {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", r.pivotTable(relation), quoteIdentifier(relation.foreignPivotKey))
	bindings := []interface{}{parentID}
	if !all {
		if len(targets) == 0 {
			return 0
		}
		placeholders := make([]string, len(targets))
		for index, target := range targets {
			if target.id == nil {
				panic(&JossError{Type: "InvalidRelation", Message: "detach() no acepta ID null"})
			}
			placeholders[index] = "?"
			bindings = append(bindings, target.id)
		}
		query += fmt.Sprintf(" AND %s IN (%s)", quoteIdentifier(relation.relatedPivotKey), strings.Join(placeholders, ","))
	}
	result, err := r.databaseExecutor().Exec(query, bindings...)
	if err != nil {
		panic(&JossError{Type: "RelationError", Message: err.Error()})
	}
	affected, _ := result.RowsAffected()
	return affected
}

func (r *Runtime) syncPivotTargets(relation *modelRelation, parentID interface{}, desired []pivotTarget) map[string]interface{} {
	currentRows, err := r.databaseExecutor().Query(fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", quoteIdentifier(relation.relatedPivotKey), r.pivotTable(relation), quoteIdentifier(relation.foreignPivotKey)), parentID)
	if err != nil {
		panic(&JossError{Type: "RelationError", Message: err.Error()})
	}
	current := map[string]interface{}{}
	for currentRows.Next() {
		var id interface{}
		if err := currentRows.Scan(&id); err != nil {
			currentRows.Close()
			panic(&JossError{Type: "RelationError", Message: err.Error()})
		}
		current[pivotToken(id)] = id
	}
	currentRows.Close()
	wanted := map[string]pivotTarget{}
	for _, target := range desired {
		if target.id == nil {
			panic(&JossError{Type: "InvalidRelation", Message: "sync() no acepta ID null"})
		}
		wanted[pivotToken(target.id)] = target
	}
	toAttach := []pivotTarget{}
	toDetach := []pivotTarget{}
	toUpdate := []pivotTarget{}
	for token, target := range wanted {
		if _, ok := current[token]; !ok {
			toAttach = append(toAttach, target)
		} else if len(target.attrs) > 0 {
			toUpdate = append(toUpdate, target)
		}
	}
	for token, id := range current {
		if _, ok := wanted[token]; !ok {
			toDetach = append(toDetach, pivotTarget{id: id})
		}
	}
	attached := r.attachPivotTargets(relation, parentID, toAttach)
	var updated int64
	for _, target := range toUpdate {
		updated += r.updatePivotTarget(relation, parentID, target)
	}
	detached := r.detachPivotTargets(relation, parentID, toDetach, false)
	return map[string]interface{}{"attached": attached, "detached": detached, "updated": updated}
}

func (r *Runtime) updatePivotTarget(relation *modelRelation, parentID interface{}, target pivotTarget) int64 {
	if len(target.attrs) == 0 {
		return 0
	}
	clauses := []string{}
	bindings := []interface{}{}
	for _, name := range sortedMapKeys(target.attrs) {
		if name == relation.foreignPivotKey || name == relation.relatedPivotKey {
			panic(&JossError{Type: "InvalidRelation", Message: "La metadata pivot no puede reemplazar claves"})
		}
		clauses = append(clauses, quoteIdentifier(name)+" = ?")
		bindings = append(bindings, target.attrs[name])
	}
	bindings = append(bindings, parentID, target.id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ? AND %s = ?", r.pivotTable(relation), strings.Join(clauses, ", "), quoteIdentifier(relation.foreignPivotKey), quoteIdentifier(relation.relatedPivotKey))
	result, err := r.databaseExecutor().Exec(query, bindings...)
	if err != nil {
		panic(&JossError{Type: "RelationError", Message: err.Error()})
	}
	affected, _ := result.RowsAffected()
	return affected
}

func pivotToken(value interface{}) string { return fmt.Sprintf("%v", value) }
func (r *Runtime) pivotTable(relation *modelRelation) string {
	return quoteIdentifier(r.applyTablePrefix(relation.pivotTable))
}

func (r *Runtime) getBelongsToMany(query *Instance) []interface{} {
	relation := query.model.relation
	parentID := relation.parent.Fields[relation.localKey]
	pivotRows, err := r.databaseExecutor().Query(fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", r.pivotTable(relation), quoteIdentifier(relation.foreignPivotKey)), parentID)
	if err != nil {
		panic(&JossError{Type: "RelationError", Message: err.Error()})
	}
	rows := rowsToMap(pivotRows)
	pivotRows.Close()
	ids := make([]interface{}, 0, len(rows))
	pivotByID := map[string]map[string]interface{}{}
	for _, row := range rows {
		id := row[relation.relatedPivotKey]
		ids = append(ids, id)
		pivot := cloneValueMap(row)
		delete(pivot, relation.foreignPivotKey)
		delete(pivot, relation.relatedPivotKey)
		pivotByID[pivotToken(id)] = pivot
	}
	if len(ids) == 0 {
		return []interface{}{}
	}
	r.executeGranDBMethod(query, "whereIn", []interface{}{relation.relatedKey, ids})
	items := r.executeGetMethod(query, nil).([]interface{})
	for _, item := range items {
		model := item.(*Instance)
		model.Fields["pivot"] = pivotByID[pivotToken(model.Fields[relation.relatedKey])]
	}
	return items
}

func (r *Runtime) eagerLoadBelongsToMany(parents []*Instance, name string, query *Instance) []*Instance {
	relation := query.model.relation
	parentIDs := []interface{}{}
	parentByToken := map[string]*Instance{}
	for _, parent := range parents {
		id := parent.Fields[relation.localKey]
		if id != nil {
			token := pivotToken(id)
			if _, ok := parentByToken[token]; !ok {
				parentIDs = append(parentIDs, id)
				parentByToken[token] = parent
			}
		}
		parent.model.relations[name] = []interface{}{}
		parent.model.loaded[name] = true
		parent.Fields[name] = []interface{}{}
	}
	if len(parentIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(parentIDs))
	for index := range placeholders {
		placeholders[index] = "?"
	}
	sqlText := fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)", r.pivotTable(relation), quoteIdentifier(relation.foreignPivotKey), strings.Join(placeholders, ","))
	rows, err := r.databaseExecutor().Query(sqlText, parentIDs...)
	if err != nil {
		panic(&JossError{Type: "RelationError", Message: err.Error()})
	}
	pivotRows := rowsToMap(rows)
	rows.Close()
	relatedIDs := []interface{}{}
	seen := map[string]bool{}
	pivotsByParent := map[string][]map[string]interface{}{}
	for _, row := range pivotRows {
		parentToken := pivotToken(row[relation.foreignPivotKey])
		relatedID := row[relation.relatedPivotKey]
		token := pivotToken(relatedID)
		if !seen[token] {
			seen[token] = true
			relatedIDs = append(relatedIDs, relatedID)
		}
		pivotsByParent[parentToken] = append(pivotsByParent[parentToken], row)
	}
	if len(relatedIDs) == 0 {
		return nil
	}
	r.executeGranDBMethod(query, "whereIn", []interface{}{relation.relatedKey, relatedIDs})
	items := r.executeGetMethod(query, nil).([]interface{})
	relatedByID := map[string]*Instance{}
	for _, item := range items {
		model := item.(*Instance)
		relatedByID[pivotToken(model.Fields[relation.relatedKey])] = model
	}
	children := []*Instance{}
	for parentToken, pivots := range pivotsByParent {
		parent := parentByToken[parentToken]
		assigned := []interface{}{}
		for _, row := range pivots {
			base := relatedByID[pivotToken(row[relation.relatedPivotKey])]
			if base == nil {
				continue
			}
			model := base.Clone()
			pivot := cloneValueMap(row)
			delete(pivot, relation.foreignPivotKey)
			delete(pivot, relation.relatedPivotKey)
			model.Fields["pivot"] = pivot
			assigned = append(assigned, model)
			children = append(children, model)
		}
		parent.model.relations[name] = assigned
		parent.Fields[name] = assigned
	}
	return children
}
