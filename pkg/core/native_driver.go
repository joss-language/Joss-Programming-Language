package core

import (
	"fmt"
	"unsafe"
)

const maxNativeDriverResponse = 128 << 20

func callLoadedNativeDriver(driver *NativeDriverDefinition, method, argsJSON string) (string, error) {
	if driver == nil {
		return "", fmt.Errorf("driver no cargado")
	}
	driver.Mu.Lock()
	defer driver.Mu.Unlock()
	if driver.Call == nil || driver.unloaded {
		return "", fmt.Errorf("driver descargado o no disponible")
	}
	pointer := driver.Call(method, argsJSON)
	if pointer == nil {
		return "", fmt.Errorf("joss_driver_call devolvio NULL")
	}
	if driver.Free != nil {
		defer driver.Free(pointer)
	}
	data := make([]byte, 0, 256)
	for index := 0; index < maxNativeDriverResponse; index++ {
		value := *(*byte)(unsafe.Add(unsafe.Pointer(pointer), index))
		if value == 0 {
			return string(data), nil
		}
		data = append(data, value)
	}
	return "", fmt.Errorf("respuesta excede %d MiB o no termina en NUL", maxNativeDriverResponse>>20)
}

// retainNativeDriver records another Runtime borrowing the same immutable
// driver definition and dynamic-library handle. Drivers inserted by older host
// integrations have an implicit initial owner when owners is still zero.
func retainNativeDriver(driver *NativeDriverDefinition) error {
	if driver == nil {
		return nil
	}
	driver.Mu.Lock()
	defer driver.Mu.Unlock()
	if driver.unloaded {
		return fmt.Errorf("driver %s descargado", driver.Name)
	}
	if driver.owners == 0 {
		driver.owners = 1
	}
	driver.owners++
	return nil
}

func adoptNativeDriver(driver *NativeDriverDefinition) error {
	if driver == nil {
		return fmt.Errorf("driver no cargado")
	}
	driver.Mu.Lock()
	defer driver.Mu.Unlock()
	if driver.unloaded {
		return fmt.Errorf("driver %s descargado", driver.Name)
	}
	if driver.owners == 0 {
		driver.owners = 1
	}
	return nil
}

func (r *Runtime) installNativeDriver(name string, driver *NativeDriverDefinition) error {
	if err := adoptNativeDriver(driver); err != nil {
		return err
	}
	if previous := r.NativeDrivers[name]; previous != nil && previous != driver {
		if err := releaseNativeDriver(previous); err != nil {
			_ = driver.Unload()
			return fmt.Errorf("reemplazar driver %s: %w", name, err)
		}
	}
	r.NativeDrivers[name] = driver
	return nil
}

// releaseNativeDriver releases one Runtime owner. The final owner is
// responsible for unloading the OS handle; request forks never unload a handle
// that remains reachable by their parent or sibling runtimes.
func releaseNativeDriver(driver *NativeDriverDefinition) error {
	if driver == nil {
		return nil
	}
	driver.Mu.Lock()
	defer driver.Mu.Unlock()
	if driver.unloaded {
		return nil
	}
	if driver.owners == 0 {
		driver.owners = 1
	}
	driver.owners--
	if driver.owners > 0 {
		return nil
	}
	return driver.unloadLocked()
}

// Unload safely unloads a loaded native dynamic library, clearing its handle and
// preventing use-after-free by rejecting subsequent invocations.
func (d *NativeDriverDefinition) Unload() error {
	if d == nil {
		return nil
	}
	d.Mu.Lock()
	defer d.Mu.Unlock()
	if d.unloaded {
		return nil
	}
	if d.owners > 1 {
		return fmt.Errorf("driver %s sigue compartido por %d runtimes", d.Name, d.owners)
	}
	d.owners = 0
	return d.unloadLocked()
}

func (d *NativeDriverDefinition) unloadLocked() error {
	d.unloaded = true
	var err error
	if d.Handle != 0 {
		err = unloadNativeDriverHandle(d.Handle)
		d.Handle = 0
	}
	d.Call = nil
	d.Free = nil
	return err
}
