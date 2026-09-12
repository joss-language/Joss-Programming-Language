# Status e limites de implantação

[Índice](README.md) · [Arquitetura](ARQUITECTURA.md) · [Auditoria](DOCUMENTATION_AUDIT.md)

Esta página separa os recursos disponíveis, implementações parciais e
objetivos de projeto. Corresponde ao código auditado, não garante uma
versão baixada anteriormente.

| Área | Situação real | Referência |
|---|---|---|
| Idioma | Analisador Pratt, variáveis ​​explícitas de tipo fixo ou misto, classes/herança/visibilidade, funções/fechamento/ref, ternários, loops, correspondência de valores, try/catch. | [Sintaxe](SINTAXIS.md) |
| Tipos | int64, float64, decimal, strings, arrays, mapas, objeto, canal, classes, uniões e anulável. Análise e defesa em tempo de execução com diferenças registradas. | [Tipos](SISTEMA_TIPOS.md) |
| Simultaneidade | Goroutines usando assíncrono, Future, bloqueando espera e canais. Sem cancelamento estruturado ou isolamento profundo. | [Simultaneidade](CONCURRENCIA.md) |
| Analisador | Símbolos em duas passagens, escopos, atribuibilidade, membros conhecidos, retornos e diagnósticos. Nenhuma prova geral de conclusão ou refinamento por parte das filiais. | [Analisador](ANALIZADOR.md) |
| Execução principal | AST interpretado com planos, frames e caches que podem ser chamados. Pacotes de construção nativos JOSSBC2Z compactados AST com runner Go. | [Arquitetura](ARQUITECTURA.md) |
| VM experimental | pkg/vm contém compilador e VM separados; não é o back-end padrão da CLI/core. | [Interno](ARQUITECTURA.md) |
| Rede | Roteador HTTP/WS, visualizações, Solicitação/Resposta, sessão, CSRF, CORS, TLS e limites configuráveis. | [Projeto Web](PROYECTO_WEB.md) |
| SQL | Adaptadores, construtor, esquema e migrações SQLite/MySQL/PostgreSQL/SQL Server. Portabilidade parcial por operação; transação não vincula consultas ao Tx. | [Modelos](MODELOS.md) |
| Plug-ins | Contêiner e índice assinado, AST e JPBC; compiladores parciais. Route Wasm gera stubs de texto, mas não executa Wasm. | [Plugins](PLUGINS.md) |
| Permissões de plug-in | Proteção para chamadas de host mapeadas; sem sandbox WASI/OS ou consentimento por pacote. | [Plugins](PLUGINS.md) |
| Ferramentas | CLI, formatador, linter/fix, test runner e extensão VS Code com catálogo gerado. Não há comando REPL ou depurador integrado. | [CLI](CLI.md) |

Não há propriedade, imutabilidade padrão, indicadores gerais, interfaces,
características, protocolos, genéricos de função/classe, adiar/finalmente, seleção de canal,
nem backend LLVM/Cranelift. As anotações de coleção não são equivalentes a genéricas
universais nem garantem que cada mutação revalide elementos.

A ausência de importações/exportações/namespaces de origem é uma decisão permanente,
não é um recurso pendente. A modularidade ALIM usa recursos integrados,
Organização física e plugins de carregamento automático.

A tese combina arquitetura e implementação alvo. Suas declarações de
back-end, isolamento e módulos devem ser verificados em relação a este estado e com o
[auditoria técnica histórica](AUDITORIA_TECNICA_2026.md). Nenhum objetivo presente
como serviços já concluídos.