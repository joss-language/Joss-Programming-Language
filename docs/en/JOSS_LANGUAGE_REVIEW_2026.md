# Evaluación crítica, técnica y comparativa de Joss — 2026

Fecha de revisión: 24 de septiembre de 2026.

Esta evaluación describe el repositorio en el commit revisado y separa hechos,
valoraciones y propuestas. Las fuentes principales son `pkg/parser`,
`pkg/typesystem`, `pkg/analyzer`, `pkg/core`, `pkg/runtime`, `pkg/server`,
`pkg/formatter`, `pkg/linter`, `pkg/tester`, `cmd/joss`, `vscode-joss`, sus tests
y los ejemplos ejecutables. La documentación se usó como índice, pero ninguna
característica se consideró real sin encontrar implementación y pruebas.

## 1. Executive Summary

### Estado actual

Joss es un lenguaje interpretado, orientado principalmente a backend, con:

- lexer propio y parser Pratt;
- AST estable y serializable;
- analyzer semántico multiparchivo;
- tipos explícitos, inferidos, dinámicos y nullables;
- clases nominales, herencia simple, interfaces múltiples, abstractos, records y enums;
- closures, argumentos nombrados, defaults, spread y referencias temporales;
- análisis de asignación definida, retornos exhaustivos y código inalcanzable;
- concurrencia mediante futures, canales y `select`;
- intérprete AST como ejecución oficial y una VM experimental separada;
- servidor HTTP, routing, vistas, autenticación y GranDB integrados;
- formatter, linter, test runner, REPL, build empaquetado y LSP.

`joss run` analiza el proyecto antes de ejecutar. El build “native” empaqueta un
AST comprimido junto con un runner Go; no compila el programa Joss a código
máquina. Los archivos de `app/` se descubren automáticamente y comparten una
tabla de símbolos de proyecto.

### Evaluación

Joss ya no es un intérprete de juguete. Su analyzer, ciclo de vida del runtime,
diagnósticos, pruebas cross database y tooling muestran trabajo de ingeniería
real. Para scripts, APIs y servicios backend pequeños o medianos controlados por
el mismo equipo, puede producir software útil y mantenible.

Como lenguaje general para sistemas grandes todavía es experimental. Los
principales problemas no son falta de funciones, sino predictibilidad:

1. La sintaxis combina PHP, Dart, Go y construcciones propias sin una regla
   uniforme que permita adivinar el resto del lenguaje.
2. No existen imports ni namespaces fuente; la comodidad inicial se convierte
   en dependencia implícita y riesgo de colisiones a escala.
3. `mixed`, firmas nativas incompletas y retornos no anotados abren huecos donde
   el analyzer deja de aportar información.
4. La verdad de valores, `??`, comparaciones y algunos fallos de APIs tienen
   conductas sorprendentes o silenciosas.
5. El manejo de errores mezcla excepciones, `null`, `false`, mapas, panics
   adaptados y un `Result<T,E>` que todavía no constituye un modelo completo de
   valores y propagación.
6. Async dispone de primitivas útiles, pero no de concurrencia estructurada ni
   cancelación fuente uniforme.

### Riesgo

El riesgo dominante es que un programa pase el analyzer porque una frontera se
volvió `unknown` o `mixed`, y falle después dentro de una API nativa. En proyectos
grandes, el espacio global automático y la ausencia de imports hacen más difícil
entender dependencias y controlar cambios.

### Posible dirección

Antes de ampliar el lenguaje conviene estabilizar su semántica: una sola forma
recomendada de declarar callables y constructores, retornos explícitos en todo
callable con nombre, verdad de valores coherente, `??` limitado a null, contrato
uniforme de errores, módulos explícitos sin boilerplate y metadata completa de
APIs nativas. El objetivo debería ser “backend interpretado con análisis previo
fuerte”, no “tener una variante de cada característica popular”.

## 2. Language Identity

### Qué intenta ser

El código y las plantillas muestran cuatro intenciones simultáneas:

- productividad de scripting mediante ejecución directa, REPL e inferencia;
- backend web integrado mediante Router, Request, Response, Auth, View y GranDB;
- garantías de lenguaje compilado mediante analyzer y `PreparedProgram`;
- concurrencia accesible mediante `async`, `await`, canales y `select`.

### Qué es realmente hoy

Joss es un **lenguaje backend interpretado, web first, con tipado gradual
intencional y un framework integrado muy visible**. No es un lenguaje compilado,
aunque impone una fase de análisis antes de la ejecución normal. Tampoco es
simplemente “PHP con tipos”: los frames aislados, nullability, narrowing,
asignación definida, canales y records lo separan claramente de PHP clásico.

Su combinación filosófica más cercana es:

```text
superficie familiar de PHP
+ interpolación y nullability cercanas a Dart/Kotlin
+ concurrencia inspirada en Go
+ analyzer gradual parecido en intención a TypeScript
+ framework integrado inspirado en Laravel
```

La comparación tiene límites. Joss carece del sistema modular de todos esos
lenguajes, no posee la VM madura de Dart/JVM/.NET, y su tipado gradual no tiene la
amplitud de control de flujo de TypeScript.

### Evaluación

La identidad de producto es clara: backend productivo con baterías incluidas.
La identidad del núcleo lingüístico es menos clara porque acepta demasiados
acentos sintácticos y porque algunas capacidades del framework aparecen como
clases nativas del mismo runtime.

### Riesgo

Si cada necesidad del framework añade sintaxis o semántica al lenguaje, Joss
terminará siendo difícil de implementar fuera de `core` y difícil de aprender
sin aprender todo el framework.

### Posible dirección

Mantener un núcleo pequeño: valores, tipos, control de flujo, callables, OOP,
errores y concurrencia. HTTP, ORM, auth y templates deben seguir siendo
bibliotecas nativas con metadata, no nuevas reglas del parser.

## 3. Syntax Review

### Fortalezas

- Las variables con `$` se distinguen visualmente de tipos, clases y funciones.
- `func`, `return`, `class`, `interface`, `enum`, `match`, `try` y `catch` son
  legibles y conocidos.
- `T?` y `T|null` expresan nullabilidad de manera concisa.
- Los argumentos nombrados y defaults hacen legibles APIs con varias opciones.
- `?->`, `??`, interpolación y trailing commas reducen código ceremonial.
- Los records proporcionan datos inmutables compactos.
- Nueva línea o `;` permite estilo ligero sin impedir generación automática.

### Inconsistencias importantes

#### No existen `if`, `else`, `for` clásico ni `switch`

El control condicional ordinario se expresa mediante ternarios con bloques,
`guard` o `match`:

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
public func estado(int $saldo): string {
    ($saldo <= 0) ? {
        return "sin saldo"
    } : {
        return "disponible"
    }
}
```

**Estado actual:** el parser soporta bloques dentro del ternario y una forma de
una sola rama. `guard ($cond) : { ... }` exige que el bloque de salida termine el
callable. `match` cubre selección por valor.

**Evaluación:** eliminar `if/else` no simplifica el modelo mental; desplaza una
sentencia universal hacia un operador que también produce valores. Un lector
debe decidir si `?` está seleccionando una expresión, ejecutando un bloque o
implementando una guarda. Es la decisión sintáctica más costosa de Joss.

**Riesgo:** ternarios anidados, herramientas más complejas y código de negocio
menos familiar. También aumenta la distancia entre lo que un principiante cree
que hace `?:` y lo que el runtime permite.

**Posible dirección:** evaluar una sentencia condicional canónica antes de 4.x.
No se necesita eliminar el ternario expresión; se necesita evitar que cargue con
todo el control condicional.

#### `let` no significa inmutabilidad

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
var $a = 1       // infiere int fijo
$b = 1           // también infiere int fijo
let $c = 1       // mixed, puede cambiar de tipo
mixed $d = 1     // mixed explícito
const $e = 1     // binding inmutable
```

**Estado actual:** `var` es contextual y no keyword; `$x =` declara por primera
asignación; `let $x` significa `mixed`; `let T $x` significa tipo explícito.

**Evaluación:** la semántica interna de `var` y `mixed` es buena, pero `let` es
un nombre engañoso. En la mayoría de lenguajes modernos sugiere binding local o
inmutable, mientras aquí selecciona dinamismo si no lleva tipo.

**Riesgo:** errores de expectativa, especialmente para usuarios de Swift,
JavaScript, Rust o Kotlin.

**Posible dirección:** conservar `var` para inferencia, `mixed` para dinamismo y
`const` para inmutabilidad; deprecar gradualmente el significado especial de
`let $x` o documentarlo como compatibilidad, no como forma recomendada.

#### Dos nombres de ausencia

`null` y `nil` producen el mismo valor. Aportan familiaridad a dos comunidades,
pero duplican vocabulario sin expresar una diferencia. Conviene elegir uno como
forma canónica y mantener el otro sólo como alias de compatibilidad.

#### Precedencias sorprendentes

El parser asigna a `%` mayor precedencia que `*` y `/`, y coloca `&&` y `||` en
el mismo nivel. Por tanto:

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
8 * 5 % 3                 // 16: 8 * (5 % 3)
true || false && false    // false: (true || false) && false
```

Esto contradice la intuición transferida de casi todos los lenguajes de la
comparación. La documentación puede advertirlo, pero no elimina el costo.

#### Truthiness irregular

`0` entero, decimal cero, `""`, `"0"` y array vacío son falsos. `0.0` float y
map vacío son verdaderos. Esta diferencia depende de representación, no de un
principio que el usuario pueda deducir.

#### Demasiados operadores con dominios distintos

`<<` significa shift, escritura a `cout` o envío a canal según el receptor;
`>>` puede significar shift o lectura de `cin`. `.` concatena, `|>` inyecta el
operando como primer argumento, `..` crea rangos y `...` sirve para spread.
Cada operación es defendible por separado, pero juntas aumentan la densidad de
símbolos y la carga del parser, formatter y lector.

### Predictibilidad global

Un usuario puede adivinar declaraciones, clases y llamadas después de ver pocos
ejemplos. No puede adivinar correctamente control condicional, precedencia,
truthiness, inicializadores o descubrimiento multiparchivo. La coherencia
sintáctica actual es **media**, no alta.

### Discrepancias documentales observadas

La referencia de sintaxis todavía contiene una sección que afirma que no hay
interfaces, aunque parser, analyzer, runtime y tests sí las implementan. La lista
resumida de keywords también ha quedado por detrás de `guard` y `record`. Esto no
cambia el lenguaje real, pero demuestra que la superficie creció más rápido que
algunas explicaciones.

## 4. Type System Review

### Estado actual

Los tipos canónicos son `int`, `float`, `decimal`, `string`, `bool`, `array`,
`map`, `object`, `channel`, `mixed`, `null`, uniones y nombres nominales. Hay
formas parametrizadas para `array<T>`, `map<K,V>`, `channel<T>` y representación
parcial de `Result<T,E>`. Los aliases históricos se rechazan.

`typesystem.Assignable` es permisivo cuando alguno de los lados es `unknown` o
`mixed`. El analyzer extiende la compatibilidad con herencia e interfaces. La
inferencia fija el tipo de `$x = valor` y `var $x = valor`; `mixed` permite
cambios deliberados. `T?` se normaliza a `T|null`.

### Lo bien diseñado

- La diferencia conceptual entre inferido y dinámico es correcta.
- Los parámetros sin tipo se rechazan; `mixed` debe escribirse.
- Las uniones nullables y el narrowing por `is`/null aportan seguridad real.
- La asignación definida considera ramas, retornos tempranos y loops.
- Los enteros comprueban overflow y división por cero con semántica compartida.
- `decimal` evita usar float para dinero.
- Las colecciones tipadas y canales tipados detectan errores comunes.
- Las clases e interfaces son nominales, lo que favorece contratos estables.

### Límites y problemas

1. **`unknown` es no acusatorio.** Es razonable para recuperación, pero cada
   firma nativa incompleta propaga una zona sin verificación.
2. **`mixed` desactiva garantías.** Es explícito, pero APIs del framework lo usan
   con frecuencia porque sus retornos son heterogéneos.
3. **Genericidad parcial.** Parser y analyzer reconocen parámetros de tipo en
   clases/métodos, mientras el runtime opera principalmente por valores borrados.
   No existe aún una historia completa de variance, constraints, especialización
   o reflexión genérica.
4. **Colecciones mutables y aliasadas.** `const` protege el binding, no congela
   profundamente arrays, maps u objetos.
5. **Promoción `int → float`.** Puede perder precisión por encima de 2^53. Para
   IDs grandes, “asignable” no equivale a “sin pérdida”.
6. **`object` y clases concretas.** `object` es un escape amplio; usarlo reduce
   resolución anticipada de miembros.
7. **`Result<T,E>` incompleto.** Existe en representación de tipos y docs, pero
   no hay un valor algebraico universal con construcción, propagación y manejo
   exhaustivo integrado en todas las APIs.

### Evaluación

El sistema ayuda realmente. En código de aplicación que evita `mixed`, usa
retornos anotados y consume firmas conocidas, Joss detecta una parte importante
de los errores antes de ejecutar. Debe describirse como **tipado gradual con
núcleo nominal**, no como tipado estático completo.

## 5. Functions

### Estado actual

Las funciones globales con nombre requieren `public` o `private`. Todo parámetro
fuente requiere tipo. Hay defaults, argumentos nombrados, spread, closures,
captura léxica, referencias temporales `ref`, recursión limitada, generadores con
`yield`, funciones genéricas parciales y pipeline. No hay overload por firma ni
sintaxis de flecha.

Cada llamada crea un frame. Una función con nombre no captura variables del
archivo ni del caller; una closure sí captura su entorno. Esta distinción está
implementada y probada.

### Fortalezas

- Parámetros obligatoriamente tipados.
- Binding posicional, nombrado, defaults y `ref` pasa por una ruta común.
- Frames aislados evitan scope dinámico accidental.
- Closures tienen semántica léxica razonable.
- `ref` es temporal, invariante, no escapable y no cruza async/plugins.
- El analyzer comprueba aridad, duplicados, nombres y tipos cuando conoce firma.
- Retornos anotados se validan en cada `return` y en todas las rutas.

### Fricción

- La visibilidad obligatoria en funciones top level aporta disciplina, pero
  resulta extraña sin módulos/imports visibles que expliquen “público para quién”.
- El tipo de retorno sigue siendo opcional; sin él, el callable queda `unknown`
  en declaraciones y la seguridad de sus consumidores disminuye.
- Función, método y closure comparten `func`, pero sólo las dos primeras tienen
  nombre/visibilidad y la inferencia de retorno no construye hoy un contrato
  completo reusable.
- Generadores, async y closures amplían mucho el espacio semántico de `func`.
- No existe overload; esto simplifica resolución y debe conservarse salvo un
  problema real muy fuerte.

Las llamadas pueden expandir arrays con `...`, pero la gramática de parámetros
fuente no declara un parámetro variádico general. Las APIs nativas sí pueden
marcarse variádicas en metadata. Esa diferencia debe explicarse como capacidad
del host, no como variadic source functions.

### Evaluación

El núcleo de llamadas está bien diseñado. La mayor debilidad es permitir
callables con nombre sin retorno anotado mientras se promete análisis previo
fuerte. La ausencia de funciones flecha no es un problema relevante.

## 6. Classes / OOP

### Estado actual

Joss implementa:

- clases públicas o privadas;
- propiedades y métodos con visibilidad obligatoria;
- herencia simple;
- múltiples interfaces e interfaces que extienden varias interfaces;
- clases y métodos abstractos;
- miembros estáticos;
- constructores `constructor` e inicializadores `Init`;
- promoción de propiedades en parámetros de constructor;
- records inmutables;
- enums;
- tipos nominales y resolución de miembros;
- metadata de clase cacheada por runtime.

### Fortalezas

- La visibilidad explícita evita defaults ambiguos de PHP.
- Los contratos de interfaz se comprueban antes de ejecutar.
- Las clases concretas deben implementar abstractos.
- El runtime vuelve a defender visibilidad y construcción.
- Records cubren DTOs sin introducir setters o builders.
- El analyzer puede resolver miembros y llamadas de receptores nominales.

### Problemas

#### Dos conceptos de inicialización

El repositorio reconoce nombres `constructor`, `Init` y ejemplos como
`Init constructor(...)`. Esta compatibilidad facilita migraciones, pero crea una
superficie difícil de explicar: ¿es `Init` una sentencia, un nombre de método o
un modificador? Una sola forma canónica reduciría parser, metadata y tooling.

#### Falta una regla explícita de override

La sustitución se deriva por nombre y contrato. Sin `override`, un typo puede
crear un método nuevo en vez de reemplazar el del padre. Este es un caso donde
una palabra adicional sí prevendría un bug real, aunque debe diseñarse junto con
final/sealed sólo si aparecen necesidades demostradas.

#### Genericidad nominal todavía parcial

Las declaraciones genéricas existen, pero no tienen todavía la madurez de Java,
C#, Kotlin o TypeScript. Deben etiquetarse como capacidad limitada hasta probar
contratos, runtime y tooling de extremo a extremo.

#### Lookup dinámico residual

Los facts del analyzer conservan llamadas resueltas, pero el intérprete sigue
teniendo rutas por nombre y fallbacks dinámicos. La metadata ayuda, aunque aún no
equivale a vtables/slots totalmente preparados.

### Evaluación comparativa

El modelo mental se parece más a PHP moderno con disciplina de Dart/C# que a
Python. Es más explícito que PHP y Python, menos completo que Java/C#/Kotlin y
menos estructural que TypeScript. Para OOP de aplicación es suficiente; para
frameworks genéricos grandes todavía faltan módulos, genericidad madura y reglas
de override.

## 7. Error Prevention

### Errores detectados antes de ejecutar

El analyzer detecta, entre otros:

- símbolos inexistentes o usados antes de inicialización;
- redeclaraciones y declaraciones duplicadas de proyecto;
- tipos de inicialización, asignación, argumentos y retornos incompatibles;
- parámetros sin tipo;
- aridad incorrecta, argumentos nombrados desconocidos o duplicados;
- miembros inexistentes cuando el receptor es resoluble;
- acceso estático/de instancia incorrecto;
- violaciones de visibilidad;
- clases base e interfaces inexistentes;
- ciclos de interfaces y contratos no implementados;
- instanciación de abstractos;
- rutas sin retorno para retornos anotados;
- código inalcanzable;
- match no exhaustivo de enums, actualmente como warning en varias rutas;
- referencias mal marcadas, a constantes, no l-values o con tipo no invariante;
- envío incompatible a `channel<T>`;
- narrowing imposible y acceso null inseguro en casos conocidos;
- mutación incompatible de colecciones tipadas.

### Errores que todavía llegan al runtime

- división por cero no constante y overflow dependiente de datos;
- índices y claves dependientes de datos;
- receiver `mixed` o `object` con miembro inexistente;
- firmas nativas de aridad desconocida;
- errores de red, filesystem, DB, FFI y plugins;
- canales cerrados y deadlocks;
- recursion/stack limitada a 1024 llamadas;
- casts dependientes de valores externos;
- carreras lógicas sobre objetos/maps compartidos;
- tasks huérfanas o bloqueadas;
- constraints, tipos y errores específicos de bases reales.

### Errores silenciosos o debilitados

1. **`??` recupera cualquier panic de la izquierda y lo trata como null.** Un
   bug de acceso, aritmética o implementación puede convertirse en fallback.
2. **`match` sin coincidencia/default devuelve null.** Si el tipo no es un enum
   comprobable, una omisión puede viajar silenciosamente.
3. **Truthiness inconsistente.** `0.0` y `{}` se comportan distinto de `0` y `[]`.
4. **Igualdad no estricta usa representación textual como fallback.** Valores de
   tipos distintos pueden parecer iguales por impresión.
5. **APIs nativas mezclan null, false, panic y mensajes impresos.** El caller no
   siempre puede deducir el contrato por la firma.
6. **Metadata nativa incompleta.** `ArityKnown=false` es honesto, pero obliga a
   descubrir errores de llamada durante ejecución.
7. **Lectura SQL legacy.** Algunas rutas históricas convierten errores en valores
   vacíos o imprimen en vez de producir un error estructurado.

Los tres primeros son problemas de lenguaje, no simples mensajes mejorables.

## 8. Static Analysis vs Runtime

| Decisión/error | Analyzer | Runtime | Evaluación |
|---|---|---|---|
| Variable inexistente | Sí | Defensa | Bien ubicado. |
| Uso antes de inicializar | Sí, con flujo | Puede encontrar nil/ausencia | Garantía valiosa. |
| Asignación incompatible | Sí si tipo conocido | Sí | Se pierde con `mixed/unknown`. |
| Retorno incompatible | Sí si anotado | Sí | Sin anotación no hay contrato fuerte. |
| Ruta sin retorno | Sí si anotado | Retorno nulo implícito | Debe cerrarse para callables nombrados. |
| Llamada/argumentos | Sí si firma conocida | Sí | Metadata nativa es el límite. |
| Miembro inexistente | Sí si receptor nominal | Sí si dinámico | Correcto para tipado gradual. |
| Null member access | Parcial con narrowing | Sí | Falta null state más uniforme. |
| División por cero | Constante/análisis aritmético parcial | Sí | División dinámica debe seguir runtime. |
| Índice fuera de rango | Sólo si demostrable | Sí | Correcto. |
| Enum match incompleto | Warning | Null posible | Debería ser error cuando se usa como valor no nullable. |
| Async mal esperado | Parcial | Sí | Falta tipo `Future<T>` fuente completo. |
| Canal cerrado/capacidad | Tipos/capacidad constante parcial | Error estructurado | Mejorado, pero deadlock es dinámico. |
| Acceso privado/protegido | Sí | Sí | Buena defensa doble. |
| Imports/dependencias | No hay imports | Descubrimiento automático | El problema se evita sintácticamente pero reaparece como global implícito. |

El reparto general es correcto: el runtime conserva defensas para datos. La
brecha principal es información perdida, no ausencia total de analyzer.

## 9. Architecture

### ¿MVC es obligatorio?

No. El parser, analyzer y runtime no exigen controllers, models o views. Un
archivo puede ejecutar:

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
public func sumar(int $a, int $b): int {
    return $a + $b
}
print(sumar(2, 3))
```

`joss run archivo.joss`, `joss new console`, `joss new package`, REPL, mobile SDK
y plugins demuestran usos sin MVC ni HTTP.

### Dónde sí existe orientación MVC

- `joss new` crea web por default.
- Existen generadores `make:controller`, `make:model`, `make:view`, `make:mvc` y
  `make:crud`.
- `server start` exige `main.joss`.
- `LoadProject` trata `main.joss`, `routes.joss` y `app/` como superficie especial.
- El runtime precarga `app/**/*.joss`.
- Muchas clases nativas web se registran siempre en `Runtime`.

Por tanto, MVC es **convención fuerte del framework y CLI**, no regla del lenguaje.

### Otras arquitecturas

| Arquitectura | Viabilidad | Fricción real |
|---|---|---|
| MVC | Alta | Camino privilegiado por plantillas y generadores. |
| API only | Alta | Router/Response funcionan sin views. |
| CLI/scripts | Alta | `run`, `eval`, REPL y plantilla console. |
| Workers | Media/alta | Async/cron/canales existen; falta supervisión estructurada. |
| Layered | Media | Carpetas funcionan, pero sin módulos explícitos. |
| Clean/Hexagonal | Media | Interfaces ayudan; auto discovery y nativos globales debilitan puertos explícitos. |
| Modular monolith | Media/baja a gran escala | Un namespace de proyecto y ausencia de imports. |
| Microservices | Media/alta | Cada servicio puede ser pequeño; deployment/ecosistema menos maduros. |
| Event driven | Media | Canales/callbacks/WebSockets; faltan contratos de broker y cancelación uniformes. |
| Librerías/SDKs | Media | Packages/plugins existen; API/versionado y genericidad necesitan madurez. |
| Desktop/mobile | Experimental/dirigido | Hay rutas de build/SDK, no equivalen a ecosistemas UI maduros. |

### Lenguaje frente a framework

```text
Lenguaje: parser + AST + tipos + analyzer
Runtime: evaluator + frames + valores + errores + concurrencia
Stdlib/nativos: filesystem, strings, JSON, procesos, red, etc.
Framework web: server + Router/Request/Response/Auth/View/Session
Persistencia: GranDB + Model + Schema
Tooling: CLI + formatter + linter + tester + LSP + build
```

Las dependencias internas están mejor separadas que la experiencia superficial:
`analyzer` no importa `core`, y `server` no define sintaxis. Sin embargo,
`Runtime.RegisterNativeClasses` instala una superficie enorme incluso para
programas que no usan web. La separación de paquetes Go es buena; la separación
de capacidades visible al usuario todavía es débil.

## 10. Developer Experience

### Instalación y primer programa

La ruta `joss run`, REPL y `joss new console` es sencilla. Un “Hola mundo” tiene
muy poco ruido. La instalación desde binario o Go es directa según docs.

### Crear proyecto

El CLI es amplio y productivo. El default web ayuda al público objetivo, aunque
refuerza la impresión de que Joss sólo sirve para MVC. La cantidad de comandos
(`make:*`, migraciones, storage, pub, plugins, build, server, program) puede
abrumar antes de entender qué pertenece al lenguaje.

### Escribir y corregir

`joss run` bloquea errores del analyzer. Los diagnósticos incluyen código,
archivo, rango, explicación y sugerencia. Esto es una fortaleza clara.
Formatter, linter y `check` cubren un flujo CI razonable.

### Testing

`joss test` reconoce `*_test.joss`, suites y asserts. Es suficiente para tests
unitarios básicos. El repositorio usa además Go tests, fuzzing puntual, race y
contratos documentales, pero esas capacidades internas no son todas parte del
test runner del usuario.

### Editor

El LSP anuncia completion, hover, go to definition, references, rename,
formatting y diagnostics. El catálogo generado evita listas manuales. La calidad
de una función depende de cuánta metadata semántica atraviesa hacia TypeScript;
no debe equipararse todavía con Dart Analyzer, Roslyn, IntelliJ o gopls.

### Debugging y profiling

Los stack frames Joss y errores estructurados son mejores que panics Go crudos.
No aparece un debugger fuente maduro con breakpoints, step, watches y evaluación
de frames, ni un profiler de usuario integrado. Esta es una carencia seria para
aplicaciones empresariales.

### Build y deployment

El build empaquetado simplifica distribución, pero no aporta optimización AOT ni
las garantías de una compilación nativa. El runtime, assets y dependencias siguen
formando parte del artefacto. El ecosistema de paquetes/plugins existe, aunque
es mucho menos probado y poblado que npm, PyPI, Maven, NuGet o crates.io.

### Evaluación

La DX inicial es buena; la DX de mantenimiento y diagnóstico profundo es media.
El recorrido se vuelve más difícil al salir del happy path del framework.

## 11. Comparison With Other Languages

### Go

Joss comparte canales, `select`, `defer`, tooling por CLI y preferencia por pocas
abstracciones. Joss ofrece OOP, exceptions, null, inferencia dinámica y framework
integrado; Go ofrece módulos explícitos, compilación nativa, interfaces simples,
concurrencia más madura y deployment más predecible. Los canales de Joss no
heredan automáticamente la disciplina del scheduler y race model de Go.

### Dart

Dart es la comparación más útil para analyzer, null safety, async y experiencia
de editor. Joss tiene menor ceremonia y backend integrado; Dart tiene tipos,
futures, paquetes, tooling y semántica mucho más uniformes. Joss debería aprender
de la coherencia, no copiar Flutter ni su sintaxis completa.

### Kotlin

Ambos buscan productividad con nullability y OOP. Kotlin tiene smart casts,
generics y contratos JVM maduros, pero mucha más complejidad. Joss debe evitar
replicar extension functions, overloads y jerarquías sofisticadas antes de
cerrar sus garantías básicas.

### Java y C#

Joss es mucho menos verboso y más rápido para CRUD. Java/C# ganan en módulos,
reflection coherente, debugging, profilers, genericidad, deployment empresarial
y décadas de ecosistema. El modelo nominal de Joss puede beneficiarse de IDs,
slots y metadata estable sin adoptar su ceremonia.

### TypeScript

Ambos permiten escapes dinámicos y análisis de flujo. TypeScript posee un sistema
estructural extremadamente expresivo, pero depende de JavaScript y puede ser
unsound deliberadamente. Joss controla su runtime y puede ofrecer garantías más
firmes, aunque hoy su inferencia y narrowing son mucho menores.

### Python

Python gana en legibilidad convencional, ecosistema, introspección y velocidad
de prototipado. Joss detecta más errores antes de ejecutar y distribuye un
framework coherente. Joss no debería imitar el dinamismo implícito de Python;
`mixed` explícito es una mejor decisión.

### PHP

PHP es la influencia superficial más visible: `$variables`, `->`, `::`, arrays,
truthiness y orientación web. Joss mejora frames, analyzer, concurrencia y
contratos nominales. También hereda riesgos cuando conserva comparaciones,
truthiness o APIs que retornan `false/null` de manera heterogénea.

### Rust

Rust ofrece ownership, enums algebraicos y `Result` rigurosos a cambio de mayor
curva. Joss está sobre Go y no necesita borrow checker completo para ser memory
safe. Sí puede adoptar el principio de fallos explícitos y exhaustividad donde
resuelva la mezcla actual de null/false/panic.

### Conclusión comparativa

Joss compite mejor por integración y velocidad de construcción que por
rendimiento, ecosistema o formalidad. Su espacio natural es entre PHP/Python y
Dart/Kotlin: más seguro que los primeros, mucho más pequeño y menos maduro que
los segundos.

## 12. Accidental Complexity

### Formas duplicadas o solapadas

1. `$x =`, `var`, `let T`, `T $x`, `let $x` y `mixed $x` cubren tres conceptos
   con seis superficies.
2. `null` y `nil` son el mismo valor.
3. `print`, `echo` y `cout <<` escriben salida con diferencias menores.
4. `constructor`, `Init` y combinaciones históricas inicializan objetos.
5. `is` e `instanceof` se solapan para comprobación nominal.
6. Ternario bloque, guard y match cubren partes de una sentencia condicional que
   otros lenguajes expresan con una regla base.
7. Intérprete, AST bytecode, VM experimental y JPBC son cuatro conceptos que el
   término “compilado” puede confundir.
8. APIs globales, clases nativas y métodos primitivos ofrecen capacidades que no
   siempre siguen la misma convención de nombres/errores.
9. Proyecto fuente sin imports, packages y plugins son tres modelos de
   composición con límites distintos.

### Evaluación

No toda duplicación debe eliminarse: `print` y streams pueden servir a públicos
distintos. Pero cada alias debe pagar su costo en parser, analyzer, formatter,
LSP, documentación y compatibilidad. Joss ya tiene suficientes características;
su siguiente ganancia vendrá de reducir formas, no de añadir sinónimos.

## 13. Safety Model

| Categoría | Protección actual | Límite real | Evaluación |
|---|---|---|---|
| Type safety | Analyzer + runtime, tipos nominales/uniones/colecciones | `mixed`, `unknown`, nativos incompletos | Media/alta en código anotado; baja en fronteras dinámicas. |
| Null safety | `T?`, narrowing, `?->`, `??` | `??` traga panics; APIs retornan null heterogéneo | Media. |
| Memory safety | Go GC, sin punteros fuente, `ref` no escapable | aliasing mutable; FFI/plugins nativos | Alta para código Joss puro. |
| Concurrency safety | canales, errores estructurados, runtime fork, race tests | objetos/maps compartidos, deadlocks, tasks huérfanas | Media. |
| API misuse | firmas, argumentos, visibilidad, capabilities | aridad nativa desconocida y retornos `mixed` | Media. |
| Definite assignment | análisis de ramas y loops | caminos dinámicos/unknown | Alta dentro del modelo soportado. |
| Control flow | retornos, unreachable, guard, enum match | retornos no anotados y constructs complejos | Media/alta con anotaciones. |
| Mutability | `const`, records, ref restringido | `const` no es deep immutable | Media/baja para grafos. |
| Encapsulation | public/protected/private en analyzer/runtime | espacio global automático | Alta dentro de clases, media entre archivos. |
| Runtime safety | overflow, profundidad, errores estructurados, cleanup | loops infinitos fuera de host con timeout; FFI/I/O | Media/alta. |
| Error handling | try/catch/throw, stack Joss | null/false/panic/Result inconsistente | Media/baja como modelo global. |
| Predictibilidad | analyzer bloquea errores, fuentes canónicas | truthiness, precedencia y aliases | Media. |

“Seguro” debe entenderse como resistencia a errores del programador, no como
sandbox. Joss tiene capabilities para host y modo restringido, pero una
aplicación normal puede usar filesystem, red, procesos y drivers según su host.

## 14. Return Type Analysis

### Estado actual

El tipo de retorno es opcional. Si existe, analyzer y runtime comprueban cada
retorno y el analyzer exige que todas las rutas retornen o lancen. Si falta, la
declaración queda con retorno `unknown`; el cuerpo se analiza, pero los callers no
reciben un contrato inferido estable equivalente.

### Alternativa A: siempre obligatorio

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
public class User {}
public func getUser(int $id): User|null {
    return null
}
private func normalize(string $value): string {
    return $value
}
```

Ventajas: mejor documentación, recursion, LSP, cambios cross file y resolución
anticipada. Desventaja: verbosidad en closures y callbacks pequeños.

### Alternativa B: inferencia general

Es cómoda, pero exige unir todos los `return`, manejar recursion, excepciones,
generadores, async, uniones y cambios incrementales. Una inferencia accidental
puede convertir un cambio local en ruptura pública sin que el autor lo vea.

### Alternativa C: híbrido por visibilidad

Obligar sólo públicos parece atractivo, pero mover un método de private a public
cambia la gramática válida. Además, los privados también participan en recursion,
tooling y mantenimiento.

### Recomendación

**Todo callable con nombre debería declarar retorno; closures deberían inferirlo.**

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
public class User {}
public func getUser(int $id): User|null {
    return null
}

var $format = func(User $user) {
    return $user->name
}
```

Los callables que sólo producen efectos necesitan un tipo canónico como `void`
o una regla explícita de ausencia de valor; usar retorno omitido para significar
simultáneamente “void” y “unknown” es la ambigüedad que debe eliminarse.

Esta regla encaja con Joss porque ya exige tipos en parámetros y análisis antes
de ejecutar. Aplicación compatible:

1. inferir internamente retornos omitidos y emitir warning;
2. ofrecer fix automático;
3. exigirlos en paquetes/CI estricto;
4. convertirlo en error en la siguiente versión mayor.

## 15. Pit of Success

### Donde la ruta fácil también es segura

- `$x = 1` infiere `int` fijo sin ceremonia.
- Parámetros exigen tipo y sugieren `mixed` sólo cuando es deliberado.
- `joss run` analiza automáticamente.
- `Model` bloquea mass assignment por default.
- GranDB usa bindings y rechaza update/delete sin filtros.
- `?` nullability y `?->` hacen visible la ausencia.
- Records ofrecen datos inmutables con poca sintaxis.
- Frames aislados y `ref` restringido evitan mutación oculta entre funciones.

### Donde el usuario debe conocer reglas especiales

- Condiciones sin `if`.
- `let` dinámico.
- precedencia de `%`, `&&` y `||`;
- truthiness de float/map;
- `??` como recuperador de panic;
- descubrimiento automático y visibilidad global;
- diferencia entre build empaquetado, VM y plugin bytecode;
- APIs que fallan con null, false o excepción según el módulo;
- cuándo una firma nativa es realmente conocida por el analyzer.

### Evaluación

Joss crea un pit of success razonable en variables tipadas, clases y CRUD. No lo
crea todavía en control condicional, errores, módulos y concurrencia. El lenguaje
requiere aprender demasiadas excepciones antes de poder predecirlo por analogía.

### Happy paths comprobados conceptualmente

#### Caso A — CLI sencillo

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
public func doble(int $valor): int {
    return $valor * 2
}
print(doble(21))
```

Fricción baja: archivo único, `joss run`, analyzer automático y build empaquetado.
La experiencia empeora si necesita flags complejos, streams o distribución de
dependencias porque no hay una biblioteca CLI tipada comparable a Cobra/clap.

#### Caso B — API REST

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
Router::get("/health", func(mixed $request): mixed {
    return Response::json({"ok": true})
})
```

Fricción baja dentro del framework. El retorno `mixed` y algunas firmas nativas
reducen comprobación estática; arquitectura, auth y middleware ya están cerca.

#### Caso C — CRUD con GranDB

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
public class User extends Model {
    protected string $table = "users"
    protected array $fillable = ["name", "email"]
}
var $user = User::create({"name": "Ada", "email": "ada@example.test"})
```

Fricción baja y seguridad razonable: bindings, mass assignment cerrada, dirty
tracking y cuatro dialectos probados. Migraciones, relaciones y errores de DB
siguen requiriendo entender el framework.

#### Caso D — Servicio async

<!-- joss-check: ejemplo conceptual de la evaluación -->
```joss
public func obtenerDatos(): mixed {
    return null
}
var $task = async { return obtenerDatos() }
mixed $result = await($task)
```

Fricción sintáctica baja, garantías medias. No existe un tipo fuente maduro
`Future<T>` ni un scope de tasks con cancelación y join automáticos.

#### Caso E — Sistema orientado a objetos

Interfaces, abstractos, records y visibilidad permiten diseño convencional. La
fricción aparece en constructores duplicados, ausencia de `override` y módulos.

#### Caso F — Procesamiento concurrente

Canales tipados y `select` son útiles. El usuario debe diseñar cierre, buffer,
cancelación y supervisión; el lenguaje no impide deadlocks ni aliasing de maps.

#### Caso G — Proyecto empresarial

El analyzer, formatter, tests, ORM y servidor proporcionan una base fuerte. El
cuello de botella es organización: namespace global, auto discovery, APIs nativas
heterogéneas, debugging limitado y ecosistema pequeño. Es viable con gobernanza
estricta de un equipo; todavía no es una plataforma cómoda para muchos equipos
con ownership de módulos independiente.

## 16. Long-Term Risks

### Antes de estabilizar 4.x

1. **Módulos y símbolos.** El namespace global automático será muy caro de
   migrar cuando existan miles de paquetes.
2. **Retornos omitidos.** Cada API pública sin contrato propaga `unknown`.
3. **Semántica condicional.** El uso de ternario como sentencia se incrustará en
   todo el código y formatter.
4. **Truthiness y comparaciones.** Cambiarlas después rompe lógica silenciosa.
5. **Modelo de errores.** La mezcla null/false/panic debe cerrarse antes de que
   más stdlib dependa de ella.
6. **Constructores duplicados.** Elegir forma canónica cuanto antes.
7. **Genericidad.** Definir alcance real o marcarla experimental; una promesa
   parcial es más peligrosa que no tenerla.
8. **Async/cancelación.** Futures sin parent scope facilitan trabajo huérfano.
9. **Nativos globales.** Registrar todo el framework en cada runtime hace difícil
   crear perfiles mínimos o sandboxes auditables.
10. **VM paralela.** Ampliarla antes de equivalencia produce dos lenguajes.
11. **Build terminology.** “Native” puede generar expectativas de AOT/rendimiento
   que el AST empaquetado no cumple.
12. **Compatibilidad documental.** Features implementadas y referencias
   contradictorias erosionan confianza incluso cuando el código es correcto.

## 17. Simplification Opportunities

Orden recomendado, prefiriendo simplificar sobre añadir:

1. Declaraciones canónicas: `var` inferido, tipo explícito, `mixed`, `const`.
2. Un solo literal canónico de ausencia.
3. Una sola forma canónica de constructor.
4. Una regla uniforme de retorno para callables con nombre.
5. Truthiness basada en categorías coherentes o condiciones obligatoriamente bool.
6. Precedencia convencional para operadores conocidos.
7. `??` que sólo maneje ausencia, nunca excepciones arbitrarias.
8. Convención uniforme de errores de stdlib.
9. Perfiles de runtime: core, server y full, construidos desde el mismo catálogo.
10. Sistema modular explícito y ligero, preservando auto discovery como opción.
11. Etiquetas inequívocas: AST package, interpreter y experimental VM.
12. Eliminar aliases de APIs sólo mediante deprecation medible.

## 18. Evolution Opportunities

### Prioridad alta

- Completar metadata de nativos para que analyzer/LSP conozcan argumentos y retornos.
- Retornos explícitos y `void` para callables con nombre.
- Error model uniforme con una decisión concreta sobre `Result<T,E>`.
- Null state y exhaustividad como errores cuando el contexto requiere valor.
- Módulos/imports ligeros con IDs de símbolos estables.
- Concurrencia estructurada: cancelación, ownership de task y espera al cerrar scope.
- Tests diferenciales analyzer/interpreter para toda feature publicada.

### Prioridad media

- Usar `PreparedProgram.ResolvedCalls` y symbol IDs en más rutas del intérprete.
- Slots de campos/métodos para receptores nominales medidos con benchmarks.
- Debugger fuente y profiler accesibles desde CLI/editor.
- Matriz de integración de relaciones ORM en los cuatro motores.
- Declarar qué genericidad es estable y añadir constraints sólo si son necesarias.

### Prioridad baja

- Ampliar VM después de equivalencia completa.
- Optimizaciones como inline caches sólo con perfiles reales.
- Nuevos sugars sólo cuando eliminen un patrón frecuente y peligroso.

## 19. Things Joss Should NOT Add

- Borrow checker completo: el runtime Go ya aporta memory safety; no resuelve el
  problema principal de Joss.
- Multiple inheritance: aumenta resolución y ambigüedad sin necesidad demostrada.
- Overload general por firma: complica named arguments, mixed y tooling.
- Macros sintácticas: harían menos predecible un parser que ya tiene mucha superficie.
- Implicit conversions adicionales: debilitarían el analyzer.
- Más aliases de keywords/tipos/constructores: aumentan complejidad accidental.
- Extension methods y traits antes de estabilizar interfaces y módulos.
- Operator overloading general antes de restringir operadores actuales.
- Lazy loading ORM implícito: introduce I/O invisible y N+1.
- Reflection irrestricta como base del framework: impide resolución anticipada.
- Anotaciones/attributes generales sin un consumidor y contrato concretos.
- AOT/LLVM por prestigio antes de medir que el intérprete/VM sea el cuello de botella.
- Nuevas arquitecturas web obligatorias dentro del lenguaje.

## 20. Final Assessment

### Evaluación general

Joss es un lenguaje ambicioso con una base técnica mejor que la que su mezcla
sintáctica deja ver. Tiene analyzer real, no sólo lint; tipos que previenen bugs
reales; runtime con defensas; OOP suficiente; concurrencia útil; framework y ORM
productivos; y tooling que cubre el recorrido básico.

No lo recomendaría todavía como plataforma principal para un sistema crítico de
gran tamaño con varios equipos independientes. La razón no es rendimiento ni
falta de features. Es que módulos, errores, retornos, async y algunas semánticas
fundamentales todavía no son suficientemente uniformes para que el lenguaje sea
predecible bajo crecimiento y refactoring.

Sí lo consideraría viable para:

- APIs y servicios backend controlados;
- CRUD empresariales con límites conocidos;
- herramientas CLI;
- automatización y scripts con análisis previo;
- servicios pequeños donde el framework integrado reduce dependencias;
- experimentación seria con un lenguaje propio.

### Respuestas explícitas

1. **¿La sintaxis es coherente?** Parcialmente. Declaraciones y OOP sí; control
   condicional, `let`, precedencias y aliases no alcanzan la misma coherencia.
2. **¿Es fácil aprender Joss?** Es fácil comenzar. La sintaxis básica y el CLI
   permiten resultados rápidos.
3. **¿Es fácil dominarlo?** No todavía. Hay muchas reglas no deducibles y varias
   capas integradas que deben distinguirse.
4. **¿Es difícil escribir código incorrecto?** En código bien tipado, bastante
   más difícil que en PHP/Python. Con `mixed`, nativos incompletos y fallos
   silenciosos sigue siendo posible.
5. **¿Es fácil escribir código mantenible?** Sí en proyectos pequeños/medios con
   convenciones. En proyectos grandes, el espacio global y la falta de módulos
   explícitos son obstáculos.
6. **¿El sistema de tipos ayuda realmente?** Sí. Asignación fija, parámetros,
   nullability, flujo, colecciones y contratos nominales previenen bugs reales.
7. **¿`var`, `mixed` y explícitos están bien separados?** Conceptualmente sí.
   Sintácticamente `$x`, `var`, `let` y `mixed` ofrecen demasiadas variantes.
8. **¿Las clases están bien diseñadas?** El núcleo sí: visibilidad, interfaces,
   abstractos y records son sólidos. Constructores, override y generics requieren
   consolidación.
9. **¿Las funciones están bien diseñadas?** Frames, parámetros y llamadas sí.
   El retorno opcional reduce demasiado el valor del analyzer.
10. **¿Los métodos deberían declarar siempre retorno?** Todo callable con nombre,
    sí. Closures pequeñas pueden inferirlo. Se necesita un `void` claro.
11. **¿Está demasiado ligado a MVC?** El producto y CLI están fuertemente
    orientados a web/MVC; el núcleo del lenguaje no está obligado a MVC.
12. **¿MVC es obligatorio?** No. Es la convención privilegiada del framework.
13. **¿Sirve fuera de web?** Sí para CLI, scripts, workers, packages, plugins y
    SDK embebido; la madurez no es igual en todas esas áreas.
14. **¿Tiene identidad clara?** Como producto backend sí. Como lenguaje general,
    necesita reducir influencias y excepciones.
15. **¿Tiene demasiadas características?** Tiene más superficie de la que puede
    presentar con igual profundidad, especialmente VM/plugins/mobile/generics.
16. **¿Faltan garantías importantes?** Sí: módulos, retornos completos, errores
    uniformes, concurrencia estructurada y null state más exhaustivo.
17. **¿Qué simplificar?** Declaraciones, constructores, ausencia, truthiness,
    errores, precedencias y modelos de composición.
18. **¿Qué fortalecer?** Analyzer en fronteras nativas, módulos, async,
    debugging, metadata, symbol IDs y tests diferenciales.
19. **¿Qué no añadir?** Overloads, multiple inheritance, macros, traits,
    reflection amplia, borrow checking o más azúcar antes de estabilizar lo actual.
20. **¿Prioridad antes de nuevas features?** Semántica predecible, módulos,
    retornos, errores, concurrencia estructurada y documentación coincidente.

### Veredicto

La forma más fácil de escribir Joss suele ser segura cuando el código permanece
en el carril tipado y usa APIs con metadata completa. Fuera de ese carril, el
lenguaje todavía exige conocer demasiadas excepciones. La evolución correcta no
consiste en ampliar su catálogo: consiste en hacer que un desarrollador pueda
predecir el comportamiento a partir de pocas reglas y que el analyzer conserve
información hasta cada frontera real del runtime.

## Apéndice A — Evidencia principal del repositorio

| Afirmación | Evidencia primaria |
|---|---|
| Keywords, símbolos y sintaxis retirada | `pkg/parser/token.go` |
| Parser Pratt y precedencia | `pkg/parser/parser.go`, `parser_expressions.go`, `parser_statements.go` |
| AST de funciones, clases, records, guard y match | `pkg/parser/ast_expressions.go`, `ast_statements.go` |
| Tipos, uniones, colecciones y assignability | `pkg/typesystem/types.go` |
| Pipeline multiparchivo | `pkg/analyzer/analyzer.go`, `declaration_collection.go`, `body_analysis.go` |
| Asignación definida y exhaustividad | `pkg/analyzer/definite_assignment_test.go`, `flow.go` |
| Resolución de llamadas y miembros | `pkg/analyzer/call_resolution.go`, `member_resolution.go` |
| Contratos nominales | `pkg/analyzer/nominal_contracts.go` y sus tests |
| Facts, symbol IDs y programa preparado | `pkg/analyzer/prepared_program.go` |
| Frames, llamadas y closures | `pkg/core/call_method.go`, `frame_runtime.go`, `closure.go` |
| Semántica de operadores | `pkg/core/evaluator_infix.go`, `evaluator_numeric.go`, `evaluator_compare.go` |
| Async, futures y canales | `pkg/core/builtins_async.go`, `channel.go`, `evaluator_select.go` |
| Lifecycle y concurrencia del runtime | `pkg/core/runtime_lifecycle.go`, `runtime.go`, tests de safety/stabilization |
| Clases y metadata | `pkg/core/class_metadata.go`, tests OOP/modern syntax |
| Framework HTTP | `pkg/server`, `pkg/core/router.go`, `response_writer.go`, `request_data.go` |
| ORM | `pkg/core/model.go`, `database_*.go`, tests y benchmarks de Model/GranDB |
| CLI y composición del proyecto | `cmd/joss/main.go`, `pkg/analyzer/project.go` |
| Formatter, linter y tests | `pkg/formatter`, `pkg/linter`, `pkg/tester` |
| LSP | `vscode-joss/src/server` y catálogo generado |
| AST empaquetado y VM experimental | `pkg/bytecode`, `pkg/vm`, `cmd/joss/build.go` |

## Apéndice B — Alcance y confianza

Esta revisión no mide adopción externa, estabilidad de una release publicada ni
calidad de servicios alojados. Evalúa lo presente en el checkout: implementación,
tests y herramientas. Las afirmaciones de seguridad se limitan al contrato
demostrable; no convierten APIs de red, DB, FFI o procesos en operaciones libres
de fallos. La comparación con otros lenguajes valora modelos técnicos y madurez,
no popularidad.
