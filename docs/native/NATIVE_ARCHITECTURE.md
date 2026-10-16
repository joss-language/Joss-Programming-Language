# Arquitectura Definitiva Joss AOT

Este documento detalla el diseño final para la compilación nativa de Joss (`joss build`), asegurando una fuente única de verdad, un solo lenguaje y soporte de binarios minúsculos (`pay-for-what-you-use`).

## 1. Single Source of Truth
Nunca duplicar lógica. Funcionalidades como `GranDB`, `HTTP` y `Storage` viven canónicamente en Go dentro de `pkg/core`. No se reescriben en ensamblador ni C en el backend nativo.

## 2. Un Solo Lenguaje, Dos Modos de Ejecución
`joss run` y `joss build` ejecutan la misma semántica estricta validada por el Front-end de Joss. No hay fallbacks a interpretadores si el AOT no soporta algo.

## 3. HostBridge y Micromódulos (Smart Selective Linking)
La solución definitiva para no sacrificar el peso del binario (`Opción D` o Enlace Modular Inteligente):
- Código máquina puro se emite para lógica, loops y algoritmos base.
- Si (y sólo si) se detecta uso de servicios como servidor HTTP o GranDB, el ejecutable enlaza `_joss_host_bridge` con optimización por build tags (descartando drivers no usados).
- **Resultado:** Binarios de ~5.5 KB para algoritmos y CLI puros, y binarios autocontenidos sellados con VFS (AES-256) para aplicaciones web completas.
