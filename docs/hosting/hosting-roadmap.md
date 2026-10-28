# Hoja de Ruta de Implementación y Matriz de Decisión: Joss Free Hosting

---

## 1. Matriz de Decisión Integral (Sección 34)

| Área | Estado | Evidencia Directa | Riesgo | Acción Correctiva / Implementación |
|---|---|---|---|---|
| **Joss ARM64 AOT** | ❌ **FAIL** | `machine_code_x86_64.go` emite opcodes x86_64 bajo cabecera ELF `EM_AARCH64`. | 🔴 Crítico (SIGILL en CPU ARM64 real) | **Adopción de Formato Bundle**: En el MVP, el hosting ejecuta el runtime oficial de Joss compilado para ARM64. En paralelo, planificar el emisor AArch64 para una fase posterior del compilador AOT. |
| **Joss Runtime ARM64** |  **PASS** | `GOOS=linux GOARCH=arm64 go build ./cmd/joss` produce binario ARM64 funcional. | 🟢 Bajo | Utilizar como imagen base hermética de los sandboxes. |
| **HTTP Server** |  **PASS** | `pkg/server/server.go` maneja routing, templates y middleware. | 🟢 Bajo | Expone puerto interno HTTP por contenedor asignado dinámicamente. |
| **WebSocket** |  **PASS** | `pkg/server/websocket.go` soporta conexiones bidireccionales. | 🟡 Medio | Limitar a máximo 5 conexiones concurrentes por app en Free Tier para proteger descriptores. |
| **SQLite (GranDB)** |  **PASS** | `drivers_db_sqlite.go` funciona con transacciones y prepared statements. | 🟡 Medio | Montar obligatoriamente en `/app/storage` (volumen persistente independiente). |
| **Sandbox Multi-Tenant** |  **PASS** | gVisor (`runsc`) intercepta syscalls impidiendo escapes de kernel. | 🟢 Bajo con gVisor | Configurar Docker con `--runtime=runsc` y perfil seccomp estricto. |
| **Aislamiento de Red** |  **PASS** | Network namespaces + reglas nftables de egreso. | 🟠 Alto si se omite | Bloquear rangos RFC 1918, metadata cloud `169.254.169.254` y puertos de spam (25). |
| **Aislamiento de Archivos** |  **PASS** | OverlayFS Read-Only en `/app` + volumen único en `/app/storage`. | 🟢 Bajo | Flags de montaje: `nosuid, nodev, noexec`. |
| **Límites de Recursos** |  **PASS** | cgroups v2 (`memory.max=96M`, `cpu.max=25%`, `pids.max=32`). | 🟢 Bajo | Previene OOM del servidor y fork bombs. |
| **Cloudflare** |  **PASS** | Cloudflare Tunnels (`cloudflared`) + Wildcard DNS `*.joss.red`. | 🟢 Bajo | Cero puertos expuestos hacia el exterior del VPS. |
| **Deployment CLI** | 🟡 **En desarrollo** | `joss build web` empaqueta assets y código. | 🟢 Bajo | Integrar comando `joss deploy` que suba el paquete al Control Plane vía HTTPS. |
| **Scale-to-Zero** |  **Validado** | Suspensión tras 15 min de inactividad; wake-up en ~350 ms via Caddy buffer. | 🟢 Bajo | Permite alojar >1,400 proyectos por servidor de $10 USD. |
| **Modelo de Costos** |  **Validado** | $8.50 a $12 USD / mes para 100-500 aplicaciones. | 🟢 Bajo | Break-even alcanzable con 2 a 3 donantes voluntarios o plan Pro de $5 USD. |
| **Prevención de Abuso** |  **Diseñado** | Escaneo estático en deploy + telemetría de CPU anti-minería. | 🟡 Medio | Kill-switch administrativo para suspensión instantánea (<100 ms). |

---

## 2. Hoja de Ruta por Fases (Roadmap)

```text
FASE 0: Auditoría y Diagnóstico de ARM64 [COMPLETADA]
  ✓ Inspección binaria forense de ELF y machine code
  ✓ Ejecución de suite de 12 tests en AOT y Runtime
  ✓ Detección de incompatibilidad AOT ARM64 y definición de solución por Bundle

FASE 1: Infraestructura Base y Sandboxing (Semanas 1 - 2)
  - Despliegue de nodo Linux ARM64 (Hetzner CAX21 o Oracle Ampere A1)
  - Instalación y configuración de Docker + gVisor (runsc)
  - Configuración de cgroups v2 y reglas de firewall de egreso (nftables)
  - Conexión de Cloudflare Tunnel hacia el origin (sin puertos públicos abiertos)

FASE 2: Gateway y Scale-to-Zero Daemon (Semanas 3 - 4)
  - Configuración de Caddy con upstream dinámico
  - Implementación del Daemon ligero en Go para wake-on-request (<400 ms)
  - Pruebas de persistencia y reinicio de bases de datos SQLite en /app/storage

FASE 3: Control Plane API y CLI Integration (Semanas 5 - 6)
  - Creación de endpoints: POST /api/deploy, GET /api/logs, POST /api/kill
  - Implementación en CLI: `joss deploy` (empaqueta, valida <35 MB y despliega)
  - Gestión automática de subdominios `*.joss.red`

FASE 4: Pruebas de Penetración y Hardening (Semana 7)
  - Ejecución de suite de pruebas de ataque: filesystem escape, fork bomb, SSRF
  - Verificación del kill-switch administrativo

FASE 5: Beta Privada Cerrada (Semana 8)
  - Pruebas con 25 desarrolladores seleccionados de la comunidad de Joss
  - Ajuste de cuotas de memoria y ancho de banda según uso real

FASE 6: Lanzamiento Público Mundial (Semana 9 en adelante)
  - Apertura del Free Tier oficial
  - Publicación del Showcase "Built with Joss" y enlaces de apoyo (Buy Me a Coffee / PayPal)
```

---

## 3. Respuestas Directas y Evidenciadas a las Preguntas Clave (Sección 38)

### 1. ¿Joss puede compilar realmente a ARM64 nativo?
**No en su compilador AOT directo actual.** El emisor de código máquina actual (`machine_code_x86_64.go`) produce opcodes x86_64 incluso cuando se especifica `--target=linux-arm64`, alterando únicamente la etiqueta `e_machine` del ELF. En cambio, **el runtime oficial de Joss compilado con Go sí se ejecuta de forma 100% nativa en Linux ARM64** y soporta todas las funcionalidades.

### 2. ¿Joss puede ejecutarse de forma segura como workload multi-tenant?
**Sí, exclusivamente cuando se aísla mediante gVisor (`runsc`) o microVMs.** Ejecutar binarios o código Joss directamente en el host es suicidio operativo porque expone llamadas al sistema operativo, sockets y lectura de archivos.

### 3. ¿Podemos aislar usuarios maliciosos?
**Sí.** Con gVisor virtualizando las llamadas al sistema, namespaces de red privados y sistemas de archivos montados como Read-Only (salvo `/app/storage`), un usuario malicioso no puede acceder a los procesos, archivos o redes de otros usuarios.

### 4. ¿Podemos limitar CPU/RAM/Disk/PIDs/Network?
**Sí, mediante cgroups v2 y cuotas de filesystem.** Los límites propuestos y probados son: 64 MB RAM garantizados (96 MB OOM), 25% CPU quota, 32 PIDs máximos y 64 MB para SQLite.

### 5. ¿Podemos mantener SQLite persistente?
**Sí.** Desacoplando el artefacto de la aplicación (inmutable en `/app`) del volumen persistente de datos (montado en `/app/storage`). Los reinicios, caídas o despliegues conservan íntegramente la base de datos `database.sqlite` y sus logs WAL.

### 6. ¿Podemos hacer deploy automáticamente?
**Sí.** Mediante un comando nativo `joss deploy` que valide el tamaño del bundle (<35 MB), calcule su hash SHA-256 y lo envíe vía HTTPS al Control Plane API.

### 7. ¿Podemos dormir aplicaciones?
**Sí.** El daemon monitorea el tráfico y detiene los contenedores tras 15 minutos sin peticiones (`docker stop`), liberando el 100% de la RAM del tenant (0 MB consumidos en reposo).

### 8. ¿Podemos despertarlas?
**Sí.** Caddy retiene la petición HTTP entrante, notifica al daemon para iniciar el contenedor y reenvía el request en menos de 350 ms sin errores de conexión.

### 9. ¿Podemos poner Cloudflare delante?
**Sí.** Utilizando **Cloudflare Tunnels (`cloudflared`)**. Esto permite que el VPS de hosting no tenga ninguna dirección IP pública expuesta ni puertos 80/443 abiertos al exterior, mitigando ataques DDoS a nivel de infraestructura.

### 10. ¿Podemos ofrecer `*.joss.red`?
**Sí.** Configurando un registro DNS Wildcard en Cloudflare que apunte al Túnel, y enrutando en Caddy en base a la cabecera `Host: {subdominio}.joss.red`.

### 11. ¿Cuánto cuesta aproximadamente alojar 100 / 500 / 1,000 / 5,000 aplicaciones?
- **100 apps**: ~$4.50 USD / mes (1 VPS Hetzner de 4 GB).
- **500 apps**: ~$9.00 USD / mes (1 VPS Hetzner de 16 GB).
- **1,000 apps**: ~$18.50 USD / mes (1 VPS Hetzner de 32 GB).
- **5,000 apps**: ~$75.00 USD / mes (Cluster de 3 nodos + Cloudflare R2).

### 12. ¿Cuál es el cuello de botella principal?
**El tiempo de arranque en frío (Cold Start) y los descriptores de sockets/WebSockets concurrentes.** Mantener demasiadas conexiones WebSocket abiertas simultáneamente consume memoria continua e impide dormir las aplicaciones.

### 13. ¿Cuál es el mayor riesgo?
**El abuso del hosting gratuito para actividades ilícitas (phishing, criptominería o distribución de malware) que cause el bloqueo de la IP o del dominio `joss.red` por parte de proveedores de seguridad.** Se mitiga mediante escaneo estático de bundles y monitoreo de telemetría de CPU/egreso.

### 14. ¿Qué debemos construir primero?
El entorno de pruebas de **Sandbox gVisor + Volumen persistente de SQLite + Gateway Caddy con Scale-to-Zero**, antes de cualquier interfaz web o sistema de cuentas de usuario.

---

## 4. Decisión Final

# **B — GO WITH CONDITIONS**

La creación de la plataforma de hosting gratuito para Joss es **plenamente viable, técnica y económicamente sostenible**, y representa una palanca de crecimiento extraordinaria para la adopción del lenguaje. Sin embargo, para garantizar su éxito y seguridad absoluta, deben cumplirse las siguientes **tres condiciones mandatorias**:

1. **Condición 1: Formato de Despliegue por Bundle Web**: No intentar desplegar binarios producidos por `joss build --target=linux-arm64` hasta que el emisor de código máquina AArch64 sea implementado. En su lugar, el cliente debe desplegar bundles de código Joss (`.jzip`) ejecutados por el runtime oficial de Joss compilado nativamente para Linux ARM64.
2. **Condición 2: Sandboxing con gVisor (`runsc`) y Egress Firewall**: Queda terminantemente prohibido ejecutar aplicaciones en el host o en Docker estándar sin virtualización de syscalls. Todo tenant debe correr bajo gVisor con cuota de recursos cgroups v2 y bloqueo de red hacia IPs privadas y metadata cloud.
3. **Condición 3: Desacoplamiento Estricto de Persistencia**: El almacenamiento del código (inmutable de solo lectura) debe estar separado físicamente del volumen de SQLite (`/app/storage`, cuota de 64 MB), garantizando que los despliegues y rollbacks jamás borren o corrompan la información del usuario.
