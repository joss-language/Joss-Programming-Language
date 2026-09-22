# Status e limites de implantação

[Índice](README.md) · [Arquitetura](ARQUITECTURA.md) · [Auditoria](DOCUMENTATION_AUDIT.md)

Esta página separa os recursos disponíveis, implementações parciais e
objetivos de projeto. Corresponde ao código auditado, não garante uma
versão baixada anteriormente.

| Área | Situação real | Referência |
|---|---|---|
| Idioma | Analisador Pratt, variáveis explícitas de tipo fixo ou misto, classes/herança/interfaces/enums/visibilidade, funções/fechamentos/ref, ternários, guard, loops, correspondência de valores, try/catch, defer e select de canais. | [Sintaxe](SINTAXIS.md) |
| Tipos | int64, float64, decimal, strings, arrays, mapas, objeto, canal, classes, uniões e anulável. Análise e defesa em tempo de execução com diferenças registradas. | [Tipos](SISTEMA_TIPOS.md) |
| Simultaneidade | Goroutines usando assíncrono, Future, espera bloqueante e canais. A execução móvel suporta cancelamento cooperativo e captura segura de saída tardia; uma chamada nativa bloqueante pode continuar após o timeout. Não existe cancelamento estruturado geral nem isolamento profundo. | [Simultaneidade](CONCURRENCIA.md) |
| Analisador | Declarações, escopos, atribuibilidade, membros conhecidos, retornos e diagnósticos; narrowing local em ternários, guard e comparações com null. Não existe CFG geral nem prova de terminação completa. | [Analisador](ANALIZADOR.md) |
| Execução principal | AST interpretado com planos, frames e caches que podem ser chamados. Pacotes de construção nativos JOSSBC2Z compactados AST com runner Go. | [Arquitetura](ARQUITECTURA.md) |
| VM experimental | pkg/vm contém compilador e VM separados; não é o back-end padrão da CLI/core. | [Interno](ARQUITECTURA.md) |
| Rede | Roteador HTTP/WS, visualizações, Solicitação/Resposta, sessão, CSRF, CORS, TLS e limites configuráveis. | [Projeto Web](PROYECTO_WEB.md) |
| SQL | Adaptadores SQLite/MySQL/PostgreSQL/SQL Server, construtor, Schema e migrações. Portabilidade parcial por operação; as consultas SQL ordinárias do runtime durante GranDB::transaction usam o Tx ativo. Transações aninhadas não são suportadas. | [Modelos](MODELOS.md) |
| Plug-ins | Contêiner e índice assinado, AST e JPBC; compiladores parciais. Route Wasm gera stubs de texto, mas não executa Wasm. | [Plugins](PLUGINS.md) |
| Permissões de plug-in | Proteção para chamadas de host mapeadas; sem sandbox WASI/OS ou consentimento por pacote. | [Plugins](PLUGINS.md) |
| Ferramentas | CLI com REPL, formatador, linter/fix, test runner e extensão VS Code com catálogo gerado. Não há depurador integrado. | [CLI](CLI.md) |

Não existem ownership geral, imutabilidade padrão, ponteiros gerais,
traits, protocolos, genéricos de função/classe, finally,
nem backend LLVM/Cranelift. As anotações de coleção não são equivalentes a genéricas
universais nem garantem que cada mutação revalide elementos.

A ausência de importações/exportações/namespaces de origem é uma decisão permanente,
não é um recurso pendente. A modularidade ALIM usa recursos integrados,
Organização física e plugins de carregamento automático.

A tese combina arquitetura e implementação alvo. Suas declarações de
back-end, isolamento e módulos devem ser verificados em relação a este estado e com o
[auditoria técnica histórica](AUDITORIA_TECNICA_2026.md). Nenhum objetivo presente
como serviços já concluídos.
