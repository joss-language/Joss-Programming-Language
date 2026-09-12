## 📌 Descripción del Cambio

<!-- Explica de forma clara y concisa qué problema resuelve o qué funcionalidad añade este Pull Request. -->

Closes #(issue) / Fixes #(issue)

---

## 🛠️ Tipo de Cambio

- [ ] 🐛 **Corrección de error** (Bug fix retrocompatible)
- [ ] ✨ **Nueva funcionalidad** (Feature retrocompatible)
- [ ] ⚠️ **Breaking change** (cambio que rompe semántica previa, sintaxis o APIs)
- [ ] ⚡ **Optimización de rendimiento** (mejoras de velocidad o memoria)
- [ ] 📝 **Documentación** (corrección o adición de guías, contratos o ejemplos)
- [ ] 🔧 **Tooling / Refactorización** (pruebas, CI, compiladores de plugins o CLI)

---

## 🔍 Resumen Técnico y Decisiones de Arquitectura

<!-- 
Describe:
1. Subsistemas afectados (pkg/parser, pkg/typesystem, pkg/analyzer, pkg/core, pkg/server, etc.).
2. Decisiones de diseño adoptadas y cómo se alinean con AGENTS.md y docs/ARQUITECTURA.md.
3. Invariantes preservados (aislamiento léxico, ownership de runtime, zero imports, etc.).
-->

---

## ✅ Lista de Verificación de Calidad (Checklist Obligatorio)

Antes de solicitar revisión, marca las casillas que confirmen el cumplimiento de los estándares del proyecto:

- [ ] **Formateo**: Ejecuté `gofmt -w <archivos>` y el código respeta el estilo canónico de Go.
- [ ] **Compilación**: `go build ./...` compila limpiamente sin errores.
- [ ] **Análisis Estático**: `go vet ./...` pasa con 0 advertencias o errores.
- [ ] **Pruebas Unitarias**: `go test ./...` pasa al 100% de éxito.
- [ ] **Carreras de Concurrencia**: `go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core ./pkg/server` pasa sin reportar condiciones de carrera.
- [ ] **Generadores Automáticos**:
  - [ ] `go run ./tools/cataloggen --check` no reporta divergencias.
  - [ ] `go run ./tools/docgen --check` no reporta divergencias.
- [ ] **Contratos de Documentación y Espejo**: `go test ./pkg/core -run TestDocumentation -v` pasa y la copia en `ejemplos/Joss-Red-JosSecurity/assets/docs/` coincide byte por byte con `docs/*.md`.
- [ ] **Aislamiento de Tests**: Ninguna prueba escribe en el `$HOME` real ni fuera de `t.TempDir()`.
- [ ] **Extensión VS Code**: Si hubo cambios en sintaxis, catálogos o LSP, la extensión compila (`npm ci && npm run compile`).
