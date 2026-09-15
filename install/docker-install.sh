#!/bin/bash
# -----------------------------------------------------------
# JosSecurity Docker Installer (Non-Interactive)
# Uso en Dockerfile:
#   RUN curl -fsSL https://raw.githubusercontent.com/joss-language/Joss-Programming-Language/main/install/docker-install.sh | bash
#
# Diferencias con remote-install.sh:
#   - Sin menú interactivo (no necesita TTY)
#   - No instala extensión de VS Code
#   - Falla rápido (set -e) — rompe el docker build si algo falla
# -----------------------------------------------------------

set -e

REPO_OWNER="joss-language"
REPO_NAME="Joss-Programming-Language"
JOSS_VERSION="${JOSS_VERSION:-latest}"
INSTALL_DIR="/usr/local/bin"
TEMP_DIR="/tmp/jossecurity-docker-install"

# --- Detectar binario según arquitectura ---
get_binary_name() {
    local ARCH
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  echo "joss-linux-amd64"  ;;
        aarch64) echo "joss-linux-arm64"  ;;
        armv7l)  echo "joss-linux-armv7"  ;;
        *)
            echo "[docker-install] ERROR: Arquitectura no soportada: $ARCH" >&2
            exit 1
            ;;
    esac
}

# --- Descarga e instalación ---
echo "[docker-install] Preparando instalación de JosSecurity..."
rm -rf "$TEMP_DIR"
mkdir -p "$TEMP_DIR"

BINARY_FILE=$(get_binary_name)
ZIP_FILE="jossecurity-linux.zip"
if [ "$JOSS_VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/latest/download/$ZIP_FILE"
else
    DOWNLOAD_URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/V$JOSS_VERSION/$ZIP_FILE"
fi

echo "[docker-install] Descargando $ZIP_FILE..."
curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_DIR/$ZIP_FILE"

echo "[docker-install] Extrayendo..."
unzip -o "$TEMP_DIR/$ZIP_FILE" -d "$TEMP_DIR"

BINARY_PATH=$(find "$TEMP_DIR" -name "$BINARY_FILE" -type f | head -n 1)
if [ -z "$BINARY_PATH" ]; then
    echo "[docker-install] ERROR: Binario '$BINARY_FILE' no encontrado en el ZIP." >&2
    exit 1
fi

echo "[docker-install] Instalando '$BINARY_FILE' en $INSTALL_DIR/joss..."
cp "$BINARY_PATH" "$INSTALL_DIR/joss"
chmod +x "$INSTALL_DIR/joss"

# --- Limpieza ---
rm -rf "$TEMP_DIR"

echo "[docker-install] ✓ JosSecurity instalado correctamente."
joss version
