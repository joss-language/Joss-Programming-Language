# ============================================================
# Joss Web — Dockerfile (Debian Minimal + Joss Release)
# ============================================================

FROM debian:bookworm-slim

# Dependencias mínimas del sistema
RUN apt-get update && apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
        unzip \
        jq \
    && rm -rf /var/lib/apt/lists/*

# Descargar e instalar la versión release oficial de Joss CLI
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    releases_url="https://api.github.com/repos/joss-language/Joss-Programming-Language/releases"; \
    rel_json="$(curl -fsSL "${releases_url}" | jq -c '.[] | select(.draft == false)' | head -n 1)"; \
    asset_url="$(printf '%s' "${rel_json}" | jq -r ".assets[] | select(.name | contains(\"${arch}\")) | .browser_download_url" | head -n 1)"; \
    if [ -z "${asset_url}" ] || [ "${asset_url}" = "null" ]; then \
        asset_url="$(printf '%s' "${rel_json}" | jq -r '.assets[] | select(.name | contains("linux")) | .browser_download_url' | head -n 1)"; \
    fi; \
    echo "Descargando Joss CLI para ${arch} desde: ${asset_url}"; \
    curl -fsSL "${asset_url}" -o /tmp/joss_pkg.zip; \
    unzip -q /tmp/joss_pkg.zip -d /tmp/joss_out; \
    mv /tmp/joss_out/joss* /usr/local/bin/joss || mv /tmp/joss_out/*/joss* /usr/local/bin/joss; \
    rm -rf /tmp/joss_pkg.zip /tmp/joss_out; \
    chmod +x /usr/local/bin/joss; \
    joss version

WORKDIR /app

# Copiar el código del proyecto
COPY . .

# Instalar dependencias/plugins si joss.yaml las declara
RUN joss pub install || true

RUN chmod +x /app/entrypoint.sh

EXPOSE 80

ENTRYPOINT ["/app/entrypoint.sh"]
