package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/shopspring/decimal"
)

// modelMetadata is immutable after class metadata construction. It is owned by
// the Runtime class metadata cache and shared by model instances from that runtime.
type modelMetadata struct {
	Class        *parser.ClassStatement
	ClassName    string
	Table        string
	PrimaryKey   string
	KeyType      string
	Incrementing bool
	Timestamps   bool
	CreatedAt    string
	UpdatedAt    string
	DeletedAt    string
	SoftDeletes  bool
	Fillable     map[string]struct{}
	Guarded      map[string]struct{}
	Hidden       map[string]struct{}
	Visible      map[string]struct{}
	Casts        map[string]string
}

type modelState struct {
	metadata    *modelMetadata
	original    map[string]interface{}
	lastChanges map[string]interface{}
	relations   map[string]interface{}
	loaded      map[string]bool
	exists      bool
	deleted     bool
	query       bool
	relation    *modelRelation
}

type modelRelation struct {
	kind            string
	parent          *Instance
	foreignKey      string
	localKey        string
	pivotTable      string
	foreignPivotKey string
	relatedPivotKey string
	relatedKey      string
}

var modelConfigurationFields = map[string]struct{}{
	"table": {}, "primaryKey": {}, "keyType": {}, "incrementing": {},
	"timestamps": {}, "createdAt": {}, "updatedAt": {}, "fillable": {},
	"guarded": {}, "hidden": {}, "visible": {}, "casts": {}, "softDeletes": {}, "deletedAt": {},
}

func buildModelMetadata(class *classMetadata, chain []*parser.ClassStatement) *modelMetadata {
	if class == nil || class.Class == nil || class.Class.Name == nil || class.Class.Name.Value == "Model" {
		return nil
	}
	isModel := false
	for _, current := range chain {
		if current != nil && current.Name != nil && current.Name.Value == "Model" {
			isModel = true
			break
		}
	}
	if !isModel {
		return nil
	}
	name := class.Class.Name.Value
	metadata := &modelMetadata{
		Class: class.Class, ClassName: name, Table: strings.ToLower(pluralizeWord(name)),
		PrimaryKey: "id", KeyType: "int", Incrementing: true, Timestamps: true,
		CreatedAt: "created_at", UpdatedAt: "updated_at", DeletedAt: "deleted_at",
		Fillable: map[string]struct{}{}, Guarded: map[string]struct{}{"*": {}},
		Hidden: map[string]struct{}{}, Visible: map[string]struct{}{}, Casts: map[string]string{},
	}
	for _, field := range class.FieldOrder {
		if field == nil || field.Declaration == nil || field.Declaration.Name == nil {
			continue
		}
		property := field.Declaration.Name.Value
		if _, configured := modelConfigurationFields[property]; !configured || field.OwnerClass == "Model" {
			continue
		}
		value, ok := modelLiteral(field.Declaration.Value)
		if !ok {
			panic(&JossError{Type: "ModelMetadataError", Message: fmt.Sprintf("La configuración %s::%s debe ser un literal", name, property), Line: field.Declaration.Name.Token.Line})
		}
		switch property {
		case "table":
			metadata.Table = requireModelString(name, property, value)
		case "primaryKey":
			metadata.PrimaryKey = requireModelString(name, property, value)
		case "keyType":
			metadata.KeyType = requireModelString(name, property, value)
		case "createdAt":
			metadata.CreatedAt = requireModelString(name, property, value)
		case "updatedAt":
			metadata.UpdatedAt = requireModelString(name, property, value)
		case "deletedAt":
			metadata.DeletedAt = requireModelString(name, property, value)
		case "incrementing":
			metadata.Incrementing = requireModelBool(name, property, value)
		case "timestamps":
			metadata.Timestamps = requireModelBool(name, property, value)
		case "softDeletes":
			metadata.SoftDeletes = requireModelBool(name, property, value)
		case "fillable":
			metadata.Fillable = stringSet(name, property, value)
		case "guarded":
			metadata.Guarded = stringSet(name, property, value)
		case "hidden":
			metadata.Hidden = stringSet(name, property, value)
		case "visible":
			metadata.Visible = stringSet(name, property, value)
		case "casts":
			metadata.Casts = castMap(name, value)
		}
	}
	return metadata
}

func (r *Runtime) lookupModelMetadata(className string) *modelMetadata {
	metadata := r.lookupClassMetadata(className)
	if metadata == nil {
		return nil
	}
	return metadata.Model
}

func modelLiteral(expression parser.Expression) (interface{}, bool) {
	switch value := expression.(type) {
	case nil:
		return nil, true
	case *parser.StringLiteral:
		return value.Value, true
	case *parser.Boolean:
		return value.Value, true
	case *parser.IntegerLiteral:
		return value.Value, true
	case *parser.FloatLiteral:
		return value.Value, true
	case *parser.DecimalLiteral:
		return value.Value, true
	case *parser.NullLiteral:
		return nil, true
	case *parser.ArrayLiteral:
		result := make([]interface{}, 0, len(value.Elements))
		for _, item := range value.Elements {
			literal, ok := modelLiteral(item)
			if !ok {
				return nil, false
			}
			result = append(result, literal)
		}
		return result, true
	case *parser.MapLiteral:
		result := make(map[string]interface{}, len(value.Pairs))
		for key, item := range value.Pairs {
			literalKey, ok := modelLiteral(key)
			if !ok {
				return nil, false
			}
			literalValue, ok := modelLiteral(item)
			if !ok {
				return nil, false
			}
			result[fmt.Sprint(literalKey)] = literalValue
		}
		return result, true
	default:
		return nil, false
	}
}

func requireModelString(className, property string, value interface{}) string {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		panic(&JossError{Type: "ModelMetadataError", Message: fmt.Sprintf("%s::%s debe ser string no vacío", className, property)})
	}
	return text
}
func requireModelBool(className, property string, value interface{}) bool {
	result, ok := value.(bool)
	if !ok {
		panic(&JossError{Type: "ModelMetadataError", Message: fmt.Sprintf("%s::%s debe ser bool", className, property)})
	}
	return result
}
func stringSet(className, property string, value interface{}) map[string]struct{} {
	items, ok := value.([]interface{})
	if !ok {
		panic(&JossError{Type: "ModelMetadataError", Message: fmt.Sprintf("%s::%s debe ser array de string", className, property)})
	}
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			panic(&JossError{Type: "ModelMetadataError", Message: fmt.Sprintf("%s::%s debe contener solo string", className, property)})
		}
		result[text] = struct{}{}
	}
	return result
}
func castMap(className string, value interface{}) map[string]string {
	items, ok := value.(map[string]interface{})
	if !ok {
		panic(&JossError{Type: "ModelMetadataError", Message: fmt.Sprintf("%s::casts debe ser map", className)})
	}
	result := make(map[string]string, len(items))
	for field, raw := range items {
		cast, ok := raw.(string)
		if !ok || !validModelCast(cast) {
			panic(&JossError{Type: "InvalidCast", Message: fmt.Sprintf("Cast inválido %s::casts[%s]", className, field)})
		}
		result[field] = strings.ToLower(cast)
	}
	return result
}
func validModelCast(cast string) bool {
	switch strings.ToLower(cast) {
	case "int", "float", "decimal", "string", "bool", "date", "datetime", "json", "array", "object":
		return true
	}
	return false
}

func newModelState(metadata *modelMetadata, exists, query bool) *modelState {
	if metadata == nil {
		return nil
	}
	return &modelState{metadata: metadata, original: map[string]interface{}{}, lastChanges: map[string]interface{}{}, relations: map[string]interface{}{}, loaded: map[string]bool{}, exists: exists, query: query}
}
func (state *modelState) clone() *modelState {
	if state == nil {
		return nil
	}
	clone := newModelState(state.metadata, state.exists, state.query)
	clone.deleted = state.deleted
	if state.relation != nil {
		relation := *state.relation
		clone.relation = &relation
	}
	clone.original = cloneValueMap(state.original)
	clone.lastChanges = cloneValueMap(state.lastChanges)
	clone.relations = cloneValueMap(state.relations)
	for key, value := range state.loaded {
		clone.loaded[key] = value
	}
	return clone
}
func removeModelConfigurationFields(fields map[string]interface{}) {
	for name := range modelConfigurationFields {
		delete(fields, name)
	}
}

func (r *Runtime) hydrateModel(metadata *modelMetadata, row map[string]interface{}) *Instance {
	attributes := make(map[string]interface{}, len(row))
	for name, raw := range row {
		attributes[name] = castModelValue(metadata, name, raw)
	}
	instance := &Instance{Class: metadata.Class, Fields: attributes, Constants: make(map[string]bool), model: newModelState(metadata, true, false)}
	instance.model.original = cloneValueMap(attributes)
	return instance
}
func (r *Runtime) hydrateModelRows(metadata *modelMetadata, rows []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(rows))
	for index, row := range rows {
		result[index] = r.hydrateModel(metadata, row)
	}
	return result
}

func castModelValue(metadata *modelMetadata, field string, raw interface{}) interface{} {
	if raw == nil || metadata == nil {
		return raw
	}
	cast := metadata.Casts[field]
	switch cast {
	case "":
		return raw
	case "string":
		return fmt.Sprint(raw)
	case "int":
		value, err := strconv.ParseInt(fmt.Sprint(raw), 10, 64)
		if err == nil {
			return value
		}
	case "float":
		value, err := strconv.ParseFloat(fmt.Sprint(raw), 64)
		if err == nil {
			return value
		}
	case "decimal":
		value, err := decimal.NewFromString(fmt.Sprint(raw))
		if err == nil {
			return value
		}
	case "bool":
		switch value := raw.(type) {
		case bool:
			return value
		case int64:
			return value != 0
		case int:
			return value != 0
		case []byte:
			parsed, err := strconv.ParseBool(string(value))
			if err == nil {
				return parsed
			}
		}
		parsed, err := strconv.ParseBool(fmt.Sprint(raw))
		if err == nil {
			return parsed
		}
	case "json", "array", "object":
		var result interface{}
		var bytes []byte
		switch value := raw.(type) {
		case string:
			bytes = []byte(value)
		case []byte:
			bytes = value
		default:
			encoded, err := json.Marshal(value)
			if err == nil {
				bytes = encoded
			}
		}
		if len(bytes) > 0 && json.Unmarshal(bytes, &result) == nil {
			return result
		}
	case "date", "datetime":
		if value, ok := raw.(time.Time); ok {
			return value
		}
		formats := []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"}
		for _, format := range formats {
			if value, err := time.Parse(format, fmt.Sprint(raw)); err == nil {
				return value
			}
		}
	}
	panic(&JossError{Type: "InvalidCast", Message: fmt.Sprintf("No se puede convertir %s.%s a %s", metadata.ClassName, field, cast)})
}

func (state *modelState) serializable(fields map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for name, value := range fields {
		if strings.HasPrefix(name, "_") {
			continue
		}
		if len(state.metadata.Visible) > 0 {
			if _, ok := state.metadata.Visible[name]; !ok {
				continue
			}
		}
		if _, hidden := state.metadata.Hidden[name]; hidden {
			continue
		}
		result[name] = value
	}
	for name, value := range state.relations {
		if state.loaded[name] {
			result[name] = value
		}
	}
	return result
}
func cloneValueMap(source map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
func modelChanges(instance *Instance) map[string]interface{} {
	result := map[string]interface{}{}
	if instance == nil || instance.model == nil {
		return result
	}
	for name, value := range instance.Fields {
		if strings.HasPrefix(name, "_") {
			continue
		}
		original, ok := instance.model.original[name]
		if !ok || !reflect.DeepEqual(original, value) {
			result[name] = value
		}
	}
	for name := range instance.model.original {
		if _, ok := instance.Fields[name]; !ok {
			result[name] = nil
		}
	}
	return result
}

func (r *Runtime) executeModelMethod(instance *Instance, method string, args []interface{}) interface{} {
	if instance == nil || instance.Class == nil || instance.Class.Name == nil {
		panic(&JossError{Type: "ModelError", Message: "Model requiere una clase concreta"})
	}
	metadata := r.lookupModelMetadata(instance.Class.Name.Value)
	if metadata == nil {
		panic(&JossError{Type: "ModelError", Message: fmt.Sprintf("%s no es un modelo concreto", instance.Class.Name.Value)})
	}
	lower := strings.ToLower(method)
	switch lower {
	case "get":
		if instance.model != nil && instance.model.relation != nil && instance.model.relation.kind == "belongstomany" {
			return r.getBelongsToMany(instance)
		}
	case "first":
		if instance.model != nil && instance.model.relation != nil && instance.model.relation.kind == "belongstomany" {
			items := r.getBelongsToMany(instance)
			if len(items) > 0 {
				return items[0]
			}
			return nil
		}
	case "query":
		return r.newModelQuery(metadata)
	case "all":
		return r.executeGranDBMethod(r.newModelQuery(metadata), "get", nil)
	case "create":
		if len(args) != 1 {
			panic(&JossError{Type: "ModelError", Message: "create() requiere un map de atributos"})
		}
		attributes, ok := args[0].(map[string]interface{})
		if !ok {
			panic(&JossError{Type: "ModelError", Message: "create() requiere un map de atributos"})
		}
		created := &Instance{Class: metadata.Class, Fields: map[string]interface{}{}, Constants: map[string]bool{}, model: newModelState(metadata, false, false)}
		r.fillModel(created, attributes, false)
		r.saveModel(created)
		return created
	case "firstornew", "firstorcreate", "updateorcreate":
		return r.firstOrWriteModel(metadata, lower, args)
	case "fill", "forcefill":
		if len(args) != 1 {
			panic(&JossError{Type: "ModelError", Message: method + "() requiere un map de atributos"})
		}
		attributes, ok := args[0].(map[string]interface{})
		if !ok {
			panic(&JossError{Type: "ModelError", Message: method + "() requiere un map de atributos"})
		}
		r.ensureModelState(instance, metadata)
		r.fillModel(instance, attributes, lower == "forcefill")
		return instance
	case "save":
		r.ensureModelState(instance, metadata)
		return r.saveModel(instance)
	case "delete":
		if instance.model != nil && !instance.model.query {
			return r.deleteModel(instance)
		}
	case "restore":
		r.ensureModelState(instance, metadata)
		return r.restoreModel(instance)
	case "forcedelete":
		r.ensureModelState(instance, metadata)
		return r.forceDeleteModel(instance)
	case "withtrashed", "onlytrashed", "withouttrashed":
		query := instance
		if instance.model == nil || !instance.model.query {
			query = r.newModelQuery(metadata)
		}
		r.configureSoftDeleteScope(query, lower)
		return query
	case "scope":
		query := instance
		if instance.model == nil || !instance.model.query {
			query = r.newModelQuery(metadata)
		}
		return r.applyLocalModelScope(query, args)
	case "refresh":
		r.ensureModelState(instance, metadata)
		return r.refreshModel(instance)
	case "belongsto", "hasone", "hasmany", "belongstomany":
		r.ensureModelState(instance, metadata)
		return r.newRelationQuery(instance, lower, args)
	case "attach", "detach", "sync":
		if instance.model == nil || instance.model.relation == nil || instance.model.relation.kind != "belongstomany" {
			panic(&JossError{Type: "InvalidRelation", Message: method + "() requiere belongsToMany"})
		}
		return r.executePivotMutation(instance.model.relation, lower, args)
	case "with":
		query := instance
		if instance.model == nil || !instance.model.query {
			query = r.newModelQuery(metadata)
		}
		names := relationNames(args)
		query.Fields["_with"] = append(relationNames([]interface{}{query.Fields["_with"]}), names...)
		return query
	case "load", "loadmissing":
		r.ensureModelState(instance, metadata)
		r.eagerLoadModels([]*Instance{instance}, relationNames(args), lower == "loadmissing")
		return instance
	case "isdirty":
		r.ensureModelState(instance, metadata)
		changes := modelChanges(instance)
		if len(args) > 0 {
			_, ok := changes[fmt.Sprint(args[0])]
			return ok
		}
		return len(changes) > 0
	case "isclean":
		r.ensureModelState(instance, metadata)
		changes := modelChanges(instance)
		if len(args) > 0 {
			_, ok := changes[fmt.Sprint(args[0])]
			return !ok
		}
		return len(changes) == 0
	case "waschanged":
		r.ensureModelState(instance, metadata)
		if len(args) > 0 {
			_, ok := instance.model.lastChanges[fmt.Sprint(args[0])]
			return ok
		}
		return len(instance.model.lastChanges) > 0
	case "getoriginal":
		r.ensureModelState(instance, metadata)
		if len(args) > 0 {
			return instance.model.original[fmt.Sprint(args[0])]
		}
		return cloneValueMap(instance.model.original)
	case "getchanges":
		r.ensureModelState(instance, metadata)
		return modelChanges(instance)
	case "tomap":
		r.ensureModelState(instance, metadata)
		return instance.model.serializable(instance.Fields)
	case "tojson":
		r.ensureModelState(instance, metadata)
		encoded, err := json.Marshal(instance.model.serializable(instance.Fields))
		if err != nil {
			panic(&JossError{Type: "ModelSerializationError", Message: err.Error()})
		}
		return string(encoded)
	}
	query := instance
	if instance.model == nil || !instance.model.query {
		query = r.newModelQuery(metadata)
	}
	return r.executeGranDBMethod(query, method, args)
}

func (r *Runtime) firstOrWriteModel(metadata *modelMetadata, operation string, args []interface{}) *Instance {
	if len(args) == 0 {
		panic(&JossError{Type: "ModelError", Message: operation + "() requiere atributos de búsqueda"})
	}
	attributes, ok := args[0].(map[string]interface{})
	if !ok || len(attributes) == 0 {
		panic(&JossError{Type: "ModelError", Message: operation + "() requiere un map no vacío"})
	}
	values := map[string]interface{}{}
	if len(args) > 1 {
		var valid bool
		values, valid = args[1].(map[string]interface{})
		if !valid {
			panic(&JossError{Type: "ModelError", Message: operation + "() requiere un segundo map"})
		}
	}
	query := r.newModelQuery(metadata)
	for _, name := range sortedMapKeys(attributes) {
		r.executeGranDBMethod(query, "where", []interface{}{name, attributes[name]})
	}
	found := r.executeFirstMethod(query, nil)
	if model, ok := found.(*Instance); ok {
		if operation == "updateorcreate" && len(values) > 0 {
			r.fillModel(model, values, false)
			r.saveModel(model)
		}
		return model
	}
	model := &Instance{Class: metadata.Class, Fields: map[string]interface{}{}, Constants: map[string]bool{}, model: newModelState(metadata, false, false)}
	combined := cloneValueMap(attributes)
	for name, value := range values {
		combined[name] = value
	}
	r.fillModel(model, combined, false)
	if operation != "firstornew" {
		r.saveModel(model)
	}
	return model
}

func (r *Runtime) ensureModelState(instance *Instance, metadata *modelMetadata) {
	if instance.model == nil {
		instance.model = newModelState(metadata, false, false)
		removeModelConfigurationFields(instance.Fields)
	}
}
func (r *Runtime) newModelQuery(metadata *modelMetadata) *Instance {
	query := &Instance{Class: metadata.Class, Fields: map[string]interface{}{}, Constants: map[string]bool{}, model: newModelState(metadata, false, true)}
	r.executeGranDBMethod(query, "table", []interface{}{metadata.Table})
	if metadata.SoftDeletes {
		r.configureSoftDeleteScope(query, "withouttrashed")
	}
	return query
}
func (r *Runtime) fillModel(instance *Instance, attributes map[string]interface{}, force bool) {
	metadata := instance.model.metadata
	for name, raw := range attributes {
		if !force && !metadata.massAssignable(name) {
			panic(&JossError{Type: "MassAssignmentError", Message: fmt.Sprintf("El atributo %q no permite asignación masiva en %s", name, metadata.ClassName)})
		}
		instance.Fields[name] = castModelValue(metadata, name, raw)
	}
}
func (metadata *modelMetadata) massAssignable(name string) bool {
	if _, ok := metadata.Fillable[name]; ok {
		return true
	}
	if len(metadata.Fillable) > 0 {
		return false
	}
	if _, all := metadata.Guarded["*"]; all {
		return false
	}
	_, guarded := metadata.Guarded[name]
	return !guarded
}
func (r *Runtime) saveModel(instance *Instance) bool {
	state := instance.model
	if state.deleted {
		panic(&JossError{Type: "ModelStateError", Message: "No se puede guardar un modelo eliminado"})
	}
	changes := modelChanges(instance)
	if state.exists && len(changes) == 0 {
		state.lastChanges = map[string]interface{}{}
		return true
	}
	creating := !state.exists
	if !r.runModelHook(instance, "saving") {
		return false
	}
	if creating {
		if !r.runModelHook(instance, "creating") {
			return false
		}
	} else if !r.runModelHook(instance, "updating") {
		return false
	}
	data := make(map[string]interface{})
	if state.exists {
		data = changes
	} else {
		for name, value := range instance.Fields {
			if !strings.HasPrefix(name, "_") {
				data[name] = value
			}
		}
	}
	for name, value := range data {
		data[name] = modelDatabaseValue(state.metadata, name, value)
	}
	if state.exists {
		key, ok := instance.Fields[state.metadata.PrimaryKey]
		if !ok || key == nil {
			panic(&JossError{Type: "ModelStateError", Message: "El modelo existente no tiene primary key"})
		}
		delete(data, state.metadata.PrimaryKey)
		if len(data) == 0 {
			state.lastChanges = map[string]interface{}{}
			return true
		}
		query := r.newModelQuery(state.metadata)
		r.executeGranDBMethod(query, "where", []interface{}{state.metadata.PrimaryKey, key})
		if result := r.executeUpdateMethod(query, []interface{}{data}); result != true {
			return false
		}
	} else {
		query := r.newModelQuery(state.metadata)
		if state.metadata.Incrementing {
			id := r.insertFromMapWithKey(r.getTable(query), data, true, state.metadata.PrimaryKey)
			if id == false {
				return false
			}
			instance.Fields[state.metadata.PrimaryKey] = id
		} else if result := r.executeInsertMethod(query, []interface{}{data}, false); result != true {
			return false
		}
		state.exists = true
	}
	state.lastChanges = cloneValueMap(changes)
	if !state.exists || len(changes) == 0 {
		state.lastChanges = cloneValueMap(instance.Fields)
	}
	state.original = cloneValueMap(instance.Fields)
	if creating {
		r.runModelHook(instance, "created")
	} else {
		r.runModelHook(instance, "updated")
	}
	r.runModelHook(instance, "saved")
	return true
}
func modelDatabaseValue(metadata *modelMetadata, name string, value interface{}) interface{} {
	if value == nil {
		return nil
	}
	switch metadata.Casts[name] {
	case "json", "array", "object":
		encoded, err := json.Marshal(value)
		if err != nil {
			panic(&JossError{Type: "InvalidCast", Message: fmt.Sprintf("No se puede serializar %s.%s como JSON", metadata.ClassName, name)})
		}
		return string(encoded)
	case "date":
		if date, ok := value.(time.Time); ok {
			return date.Format("2006-01-02")
		}
	case "datetime":
		if date, ok := value.(time.Time); ok {
			return date.Format("2006-01-02 15:04:05")
		}
	case "decimal":
		if number, ok := value.(decimal.Decimal); ok {
			return number.String()
		}
	}
	return value
}
func (r *Runtime) refreshModel(instance *Instance) *Instance {
	state := instance.model
	key, ok := instance.Fields[state.metadata.PrimaryKey]
	if !state.exists || !ok || key == nil {
		panic(&JossError{Type: "ModelStateError", Message: "refresh() requiere un modelo persistido con primary key"})
	}
	query := r.newModelQuery(state.metadata)
	found := r.executeFindMethod(query, []interface{}{key})
	hydrated, ok := found.(*Instance)
	if !ok {
		panic(&JossError{Type: "ModelNotFound", Message: fmt.Sprintf("No se encontró %s con %s=%v", state.metadata.ClassName, state.metadata.PrimaryKey, key)})
	}
	instance.Fields = cloneValueMap(hydrated.Fields)
	instance.model.original = cloneValueMap(hydrated.model.original)
	instance.model.lastChanges = map[string]interface{}{}
	instance.model.relations = map[string]interface{}{}
	instance.model.loaded = map[string]bool{}
	return instance
}

func (r *Runtime) deleteModel(instance *Instance) bool {
	state := instance.model
	if state == nil || !state.exists || state.deleted {
		return false
	}
	if !r.runModelHook(instance, "deleting") {
		return false
	}
	key, ok := instance.Fields[state.metadata.PrimaryKey]
	if !ok || key == nil {
		panic(&JossError{Type: "ModelStateError", Message: "delete() requiere primary key"})
	}
	query := r.newModelQuery(state.metadata)
	if state.metadata.SoftDeletes {
		now := time.Now().UTC().Format("2006-01-02 15:04:05")
		r.configureSoftDeleteScope(query, "withtrashed")
		r.executeGranDBMethod(query, "where", []interface{}{state.metadata.PrimaryKey, key})
		if result := r.executeUpdateMethod(query, []interface{}{map[string]interface{}{state.metadata.DeletedAt: now}}); result != true {
			return false
		}
		instance.Fields[state.metadata.DeletedAt] = now
		state.original = cloneValueMap(instance.Fields)
		state.deleted = true
		r.runModelHook(instance, "deleted")
		return true
	}
	r.executeGranDBMethod(query, "where", []interface{}{state.metadata.PrimaryKey, key})
	if result := r.executeDeleteMethod(query); result != true {
		return false
	}
	state.deleted = true
	state.exists = false
	r.runModelHook(instance, "deleted")
	return true
}

func (r *Runtime) forceDeleteModel(instance *Instance) bool {
	state := instance.model
	if state == nil {
		return false
	}
	key := instance.Fields[state.metadata.PrimaryKey]
	if key == nil {
		return false
	}
	if !r.runModelHook(instance, "deleting") {
		return false
	}
	query := r.newModelQuery(state.metadata)
	r.configureSoftDeleteScope(query, "withtrashed")
	r.executeGranDBMethod(query, "where", []interface{}{state.metadata.PrimaryKey, key})
	if result := r.executeDeleteMethod(query); result != true {
		return false
	}
	state.deleted = true
	state.exists = false
	r.runModelHook(instance, "deleted")
	return true
}
func (r *Runtime) restoreModel(instance *Instance) bool {
	state := instance.model
	if state == nil || !state.metadata.SoftDeletes || !state.exists {
		return false
	}
	key := instance.Fields[state.metadata.PrimaryKey]
	if key == nil {
		return false
	}
	if !r.runModelHook(instance, "restoring") {
		return false
	}
	query := r.newModelQuery(state.metadata)
	r.configureSoftDeleteScope(query, "withtrashed")
	r.executeGranDBMethod(query, "where", []interface{}{state.metadata.PrimaryKey, key})
	if result := r.executeUpdateMethod(query, []interface{}{map[string]interface{}{state.metadata.DeletedAt: nil}}); result != true {
		return false
	}
	instance.Fields[state.metadata.DeletedAt] = nil
	state.original = cloneValueMap(instance.Fields)
	state.deleted = false
	r.runModelHook(instance, "restored")
	return true
}

func (r *Runtime) runModelHook(instance *Instance, name string) bool {
	if instance == nil || instance.Class == nil || instance.Class.Name == nil {
		return true
	}
	meta := r.lookupClassMetadata(instance.Class.Name.Value)
	if meta == nil {
		return true
	}
	info := meta.Methods[name]
	if info == nil || info.Method.Body == nil {
		return true
	}
	result := r.CallMethodEvaluated(info.Method, instance, nil)
	if allowed, ok := result.(bool); ok {
		return allowed
	}
	return true
}
func (r *Runtime) configureSoftDeleteScope(query *Instance, mode string) {
	if query == nil || query.model == nil || !query.model.metadata.SoftDeletes {
		return
	}
	column := query.model.metadata.DeletedAt
	quoted := quoteIdentifier(column)
	wheres, _ := query.Fields["_wheres"].([]string)
	filtered := wheres[:0]
	for _, where := range wheres {
		if where != quoted+" IS NULL" && where != quoted+" IS NOT NULL" {
			filtered = append(filtered, where)
		}
	}
	query.Fields["_wheres"] = filtered
	switch mode {
	case "withouttrashed":
		r.executeGranDBMethod(query, "whereNull", []interface{}{column})
	case "onlytrashed":
		r.executeGranDBMethod(query, "whereNotNull", []interface{}{column})
	}
}
func (r *Runtime) applyLocalModelScope(query *Instance, args []interface{}) *Instance {
	if len(args) == 0 {
		panic(&JossError{Type: "ModelScopeError", Message: "scope() requiere un nombre"})
	}
	name := fmt.Sprint(args[0])
	if name == "" {
		panic(&JossError{Type: "ModelScopeError", Message: "scope() no acepta un nombre vacío"})
	}
	methodName := "scope" + strings.ToUpper(name[:1]) + name[1:]
	meta := r.lookupClassMetadata(query.model.metadata.ClassName)
	info := meta.Methods[methodName]
	if info == nil || info.Method.Body == nil {
		panic(&JossError{Type: "ModelScopeError", Message: fmt.Sprintf("El scope %s::%s no existe", meta.Class.Name.Value, name)})
	}
	scopeArgs := []interface{}{query}
	if len(args) > 1 {
		scopeArgs = append(scopeArgs, args[1:]...)
	}
	result := r.CallMethodEvaluated(info.Method, query, scopeArgs)
	if scoped, ok := result.(*Instance); ok {
		return scoped
	}
	return query
}

func (r *Runtime) newRelationQuery(parent *Instance, kind string, args []interface{}) *Instance {
	if len(args) == 0 {
		panic(&JossError{Type: "InvalidRelation", Message: kind + "() requiere la clase relacionada"})
	}
	relatedName := fmt.Sprint(args[0])
	related := r.lookupModelMetadata(relatedName)
	if related == nil {
		panic(&JossError{Type: "InvalidRelation", Message: fmt.Sprintf("%s no es un Model relacionado", relatedName)})
	}
	query := r.newModelQuery(related)
	parentName := snakeCase(parent.model.metadata.ClassName)
	foreign, local := "", ""
	switch kind {
	case "belongsto":
		foreign = parentName
		foreign = snakeCase(related.ClassName) + "_id"
		local = related.PrimaryKey
		if len(args) > 1 {
			foreign = fmt.Sprint(args[1])
		}
		if len(args) > 2 {
			local = fmt.Sprint(args[2])
		}
		value := parent.Fields[foreign]
		if value == nil {
			r.executeGranDBMethod(query, "whereNull", []interface{}{local})
		} else {
			r.executeGranDBMethod(query, "where", []interface{}{local, value})
		}
	case "hasone", "hasmany":
		foreign = parentName + "_id"
		local = parent.model.metadata.PrimaryKey
		if len(args) > 1 {
			foreign = fmt.Sprint(args[1])
		}
		if len(args) > 2 {
			local = fmt.Sprint(args[2])
		}
		value := parent.Fields[local]
		if value == nil {
			panic(&JossError{Type: "InvalidRelation", Message: "El modelo padre no tiene local key"})
		}
		r.executeGranDBMethod(query, "where", []interface{}{foreign, value})
	case "belongstomany":
		local = parent.model.metadata.PrimaryKey
		relatedKey := related.PrimaryKey
		pivot := defaultPivotTable(parent.model.metadata.ClassName, related.ClassName)
		foreignPivot := snakeCase(parent.model.metadata.ClassName) + "_id"
		relatedPivot := snakeCase(related.ClassName) + "_id"
		if len(args) > 1 {
			pivot = fmt.Sprint(args[1])
		}
		if len(args) > 2 {
			foreignPivot = fmt.Sprint(args[2])
		}
		if len(args) > 3 {
			relatedPivot = fmt.Sprint(args[3])
		}
		if len(args) > 4 {
			local = fmt.Sprint(args[4])
		}
		if len(args) > 5 {
			relatedKey = fmt.Sprint(args[5])
		}
		if parent.Fields[local] == nil {
			panic(&JossError{Type: "InvalidRelation", Message: "El modelo padre no tiene local key"})
		}
		query.model.relation = &modelRelation{kind: kind, parent: parent, localKey: local, relatedKey: relatedKey, pivotTable: pivot, foreignPivotKey: foreignPivot, relatedPivotKey: relatedPivot}
		return query
	}
	query.model.relation = &modelRelation{kind: kind, parent: parent, foreignKey: foreign, localKey: local}
	return query
}

func defaultPivotTable(left, right string) string {
	names := []string{snakeCase(left), snakeCase(right)}
	if names[0] > names[1] {
		names[0], names[1] = names[1], names[0]
	}
	return names[0] + "_" + names[1]
}

func relationNames(args []interface{}) []string {
	result := []string{}
	for _, arg := range args {
		switch value := arg.(type) {
		case string:
			if value != "" {
				result = append(result, value)
			}
		case []interface{}:
			result = append(result, relationNames(value)...)
		case []string:
			result = append(result, value...)
		}
	}
	return result
}

func (r *Runtime) applyEagerLoads(query *Instance, models []interface{}) {
	names := relationNames([]interface{}{query.Fields["_with"]})
	if len(names) == 0 {
		return
	}
	parents := make([]*Instance, 0, len(models))
	for _, item := range models {
		if model, ok := item.(*Instance); ok {
			parents = append(parents, model)
		}
	}
	r.eagerLoadModels(parents, names, false)
}

func (r *Runtime) eagerLoadModels(parents []*Instance, names []string, missingOnly bool) {
	if len(parents) == 0 {
		return
	}
	grouped := map[string][]string{}
	for _, path := range names {
		parts := strings.SplitN(path, ".", 2)
		if len(parts) == 1 {
			grouped[parts[0]] = grouped[parts[0]]
		} else {
			grouped[parts[0]] = append(grouped[parts[0]], parts[1])
		}
	}
	for name, nested := range grouped {
		pending := parents[:0]
		for _, parent := range parents {
			if !missingOnly || !parent.model.loaded[name] {
				pending = append(pending, parent)
			}
		}
		if len(pending) == 0 {
			continue
		}
		meta := r.lookupClassMetadata(pending[0].model.metadata.ClassName)
		methodInfo := meta.Methods[name]
		if methodInfo == nil || methodInfo.Method.Body == nil {
			panic(&JossError{Type: "InvalidRelation", Message: fmt.Sprintf("La relación %s::%s no existe", meta.Class.Name.Value, name)})
		}
		value := r.CallMethodEvaluated(methodInfo.Method, pending[0], nil)
		relationQuery, ok := value.(*Instance)
		if !ok || relationQuery.model == nil || relationQuery.model.relation == nil {
			panic(&JossError{Type: "InvalidRelation", Message: fmt.Sprintf("%s::%s no retorna una relación", meta.Class.Name.Value, name)})
		}
		relation := relationQuery.model.relation
		if relation.kind == "belongstomany" {
			children := r.eagerLoadBelongsToMany(pending, name, relationQuery)
			if len(nested) > 0 {
				r.eagerLoadModels(children, nested, missingOnly)
			}
			continue
		}
		keys := []interface{}{}
		seen := map[string]bool{}
		keyName := relation.localKey
		if relation.kind == "belongsto" {
			keyName = relation.foreignKey
		}
		for _, parent := range pending {
			key := parent.Fields[keyName]
			token := fmt.Sprintf("%T:%v", key, key)
			if key != nil && !seen[token] {
				seen[token] = true
				keys = append(keys, key)
			}
		}
		delete(relationQuery.Fields, "_wheres")
		relationQuery.Fields["_wheres"] = []string{}
		relationQuery.Fields["_bindings"] = []interface{}{}
		matchColumn := relation.foreignKey
		if relation.kind == "belongsto" {
			matchColumn = relation.localKey
		}
		if len(keys) > 0 {
			r.executeGranDBMethod(relationQuery, "whereIn", []interface{}{matchColumn, keys})
		}
		items := []interface{}{}
		if len(keys) > 0 {
			items = r.executeGetMethod(relationQuery, nil).([]interface{})
		}
		index := map[string][]*Instance{}
		for _, item := range items {
			related := item.(*Instance)
			token := fmt.Sprintf("%T:%v", related.Fields[matchColumn], related.Fields[matchColumn])
			index[token] = append(index[token], related)
		}
		children := []*Instance{}
		for _, parent := range pending {
			key := parent.Fields[keyName]
			matches := index[fmt.Sprintf("%T:%v", key, key)]
			var assigned interface{}
			if relation.kind == "hasmany" {
				assigned = instancesToInterfaces(matches)
			} else if len(matches) > 0 {
				assigned = matches[0]
			} else {
				assigned = nil
			}
			parent.model.relations[name] = assigned
			parent.model.loaded[name] = true
			parent.Fields[name] = assigned
			children = append(children, matches...)
		}
		if len(nested) > 0 {
			r.eagerLoadModels(children, nested, missingOnly)
		}
	}
}
func instancesToInterfaces(items []*Instance) []interface{} {
	result := make([]interface{}, len(items))
	for index, item := range items {
		result[index] = item
	}
	return result
}
func snakeCase(value string) string {
	var result strings.Builder
	for index, char := range value {
		if index > 0 && char >= 'A' && char <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(char)
	}
	return strings.ToLower(result.String())
}

func pluralizeWord(word string) string {
	lower := strings.ToLower(word)
	if strings.HasSuffix(lower, "y") && len(lower) > 1 {
		return lower[:len(lower)-1] + "ies"
	}
	if strings.HasSuffix(lower, "s") {
		return lower + "es"
	}
	return lower + "s"
}
