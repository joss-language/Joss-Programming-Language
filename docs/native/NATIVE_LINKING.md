# Smart Selective Linking (Pay for What You Use)

El linker de Joss Native (`pkg/backend/native/standalone.go`) escanea las instrucciones IR buscando llamadas a runtimes y Stdlib (`CallRuntimeInst`).

- **Si NO encuentra llamadas pesadas:**
  El compilador emite ensamblador puro con retornos dummy en los stubs del HostBridge.
  **Peso esperado:** < 50 KB. Cero runtime overhead.

- **Si ENCUENTRA llamadas (ej. `GranDB`, `HTTP`):**
  El compilador marca `needsHostBridge = true` y enlaza el Host Bridge nativo (`_joss_host_bridge`) optimizado con poda selectiva de drivers (`no<driver>`) y sellado VFS AES-256.
  **Beneficio:** Código nativo exacto idéntico al de Go (Single Source of Truth), pagando el peso del runtime sólo cuando la aplicación levanta servidor o bases de datos.
