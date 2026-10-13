# Arquitectura Definitiva Joss AOT

Este documento detalla el diseño final para la compilación nativa de Joss (`joss build`), asegurando una fuente única de verdad, un solo lenguaje y soporte de binarios minúsculos (`pay-for-what-you-use`).

## 1. Single Source of Truth
Nunca duplicar lógica. Funcionalidades como `GranDB`, `HTTP` y `Storage` viven canónicamente en Go dentro de `pkg/core`. No se reescriben en ensamblador ni C en el backend nativo.

## 2. Un Solo Lenguaje, Dos Modos de Ejecución
`joss run` y `joss build` ejecutan la misma semántica estricta validada por el Front-end de Joss. No hay fallbacks a interpretadores si el AOT no soporta algo.

## 3. HostBridge y Micromódulos (Smart Selective Linking)
La solución definitiva para no sacrificar el peso del binario (`Opción D` o Enlace Modular Inteligente):
- Código máquina puro se emite para lógica, loops y algoritmos base.
- Si (y sólo si) se detecta uso de `GranDB` o Standard Library, el linker estático interno une `libjoss_base.a` con el binario AOT generado.
- **Resultado:** Binarios de 5KB para algoritmos simples y 5MB para apps complejas con Base de Datos o servidor Web integrados.
