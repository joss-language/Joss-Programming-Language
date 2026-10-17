# Proyecto práctico: Aplicación de consola con persistencia JSON

Antes: [Concurrencia y canales](CONCURRENCIA.md). Después: [Proyecto web completo](PROYECTO_WEB.md).
Referencia técnica: [Línea de comandos (CLI)](CLI.md), [Estructura de proyectos](ESTRUCTURA_PROYECTO.md).

---

## ¿Qué vas a construir aquí?

Una **aplicación de consola** (CLI) es un programa que se ejecuta directamente en la terminal sin interfaz gráfica. Es el tipo de software utilizado en automatizaciones, procesamiento de datos por lotes (*batch processing*), scripts de mantenimiento de servidores y utilidades para desarrolladores.

En este tutorial guiado construiremos un **gestor de compras e inventario**:
1. Crear la estructura oficial del proyecto con `joss new console`.
2. Estructurar una lista de artículos con cantidades y precios decimales exactos.
3. Guardar los datos en un archivo físico en el disco (`compras.json`) en formato JSON estructurado.
4. Leer el archivo del disco, validar su integridad sintáctica (`json_verify`) y reconstruir las estructuras de datos en memoria (`json_decode`).
5. Procesar y totalizar los importes con precisión decimal financiera usando funciones modulares.
6. Compilar el proyecto a un ejecutable nativo independiente con `joss build`.

---

## 1. Preparar el entorno de trabajo con `joss new console`

Abre tu terminal y genera un proyecto de consola oficial con el CLI de Joss:

```bash
joss new console proyecto-compras
cd proyecto-compras
```

Joss habrá creado la estructura base del proyecto (incluyendo `main.joss`, `env.joss`, `joss.yaml` y la carpeta `app/`). Abre `main.joss` en tu editor para escribir la lógica de nuestra aplicación.

---

## 2. El código completo del programa

Reemplaza el contenido de `main.joss` por el siguiente programa completo:

<!-- joss-run: ["Articulos: 2", "Total: 42.5", "Archivo guardado"] -->
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}

var $compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]

var $guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }

var $texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }

var $leidas = json_decode($texto)
print("Articulos: " . count($leidas))
print("Total: " . totalizar($leidas))
print("Archivo guardado")
```

---

## 3. Explicación paso a paso de la arquitectura

Analicemos cómo interactúan los distintos subsistemas del lenguaje en este programa:

### 1. La función de cálculo (`totalizar`)
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}
```
- Recibe un `array` de compras y promete retornar un valor de tipo `decimal`.
- Inicializa un acumulador exacto: `decimal $total = 0m`.
- Recorre cada elemento con `foreach ($compras as $compra)`.
- Extrae `"precio"` y `"cantidad"` usando claves de mapa. Observa cómo convertimos explícitamente:
  - `decimal($compra["precio"])`: Convierte el texto numérico a decimal de coma fija.
  - `intval($compra["cantidad"])`: Convierte la cantidad a entero de 64 bits.
- Multiplica ambos valores y los suma a `$total`.

### 2. Estructura de datos en memoria
```joss
var $compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]
```
- Definimos un array cuyos elementos son mapas asociativos (`{"clave": valor}`).
- Guardar los precios como texto (`"12.50"`) dentro del JSON es una buena práctica contable: evita que los decodificadores JSON estándar introduzcan imprecisiones binarias al leer números flotantes.

### 3. Persistencia en disco con JSON
```joss
var $guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }
```
- `json_encode($compras)`: Transforma la estructura en memoria de Joss a un texto estándar JSON.
- `file_put_contents("compras.json", ...)`: Escribe ese texto en el archivo físico en el disco duro. Devuelve `true` si tuvo éxito o `false` si falló por permisos o falta de espacio.
- La expresión ternaria actúa como salvaguarda: si `$guardado` es falso, lanza un error con `throw`.

### 4. Lectura y validación de seguridad
```joss
var $texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }
```
- `file_get_contents(...)`: Recupera los bytes del archivo en una cadena de texto. Si el archivo no existe, devuelve `null`.
- `json_verify($texto)`: Función nativa de Joss que comprueba si una cadena cumple con la especificación JSON válida sin llegar a parsear todo el árbol a memoria. Si el archivo está corrupto o fue editado incorrectamente por un usuario, lo detecta de inmediato.
- `json_decode($texto)`: Reconstruye los datos JSON en arrays y maps nativos de Joss listos para ser procesados.

---

## 4. Análisis, Ejecución y Compilación Nativa

Primero, ejecutemos el análisis estático para garantizar que no haya inconsistencias:

```bash
joss analyze main.joss
```

Si todo es correcto, ejecuta el programa con el intérprete:

```bash
joss run main.joss
```

Salida producida en consola:

```text
Articulos: 2
Total: 42.5
Archivo guardado
```

Si revisas tu carpeta con `ls` (o `dir` en Windows) o el explorador de archivos, verás que se ha creado el archivo físico `compras.json`:

```json
[{"cantidad":2,"nombre":"Cuaderno","precio":"12.50"},{"cantidad":5,"nombre":"Lapiz","precio":"3.50"}]
```

### Compilar a un binario nativo independiente (`joss build`)

Cuando quieras distribuir tu herramienta de consola a otros equipos sin que necesiten tener instalado Joss ni entregar tu código fuente, compílala con:

```bash
joss build main.joss
```

Joss generará un ejecutable nativo dentro de la carpeta `build/` (`build/main.exe` en Windows o `./build/main` en Linux/macOS) listo para ejecutarse directamente.

---

## 5. Salida visual con colores: El módulo nativo `Console`

Para que tu aplicación de consola ofrezca una experiencia visual atractiva y profesional, puedes colorear y destacar los mensajes en la terminal mediante la clase nativa **`Console`**:

```joss
print(Console::green("✓ Archivo guardado con éxito"))
print(Console::yellow("⚠ Advertencia: El stock es bajo"))
print(Console::red("✗ Error al procesar datos"))
print(Console::bold("Total a pagar: S/ 42.50"))
```

### Métodos disponibles de `Console`:

| Método | Propósito y Color | Uso típico |
|---|---|---|
| `Console::green($t)` | Verde | Operaciones exitosas, confirmaciones (`✓ OK`). |
| `Console::red($t)` | Rojo | Errores, fallos de validación, excepciones. |
| `Console::yellow($t)` | Amarillo | Advertencias, avisos que requieren atención. |
| `Console::blue($t)` / `Console::cyan($t)` | Azul / Cian | Títulos, enlaces, información descriptiva. |
| `Console::bold($t)` | Negrita | Totales numéricos, nombres destacados. |
| `Console::clear()` | Limpiar pantalla | Reinicia la terminal antes de mostrar un menú. |

---

## 6. Ejercicios prácticos

### Nivel 1 — Prueba rápida
Agrega un tercer artículo al array `$compras` (por ejemplo, `{"nombre": "Mochila", "precio": "35.00", "cantidad": 1}`), vuelve a ejecutar `joss run main.joss` y verifica que el total se actualice a `77.5` y que `compras.json` refleje los 3 artículos.

### Nivel 2 — Ejercicio guiado
1. Crea una función `public func articulosCaros(array $compras, decimal $umbral): array` que recorra `$compras` con `foreach` y retorne un nuevo array solo con los productos cuyo `decimal($compra["precio"])` supere `$umbral`.
2. Imprime cuántos artículos superan `10.00m` usando `count(articulosCaros($leidas, 10.00m))`.
3. Colorea el mensaje final `"Archivo guardado"` usando `Console::green("Archivo guardado")`.

### Nivel 3 — Reto
Modifica el programa para que antes de guardar el archivo le pida al usuario un artículo adicional por teclado:
1. Declara `string $nuevoNombre = ""`, `string $nuevoPrecio = ""` y `int $nuevaCantidad = 0`.
2. Pide cada dato usando `cin >> $nuevoNombre`, `cin >> $nuevoPrecio` y `cin >> $nuevaCantidad`.
3. Agrégalo a `$compras` con `$compras[] = {"nombre": $nuevoNombre, "precio": $nuevoPrecio, "cantidad": $nuevaCantidad}`, guarda el JSON y compila tu aplicación con `joss build main.joss`.

---

## Siguiente paso

Ahora que has dominado la persistencia de datos en disco y el desarrollo de utilidades de consola, es hora de explorar el área más fuerte de Joss: el desarrollo de aplicaciones web de alto rendimiento con rutas, controladores, bases de datos y vistas HTML.

Continúa con: [Construir una aplicación web completa con el stack nativo](PROYECTO_WEB.md).
