# Contribuir a Joss

[Índice](README.md) · Antes: [arquitectura](ARQUITECTURA.md) · [Auditoría documental](DOCUMENTATION_AUDIT.md)

Empieza por [AGENTS.md](../AGENTS.md), la arquitectura, tipos, diagnósticos y los
tests del subsistema. El código y los tests ejecutados son la fuente de verdad;
una tesis, comentario o documento histórico puede describir un objetivo distinto.

## Flujo de una contribución

1. Escribe el contrato observable y sus límites.
2. Localiza la fuente canónica, sin crear otra lista paralela.
3. Añade el caso válido y el inválido vecino donde corresponda.
4. Implementa la misma regla en las capas afectadas.
5. Actualiza tutorial, referencia, catálogo y diagnóstico relacionados.
6. Ejecuta las validaciones de esta página.

## Cambiar el lenguaje

Una sintaxis nueva suele recorrer token/lexer, parser Pratt, AST, analizador y
evaluator. Define precedencia y asociatividad, recuperación ante error y ejemplos.
Un operador requiere reglas de tipos y defensa runtime. No añadas `if`, imports,
namespaces o aliases retirados como atajo de compatibilidad: son decisiones
explícitas del lenguaje.

Para un tipo, comienza en `pkg/typesystem`: Kind, nombre canónico, asignabilidad,
inferencia y coerción. Después intégralo en analyzer/runtime y sólo toca parser si
hay sintaxis nueva. Actualiza [tipos](SISTEMA_TIPOS.md) y regenera el catálogo.

Un diagnóstico público usa código estable `JOSS-...`, severidad, archivo/rango,
explicación y sugerencia. Añade evidencia suficiente y protege contra el falso
positivo vecino. Documenta el código en [diagnósticos](DIAGNOSTICOS.md).

## Añadir APIs nativas

Para un built-in global, el nombre vive en `pkg/core/builtins.go`; debe existir
un case alcanzable en uno de los dispatchers y un retorno en
`native_signatures.go`. Para una clase usa el registro ejecutado por
`Runtime.RegisterNativeClasses()` y `GetNativeClassMethods()`. Publicar sólo un
nombre sin handler crea una API fantasma; implementar sólo un case sin registrarlo
crea código inalcanzable.

Añade parámetros al metadata cuando exista soporte; no inventes aridad en el
analizador. Actualiza [contratos](MODULOS_NATIVOS.md), ejecuta
`go run ./tools/docgen` y comprueba el ejemplo en el contexto real.

Para plugins, conserva separados JOSSBC2Z, JPBC y la VM experimental. Todo nuevo
estado mutable de Runtime necesita una decisión de copia/compartición, limpieza en
`Free` y una prueba concurrente. Una operación host sensible debe cruzar una
frontera de permisos real; declararla en metadata no basta.

## Vistas, editor y publicación

Las keywords se proyectan con `parser.KeywordNames()`. VS Code consume
`vscode-joss/src/server/generated/languageCatalog.json`; nunca se edita a mano.
Las guías canónicas en español viven en `docs/*.md`; inglés y portugués conservan
el mismo nombre de archivo en `docs/en` y `docs/pt`. La publicación versionada de
JosSecurity usa `assets/docs/{es,en,pt}` y debe coincidir byte por byte con cada
idioma fuente. Tras modificar español, ejecuta
`go run ./tools/docsi18n -translate -sync`; el manifest de hashes evita trabajo
innecesario y `go run ./tools/docsi18n -check` detecta traducciones obsoletas,
faltantes, enlaces locales rotos y diferencias del espejo público.

Los ejemplos completos verificables usan marcadores `joss-run`, `joss-check` o
`joss-error` inmediatamente antes de su fence. `documentation_test.go` analiza
los tres y ejecuta `joss-run`, comparando líneas de salida. Usa `joss-check` para
fragmentos que dependen de servidor, DB o plugins y explica ese contexto.

## Validación

```sh
gofmt -w archivos_go_modificados
go run ./tools/cataloggen --check
go run ./tools/docgen --check
go run ./tools/docsi18n -check
go vet ./...
go test ./...
go test -race ./pkg/parser ./pkg/typesystem ./pkg/analyzer ./pkg/core
go build ./...
```

En `vscode-joss`: `npm ci` y `npm run compile`. Construye un binario temporal de
`./cmd/joss` y analiza el proyecto real de integración. Cambios de plantillas,
migraciones o CRUD deben pasar las matrices nombradas en AGENTS.md.

Una revisión documental debe buscar enlaces locales, fences Joss, términos legacy,
índices y diferencias entre registro y dispatcher. Evita afirmar portabilidad,
atomicidad, seguridad o compatibilidad total sin una prueba que lo demuestre.
