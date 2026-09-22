package core

import (
	"sync"
)

// executeMutexMethod handles Mutex instance methods
func (r *Runtime) executeMutexMethod(instance *Instance, method string, args []interface{}) interface{} {
	if _, ok := instance.Fields["_mu"]; !ok {
		instance.Fields["_mu"] = &sync.Mutex{}
	}
	mu := instance.Fields["_mu"].(*sync.Mutex)

	switch method {
	case "lock":
		mu.Lock()
		return nil
	case "unlock":
		mu.Unlock()
		return nil
	case "tryLock":
		return mu.TryLock()
	}
	return nil
}

// executeRWMutexMethod handles RWMutex instance methods
func (r *Runtime) executeRWMutexMethod(instance *Instance, method string, args []interface{}) interface{} {
	if _, ok := instance.Fields["_rwmu"]; !ok {
		instance.Fields["_rwmu"] = &sync.RWMutex{}
	}
	rwmu := instance.Fields["_rwmu"].(*sync.RWMutex)

	switch method {
	case "lock":
		rwmu.Lock()
		return nil
	case "unlock":
		rwmu.Unlock()
		return nil
	case "rLock":
		rwmu.RLock()
		return nil
	case "rUnlock":
		rwmu.RUnlock()
		return nil
	}
	return nil
}

// executeWaitGroupMethod handles WaitGroup instance methods
func (r *Runtime) executeWaitGroupMethod(instance *Instance, method string, args []interface{}) interface{} {
	if _, ok := instance.Fields["_wg"]; !ok {
		instance.Fields["_wg"] = &sync.WaitGroup{}
	}
	wg := instance.Fields["_wg"].(*sync.WaitGroup)

	switch method {
	case "add":
		delta := 1
		if len(args) > 0 {
			if n, ok := args[0].(int64); ok {
				delta = int(n)
			} else if n, ok := args[0].(int); ok {
				delta = n
			}
		}
		wg.Add(delta)
		return nil
	case "done":
		wg.Done()
		return nil
	case "wait":
		wg.Wait()
		return nil
	}
	return nil
}
