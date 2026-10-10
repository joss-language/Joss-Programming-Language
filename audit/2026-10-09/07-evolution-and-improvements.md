# 07. Evolución Histórica, Mejoras Verificadas y Deuda Técnica

**Proyecto:** Joss Programming Language  
**Periodo Analizado:** Noviembre 2025 – Octubre 2026 (416 commits)  

---

## 1. Trayectoria de Evolución y Maduración del Código

El historial de Git demuestra una evolución técnica intensa y continuada a lo largo de 11 meses de desarrollo:

1. **Noviembre – Diciembre 2025 (Fase Inicial):**  
   - Creación del lexer inicial, parser Pratt básico y evaluador de expresiones de script simple.
   - El proyecto originalmente se denominaba "JosSecurity" antes de consolidarse como lenguaje independiente de propósito general ("Joss").
2. **Enero – Junio 2026 (Consolidación de Arquitectura):**  
   - Separación formal de responsabilidades entre `pkg/parser`, `pkg/analyzer` y `pkg/core`.
   - Incorporación del sistema de diagnósticos formales estructurados con códigos canónicos `JOSS-...`.
   - Implementación de GranDB y el modelo ORM.
3. **Julio – Agosto 2026 (Fases de Refinamiento y Seguridad):**  
   - Incorporación de paquetes de plugins `.jp` con firmas criptográficas Ed25519.
   - Retiro formal de las palabras clave históricas `function` (sustituida por `func`) e `import`/`use` (sustituidas por el modelo de topología canónica VFS).
   - Protección contra condiciones de carrera y mejoras en la serialización de WebSockets.
4. **Septiembre – Octubre 2026 (Fase AOT e IR Nativo):**  
   - Introducción de la Representación Intermedia (`pkg/ir`), lowerer de expresiones y el verificador estático de instrucciones de bajo nivel (`ir.Verifier`).
   - Implementación de poda de código muerto basada en alcanzabilidad (`reachability.go`).

---

## 2. Mejoras Arquitectónicas Verificadas

- **Aislamiento del Analizador Semántico:**  
  Se verificó que `pkg/analyzer` no contiene dependencias cruzadas con el runtime ejecutable de `pkg/core`. Todo el análisis se realiza de forma pura e inmutable sobre `PreparedProgram`.
- **Eliminación de Tipos Ambiguos:**  
  Se eliminaron exitosamente los tipos legados (`integer`, `double`, `dynamic`, `any`), forzando una semántica estricta (`int`, `float`, `decimal`, `mixed`).
- **Pruebas de Documentación Automatizadas:**  
  La inclusión de marcadores ejecutables como `<!-- joss-run: [...] -->` en los archivos `docs/*.md` auditados mediante `TestDocumentationContracts` asegura que la documentación no diverge de la implementación del lenguaje.

---

## 3. Deuda Técnica y Desalineaciones Persistentes

1. **Desfase en el Versionado y Changelog:**  
   El archivo `CHANGELOG.md` únicamente documenta cambios hasta la versión `3.6.4` (agosto 2026), mientras que en el repositorio existen etiquetas Git hasta la versión `v3.6.7.8` (octubre 2026). Las últimas 8 versiones carecen de notas de lanzamiento documentadas.
2. **Duplicación en el Mecanismo de Construcción (`joss build`):**  
   Coexisten dos rutas de compilación descoordinadas: la ruta de emisión nativa real (`pkg/backend/native/driver.go` + `pkg/ir`) y la ruta pragmática de empaquetado del runner (`cmd/joss/native_builder.go`).
3. **Uso Excesivo de `panic` en el Evaluador Runtime:**  
   Se contabilizan más de 320 ocurrencias de `panic(` en el código de producción de `pkg/core`. Aunque el CLI principal implementa un `recover()` que transforma estos panics en mensajes formateados con stack trace, es un mecanismo frágil comparado con el retorno estructurado de errores o resultados.
