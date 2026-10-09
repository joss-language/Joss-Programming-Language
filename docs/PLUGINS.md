# Plugins y paquetes

[Índice](README.md) · Antes: [estructura](ESTRUCTURA_PROYECTO.md) · Después: [contribuir](CONTRIBUIR.md)

Un **paquete** reúne archivos y metadatos para distribuir una capacidad.
Un **plugin** incorpora funciones, clases o comandos a una aplicación. Joss
carga sus paquetes `.jp` automáticamente; no se escribe un import en el programa.

## Crear un plugin Joss

Con el CLI instalado, en una carpeta de trabajo:

```sh
joss new plugin calculadora
cd calculadora
joss plugin compile .
```

La plantilla contiene `joss.yaml`, `src/plugin.joss` y un workflow de
publicación. Lee el manifiesto generado antes de cambiar nombres o exports.
La variante `joss new package calculadora` produce un paquete más pequeño;
se construye con `joss build package .`. Las pruebas
`TestNewPackageAndPluginTemplatesCompileEndToEnd` construyen ambas variantes,
verifican la firma y decodifican el contenido.

```sh
joss plugin inspect calculadora.jp
joss plugin verify calculadora.jp
```

`inspect` permite descubrir nombres, exports, permisos y símbolos.
`verify` comprueba integridad y firma Ed25519. La clave pública incluida en
un archivo demuestra consistencia con su firma; por sí sola **no establece
que el editor sea alguien de confianza**.

## Instalar y utilizar

```sh
joss pub add nombre_del_paquete ^1.0.0
joss pub install
```

Sustituye el nombre y la versión por un paquete existente en el registro
configurado. `pub` mantiene dependencias; no agrega sintaxis fuente. Consulta
[CLI](CLI.md) y la documentación del paquete para su API concreta.

Los paquetes declarados en `joss.yaml` o presentes en `plugins/` se descubren
al preparar el runtime. Su `SymbolIndex` publica parámetros y retornos para
el analizador. Un paquete antiguo sin retorno publicado aporta `unknown`:
significa falta de información, no una garantía de compatibilidad.

Ejemplo de integración **dependiente de un plugin que exporte estos símbolos**:

```joss
$resultado = calculadora::sumar(2, 3)
```

No copies ese nombre sin comprobarlo con `inspect`. Una función exportada
también puede estar disponible por su nombre directo; las clases exportadas
se instancian con `new`. No existen namespaces fuente ni módulos con imports.

## Qué contiene realmente un JP

El contenedor firmado guarda metadatos, archivos, índice de símbolos y una
entrada de bytecode. El runtime detecta dos formatos distintos:

| Contenido | Ejecutor | Alcance |
|---|---|---|
| `JOSSBC2Z` | Adaptador AST de `pkg/core` | Árbol Joss serializado y comprimido, interpretado. |
| `JPBC` | `pkg/pluginruntime.JPBCVM` | Máquina de instrucciones específica de plugins. |

Ninguno de estos formatos de plugin sustituye la compilación nativa del programa principal realizada por `joss build` (que utiliza Joss Native IR y el backend nativo).
La VM experimental de `pkg/vm` es un componente independiente y no es el ejecutor
predeterminado de `joss run` ni de `joss build`.

## Compilación desde otros lenguajes: estado y límites

`joss plugin compile archivo --lang=... --name=... --exports=...` selecciona
un backend de `pkg/plugincompiler`. Que un nombre sea aceptado por el CLI
no significa que todo ese lenguaje esté implementado.

| Entrada | Implementación actual |
|---|---|
| Joss / proyecto generado | Empaquetado del AST de Joss. |
| Python | Traductor de un subconjunto de expresiones y funciones a IR/JPBC; no ejecuta CPython ni incorpora todo su ecosistema. |
| PHP | Traductor parcial a IR/JPBC; no equivale a un runtime PHP. |
| Java / Kotlin | Lectura de `.class` o `.jar` y traducción parcial; no ofrece toda la JVM. |
| Rust, C, C++, Dart, Flutter, Wasm | El backend comprueba la cabecera `\\0asm` y genera por export una función que retorna un texto de demostración. **No interpreta ni traduce las instrucciones Wasm.** |

Por tanto no uses la ruta Wasm para cifrar, transformar archivos ni ejecutar
una biblioteca real. Un paquete firmado producido por esa ruta puede ser
estructuralmente válido y aun así no implementar la operación solicitada.

La optimización de plugins elimina funciones no alcanzables desde exports
según el IR construido. No demuestra equivalencia con cualquier programa
del lenguaje de origen. El límite de tamaño configurado genera una advertencia,
no una garantía de tamaño máximo.

## Permisos y aislamiento

`PermissionGuard` comprueba llamadas al host que están mapeadas:

| Operación del host | Permiso |
|---|---|
| `http_get`, `http_post`, `fetch` | `network.http` |
| `file_read`, `file_write` | `filesystem.read`, `filesystem.write` |
| `env_read`, `env_write` | `env.read`, `env.write` |
| `db_query`, `db_exec` | `database.query`, `database.exec` |

Comprueba la tabla de `pkg/pluginruntime/jpbc_vm.go` al extender el host.
Los permisos declarados se conceden al construir el guard; no hay un diálogo
de aprobación del usuario. Los comodines amplían permisos por prefijo y una
revocación exacta no anula un comodín concedido.

Esto **no es un sandbox WASI ni aislamiento de proceso o del sistema operativo**.
La comprobación se limita a las operaciones integradas en esa frontera. Los
drivers ABI C, procesos externos y el ejecutor AST tienen mecanismos distintos.
El presupuesto JPBC de instrucciones se aplica por invocación de función;
no debe anunciarse como límite global de recursos de toda una aplicación.

## Puentes y ciclo de vida

`Plugin::call`, `stream`, `path` y `platform` conectan formatos de plugin;
`System::load_driver` y `driver_call` cargan bibliotecas ABI C v1 específicas
de plataforma. Sus contratos están en el [catálogo](CATALOGO_NATIVO.md) y
la [referencia nativa](MODULOS_NATIVOS.md).

Un fork del runtime comparte el registro de plugins y otros recursos.
Al liberar una instancia del pool, `Runtime.Free()` limpia también
`PluginRegistry`; conservar el registro y borrar sólo clases/símbolos rompe
la siguiente petición. Los plugins no deben asumir que variables de una
petición anterior siguen disponibles.

Fuentes: [compilador](../pkg/plugincompiler/plugincompiler.go),
[backend Wasm](../pkg/plugincompiler/backends/nativewasm/nativewasm_backend.go),
[runtime](../pkg/pluginruntime/), [contenedor](../pkg/pluginpkg/).
