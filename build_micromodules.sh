#!/bin/bash
set -e
echo "Construyendo micromódulos estáticos para Joss Native ABI..."
mkdir -p pkg/backend/native/abi/build/

# Nota Arquitectónica: La compilación cruzada de C-Archives requiere
# toolchains de C (MinGW para Windows, gcc-arm-linux-gnueabihf para ARM, etc.)
# Por defecto compilaremos el HostBridge para la plataforma actual del desarrollador.
# La integración continua (CI) de GitHub ejecutará esto nativamente en sus runners de Windows/Mac/Linux.

echo "Generando C-Archive HostBridge para $(go env GOOS) $(go env GOARCH)..."
CGO_ENABLED=1 go build -buildmode=c-archive -o pkg/backend/native/abi/build/libjoss_base_$(go env GOOS)_$(go env GOARCH).a pkg/backend/native/abi/host_bridge.go

echo "Micromódulo generado exitosamente en pkg/backend/native/abi/build/"
