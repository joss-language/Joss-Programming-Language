# Comandos rápidos

## Instalar o actualizar

Windows, desde PowerShell como administrador:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope Process; iwr -useb https://joss.red/install | iex
```

Linux o macOS:

```bash
curl -fsSL https://joss.red/install.sh | bash
```

Los scripts abren un menú para instalar, actualizar o desinstalar el runtime, el SDK y, cuando VS Code está disponible, la extensión.

## Verificar y empezar

```bash
joss version
joss new mi_aplicacion
cd mi_aplicacion
joss server start
```

Consulta los detalles y requisitos en [install/README.md](install/README.md).

La opción **Reinstall** permite indicar una versión concreta como `3.6.1`; si se deja vacía, reinstala el último release disponible.
