# Smart Selective Linking (Pay for What You Use)

El linker de Joss Native (`pkg/backend/native/standalone.go`) escanea las instrucciones IR buscando llamadas a runtimes y Stdlib (`CallRuntimeInst`).

- **Si NO encuentra llamadas pesadas:**
  El compilador emite ensamblador puro con retornos dummy en los stubs del HostBridge.
  **Peso esperado:** < 50 KB. Cero runtime overhead.

- **Si ENCUENTRA llamadas (ej. `GranDB`, `HTTP`):**
  El compilador marca `needsHostBridge = true` y exige enlazar contra el Micromódulo Go-estático `libjoss_base.a`.
  **Beneficio:** Código nativo exacto idéntico al de Go (Single Source of Truth), pagando el peso del GC de Go sólo cuando el usuario importa bases de datos.
