# JOSS Native ABI (Application Binary Interface)

La interfaz binaria (ABI) define cómo el código máquina generado para `joss build` invoca servicios pesados (como GranDB) sin embeber el AST.

## Calling Convention (Host Bridge)
Todas las llamadas complejas a la Standard Library se rigen por la frontera C-ABI definida en `pkg/backend/native/abi/host_bridge.go`.
El generador `x86_64` emite una rutina explícita a `_joss_host_bridge`.

1. **RCX / RDI**: Puntero C-String al nombre de la función (ej. `GranDB_select`).
2. **RDX / RSI**: Puntero al array empaquetado de argumentos IR Joss.

Esta interfaz aísla perfectamente el binario nativo AOT del runtime de soporte, permitiendo binarios de peso cero (Dead Code Elimination) o binarios ricos según demanda del import en el `.joss`.
