# Instalación oficial

Los instaladores remotos descargan dos artefactos del último release: el runtime de la plataforma y `jossecurity-vscode.zip`. El SDK multilenguaje legacy ya no forma parte de la distribución.

## Windows

Ejecuta PowerShell como administrador:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope Process; iwr -useb https://joss.red/install | iex
```

El runtime se instala en `C:\Program Files\JosSecurity`. El instalador añade esa carpeta al `PATH`. Si `code` está disponible, también instala el VSIX; si VS Code no existe, ofrece instalarlo con Winget. Al instalar, actualizar o desinstalar también limpia un directorio `sdk` dejado por versiones antiguas.

## Linux y macOS

```bash
curl -fsSL https://joss.red/install.sh | bash
```

El runtime queda en `/usr/local/bin/joss`. Si se ejecuta como `root`, realiza las operaciones directamente; para usuarios normales utiliza `sudo` cuando está disponible. Si `code` está en el `PATH`, también instala el VSIX. Al instalar, actualizar o desinstalar también limpia `/usr/local/share/joss/sdk` cuando fue creado por una versión antigua.

Ambos scripts muestran un menú para instalar, buscar una actualización o desinstalar. La descarga requiere `curl` y `unzip` en Linux/macOS; en Windows requiere PowerShell 5.1 o posterior.

Verificación:

```bash
joss version
```

Logs: `%TEMP%\jossecurity-action.log` en Windows y `/tmp/jossecurity-action.log` en Linux/macOS.

## Reinstalar una versión

Selecciona **Reinstall** en el menú. Puedes escribir una versión concreta, por ejemplo `3.6.7.2`, o dejarla vacía para descargar e instalar nuevamente el release más reciente aunque esa misma versión ya esté instalada. La reinstalación reemplaza el runtime y la extensión de VS Code.
