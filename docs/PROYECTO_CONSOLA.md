# Proyecto práctico: Aplicación de consola con persistencia JSON

Antes: [Concurrencia y canales](CONCURRENCIA.md). Después: [Proyecto web completo](PROYECTO_WEB.md).
Referencia técnica: [Línea de comandos (CLI)](CLI.md), [Estructura de proyectos](ESTRUCTURA_PROYECTO.md).

---

## ¿Qué vas a construir aquí?

Una **aplicación de consola** (CLI) es un programa que se ejecuta directamente en la terminal sin interfaz gráfica. Es el tipo de software utilizado en automatizaciones, procesamiento de datos por lotes (*batch processing*), scripts de mantenimiento de servidores y utilidades para desarrolladores.

En este tutorial guiado construiremos un **gestor de compras e inventario**:
1. Estructura una lista de artículos con cantidades y precios decimales exactos.
2. Guarda los datos en un archivo físico en el disco (`compras.json`) en formato JSON estructurado.
3. Lee el archivo del disco, valida su integridad sintáctica (`json_verify`) y reconstruye las estructuras de datos en memoria (`json_decode`).
4. Procesa y totaliza los importes con precisión decimal financiera usando funciones modulares.
5. Gestiona posibles fallos de lectura o escritura de forma robusta.

---

## 1. Preparar el entorno de trabajo

Abre tu terminal y crea un directorio limpio para este proyecto:

```bash
mkdir proyecto-compras
cd proyecto-compras
```

Abre tu editor y crea un archivo llamado `main.joss`.

---

## 2. El código completo del programa

Escribe o copia el siguiente programa completo dentro de `main.joss`:

<!-- joss-run: ["Articulos: 2", "Total: 42.5", "Archivo guardado"] -->
```joss
public func totalizar(array $compras): decimal {
    decimal $total = 0m
    foreach ($compras as $compra) {
        $total = $total + decimal($compra["precio"]) * intval($compra["cantidad"])
    }
    return $total
}

$compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]

$guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }

$texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }

$leidas = json_decode($texto)
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
    return 0m
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
$compras = [
    {"nombre": "Cuaderno", "precio": "12.50", "cantidad": 2},
    {"nombre": "Lapiz", "precio": "3.50", "cantidad": 5}
]
```
- Definimos un array cuyos elementos son mapas asociativos (`{"clave": valor}`).
- Guardar los precios como texto (`"12.50"`) dentro del JSON es una buena práctica contable: evita que los decodificadores JSON estándar introduzcan imprecisiones binarias al leer números flotantes.

### 3. Persistencia en disco con JSON
```joss
$guardado = file_put_contents("compras.json", json_encode($compras))
$guardado ? {} : { throw "No se pudo guardar compras.json" }
```
- `json_encode($compras)`: Transforma la estructura en memoria de Joss a un texto estándar JSON.
- `file_put_contents("compras.json", ...)`: Escribe ese texto en el archivo físico en el disco duro. Devuelve `true` si tuvo éxito o `false` si falló por permisos o falta de espacio.
- La expresión ternaria actúa como salvaguarda: si `$guardado` es falso, lanza un error con `throw`.

### 4. Lectura y validación de seguridad
```joss
$texto = file_get_contents("compras.json")
$texto == null ? { throw "No se pudo leer compras.json" } : {}
json_verify($texto) ? {} : { throw "El archivo no contiene JSON valido" }
```
- `file_get_contents(...)`: Recupera los bytes del archivo en una cadena de texto. Si el archivo no existe, devuelve `null`.
- `json_verify($texto)`: Función nativa de Joss que comprueba si una cadena cumple con la especificación JSON válida sin llegar a parsear todo el árbol a memoria. Si el archivo está corrupto o fue editado incorrectamente por un usuario, lo detecta de inmediato.
- `json_decode($texto)`: Reconstruye los datos JSON en arrays y maps nativos de Joss listos para ser procesados.

---

## 4. Análisis y Ejecución

Primero, ejecutemos el análisis estático para garantizar que no haya inconsistencias:

```bash
joss analyze main.joss
```

Si todo es correcto, ejecuta el programa:

```bash
joss run main.joss
```

Salida producida en consola:

```text
Articulos: 2
Total: 42.5
Archivo guardado
```

Si revisas tu carpeta con `ls` o el explorador de archivos, verás que se ha creado el archivo físico `compras.json`. Si lo abres, verás:

```json
[{"cantidad":2,"nombre":"Cuaderno","precio":"12.50"},{"cantidad":5,"nombre":"Lapiz","precio":"3.50"}]
```

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

## 6. Ejercicios para expandir el proyecto

1. **Añadir artículos dinámicamente**:
   - Modifica el programa para pedirle al usuario el nombre, precio y cantidad del siguiente producto usando `cin >> $nombre`.
   - Agrégalo al array con `$compras[] = ...` antes de guardar el archivo.
2. **Filtrar artículos caros**:
   - Crea una función `public func articulosCaros(array $compras, decimal $umbral): array` que retorne un nuevo array con solo aquellos productos cuyo precio supere el umbral.
3. **Colorear el reporte final**:
   - Usa `Console::green(...)` para mostrar `"Archivo guardado"` y `Console::bold(...)` para el total.

---

## Siguiente paso

Ahora que has dominado la persistencia de datos en disco y el desarrollo de utilidades de consola, es hora de explorar el área más fuerte de Joss: el desarrollo de aplicaciones web de alto rendimiento con rutas, controladores, bases de datos y vistas HTML.

Continúa con: [Construir una aplicación web completa con el stack nativo](PROYECTO_WEB.md).
