#!/bin/sh
set -e

ENV_FILE="/app/env.joss"
mkdir -p /app/storage

echo "[entrypoint] Generando env.joss..."
rm -f "$ENV_FILE"
touch "$ENV_FILE"

for var in $(env | cut -d= -f1); do
    case "$var" in
        PATH|HOME|HOSTNAME|TERM|SHLVL|PWD|_|OLDPWD|DEBIAN_FRONTEND|LANG)
            continue
            ;;
        *)
            val=$(eval echo "\$$var" | sed 's/"/\\"/g')
            echo "${var}=\"${val}\"" >> "$ENV_FILE"
            ;;
    esac
done

echo "[entrypoint] ✓ env.joss generado."
echo "[entrypoint] Iniciando servidor web Joss..."
exec joss server start
