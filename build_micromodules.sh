#!/bin/bash
set -e
echo "Construyendo micromódulos estáticos para Joss Native ABI..."
mkdir -p pkg/backend/native/abi/build/
# Construir libjoss_base.a
go build -buildmode=c-archive -o pkg/backend/native/abi/build/libjoss_base.a pkg/backend/native/abi/host_bridge.go
echo "Micromódulos generados exitosamente en pkg/backend/native/abi/build/"
