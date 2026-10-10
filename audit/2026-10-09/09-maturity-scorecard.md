# 09. Matriz de Puntuación de Madurez Técnica (Scorecard)

**Proyecto:** Joss Programming Language  
**Escala:** 0 a 5  
- **0:** Inexistente o bloqueado por deficiencia fundamental confirmada.
- **1:** Rudimentario, experimental o con fallos críticos.
- **2:** Parcialmente implementado o con validación insuficiente.
- **3:** Funcional para un alcance definido, con limitaciones conocidas.
- **4:** Sólido, documentado y probado para el alcance declarado.
- **5:** Altamente maduro, con evidencias extensas y procesos reproducibles.

---

## Matriz de Evaluación Detallada

| Dimensión Evaluada | Puntuación (0-5) | Peso | Puntuación Ponderada | Justificación Basada en Evidencia |
| :--- | :---: | :---: | :---: | :--- |
| **1. Coherencia con la Tesis y ALIM** | **2.5** | 7% | 0.175 | Cumple principios sintácticos y baterías integradas, pero la compilación AOT real diverge y los datos empíricos carecen de trazabilidad. |
| **2. Diseño del Lenguaje** | **4.2** | 6% | 0.252 | Gramática limpia, ortogonal, consistente y orientada a cero ambigüedades. |
| **3. Corrección del Parser** | **3.8** | 5% | 0.190 | Parser Pratt robusto, pero el lexer tolera cadenas no cerradas (HAL-02). |
| **4. Análisis Semántico** | **4.5** | 8% | 0.360 | Excelente pipeline estático; detecta variables, funciones, firmas y alcances con rigor previo a ejecución. |
| **5. Sistema de Tipos** | **4.2** | 7% | 0.294 | Tipado nominal y por inferencia riguroso; defensa activa contra conversiones implícitas con pérdida. |
| **6. Compilador Nativo AOT** | **1.8** | 6% | 0.108 | Lowerer e IR incompletos para apps reales; recurre a fallback de runner empaquetado (HAL-01). |
| **7. Runtime e Intérprete AST** | **4.3** | 8% | 0.344 | Intérprete rápido, determinista, con marcos léxicos y protección contra recursión infinita. |
| **8. Gestión y Diagnósticos de Error**| **4.6** | 5% | 0.230 | Códigos estructurados canónicos `JOSS-...` con posición exacta y sugerencias accionables. |
| **9. Consistencia entre Modos** | **3.0** | 5% | 0.150 | Divergencia entre lo que compila LLVM y lo que ejecuta el intérprete. |
| **10. Calidad Arquitectónica del Código**| **4.0** | 6% | 0.240 | Separación limpia de paquetes Go, sin dependencias cíclicas; dependencias unidireccionales respetadas. |
| **11. Cobertura y Calidad de Pruebas**| **4.4** | 7% | 0.308 | 154 suites `_test.go` pasando al 100%, pruebas de arquitectura y contratos de documentación. |
| **12. Seguridad Defensiva** | **2.8** | 8% | 0.224 | Riesgos en OTP con `math/rand`, fuga de CSRF a stdout y WebSockets sin validación de origen. |
| **13. Rendimiento Observado** | **3.7** | 5% | 0.185 | Muy rápido en desarrollo local; faltan arneses de benchmarking reproducibles. |
| **14. Documentación Técnica** | **4.2** | 5% | 0.210 | Extensa, precisa, con traducción i18n y validación contractual automatizada. |
| **15. Experiencia del Desarrollador (DX)**| **4.3** | 4% | 0.172 | Cero imports, CLI integrada completa (`new`, `make:*`, `test`, `format`), extensión VS Code. |
| **16. Portabilidad** | **3.8** | 3% | 0.114 | Ejecuta en Windows y Linux; instaladores multiplataforma. |
| **17. Mantenibilidad del Código** | **3.9** | 3% | 0.117 | Código Go legible y bien estructurado; exceso de `panic` en core. |
| **18. Versionado y Compatibilidad** | **3.0** | 3% | 0.090 | Etiquetas Git v3.6.7.8 no reflejadas en el archivo CHANGELOG.md. |
| **19. Preparación para Publicación** | **3.2** | 5% | 0.160 | Listo como prototipo/lenguaje experimental; bloqueado para producción general. |
| **20. Reproducibilidad de Resultados**| **3.5** | 4% | 0.140 | Tests unitarios e intérprete 100% reproducibles; benchmarks académicos no reproducibles. |
| **TOTAL PONDERADO** | — | **100%** | **3.67 / 5.0** | **Nivel Global: Funcional Sólido para Ámbito Experimental / Educativo** |

---

## Condiciones de Veto y Evaluación de Riesgo

1. **Condición de Veto AOT (HAL-01):** Impide calificar al proyecto como *"Compilador Nativo Independiente"*. Debe anunciarse como *"Intérprete AST con empaquetador autónomo"*.
2. **Condición de Veto de Seguridad (HAL-03, HAL-04, HAL-05):** Impide calificar al runtime como *"Listo para Producción Web Crítica"* hasta parchear las vulnerabilidades identificadas.
