package core

import (
	"strconv"
	"sync"

	"github.com/jossecurity/joss/pkg/parser"
)

// staticPropertyStore is owned by the root Runtime and shared by its request
// forks. Static values survive a request fork but are cleared when the owner
// Runtime is freed back to the pool.
type staticPropertyStore struct {
	mu          sync.RWMutex
	values      map[string]map[string]interface{}
	initialized map[string]map[string]bool
}

func newStaticPropertyStore() *staticPropertyStore {
	return &staticPropertyStore{
		values:      make(map[string]map[string]interface{}),
		initialized: make(map[string]map[string]bool),
	}
}

func (s *staticPropertyStore) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.values = make(map[string]map[string]interface{})
	s.initialized = make(map[string]map[string]bool)
	s.mu.Unlock()
}

func (r *Runtime) initializeStaticProperty(className, name string, value interface{}) {
	if r == nil || r.staticProperties == nil {
		return
	}
	s := r.staticProperties
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialized[className] == nil {
		s.initialized[className] = make(map[string]bool)
	}
	if s.initialized[className][name] {
		return
	}
	if s.values[className] == nil {
		s.values[className] = make(map[string]interface{})
	}
	s.values[className][name] = value
	s.initialized[className][name] = true
}

func (r *Runtime) getStaticProperty(className, name string) (interface{}, bool) {
	if r == nil || r.staticProperties == nil {
		return nil, false
	}
	r.staticProperties.mu.RLock()
	value, exists := r.staticProperties.values[className][name]
	r.staticProperties.mu.RUnlock()
	if !exists {
		return nil, false
	}
	return cloneStaticPropertyValue(value), true
}

func (r *Runtime) setStaticProperty(className, name string, value interface{}) {
	if r == nil || r.staticProperties == nil {
		return
	}
	s := r.staticProperties
	s.mu.Lock()
	if s.values[className] == nil {
		s.values[className] = make(map[string]interface{})
	}
	s.values[className][name] = cloneStaticPropertyValue(value)
	if s.initialized[className] == nil {
		s.initialized[className] = make(map[string]bool)
	}
	s.initialized[className][name] = true
	s.mu.Unlock()
}

func (r *Runtime) readStaticMapIndex(className, name string, key interface{}) (interface{}, bool) {
	if r == nil || r.staticProperties == nil {
		return nil, false
	}
	keyString, ok := normalizeStaticMapKey(key)
	if !ok {
		return nil, false
	}
	r.staticProperties.mu.RLock()
	value := r.staticProperties.values[className][name]
	var result interface{}
	var exists bool
	if values, isMap := value.(map[string]interface{}); isMap {
		result, exists = values[keyString]
	}
	r.staticProperties.mu.RUnlock()
	if !exists {
		return nil, false
	}
	return cloneStaticPropertyValue(result), true
}

func (r *Runtime) writeStaticMapIndex(className, name string, key, value interface{}) bool {
	if r == nil || r.staticProperties == nil {
		return false
	}
	keyString, ok := normalizeStaticMapKey(key)
	if !ok {
		return false
	}
	s := r.staticProperties
	s.mu.Lock()
	values, isMap := s.values[className][name].(map[string]interface{})
	if !isMap {
		s.mu.Unlock()
		return false
	}
	values[keyString] = cloneStaticPropertyValue(value)
	s.mu.Unlock()
	return true
}

func (r *Runtime) staticPropertyReference(expression parser.Expression) (string, string, bool) {
	member, ok := expression.(*parser.MemberExpression)
	if !ok || member == nil || member.Token.Type != parser.DOUBLE_COLON || member.Property == nil {
		return "", "", false
	}
	class, ok := member.Left.(*parser.Identifier)
	if !ok {
		return "", "", false
	}
	metadata := r.lookupClassMetadata(class.Value)
	if metadata == nil {
		return "", "", false
	}
	field, exists := metadata.Fields[member.Property.Value]
	if !exists || field == nil || !field.IsStatic {
		return "", "", false
	}
	return class.Value, member.Property.Value, true
}

func normalizeStaticMapKey(key interface{}) (string, bool) {
	switch value := key.(type) {
	case string:
		return value, true
	case int:
		return strconv.Itoa(value), true
	case int64:
		return strconv.FormatInt(value, 10), true
	default:
		return "", false
	}
}

func cloneStaticPropertyValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		cloned := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			cloned[key] = cloneStaticPropertyValue(item)
		}
		return cloned
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for index, item := range typed {
			cloned[index] = cloneStaticPropertyValue(item)
		}
		return cloned
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}
