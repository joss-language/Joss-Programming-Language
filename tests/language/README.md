# Corpus de conformidad semántica

Cada archivo JSON es un contrato ejecutado por
pkg/core.TestLanguageCompatibilitySpecificationSuite. La fuente atraviesa
parser, analyzer e intérprete. expected_diagnostics contiene los códigos de
error exactos y, para programas válidos con main, se declara el resultado
runtime tipado.

## Divergencias intencionales

| Feature | Diferencia | Autoridad | Motivo |
|---|---|---|---|
| Valores mixed provenientes de host/plugins | El analyzer no puede demostrar el tipo concreto que aparecerá en ejecución | Runtime/contrato host | Comportamiento dinámico explícito |
| Métodos nativos con ArityKnown=false | Analyzer acepta aridad variable; el handler puede rechazar combinaciones durante ejecución | Handler runtime + metadata parcial | Publicar una aridad inventada sería incorrecto |

Una divergencia nueva debe añadirse aquí con su autoridad y motivo; no debe
ocultarse relajando la comparación de diagnósticos.
