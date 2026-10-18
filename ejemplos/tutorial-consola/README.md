# Proyecto de Consola Joss

Este es un proyecto backend-only de Joss, diseñado para aplicaciones de línea de comandos.

## Estructura del Proyecto

```
/
├── main.joss              # Punto de entrada de la aplicación
├── env.joss               # Variables de entorno
├── config/
│   └── reglas.joss        # Constantes globales
├── app/
│   ├── controllers/       # Lógica de negocio
│   ├── models/            # Acceso a datos
│   ├── libs/              # Librerías personalizadas
│   └── database/
│       └── migrations/    # Migraciones de base de datos
└── README.md              # Este archivo
```

## Ejecutar la Aplicación

```bash
joss run main.joss
```

## Comandos Útiles

```bash
# Ejecutar migraciones
joss migrate

# Crear un nuevo controlador
joss make:controller MiControlador

# Crear un nuevo modelo
joss make:model MiModelo
```

## Desarrollo

1. Edita `main.joss` para agregar tu lógica principal
2. Crea controladores en `app/controllers/`
3. Crea modelos en `app/models/`
4. Configura variables de entorno en `env.joss`

## Base de Datos

Por defecto, este proyecto usa SQLite. Para cambiar a MySQL:

1. Edita `env.joss` y cambia `DB="mysql"`
2. Configura las credenciales de MySQL
3. Ejecuta `joss migrate` para crear las tablas

## Notas

Este es un proyecto de **consola** (backend-only). No incluye:
- Rutas web (`routes.joss`)
- API REST (`api.joss`)
- Vistas HTML (`app/views/`)
- Assets estáticos (`assets/`)

Si necesitas un proyecto web completo, usa:
```bash
joss new web mi_proyecto_web
```
