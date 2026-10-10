# Trabajo Pendiente y Hoja de Ruta Futura

**Proyecto:** Joss Programming Language  
**Fecha:** 9 de octubre de 2026  

---

## 1. Tareas Pendientes Priorizadas

### Prioridad Media (P2)
1. **Ampliación de Cobertura en Lowering de Joss Native IR:**
   - **Objetivo:** Ampliar `pkg/ir/lower.go` y `pkg/backend/native/llvm.go` para compilar nativamente clases con métodos complejos y despacho dinámico sin requerir el backend Standalone.
   - **Criterio de Aceptación:** Nuevas pruebas en `tests/native` que compilen clases con herencia utilizando `--backend=llvm`.
2. **Presupuesto Finito de Instrucciones (Fuel Quota) en Evaluador AST:**
   - **Objetivo:** Introducir un contador de pasos/instrucciones configurable para mitigar bucles infinitos en ejecuciones no confiables.
   - **Criterio de Aceptación:** Prueba en `pkg/core` donde un `while(true)` sin condición de salida termine de manera segura al alcanzar la cuota máxima configurada.

### Prioridad Baja (P3)
1. **Limpieza del Espacio de Trabajo de la Raíz:**
   - **Objetivo:** Eliminar o archivar en `.gitignore` los binarios precompilados presentes en el working tree (`runner.exe`, `joss-android-arm64`) y renombrar la carpeta `hestia cp install` corrigiendo el error tipográfico en `IMSTALLATION_HESTIA_CP.md`.
2. **Depuración Metodológica de la Tesis Académica:**
   - **Objetivo:** Retirar los scripts de generación programática de páginas en `C:\Users\joss\Documents\tesis` y construir un laboratorio con contenedores Docker para reproducir los benchmarks del Capítulo 9 con datos brutos auditables.
