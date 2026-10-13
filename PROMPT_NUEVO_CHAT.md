# GOAL — JOSS NATIVE AOT
## Lenguaje único, una fuente de verdad, runtime/stdlib nativa y compilación standalone

---

# 0. MISIÓN FUNDAMENTAL

Quiero llevar la arquitectura de compilación nativa de **Joss Programming Language** a su diseño definitivo.

El objetivo NO es simplemente conseguir que algunos programas `.joss` puedan convertirse en ejecutables pequeños.

El objetivo real es:

> **Que TODO el lenguaje Joss existente tenga una única semántica y una única fuente de verdad, y que esa misma semántica pueda ejecutarse tanto mediante `joss run` como mediante `joss build`, sin duplicar la implementación del lenguaje, de la stdlib ni de los servicios del runtime.**

La diferencia entre ambos mecanismos debe ser únicamente:

```text
joss run
    → interpreta/ejecuta Joss

joss build
    → compila Joss a código nativo
```

Pero ambos deben ejecutar:

```text
EL MISMO LENGUAJE JOSS
```

No queremos:

```text
Joss Interpreter Language
+
Joss Native Language
```

Queremos:

```text
                    JOSS LANGUAGE
                         │
                  única semántica
                         │
              ┌──────────┴──────────┐
              │                     │
              ▼                     ▼
        joss run               joss build
        Interpreter             Native AOT
              │                     │
              ▼                     ▼
          ejecución             machine code
```

---

# 1. EL PROBLEMA ARQUITECTÓNICO QUE DEBEMOS RESOLVER

Actualmente existe una implementación real y madura en Go dentro de partes de:

```text
pkg/core
```

y otros paquetes del proyecto.

Ahí existen capacidades como:

- GranDB
- Storage / OCI
- `.env`
- HTTP
- WebSockets
- JSON
- filesystem
- criptografía
- templates
- configuración
- autenticación
- servicios auxiliares
- runtime del lenguaje
- otras capacidades de la stdlib

Al mismo tiempo, el backend nativo ha comenzado a implementar nuevamente algunas de estas funcionalidades mediante generación manual de machine code/opcodes x86_64.

Por ejemplo, archivos como:

```text
pkg/backend/native/native_grandb_stdlib_amd64.go
```

representan un problema arquitectónico si contienen una segunda implementación de lógica que ya existe en:

```text
pkg/core
```

Esto crea:

```text
Implementación A
    Go / pkg/core

Implementación B
    x86_64 / pkg/backend/native
```

Eso es exactamente lo que NO queremos.

---

# 2. REGLA DE ORO: UNA SOLA FUENTE DE VERDAD

`AGENTS.md` establece el principio:

> **Nunca duplicar lógica entre subsistemas.**

Esta regla debe ser fundamental para todo el trabajo.

Si una funcionalidad ya tiene una implementación canónica, esa implementación debe continuar siendo la fuente de verdad.

No debemos crear:

```text
Go implementation
+
C implementation
+
Assembly implementation
```

para la misma funcionalidad.

Especialmente NO debemos hacer esto para:

```text
GranDB
Storage
HTTP
WebSocket
JSON
.env
filesystem
crypto
templates
etc.
```

Si mañana cambia GranDB, Storage o HTTP, **no debería ser necesario modificar una segunda implementación escrita manualmente en assembly**.

---

# 3. LO QUE REALMENTE QUEREMOS CONSTRUIR

Queremos que Joss funcione conceptualmente así:

```text
                         JOSS SOURCE
                              │
                              ▼
                         Lexer / Parser
                              │
                              ▼
                     Semantic Analysis
                              │
                              ▼
                   Canonical Joss Semantics
                              │
                    ┌─────────┴─────────┐
                    │                   │
                    ▼                   ▼
              Interpreter            Native IR
                    │                   │
                    ▼                   ▼
               joss run           Native Backend
                                        │
                                        ▼
                                  Machine Code
                                        │
                                        ▼
                                  Native Linker
                                        │
                                        ▼
                              Standalone Executable
```

La semántica del lenguaje existe **una sola vez**.

El intérprete la ejecuta.

El compilador la transforma a código nativo.

---

# 4. NO EXISTE UN "JOSS NATIVE" DIFERENTE

Esto es extremadamente importante.

No quiero que el proyecto termine teniendo:

```text
Joss:
    funciones
    clases
    structs
    closures
    arrays
    maps
    etc.

Joss Native:
    algunas funciones
    algunas clases
    algunos structs
    algunos tipos
```

Eso crearía dos lenguajes.

El objetivo es:

```text
Joss
```

y:

```text
Joss AOT
```

representan exactamente el mismo lenguaje.

Si una característica existe actualmente y es válida en Joss, el objetivo final es que también pueda ser compilada nativamente.

Si todavía no puede compilarse, eso significa:

```text
AOT implementation gap
```

No significa:

```text
different language
```

---

# 5. NO LIMITAR EL OBJETIVO A INT, FLOAT, STRING, ETC.

No quiero que el desarrollo termine reducido a:

```text
int      ✓
float    ✓
bool     ✓
string   ✓
array    ✓
```

mientras el resto del lenguaje continúa dependiendo del intérprete.

Los tipos primitivos son solamente una parte del lenguaje.

El objetivo debe abarcar **todo lo que actualmente exista en Joss**.

Por ejemplo, si actualmente Joss soporta:

```text
variables
constantes
funciones
métodos
clases
structs
interfaces
enums
closures
callbacks
arrays
maps
strings
objetos
herencia
polimorfismo
módulos
imports
operadores
control de flujo
loops
switch/match
exceptions
try/catch
throw
return
break
continue
defer
concurrencia
async/await
generics
etc.
```

deben analizarse todos los constructos realmente existentes en el repositorio.

La lista anterior no es una especificación nueva ni una lista cerrada.

**La fuente de verdad es el lenguaje real que actualmente existe en el proyecto.**

---

# 6. TODAS LAS COMBINACIONES VÁLIDAS TAMBIÉN FORMAN PARTE DEL LENGUAJE

No basta con que cada feature funcione individualmente.

Si Joss permite combinar características, esas combinaciones también deben funcionar.

Por ejemplo:

```text
class + function
class + closure
class + struct
struct + array
object + map
function + callback
class + exception
array + function
map + function
HTTP + callback
GranDB + object
Storage + closure
concurrency + callback
etc.
```

La compatibilidad no debe medirse solamente como:

```text
"¿Funciona class?"
```

sino como:

```text
"¿El compilador puede representar correctamente la semántica de Joss cuando class interactúa con el resto del lenguaje?"
```

---

# 7. NO REDECLARAR CÓMO FUNCIONA EL LENGUAJE

No quiero que el compilador tenga que definir nuevamente:

```text
"Así funciona una función."

"Así funciona una clase."

"Así funciona un struct."

"Así funciona un método."

"Así funciona una closure."

"Así funciona un array."

"Así funciona un objeto."
```

La semántica debe existir una sola vez.

El backend nativo debe **representar** esa semántica, no inventar otra.

Arquitectura:

```text
                  JOSS SEMANTICS
                        │
             ┌──────────┴──────────┐
             │                     │
             ▼                     ▼
        Interpreter            Native IR
             │                     │
             ▼                     ▼
          Runtime             Machine Code
```

---

# 8. FUNCIONES

Si Joss permite:

```joss
fn calculate(a, b) {
    return a + b;
}
```

debe existir una única definición semántica de qué significa esa función.

`joss run` puede ejecutarla mediante el intérprete.

`joss build` puede generar:

```text
IR
→ native function
→ machine code
```

Pero no deben existir dos definiciones independientes de la semántica de funciones.

---

# 9. CLASES

Si Joss tiene:

```joss
class User {
    string name;

    fn getName() {
        return this.name;
    }
}
```

el lenguaje define:

```text
class
field
object
this
method
method dispatch
etc.
```

El backend nativo debe representar esa semántica.

No debe inventar una segunda versión:

```text
Interpreter User
+
Native User
```

con reglas diferentes.

---

# 10. STRUCTS

La misma regla aplica a:

```text
struct
```

No queremos:

```text
Struct interpreter semantics
+
Struct native semantics
```

como dos diseños independientes.

Queremos:

```text
Joss struct semantics
        ↓
Interpreter backend
        +
Native backend
```

---

# 11. CLOSURES, CALLBACKS Y FUNCIONES DE PRIMERA CLASE

Este punto es especialmente importante.

Si Joss permite:

```text
closures
callbacks
function values
higher-order functions
```

el Native IR y el ABI deben poder representar correctamente:

```text
function references
environment/captured variables
function pointers
closures
arguments
returns
lifetime
ownership
```

No se debe resolver esto creando una segunda semántica de closures exclusiva para AOT.

---

# 12. EL FRONTEND DEBE SER LA FUENTE DE VERDAD SEMÁNTICA

El frontend debe encargarse de:

```text
Source
→ Lexer
→ Parser
→ Semantic Analyzer
→ Type information
→ Symbol resolution
→ Scope resolution
→ Declarations
→ Functions
→ Classes
→ Structs
→ Methods
→ etc.
```

El backend no debe volver a analizar el lenguaje independientemente.

Si el Native IR actualmente pierde información necesaria, la solución correcta es mejorar el IR.

No duplicar el análisis.

---

# 13. EL NATIVE IR DEBE REPRESENTAR LA SEMÁNTICA COMPLETA

Si actualmente existen problemas como:

```text
calls demasiado genéricas
pérdida de tipos
arrays sin información suficiente
maps sin información suficiente
new dependiente del orden de declaraciones
global variables ignoradas
closures incompletas
method resolution perdida
```

no resolverlos copiando lógica al backend.

La solución correcta es:

```text
Frontend
    ↓
Canonical Semantic Representation
    ↓
Native IR enriquecido
    ↓
Machine Code
```

El IR debe ser una representación intermedia de la semántica real del lenguaje.

No una segunda especificación del lenguaje.

---

# 14. RUNTIME NATIVO VS STDLIB

Debemos separar conceptualmente dos capas.

## Native Runtime

Responsable de primitivas fundamentales necesarias para ejecutar código nativo:

```text
memory allocation
strings
arrays
objects
maps
exceptions
basic I/O
ABI
runtime initialization
calls
etc.
```

## Standard Library / Host Services

Responsable de capacidades superiores como:

```text
GranDB
Storage
HTTP
WebSocket
JSON
filesystem
environment
templates
crypto
etc.
```

Esta separación debe ser arquitectónica.

No debe convertirse el backend nativo en una copia completa de `pkg/core`.

---

# 15. LA STDLIB DEBE TENER UNA ÚNICA IMPLEMENTACIÓN CANÓNICA

Para funcionalidades como:

```text
GranDB
Storage
HTTP
WebSocket
JSON
.env
filesystem
crypto
templates
```

debemos identificar cuál es la implementación canónica.

Después debemos diseñar cómo esa implementación puede ser utilizada por el código AOT.

Conceptualmente:

```text
Canonical Implementation
        ↓
Native Compilation / Native Artifact
        ↓
Joss Native ABI
        ↓
Integrated Linker
        ↓
Final Executable
```

La implementación no debe copiarse manualmente a:

```text
native_*.go
```

simplemente para hacerla compilable.

---

# 16. INVESTIGAR LA ESTRATEGIA DE ARTEFACTOS NATIVOS

No asumir automáticamente una tecnología.

Investigar técnicamente las posibilidades:

```text
static object files
static archives
native runtime artifacts
Go native compilation artifacts
c-archive
C ABI exports
embedded binary objects
precompiled platform libraries
integrated linker inputs
```

y determinar cuál es la arquitectura correcta.

La solución debe cumplir:

```text
Una fuente canónica
+
Native ABI
+
Native artifact
+
Integrated linking
```

sin exigir herramientas externas al usuario.

---

# 17. IMPORTANTE: BUILD DEL COMPILADOR VS BUILD DEL USUARIO

No confundir:

### Construcción de Joss

El proyecto Joss puede estar implementado en Go y utilizar Go durante su propio desarrollo/build.

Eso no es un problema.

### Uso de Joss

Cuando el usuario ejecuta:

```bash
joss build main.joss
```

NO debe necesitar:

```text
Go
GCC
Clang
MSVC
MinGW
CMake
Ninja
LLVM CLI
etc.
```

`joss` debe contener o tener integrado todo lo necesario para producir el ejecutable.

---

# 18. `joss build` ES EL COMANDO OFICIAL DE AOT

No crear como arquitectura principal:

```text
joss build --native
```

El flujo oficial es:

```bash
joss build main.joss
```

Esto significa:

```text
joss build
=
Native AOT compilation
```

No debe existir un fallback silencioso a:

```text
Go runner
AST interpreter
```

Si una característica todavía no puede compilarse, debe producir un error explícito y diagnosticable.

Nunca:

```text
native failed
→ secretly run interpreter
```

---

# 19. `joss run` PUEDE CONTINUAR EXISTIENDO

No es necesario eliminar el intérprete.

Puede existir:

```bash
joss run main.joss
```

para ejecución interpretada.

Pero:

```bash
joss build main.joss
```

debe ser AOT real.

La diferencia es:

```text
joss run
    → Interpreter backend

joss build
    → Native backend
```

No:

```text
joss run
    → versión completa

joss build
    → subconjunto diferente
```

---

# 20. NO CREAR FALLBACKS

Queda prohibido:

```text
AOT compilation failed
        ↓
AST interpreter
```

o:

```text
Native unsupported
        ↓
Go runner
```

o:

```text
Native unsupported
        ↓
execute Joss source at runtime
```

Si algo no está soportado todavía:

```text
JOSS-NATIVE-001
```

o el diagnóstico equivalente debe explicar:

```text
qué constructo no puede compilarse
por qué
dónde ocurre
qué componente falta
```

---

# 21. MACHINE CODE GENERATOR

El backend x86_64 debe concentrarse en traducir:

```text
Joss Native IR
        ↓
Machine Code
```

Su responsabilidad incluye:

```text
register allocation
calling convention
control flow
loads/stores
arithmetic
comparisons
calls
returns
branches
relocations
object emission
linking
```

y las primitivas fundamentales del runtime.

No debe contener implementaciones duplicadas de alto nivel de:

```text
database
HTTP
Storage
templates
etc.
```

---

# 22. ABI NATIVO

Diseñar formalmente:

```text
JOSS NATIVE ABI
```

Debe definir como mínimo:

## Values

```text
integers
floating point
bool
strings
arrays
maps
objects
null
functions
errors
handles
```

## Strings

```text
pointer
length
encoding
ownership
lifetime
mutability
```

## Arrays

```text
pointer
length
capacity
element representation
ownership
lifetime
```

## Objects

```text
layout
fields
metadata
method dispatch
ownership
lifetime
```

## Functions

```text
calling convention
arguments
returns
function pointers
closures
callbacks
captured environments
```

## Errors

```text
throw
catch
propagation
panic
error representation
```

## Native services

```text
GranDB
Storage
HTTP
filesystem
etc.
```

---

# 23. NO PERDER INFORMACIÓN DE TIPOS

No quiero que el backend diga:

```text
"Todo es un CallInst genérico."
```

si eso provoca pérdida de información.

La información semántica necesaria debe llegar correctamente desde:

```text
Parser
→ Analyzer
→ Typed representation
→ Native IR
→ Backend
```

El backend no debe adivinar la semántica que el frontend ya conocía.

---

# 24. MEMORY MODEL

Definir y probar:

```text
allocation
deallocation
ownership
references
pointers
lifetimes
strings
arrays
objects
maps
closures
```

especialmente cuando diferentes componentes interactúan.

Por ejemplo:

```text
Joss object
→ native runtime
→ stdlib
→ callback
→ return to Joss
```

debe tener una política clara de ownership y lifetime.

---

# 25. GRANDB, STORAGE, HTTP, ETC.

Para cada servicio complejo:

```text
GranDB
Storage
HTTP
WebSocket
JSON
filesystem
crypto
templates
```

determinar:

1. Fuente canónica.
2. Dependencias.
3. Si es AST-independent.
4. ABI requerido.
5. Artefacto nativo necesario.
6. Cómo se enlaza.
7. Cómo se prueba.
8. Cómo `joss run` y `joss build` utilizan la misma semántica.
9. Si existe actualmente una implementación duplicada.
10. Cómo eliminar la duplicación.

---

# 26. PRUEBA DE UNA SOLA FUENTE DE VERDAD

Esta prueba es fundamental.

Debemos poder demostrar conceptualmente:

```text
Canonical implementation
        ↓
change
        ↓
rebuild native artifact/toolchain
        ↓
joss build
        ↓
program reflects change
```

sin modificar:

```text
native_grandb_stdlib_amd64.go
```

ni crear otra implementación equivalente.

Si modificar una funcionalidad requiere cambiar dos lugares, la arquitectura todavía está duplicada.

---

# 27. CONFORMANCE

Las pruebas deben comparar:

```text
Programa Joss
      │
      ├── joss run
      │
      └── joss build
              ↓
        Native executable
```

Ambos deben tener comportamiento equivalente.

No porque existan dos implementaciones del lenguaje, sino porque ambos son backends diferentes del mismo lenguaje.

Las pruebas deben incluir:

```text
functions
classes
structs
methods
closures
callbacks
arrays
maps
objects
exceptions
modules
control flow
concurrency
stdlib
GranDB
Storage
HTTP
etc.
```

y, especialmente, combinaciones entre ellos.

---

# 28. NO HACER CONFORMANCE SOLO CON PRIMITIVAS

No basta con:

```text
1 + 2
```

o:

```text
"hello"
```

Las pruebas deben crecer hacia programas reales:

```text
simple programs
→ language programs
→ object-oriented programs
→ closure/callback programs
→ stdlib programs
→ database programs
→ web programs
→ real applications
```

---

# 29. APLICACIONES REALES

Probar progresivamente con proyectos reales existentes en el repositorio.

Especialmente investigar:

```text
ejemplos/Joss-Red-JosSecurity
```

y cualquier aplicación que actualmente utilice:

```text
GranDB
Storage
HTTP
WebSockets
templates
JSON
authentication
environment
```

El objetivo no es hacer hacks específicos para que una aplicación pase.

El objetivo es comprobar que la arquitectura general del lenguaje soporta programas reales.

---

# 30. WINDOWS Y LINUX

Como mínimo:

```text
Windows x86_64
Linux x86_64
```

Windows:

```text
PE32+
x86_64
```

Linux:

```text
ELF64
x86_64
```

El ejecutable final no debe depender de:

```text
Joss CLI
Go runtime externo
compiler
interpreter
```

para ejecutarse.

Las dependencias normales del sistema operativo son aceptables, por ejemplo:

```text
Windows system DLLs
Linux system facilities
```

siempre que estén correctamente documentadas.

---

# 31. CLEAN ENVIRONMENT

Crear pruebas reales donde:

```text
PATH
```

contenga únicamente Joss.

No deben estar disponibles:

```text
go
gcc
clang
cl
msvc
mingw
cmake
ninja
llvm
lld
```

Entonces:

```bash
joss build hello.joss
```

debe funcionar.

Después:

```text
copy executable
→ clean directory
→ remove Joss
→ execute binary
```

debe funcionar.

---

# 32. AUDITORÍA DE PROCESOS EXTERNOS

Buscar en TODO el camino de:

```text
joss build
```

no únicamente en:

```text
pkg/backend/native
```

Buscar:

```go
os/exec
exec.Command
exec.LookPath
```

y equivalentes.

La auditoría debe comprobar que ninguna dependencia indirecta termina invocando:

```text
Go
GCC
Clang
MSVC
CMake
Ninja
LLVM CLI
```

durante:

```text
joss build
```

---

# 33. AUDITORÍA DEL BINARIO

Analizar los ejecutables generados.

Comprobar:

```text
PE/ELF validity
architecture
sections
imports
exports
relocations
symbols
dynamic dependencies
strings
```

Buscar evidencia de:

```text
Go runtime
AST evaluator
Joss interpreter
runner
JOSSBC2Z
JOSS_RUNNER_DATA
ExecutePrepared
evaluateExpression
pkg/core
pkg/parser
```

Pero NO depender únicamente de búsqueda de strings.

La validación debe analizar realmente:

```text
imports
dependencies
code paths
runtime initialization
```

---

# 34. NO CONFUNDIR TAMAÑO DEL BINARIO CON COMPLETITUD

Un ejecutable de:

```text
2.5 KB
```

puede demostrar que una pequeña vertical slice funciona.

No demuestra que:

```text
Joss completo
+
stdlib completa
+
runtime completo
```

estén implementados.

El tamaño no debe utilizarse como evidencia de completitud.

---

# 35. AUDITAR EL RUNTIME NATIVO ACTUAL

Revisar profundamente cualquier runtime generado mediante:

```text
machine code
byte arrays
opcode emitters
native runtime generators
```

Determinar si cada componente es:

```text
A — runtime fundamental legítimo
B — ABI adapter
C — wrapper
D — implementación duplicada
E — stub
F — experimental
G — obsoleto
```

Eliminar/revertir las implementaciones duplicadas cuando exista una arquitectura mejor.

---

# 36. NO VOLVER A CREAR EL PROBLEMA EN C

Anteriormente se exploró crear:

```text
runtime/native/csrc/joss_rt.c
```

y:

```text
pkg/backend/native/c_emitter.go
```

para copiar funcionalidades de Go.

No repetir ese patrón.

Cambiar:

```text
Go
→ C duplicate
```

por:

```text
Go
→ Assembly duplicate
```

no soluciona el problema.

El objetivo es:

```text
Canonical implementation
→ native artifact
→ ABI
→ linker
```

---

# 37. CLASIFICAR `pkg/core`

Analizar profundamente `pkg/core` y separar conceptualmente:

```text
INTERPRETER-ONLY
LANGUAGE SEMANTICS
NATIVE RUNTIME
STDLIB
HOST SERVICE
AST-DEPENDENT
AST-INDEPENDENT
```

No asumir que todo `pkg/core` debe entrar al ejecutable.

El objetivo es identificar qué parte constituye:

```text
canonical language/runtime/stdlib implementation
```

y qué parte pertenece exclusivamente al intérprete.

---

# 38. SI `pkg/core` ESTÁ DEMASIADO MEZCLADO

Si se descubre que una funcionalidad contiene:

```text
interpreter logic
+
language semantics
+
runtime logic
+
host integration
```

no copiarla.

Analizar si debe refactorizarse para separar responsabilidades.

La refactorización debe mantener:

```text
ONE SOURCE OF TRUTH
```

---

# 39. CAPABILITY MATRIX CORRECTA

La matriz NO debe utilizarse para diseñar dos lenguajes.

Debe servir para auditar el progreso hacia:

```text
Joss completo
→ Interpreter
+
Joss completo
→ Native AOT
```

Ejemplo:

| Constructo | Fuente semántica única | Interpreter | Native AOT | Conformance | Estado |
|---|---|---:|---:|---:|---|
| Functions | ✓ | ✓ | ✓ | ✓ | READY |
| Classes | ✓ | ✓ | ✓ | ✓ | READY |
| Structs | ✓ | ✓ | ✓ | ✓ | READY |
| Methods | ✓ | ✓ | ✓ | ✓ | READY |
| Closures | ✓ | ✓ | ✓ | ✓ | READY |
| Arrays | ✓ | ✓ | ✓ | ✓ | READY |
| Maps | ✓ | ✓ | ✓ | ✓ | READY |
| GranDB | ✓ | ✓ | ✓ | ✓ | READY |
| Storage | ✓ | ✓ | ✓ | ✓ | READY |

Pero si una feature todavía no compila:

```text
Native AOT = GAP
```

no:

```text
Joss Native no soporta esa feature.
```

La diferencia es fundamental.

---

# 40. EL OBJETIVO NO ES AUMENTAR EL PORCENTAJE DE FEATURES

No quiero una implementación basada en:

```text
"Ya soportamos 80%."
```

si ese 80% está compuesto por implementaciones duplicadas.

La prioridad es:

```text
CORRECT ARCHITECTURE
        ↓
ONE SOURCE OF TRUTH
        ↓
ONE LANGUAGE
        ↓
ONE SEMANTICS
        ↓
NATIVE ABI
        ↓
NATIVE ARTIFACTS
        ↓
NATIVE LINKING
        ↓
CONFORMANCE
        ↓
FULL LANGUAGE
```

---

# 41. DOCUMENTACIÓN OBLIGATORIA

Crear o actualizar:

```text
docs/native/NATIVE_ARCHITECTURE.md
docs/native/NATIVE_ABI.md
docs/native/NATIVE_RUNTIME.md
docs/native/NATIVE_STDLIB.md
docs/native/NATIVE_LINKING.md
docs/native/NATIVE_CAPABILITY_MATRIX.md
docs/native/NATIVE_CONFORMANCE.md
docs/native/NATIVE_STANDALONE.md
docs/native/NATIVE_SOURCE_OF_TRUTH.md
```

Debe documentarse claramente:

```text
qué define el lenguaje
qué define el frontend
qué es el interpreter
qué es el Native IR
qué es el native runtime
qué es la stdlib
qué es el ABI
cómo se producen los artefactos nativos
cómo se enlazan
cómo funciona Windows
cómo funciona Linux
```

---

# 42. ORDEN DE TRABAJO

## FASE 1 — COMPRENDER ANTES DE MODIFICAR

Analizar:

```text
AGENTS.md
pkg/core
pkg/backend/native
pkg/ir
parser
analyzer
runtime
stdlib
tests
```

Crear un mapa arquitectónico real.

NO comenzar implementando features.

---

## FASE 2 — IDENTIFICAR DUPLICACIONES

Buscar:

```text
native_*
machine code emitters
opcode generation
assembly
C runtime
Go runtime
```

y comparar con:

```text
pkg/core
stdlib
host services
```

Clasificar duplicaciones.

---

## FASE 3 — DEFINIR LA FUENTE DE VERDAD

Para cada funcionalidad determinar:

```text
Canonical source
```

Debe existir exactamente una implementación canónica.

---

## FASE 4 — DEFINIR EL ABI

Diseñar:

```text
JOSS Native ABI
```

antes de seguir expandiendo el backend.

---

## FASE 5 — DEFINIR EL BOUNDARY

Separar:

```text
Language semantics
Interpreter
Native IR
Native runtime
Stdlib
Host services
Native backend
Linker
```

---

## FASE 6 — DEFINIR NATIVE ARTIFACT STRATEGY

Determinar cómo las implementaciones canónicas pueden convertirse en artefactos nativos reutilizables.

Comparar técnicamente las opciones.

No elegir una solución por conveniencia.

---

## FASE 7 — CONECTAR EL LINKER

Construir:

```text
Joss machine code
+
runtime artifact
+
stdlib artifacts
```

mediante el linker integrado.

---

## FASE 8 — ELIMINAR DUPLICACIÓN

Eliminar/revertir:

```text
manual GranDB
manual Storage
manual HTTP
manual stdlib
```

cuando sean duplicaciones.

---

## FASE 9 — COMPLETAR EL NATIVE IR

Asegurar que no se pierda:

```text
types
symbols
methods
objects
closures
arrays
maps
globals
etc.
```

---

## FASE 10 — CONFORMANCE

Comparar:

```text
joss run
vs
joss build
```

para el mismo código fuente.

---

## FASE 11 — STRESS

Probar:

```text
memory
objects
arrays
maps
strings
closures
exceptions
callbacks
concurrency
```

y sus combinaciones.

---

## FASE 12 — STDLIB

Probar progresivamente:

```text
JSON
filesystem
environment
crypto
GranDB
Storage
HTTP
WebSocket
templates
etc.
```

---

## FASE 13 — APLICACIONES REALES

Probar:

```text
ejemplos/Joss-Red-JosSecurity
```

y otras aplicaciones reales.

---

## FASE 14 — CLEAN ENVIRONMENT

Verificar:

```text
only Joss installed
```

sin toolchains externos.

---

## FASE 15 — BINARY FORENSICS

Analizar ejecutables Windows/Linux.

---

## FASE 16 — DOCUMENTACIÓN

Actualizar toda la arquitectura y capability matrix.

---

# 43. CRITERIOS DE ÉXITO

El trabajo será exitoso cuando podamos demostrar:

### Lenguaje

```text
Joss tiene una sola semántica.
```

### Ejecución

```text
joss run
+
joss build
```

ejecutan el mismo lenguaje.

### AOT

```text
joss build
```

produce machine code real.

### Independencia

El usuario no necesita:

```text
Go
GCC
Clang
MSVC
LLVM CLI
CMake
Ninja
```

### Standalone

El ejecutable final no necesita:

```text
Joss
interpreter
compiler
```

para ejecutarse.

### Fuente de verdad

GranDB, Storage, HTTP, etc. no tienen una segunda implementación equivalente.

### ABI

Existe un ABI formal y probado.

### Conformance

El mismo programa tiene comportamiento equivalente entre:

```text
joss run
joss build
```

### Lenguaje completo

El objetivo final no es un subconjunto.

Es:

```text
TODO EL LENGUAJE JOSS ACTUAL
```

incluyendo combinaciones válidas.

---

# 44. CRITERIOS DE FRACASO

Detenerse y revisar la arquitectura si la solución propuesta requiere:

```text
copiar Go a assembly
```

o:

```text
copiar Go a C
```

o:

```text
crear otra implementación de GranDB
```

o:

```text
crear otra implementación de Storage
```

o:

```text
crear otra implementación de HTTP
```

o:

```text
crear una segunda semántica para classes
```

o:

```text
crear una segunda semántica para structs
```

o:

```text
crear una segunda semántica para functions
```

o:

```text
hacer que native funcione diferente a interpreter
```

o:

```text
usar AST interpreter como fallback
```

o:

```text
ejecutar Go/GCC/Clang durante joss build
```

En todos estos casos, detenerse y revisar la arquitectura.

---

# 45. REGLA PARA CUALQUIER NUEVA FEATURE

Cuando aparezca una nueva feature o una feature que no compile, preguntar:

### Pregunta incorrecta:

> "¿Cómo implementamos una versión native de esto?"

### Pregunta correcta:

> **"¿Cómo representamos en Native IR y runtime la semántica que Joss ya tiene?"**

Para stdlib:

### Pregunta incorrecta:

> "¿Cómo reescribimos GranDB en assembly?"

### Pregunta correcta:

> **"¿Cuál es la implementación canónica de GranDB y cómo hacemos que AOT pueda consumirla nativamente?"**

---

# 46. PRINCIPIO FINAL

La arquitectura final debe poder resumirse así:

```text
                         JOSS
                          │
                          ▼
                  SINGLE LANGUAGE
                          │
                          ▼
                 SINGLE SEMANTICS
                          │
             ┌────────────┴────────────┐
             │                         │
             ▼                         ▼
        INTERPRETER                  AOT
             │                         │
        joss run                 joss build
             │                         │
             ▼                         ▼
         Execution              Native IR
                                       │
                                       ▼
                                Machine Code
                                       │
                                       ▼
                                Native Runtime
                                       │
                                       ▼
                                Native Stdlib
                                       │
                                       ▼
                               Integrated Linker
                                       │
                                       ▼
                              Standalone Binary
```

El principio fundamental es:

> **Una sola definición del lenguaje.**
>
> **Una sola semántica.**
>
> **Una sola fuente de verdad para cada funcionalidad.**
>
> **Dos mecanismos de ejecución: intérprete y AOT.**
>
> **Nunca dos implementaciones paralelas de la misma lógica.**

La diferencia entre:

```text
joss run
```

y:

```text
joss build
```

debe ser únicamente:

> **HOW se ejecuta Joss.**

Nunca:

> **WHAT significa Joss.**

---

# 47. PRIMERA ACCIÓN OBLIGATORIA

Antes de implementar más funcionalidades:

1. Leer `AGENTS.md`.
2. Auditar el estado actual del Native Backend.
3. Auditar `pkg/core`.
4. Auditar `pkg/ir`.
5. Auditar parser/analyzer.
6. Encontrar todas las implementaciones nativas duplicadas.
7. Identificar la fuente canónica de cada funcionalidad.
8. Identificar qué información semántica se pierde actualmente entre analyzer → IR → backend.
9. Diseñar el ABI.
10. Diseñar la estrategia de artefactos nativos.
11. Diseñar el boundary entre runtime, stdlib, interpreter y native backend.
12. Proponer qué código debe eliminarse/revertirse.
13. Proponer la arquitectura final.

**No comiences implementando decenas de features.**

Primero demuestra que la arquitectura elimina definitivamente el problema de:

```text
Go implementation
        +
manual native implementation
```

y que nos lleva a:

```text
ONE SOURCE OF TRUTH
        +
ONE JOSS LANGUAGE
        +
INTERPRETER BACKEND
        +
NATIVE AOT BACKEND
```

Después de validar esa arquitectura, comienza la implementación.