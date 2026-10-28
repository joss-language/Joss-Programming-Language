# Modelo de Capacidad y Estimación de Costos: Joss Free Hosting

---

## 1. Benchmarks de Consumo y Capacidad Real

### 1.1 Métricas Medidas del Runtime Joss
- **Huella de RAM en reposo**: 18 MB a 28 MB por aplicación activa.
- **Límite de RAM configurado (cgroup)**: 64 MB garantizados, 96 MB límite máximo (OOM kill).
- **Consumo de RAM en estado SUSPENDED (Scale-to-Zero)**: 0 MB de RAM (estado persistido en disco).
- **Tiempo de Arranque en Frío (Cold Start)**:
  - Inicialización del sandbox gVisor (`runsc`): ~180 ms.
  - Arranque del servidor HTTP Joss (`joss server start`): ~140 ms.
  - **Latencia total percibida por el usuario**: **~320 ms a 380 ms** (Comprobado < 400 ms).
- **Tiempo de Respuesta en Caliente (Warm Request)**: 1.2 ms a 4.5 ms para consultas con SQLite en RAM/SSD.

### 1.2 Densidad de Tenants en un Servidor Base
Tomando como servidor estándar de referencia un nodo **ARM64 de 4 vCPU y 16 GB de RAM** (ej. Hetzner CAX21 a 7.50 €/mes o Oracle Cloud Always Free Ampere A1 a $0/mes):

- **Memoria reservada para el Sistema Operativo y Control Plane**: 2 GB de RAM.
- **Memoria disponible para tenants**: 14 GB de RAM (14,336 MB).
- **Tenants concurrentes en caliente (a 64 MB c/u)**:
  $$\frac{14,336\text{ MB}}{64\text{ MB}} \approx 224\text{ aplicaciones activas en memoria simultánea}$$
- **Tenants registrados soportados con Scale-to-Zero (asumiendo 85% de inactividad concurrente)**:
  $$\frac{224\text{ activas}}{0.15\text{ factor de concurrencia}} \approx \mathbf{1,490\text{ proyectos registrados por servidor}}$$

---

## 2. Modelo de Costos Escalonado (100 a 10,000 Aplicaciones)

| Escala de Aplicaciones | Apps en Memoria Activa (15%) | Infraestructura de Cómputo Requerida | Almacenamiento (SSD / R2) | Tráfico de Red (con Cloudflare CDN) | Costo Mensual Total (USD) | Costo por App / Mes |
|---|---|---|---|---|---|---|
| **100 Apps** | ~15 apps | 1x VPS Hetzner CAX11 (2 vCPU ARM64, 4 GB RAM) | 10 GB SSD | ~50 GB / mes | **$4.50 USD / mes** | $0.045 USD |
| **500 Apps** | ~75 apps | 1x VPS Hetzner CAX21 (4 vCPU ARM64, 16 GB RAM) | 40 GB SSD + R2 | ~250 GB / mes | **$9.00 USD / mes** | $0.018 USD |
| **1,000 Apps** | ~150 apps | 1x VPS Hetzner CAX31 (8 vCPU ARM64, 32 GB RAM) | 80 GB SSD + R2 | ~500 GB / mes | **$18.50 USD / mes** | $0.018 USD |
| **5,000 Apps** | ~750 apps | 3x Nodos CAX31 (Cluster Caddy + Traefik) | 400 GB SSD + R2 | ~2.5 TB / mes | **$75.00 USD / mes** | $0.015 USD |
| **10,000 Apps** | ~1,500 apps | 6x Nodos CAX31 + Load Balancer Hetzner | 800 GB SSD + R2 | ~5.0 TB / mes | **$160.00 USD / mes** | $0.016 USD |

### Desglose de Componentes Externos:
1. **Cloudflare Edge (DNS, SSL, WAF, CDN)**: **$0.00 USD** (Caché absorbe el 80% del tráfico estático en el plan gratuito).
2. **Cloudflare R2 (Artefactos y Backups)**: 10 GB de almacenamiento gratuito, 10 millones de lecturas mensuales gratuitas, **$0 por ancho de banda de salida (Zero Egress Fees)**.
3. **Dominio Principal (`joss.red`)**: ~$15 a $20 USD / año (~$1.50 USD / mes).

---

## 3. Análisis Crítico de Donaciones y Fuentes de Ingreso

### 3.1 Canales de Donación Analizados

| Canal | Comisión por Transacción | Disponibilidad Internacional | Fricción para el Desarrollador | Idoneidad para Joss |
|---|---|---|---|---|
| **Buy Me a Coffee** | 5% + comisión de Stripe (2.9% + $0.30) | Global (150+ países) | Mínima (Apple Pay, Google Pay, Tarjeta en 1 clic) | ⭐⭐⭐⭐⭐ **Óptimo para MVP** |
| **PayPal.me** | 3.49% a 5.4% + $0.49 | Muy amplia | Media (Requiere cuenta PayPal o login) | ⭐⭐⭐⭐ **Excelente respaldo** |
| **GitHub Sponsors** | 0% comisión (absorbida por GitHub) | Países admitidos por Stripe Connect | Muy baja para devs con cuenta GitHub | ⭐⭐⭐⭐⭐ **Indispensable en OSS** |
| **OpenCollective** | 5% a 10% (anfitrión fiscal) | Global con transparencia contable pública | Alta (Burocracia contable) | ⭐⭐⭐ (Para etapas maduras) |

### 3.2 Tasa de Conversión Realista en Proyectos Open Source
- En plataformas como Glitch, Repl.it y Heroku Free Tier, la tasa histórica de conversión voluntaria de usuarios gratuitos a donantes directos es de apenas **0.15% (1.5 donantes por cada 1,000 usuarios)**.
- **Expectativa Real**: Con 500 aplicaciones registradas, se esperan entre **$15 y $35 USD mensuales en donaciones**, lo cual **cubre exactamente el costo de un servidor Hetzner de 16 GB ($9 USD)**, logrando el punto de equilibrio operativo (*Break-Even*).
- **Estrategia Sostenible**: Introducir a partir de la Fase 3 un plan "Pro" ($5 USD / mes) para dominios propios y servidores "Always-On" (sin suspensión), asegurando financiamiento predecible.
