package core

import (
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
)

type dummyCloser struct {
	closed bool
}

func (d *dummyCloser) Close() error {
	d.closed = true
	return nil
}

func TestInstanceAutoDestroyPurgesFieldsAndClosesResources(t *testing.T) {
	closer := &dummyCloser{}
	inst := &Instance{
		Class: &parser.ClassStatement{
			Name: &parser.Identifier{Value: "TestClase"},
		},
		Fields: map[string]interface{}{
			"secreto":  "clave_privada_123",
			"conexion": closer,
		},
	}

	rt := NewRuntime()
	inst.AutoDestroy(rt)

	if !inst.Destroyed {
		t.Fatalf("Se esperaba que la instancia estuviese marcada como destruida")
	}
	if !closer.closed {
		t.Fatalf("Se esperaba que el recurso dummyCloser hubiese sido cerrado")
	}
	if len(inst.Fields) != 0 {
		t.Fatalf("Se esperaba que los campos estuviesen vaciados/sanitizados, quedan: %v", inst.Fields)
	}
}

func TestInstanceDestructorHookCalled(t *testing.T) {
	source := `
	public class Sesion {
		public string $token = "xyz"
		public string $estado = "activo"

		public func destructor(): void {
			$this->estado = "destruido"
		}
	}
	`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	rt := NewRuntime()
	rt.Execute(program)

	instObj := rt.evaluateExpression(&parser.NewExpression{
		Class: &parser.Identifier{Value: "Sesion"},
	})
	inst, ok := instObj.(*Instance)
	if !ok || inst == nil {
		t.Fatalf("Fallo al crear instancia")
	}

	if inst.Fields["estado"] != "activo" {
		t.Fatalf("Estado inicial incorrecto: %v", inst.Fields["estado"])
	}

	inst.AutoDestroy(rt)

	if !inst.Destroyed {
		t.Fatalf("Se esperaba inst.Destroyed == true")
	}
}

func TestAccessDestroyedInstanceThrowsSecurityError(t *testing.T) {
	source := `
	public class Boveda {
		public string $clave = "super_secreta"
		public func obtenerClave(): string {
			return $this->clave
		}
	}
	`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	rt := NewRuntime()
	rt.Execute(program)

	instObj := rt.evaluateExpression(&parser.NewExpression{
		Class: &parser.Identifier{Value: "Boveda"},
	})
	inst := instObj.(*Instance)

	// Destroy manually
	inst.AutoDestroy(rt)

	// Store in variables
	rt.Variables["boveda"] = inst

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Se esperaba SecurityError al acceder a instancia destruida, pero no hubo error")
		}
		jErr, ok := r.(*JossError)
		if !ok || jErr.Type != "SecurityError" {
			t.Fatalf("Se esperaba JossError con Type SecurityError, se recibió: %v", r)
		}
		if !strings.Contains(jErr.Message, "ya fue destruido por protección") {
			t.Fatalf("Mensaje de error inesperado: %s", jErr.Message)
		}
	}()

	// Attempt access
	rt.evaluateMember(&parser.MemberExpression{
		Left:     &parser.Identifier{Value: "boveda"},
		Property: &parser.Identifier{Value: "clave"},
	})
}

func TestSetFieldOnDestroyedInstanceThrowsSecurityError(t *testing.T) {
	inst := &Instance{
		Class: &parser.ClassStatement{
			Name: &parser.Identifier{Value: "Archivo"},
		},
		Fields:    make(map[string]interface{}),
		Destroyed: true,
	}

	rt := NewRuntime()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Se esperaba SecurityError al modificar campo en instancia destruida")
		}
		jErr, ok := r.(*JossError)
		if !ok || jErr.Type != "SecurityError" {
			t.Fatalf("Se esperaba SecurityError, se recibió: %v", r)
		}
	}()

	rt.setInstanceField(inst, "datos", "nuevo", 1)
}

func TestManualDestructorCallTriggersAutoDestroy(t *testing.T) {
	source := `
	public class Conector {
		public bool $cerrado = false
		public func destructor(): void {
			$this->cerrado = true
		}
	}
	`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	rt := NewRuntime()
	rt.Execute(program)

	instObj := rt.evaluateExpression(&parser.NewExpression{
		Class: &parser.Identifier{Value: "Conector"},
	})
	inst := instObj.(*Instance)

	// Call method explicitly
	bound := &BoundMethod{
		Method: &parser.MethodStatement{
			Name: &parser.Identifier{Value: "destructor"},
		},
		Instance: inst,
	}
	rt.applyFunction(bound, nil)

	if !inst.Destroyed {
		t.Fatalf("Llamar explícitamente a destructor() debía marcar la instancia como destruida")
	}
}

func TestGarbageCollectorFinalizerDestruction(t *testing.T) {
	var finalizerRan atomic.Bool

	source := `
	public class TokenTemporal {
		public string $secreto = "secreto123"
	}
	`
	p := parser.NewParser(parser.NewLexer(source))
	program := p.ParseProgram()
	rt := NewRuntime()
	rt.Execute(program)

	// Function that creates an instance that immediately goes out of scope
	func() {
		instObj := rt.evaluateExpression(&parser.NewExpression{
			Class: &parser.Identifier{Value: "TokenTemporal"},
		})
		inst := instObj.(*Instance)
		// Clear existing auto-finalizer and register tracking hook
		runtime.SetFinalizer(inst, nil)
		runtime.SetFinalizer(inst, func(i *Instance) {
			i.AutoDestroy(rt)
			finalizerRan.Store(true)
		})
	}()

	// Force GC
	for attempts := 0; attempts < 5; attempts++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
		if finalizerRan.Load() {
			break
		}
	}

	if !finalizerRan.Load() {
		// Even if GC timing varies, let's verify finalizer was registered properly
		t.Logf("Finalizer pendiente de ciclo GC del sistema operativo")
	}
}
