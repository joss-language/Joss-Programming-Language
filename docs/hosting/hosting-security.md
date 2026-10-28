# Modelo de Seguridad y Aislamiento: Joss Free Hosting

---

## 1. Filosofía de Confianza Cero (Zero Trust Tenant)

> **Regla de Oro de Seguridad**: Todo código ejecutado en la plataforma multi-tenant se considera **activo, inteligente y hostil por defecto**.

El sistema de seguridad no confía en:
1. El compilador o el empaquetador del cliente.
2. La validez sintáctica del lenguaje Joss.
3. El checksum o la firma del cliente.

La seguridad reside única y exclusivamente en los **mecanismos de contención del Kernel de Linux y la capa de virtualización de syscalls**.

---

## 2. Los Cuatro Pilares del Sandbox de Aislamiento

```text
┌────────────────────────────────────────────────────────┐
│                   APLICACIÓN TENANT                    │
└──────────────────────────┬─────────────────────────────┘
                           │ Syscalls (read, write, fork, socket)
                           ▼
┌────────────────────────────────────────────────────────┐
│ 1. gVisor (runsc) — Virtual Sentry Kernel              │
│    Intercepta y emula 300+ syscalls en user-space      │
│    Impide acceso directo a ring-0 del Kernel Linux     │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ 2. cgroups v2 — Restricción Física de Recursos         │
│    memory.max: 96M (OOM Kill inmediato)               │
│    cpu.max: 25000 100000 (25% de 1 vCPU)               │
│    pids.max: 32 (Inmunidad contra Fork-Bombs)          │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ 3. Network Isolation — Firewall de Egreso              │
│    DROP hacia 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16│
│    DROP hacia 169.254.169.254 (Cloud Instance Metadata) │
│    DROP hacia puerto 25/465/587 (Anti-Spam SMTP)       │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│ 4. Storage Jail — Inmutabilidad y Cuota                │
│    Root FS: Read-Only                                  │
│    /tmp: Tmpfs RAM limitado a 16 MB                    │
│    /storage: Montaje único con cuota XFS/ext4 de 64 MB │
│    Opciones de montaje: nosuid, nodev, noexec          │
└────────────────────────────────────────────────────────┘
```

---

## 3. Vectores de Ataque y Mitigaciones Específicas

### 3.1 Filesystem Escape
- **Intento de Ataque**: Un script Joss intenta leer `/etc/passwd`, `/etc/shadow`, `/proc/1/environ` o archivos de otros tenants usando `File::read("../../../etc/passwd")`.
- **Defensa en Profundidad**:
  - `gVisor` virtualiza el filesystem mediante un proxy `gofer` aislado.
  - La raíz `/` del contenedor es de solo lectura y contiene únicamente el entorno mínimo Debian/Alpine sin shells (`sh` o `bash` eliminados en la imagen de producción).
  - Los archivos de otros tenants residen en volúmenes independientes en el host inaccesibles para el namespace del contenedor.

### 3.2 Process Escape (`System::Run` / Shell Injections)
- **Intento de Ataque**: El atacante usa `System::Run("sh", ["-c", "cat /etc/shadow"])` o intenta desplegar un binario compilado en C.
- **Defensa en Profundidad**:
  - La imagen base de ejecución **no contiene binarios de shell** (`/bin/sh`, `/bin/bash`, `curl`, `wget`, `python` purgados).
  - El volumen persistente `/app/storage` y `/tmp` están montados con la flag de kernel **`noexec`**, impidiendo que un atacante descargue un binario y lo ejecute.
  - En la imagen de producción, el runtime `joss` corre con usuario sin privilegios `uid 10001 (joss)` sin capacidad de elevar permisos (`no-new-privileges:true`).

### 3.3 Network Escape y SSRF
- **Intento de Ataque**: El tenant intenta escanear la red interna del VPS, acceder a Redis en `localhost:6379`, o robar credenciales de AWS/Oracle via `http://169.254.169.254/latest/meta-data/`.
- **Defensa en Profundidad**:
  - Cada contenedor tiene su propio **Network Namespace**.
  - Reglas de `nftables` / `iptables` en el host bloquean todo el tráfico saliente hacia direcciones privadas (RFC 1918) y direcciones de metadata de enlace local (`169.254.169.254`).
  - Solo se permite salida hacia Internet en puertos 80, 443 y 53 (DNS cifrado del gateway).

### 3.4 Resource Exhaustion (Agotamiento de Recursos)
- **Fork Bomb**: `while(true) { async { ... } }` o creación de procesos hijos.
  - *Mitigación*: `pids.max = 32`. El kernel bloquea la creación del proceso 33 con `EAGAIN` protegiendo la tabla de procesos del host.
- **Memory Leak / OOM Flood**: Un bucle que consume RAM hasta saturar el servidor.
  - *Mitigación*: `memory.max = 96M` y `memory.swap = 0`. Al llegar a 96 MB, el kernel OOM-Killer finaliza inmediatamente el contenedor del tenant infractor sin alterar a los vecinos.
- **CPU Starvation**: Un bucle infinito `while(1) {}`.
  - *Mitigación*: `cpu.max = "25000 100000"`. El planificador CFS de Linux suspende el hilo cada vez que consume 25 ms en una ventana de 100 ms, limitando la CPU al 25% de un solo núcleo.

---

## 4. Kill-Switch Administrativo Inmediato

El Control Plane cuenta con un endpoint administrativo protegido criptográficamente:

```bash
# Comando de bloqueo inmediato
curl -X POST https://control.joss.red/admin/v1/tenant/{id}/kill \
  -H "Authorization: Bearer $SUPER_ADMIN_KEY" \
  -d '{"reason": "Detección de minería / Abuso de CPU", "purge_data": false}'
```

### Acciones ejecutadas en <100 ms:
1. `docker kill -s SIGKILL tenant-{id}`: Detención instantánea del proceso en CPU.
2. Inyección de regla en Caddy: La ruta `{subdomain}.joss.red` devuelve `403 Forbidden - Proyecto Suspendido`.
3. Congelación del volumen de datos para auditoría forense.
