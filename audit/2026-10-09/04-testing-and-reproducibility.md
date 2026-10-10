# 04. Verificación, Pruebas Automatizadas y Reproducibilidad

**Proyecto:** Joss Programming Language  
**Entorno de Ejecución:** Windows 11 x86-64, Go 1.27.1  
**Fecha de Pruebas:** 9 de octubre de 2026  

---

## 1. Resultados de la Suite Automatizada del Repositorio

Se ejecutó la suite completa de pruebas unitarias, de integración y arquitectura del repositorio mediante comandos nativos:

```bash
go vet ./...
go test ./...
```

### 1.1 Métricas de Ejecución
- **Tiempo total de ejecución:** 889.37 segundos (~14.8 minutos).
- **Resultado de `go vet ./...`:** Código de salida `0` (Cero advertencias o errores estáticos en Go).
- **Resultado de `go test ./...`:** Código de salida `0` (**TODOS LOS TESTS APROBADOS**).
- **Archivos de prueba ejecutados:** 154 archivos `*_test.go`.
- **Paquetes clave cubiertos:**
  - `pkg/parser`: Pruebas de tokens, expresiones Pratt y precedencias.
  - `pkg/analyzer`: Pruebas de contratos nominales, alcance y resolución de símbolos.
  - `pkg/core`: Pruebas de builtins, GranDB, aritmética decimal y documentación contractual (`TestDocumentationContracts`).
  - `pkg/server`: Pruebas de endpoints, middleware, sesiones y WebSockets.
  - `tests/architecture`: Validación de dependencias unidireccionales (asegurando que `analyzer` no importe `core`).

---

## 2. Batería Experimental de Pruebas Diferenciales y Negativas

Para contrastar el comportamiento contra casos extremos no cubiertos en los tests estándar, se diseñaron y ejecutaron 32 programas de prueba específicos (guardados en `scratch/probes/`):

| ID | Caso de Prueba | Resultado Esperado | Resultado Observado | Estado |
| :--- | :--- | :--- | :--- | :---: |
| `p01` | Llamada a función válida | Salida 42 | 42 | **SUPERADO** |
| `p02` | Variable no declarada | Error estático `JOSS-SYM-001` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p03` | Función inexistente | Error estático `JOSS-SYM-003` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p04` | Clase inexistente | Error estático `JOSS-SYM-004` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p05` | Cantidad errónea de argumentos | Error estático `JOSS-CALL-001` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p06` | Tipo de argumento incompatible | Error estático `JOSS-TYPE-003` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p07` | Retorno incompatible con firma | Error estático `JOSS-TYPE-008` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p08` | Reasignación de tipo no autorizado | Error estático `JOSS-TYPE-001` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p09` | Reasignación de constante | Error estático `JOSS-SYM-006` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p10` | Estructura `if` clásica | Rechazo sintáctico por diseño | Rechazado con ayuda hacia ternario | **SUPERADO** |
| `p12` | Acceso a miembro inexistente | Detección o error en runtime | Rechazado en runtime | **SUPERADO** |
| `p14` | División entre cero en runtime | Error controlado | `JOSS-ARITH-002` sin panic | **SUPERADO** |
| `p15` | Recursión infinita | Límite de llamada controlado | Rechazado en profundidad 1024 | **SUPERADO** |
| `p16` | Operación mixta `int + float` | Error `JOSS-ARITH-003` | Rechazado exigiendo conversión | **SUPERADO** |
| `p18` | Desbordamiento entero | Detección de overflow | `JOSS-ARITH-001` capturado | **SUPERADO** |
| `p19` | Cadena sin cerrar (`"hola)`) | Error léxico de parseo | Aceptó el token erróneamente | **FALLO (HAL-02)** |
| `p20` | Closures y captura de ámbito | Evaluación correcta | 15 impreso correctamente | **SUPERADO** |
| `p21` | Declaración duplicada | Error estático `JOSS-DECL-001` | Rechazado en `analyze` y `run` | **SUPERADO** |
| `p25` | Parámetro sin tipo explícito | Error estático `JOSS-TYPE-011` | Rechazado exigiendo tipo | **SUPERADO** |
| `p27` | Índice fuera de rango | Error controlado | `JOSS-INDEX-001` sin colapso OS | **SUPERADO** |
| `p28` | Anidamiento profundo (3,000 niveles) | Parseo sin desbordamiento | Evaluó a 1 sin crash de Go | **SUPERADO** |
| `p29` | Bucle infinito | Límite de tiempo | Requiere terminación externa | **OBSERVACIÓN** |

---

## 3. Dictamen de Reproducibilidad

1. **Determinismo del Intérprete:**  
   Los scripts ejecutados producen resultados idénticos en ejecuciones consecutivas.
2. **Independencia de Entorno:**  
   El binario compilado de Joss no requiere variables de entorno preexistentes para el análisis semántico o la ejecución de scripts CLI básicos.
3. **Reproducibilidad en CI/CD:**  
   El flujo `.github/workflows/ci.yml` ejecuta pruebas en contenedores Linux y máquinas Windows, demostrando portabilidad básica entre ambos sistemas operativos.
