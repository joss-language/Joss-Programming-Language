# Auditoria Profunda da Linguagem de Programação Joss (2026) > **Documento Canônico de Auditoria Técnica, Ergonomia, Sistema de Tipos, Ferramentas e Arquitetura** > **Data de Preparação:** Setembro de 2026 > **Escopo:** Repositório oficial `jossecurity/joss`, subsistemas de kernel (`pkg/*`), ferramentas CLI, Servidor LSP (`vscode-joss`) e projeto de referência real `Joss-Red-JosSecurity`. > **Objetivo:** Diagnóstico exaustivo, evidência empírica em código, catálogo de propostas e plano de evolução tecnológica rumo a uma linguagem mais expressiva, coerente e segura. --- ## Índice Geral 1. [Resumo Executivo](#1-resumen-ejecutivo) 2. [Filosofia Atual Detectada em Joss](#2-filosofía-actual-detectada-en-joss) 3. [Pontos fortes atuais](#3-fortalezas-actuales) 4. [Pontos fracos estruturais](#4-debilidades-estructurales) 5. [Fricção encontrada no código de gravação](#5-fricción-encontrada-al-escribir-código) 6. [Boilerplate e cerimônia identificadas](#6-boilerplate-y-ceremonia-identificados) 7. [Inconsistências de idioma](#7-inconsistencias-del-lenguaje) 8. [Complexidade conceitual](#8-complejidad-conceptual) 9. [Tipo System Audit](#9-auditoría-del-sistema-de-tipos) 10. [Auditoria de segurança e defesa em tempo de execução](#10-auditoría-de-seguridad-y-defensas-runtime) 11. [Auditoria de erros e diagnóstico](#11-auditoría-de-errores-y-diagnósticos) 12. [Auditoria da Biblioteca Padrão (Stdlib)](#12-auditoría-de-la-biblioteca-estándar-stdlib) 13. [Auditoria de ferramentas e ecossistemas](#13-auditoría-de-tooling-y-ecosistema) 14. [Auditoria oficial do formatador](#14-auditoría-del-formatter-oficial) 15. [Analisador e auditoria de Linter](#15-auditoría-del-analyzer-y-linter) 16. [Comparação seletiva com outros idiomas](#16-comparación-selectiva-con-otros-lenguajes) 17. [Oportunidades de simplificação](#17-oportunidades-de-simplificación) 18. [Recursos que NÃO devem ser implementados](#18-características-que-no-conviene-implementar) 19. [Catálogo de Propostas Priorizadas (P1 a P8)](#19-catálogo-de-propuestas-priorizadas-p1-a-p8) 20. [Alterações que exigiriam suspensão de uso](#20-cambios-que-necesitarían-deprecación) 21. [Possíveis alterações significativas justificadas](#21-posibles-breaking-changes-justificados) 22. [Proposta e Arquitetura de `joss fix`](#22-propuesta-y-arquitectura-de-joss-fix) 23. [Definição de "Código Joss Idiomático"](#23-definición-de-código-joss-idiomático) 24. [Roteiro recomendado (Fases 0 a 5)](#24-roadmap-recomendado-fases-0-a-5) --- ## 1. Resumo executivo Joss é uma moderna linguagem de programação multiparadigma desenvolvida em Go, concebida para o desenvolvimento de serviços web, APIs de alto desempenho, microsserviços e utilitários de infraestrutura. Sua proposta fundadora busca combinar a **agilidade de iteração e familiaridade sintática** de linguagens de servidores dinâmicos (PHP, JavaScript, Python) com a **segurança estática, robustez simultânea e velocidade** do ecossistema Go. Através da sua evolução, Joss estabeleceu pilares arquitetônicos excepcionais:- **Pipeline de compilação desacoplado:** Arquitetura formal baseada em um lexer especializado, um analisador de precedência de operador Pratt, um AST unificado, um analisador semântico estrito (`pkg/analyzer`), diagnósticos estruturados e um tempo de execução de avaliação de árvore otimizado para slot lexical (`pkg/runtime/plan`). - **Arquitetura Zero-Imports:** Carregamento e resolução automáticos de símbolos no nível do projeto com base em convenções de estrutura, eliminando o gerenciamento manual de gráficos de dependência em aplicações web. - **Segurança aritmética e de memória:** inteiros de 64 bits com detecção de overflow em tempo de compilação e execução (`JOSS-ARITH-001`), ponto fixo monetário nativo (`decimal` com literal `m`), referências temporais seguras e invariantes (`ref`) e controle de recursão profundo. - **Assincronia limpa:** Suporte nativo para goroutines usando `async { ... }` e canais com operadores diretos (`$c << $msg`), com a função `await(...)` habilitada em qualquer contexto sem forçar a fragmentação do código em funções coloridas. ### O diagnóstico de auditoria Apesar de seus pontos fortes, esta auditoria técnica detectou que **Joss atualmente impõe um atrito acidental notável ao desenvolvedor**: 1. **Assimetria do verificador de tipo:** O compilador requer anotações estritas nos parâmetros (`JOSS-TYPE-011`) e validação exaustiva de ramificações nos retornos (`JOSS-TYPE-010`), mas carece de **propagação de refinamento de tipo (estreitamento de tipo sensível ao fluxo)** após cláusulas de guarda. Isso leva os programadores em produção a desfazer a digitação usando `mixed` generalizado. 2. **Controle de fluxo forçado:** A erradicação dogmática de declarações condicionais clássicas em favor do operador ternário generalizado com blocos produz pirâmides de aninhamento de até 6 níveis e a proliferação de ramos vazios falsos `: {}`. 3. **Perda de identidade do objeto em exceções:** Instâncias de erro capturadas em `catch ($e)` são rebaixadas para texto usando `fmt.Sprintf`, destruindo o OOP no tratamento de exceções. 4. **Biblioteca padrão inconsistente:** Nomes duplicados de funções globais herdadas do PHP coexistem com métodos fluidos ausentes em strings e coleções. 5. **Ferramentas Fragmentadas:** Formatador e linter reimplementam analisadores independentes em vez de compartilhar o kernel AST oficial. Este documento apresenta a análise detalhada, as evidências empíricas coletadas no código e o catálogo priorizado de soluções para consolidar Joss como uma linguagem previsível, segura e produtiva. --- ## 2. Filosofia Atual Detectada em Joss A análise da implementação revela as seguintes premissas que definem o caráter atual de Joss:```text
┌─────────────────────────────────────────────────────────────────────────┐
│                        PRINCIPIOS REALES DE JOSS                        │
├─────────────────────────────────────────────────────────────────────────┤
│ 1. Seguridad estática demostrable sin compilación pesada a máquina.     │
│ 2. Cero fricción de importación de archivos de proyecto (Zero-Imports). │
│ 3. Unificación sintáctica: las bifurcaciones son expresiones evaluables.│
│ 4. Tipado numérico defensivo: protección monetaria y anti-overflow.     │
│ 5. Concurrencia de paso de mensajes inspirada en CSP / Go.              │
│ 6. Baterías incluidas para desarrollo web y servicios backend.          │
└─────────────────────────────────────────────────────────────────────────┘
```
### Inconsistências Históricas e Tensões Filosóficas 1. **Expressividade vs. Burocracia nas Declarações:** - Embora Joss busque eliminar a cerimônia nas importações, ele impõe rigidez formal nas assinaturas exigíveis: visibilidade explícita obrigatória (`public`/`private`/`protected`), tipos de parâmetros obrigatórios e contratos de retorno exaustivos. Na prática, o código de produção contorna essa rigidez digitando `mixed` e omitindo o retorno. 2. **"Tudo é Expressão" vs. Código Imperativo com Efeitos Colaterais:** - A unificação de decisões sob o operador ternário `($cond) ? { ... } : { ... }` funciona elegantemente para atribuições simples:     ```joss-snippet
     $estado = ($puntos >= 60) ? "Aprobado" : "Reprobado"
     ```
No entanto, no código do controlador de negócios onde é necessário validar pré-condições, registrar auditorias e abortar prematuramente, o ternário é usado como uma instrução, forçando a escrita de blocos vazios `: {}` ou aninhamento profundo que contradiz o objetivo de clareza. 3. **Digitação forte versus biblioteca de procedimentos que não conhece tipos:** - O sistema de tipos permite que você especifique `array<int>` e `map<string, Usuario>`, mas quase todas as funções em `builtins_array.go` retornam `[]interface{}` ou `mixed`, retirando a coleção de seu tipo de garantias. 4. **Importações Zero vs. Espaço Global Contaminado:** - Como não há namespaces no código-fonte do Joss, as classes devem ser renomeadas com prefixos artificiais (`XiaomiCategory`, `CmsPost`, `AuthUser`) para evitar colisões no global do projeto. dicionário. --- ## 3. Pontos fortes atuais Joss possui pilares técnicos excepcionais que devem ser preservados como vantagens competitivas: 1. **Pipeline arquitetônico unidirecional:** - O compilador respeita estritamente o limite da camada: `parser`, `typesystem` e `analyzer` são independentes de tempo de execução e nunca importe bancos de dados ou serviços de rede. 2. **Modelo de diagnóstico estruturado:** - Saída determinística via `pkg/diagnostics` com intervalos precisos de linha/coluna, códigos estáveis ​​e sugestões de correção legíveis e consumíveis por IDEs. 3. **Precisão Numérica Financeira (`decimal`):** - Suporte integrado de ponto fixo de base dez (`decimal $precio = 99.99m`) via `shopspring/decimal` elimina erros de arredondamento IEEE-754 comuns em PHP, Python ou JavaScript. 4. **Rigorosa Defesa contra Overflow:** - Interceptação de overflow na aritmética de 64 bits via `typesystem.CheckedIntBinary` antes que ocorram corrupções de dados. 5. **Simultaneidade Pragmática e Leve:** - Canais como cidadãos de primeira ordem (`channel $c = make_chan(10)`), operador emissor `$c << $msg`, consumo sequencial `foreach ($c as $msg)` e controle múltiplo `select`. - `async { ... }` produz objetos `Future` simultâneos em goroutines sem impor "coloração de função" ao longo da árvore de chamadas. 6. **Promoção de propriedade do construtor:** - A sintaxe `Init(public string $nombre, public int $edad = 30) {}` erradica o código de atribuição de campo cerimonial. 7. **Verificação automatizada de documentação:** - `pkg/core/documentation_test.go` valida automaticamente cada fragmento executável da documentação oficial, garantindo que os manuais nunca fiquem fora de sincronia com o comportamento real do mecanismo. --- ## 4. Fraquezas Estruturais 1. **Ausência de Estreitamento Exterior do Tipo Sensível ao Fluxo:** - O analisador reconhece apenas o estreitamento de um tipo anulável dentro dos ramos do ternário. Após uma cláusula de guarda com saída prematura, a variável externa não é promovida. 2. **Atrito no controle de fluxo:**- Ausência de uma declaração de decisão simples (`guard` ou `if`), resultando em construções forçadas com blocos falsos vazios e ramificações `: {}`. 3. **Destruição de instâncias no tratamento de exceções:** - O tempo de execução degrada as instâncias de classe iniciadas via `throw new CustomException(...)` em strings de texto simples `string` quando presas em `catch ($e)`. 4. **Comportamento Surpreendente em Igualdade (`==`) e Falsidade (`isFalsy`):** - O operador `==` recorre à conversão para texto usando `fmt.Sprintf` quando os operandos são não numérico, gerando que `null == ""` e `[1, 2] == ["1", "2"]` são avaliados como verdadeiros. - Em `isFalsy`, o número flutuante `0.0` e os mapas vazios `{}` são tratados como verdadeiros, enquanto a string `"0"` é tratada como falsa. 5. **Biblioteca padrão fragmentada:** - Sobrevivência de nomes duplicados (`str_contains` vs `contains`, `len` vs `count` vs `strlen`) e ausência completa de métodos de instância orientados a objetos em strings e coleções. 6. **Ferramentas dissociadas do AST Central:** - O formatador reimplementa um scanner léxico independente, o fixador depende de expressões regulares e a extensão VS Code replica análises TypeScript. --- ## 5. Atrito encontrado ao escrever código ### Evidência 1: Pirâmide de ternários aninhados em controladores No arquivo real [BackupController.joss:10-66](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss#L10-L66) do projeto de referência, o seguinte padrão de validação em cadeia é observado:```joss-snippet
// CÓDIGO REAL EN JOSS-RED-JOSSECURITY:
public func saveOrUpdateBackup(mixed $appName) {
    return ($appName == "otp_backup") ? json({"error": "..."}, 400) : {
        $allowedExtension = $this->getAllowedExtension($appName)

        return (!$allowedExtension) ? json({"error": "..."}, 404) : {
            $file = Request::file("file")
            
            return (!$file) ? json({"error": "..."}, 422) : {
                $originalName = $file["name"]
                $parts = explode(".", $originalName)
                $ext = end($parts)

                return ($ext != $allowedExtension) ? json({"error": "..."}, 422) : {
                    $u = Auth::user()
                    return (!$u) ? json({"error": "..."}, 401) : {
                        // ... Lógica de negocio desplazada a más de 24 espacios de sangría ...
                    }
                }
            }
        }
    }
}
```
**Diagnóstico:** O programador é forçado a converter validações lineares sequenciais em uma hierarquia de blocos falsos aninhados. Qualquer modificação intermediária do método requer reformatação e reinserção de dezenas de linhas de código. ### Evidência 2: The Phantom Fake Branch `: {}` Em [AuthController.joss:3](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/auth/AuthController.joss#L3) e em vários controladores:```joss-snippet
// CÓDIGO REAL:
public func showLogin() {
    (!Auth::guest()) ? { return redirect("/dashboard") } : {}
    SEO::title("Iniciar Sesión — Joss Red")
    return view("auth.login", { ... })
}
```
**Diagnóstico:** O desenvolvedor escreve `: {}` no final de cada ternário condicional para ter certeza de que o compilador não interpretará a próxima linha como uma continuação do operador. ### Evidência 3: Transformações processuais desnecessárias em [BackupController.joss:70-97](file:///c:/Users/Asus/Documents/proyectos/Joss-language/ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss#L70-L97):```joss-snippet
// CÓDIGO REAL:
$files = $db->table("backups")->where("user_token", $userToken)->get()
$filesList = $files
$mapped = []

foreach ($filesList as $file) {
    $parts = explode("/", $file["file_name"])
    $fileName = end($parts)
    
    $mapped[] = {
        "id": $file["id"],
        "name": $fileName
    }
}
return json({"files": $mapped})
```
**Diagnóstico:** 16 linhas de código processual com acumulador mutável `$mapped[] = ...` para executar uma operação que é conceitualmente uma única transformação de mapeamento. --- ## 6. Padrão e Cerimônia Identificados | Padrão Cerimonial | Impacto nas Linhas | Causa subjacente | Solução Proposta | |---|:---:|---|---| | **Branch `: {}` em declarações condicionais** | 1 linha por condicional | Sintaxe ternária forçada para controle de fluxo | Introdução de `guard` ou declaração condicional simples | | **Parâmetros marcados como `mixed`** | 1 por parâmetro | Falta de estreitamento de fluxo em variáveis ​​com tipos de união | Estreitamento automático do escopo (smart casts) | | **Extração manual de nome/extensão de arquivo** | 3-4 linhas | Ausência de métodos em strings ou classe utilitária `Path` | `$archivo->extension()` ou `Path::extension($archivo)` | | **Instanciação repetitiva `new GranDB()`** | 1 linha por consulta | Acesso a instâncias não unificadas com fachadas estáticas | Unifique em `GranDB::table(...)` | | **Mapeamento cumulativo `$acc[] = ...`** | 10-15 linhas por loop | Matrizes sem métodos funcionais fluentes | `$files->map(func($f) => ...)` | | **Verificação manual da existência de pré-leitura** | 2-3 linhas | Falta de métodos seguros em dicionários/mapas | `$map->get("clave", "default")` | --- ## 7. Inconsistências de idioma ### 1. Inconsistência furtiva `$` - Variáveis ​​locais: obrigatórias (`$usuario = new Usuario()`). - Parâmetros: Obrigatórios (`func(string $nombre)`). - Propriedades em declaração: Obrigatório (`public string $nombre = ""`). - Acesso à propriedade da instância: **Proibido** (`$this->nombre`, não `$this->$nombre`). - Acesso à propriedade estática: **Obrigatório** (`Clase::$contador`). - Parâmetro Catch: **Obrigatório** (`catch ($e)`). ### 2. Duplicidade de Paradigmas em Bibliotecas Três estilos irreconciliáveis coexistem nas APIs integradas: - **Estilo PHP histórico:** `str_contains`, `str_replace`, `array_keys`, `file_get_contents`. - **Resumo de estilo moderno:** `contains`, `keys`, `values`, `file_read`. - **Estilo C++:** Operadores de streaming `cout << $val` e `cin >> $var`. - **Estilo Web Fachada:** `Response::json()`, `View::render()`, `Request::input()`. ### 3. Requisito de digitação assimétrica - Em vigor: `func($x)` é estritamente proibido (`JOSS-TYPE-011`); requer `func(mixed $x)`. - Em `catch`: `catch (MiError $e)` gera um erro de sintaxe do compilador; requer estritamente `catch ($e)`. ### 4. Indexação fora do intervalo - Em arrays e strings: `$arr[99]` causa um **pânico fatal em tempo de execução** (`JOSS-INDEX-001`). - Em mapas: `$map["clave_inexistente"]` retorna **`null` silenciosamente**. --- ## 8. Complexidade Conceitual Atualmente, um desenvolvedor deve memorizar um número desnecessário de regras para tarefas idênticas:```text
DECLARACIÓN DE VARIABLES (7 Formas Convivientes):
  1. $x = 10         (inferencia fija)
  2. var $x = 10     (inferencia fija explícita)
  3. int $x = 10     (tipo estático estricto)
  4. let int $x = 10 (tipo estático con prefijo let)
  5. let $x = 10     (variable dinámica mixed)
  6. mixed $x = 10   (variable dinámica explícita)
  7. const $x = 10   (inmutable)
```
**Proposta de unificação:** Reduzir para três formas intuitivas e previsíveis: - `$x = 10` ou `var $x = 10`: Inferência fixa do valor atribuído. - `int $x = 10`: tipo estático declarado explicitamente. - `mixed $x = 10`: Dinamismo voluntário (recomendando `let $x`). - `const $x = 10`: Constante imutável. --- ## 9. Digite System Audit O núcleo de digitação (`pkg/typesystem/types.go`) possui bases matemáticas sólidas: - Promoção numérica estrita: `int` → `float` → `decimal`. - Operações seguras com tipos de união normalizados (`T|U`) e opcionais (`T?` → `T|null`). - Compatibilidade nominal de classes e interfaces. ### A falha crítica: refinamento de escopo isolado Em `pkg/analyzer/infer.go:929` (`narrowScopeFromCondition`), o estreitamento de tipos anuláveis ​​opera através da criação de escopos isolados:```go
trueScope := newScope(current)
falseScope := newScope(current)
```
Esses escopos **afetam apenas as expressões localizadas no ternário**. Depois que a análise continua para a próxima instrução no bloco principal, a variável retorna ao seu tipo original com `null`.```joss-snippet
public func formatearNombre(string? $nombre): string {
    ($nombre == null) ? {
        return "Anónimo"
    }

    // EL COMPILADOR FALLA AQUÍ:
    // $nombre sigue siendo inferido como string|null.
    return $nombre // JOSS-TYPE-008: se esperaba 'string', se obtuvo 'string|null'.
}
```
Esta limitação impede a escrita de cláusulas de proteção idiomáticas e leva ao abuso de `mixed` em projetos de software reais. --- ## 10. Auditoria de segurança e defesas de tempo de execução ### Defesas validadas e bem-sucedidas: - **Detecção de overflow:** `CheckedIntBinary` bloqueia adições ou produtos que excedem o intervalo `int64`. - **Divisão controlada por zero:** Inicia diagnósticos imediatos sem produzir estados de memória corrompidos. - **Proteção de recursão infinita:** Limite máximo configurável de frames de chamada. ### Lacunas e vulnerabilidades ocultas: 1. **O operador `??` oculta erros críticos:** - Em [evaluator_infix.go:54-60](file:///c:/Users/Asus/Documents/proyectos/Joss-language/pkg/core/evaluator_infix.go#L54-L60):   ```go
   if ie.Operator == "??" {
       var left interface{}
       func() {
           defer func() {
               if rec := recover(); rec != nil {
                   left = nil
               }
           }()
           left = r.evaluateExpression(ie.Left)
       }()
   ```
Qualquer exceção lançada no branch esquerdo (divisão por zero, overflow ou exceção de banco de dados) é capturada silenciosamente e substituída pelo valor padrão, impossibilitando o diagnóstico de bugs na produção. 2. **Rebaixamento silencioso de caracteres UTF-8 no Lexer:** - Em [lexer.go:327-332](file:///c:/Users/Asus/Documents/proyectos/Joss-language/pkg/parser/lexer.go#L327-L332), o lexer descarta sem notificação quaisquer bytes maiores que 127 fora dos literais de string. Um identificador como `$año` é silenciosamente transformado em `$ao`. --- ## 11. Auditoria e diagnóstico de erros O subsistema `pkg/diagnostics` é de alta qualidade arquitetural, mas requer maior empatia contextual: 1. **Mensagens tecnicamente corretas, mas não de orientação:** - *Atual:* `error[JOSS-TYPE-001] app.joss:15:5: Asignación incompatible: se esperaba 'int', se obtuvo 'string'.` - *Orientação:* `error[JOSS-TYPE-001] app.joss:15:5: No se puede asignar 'string' a la variable '$edad' porque fue inferida como 'int' en la línea 4. Sugerencia: Modifica el valor asignado o declara explícitamente 'mixed $edad'.` 2. **Cascata de erros no analisador:** - Na ausência de um delimitador ou chave, o analisador Pratt emite vários diagnósticos derivados. É uma prioridade sincronizar o analisador com o seguinte `SEMICOLON` ou `NEWLINE`. --- ## 12. Auditoria de biblioteca padrão (Stdlib) ### Principais oportunidades de redesenho: 1. **Unificação de strings e coleções orientadas a objetos:** - Substitua funções aninhadas por chamadas fluidas:     ```joss-snippet
     // Actual:
     $limpio = trim(strtolower(substr($texto, 0, 10)))

     // Propuesto:
     $limpio = $texto->slice(0, 10)->lower()->trim()
     ```
2. **Avaliação preguiçosa do operador Range (`..`):** - Modifique `evaluator_infix.go:458` para que `1..1000000` retorne um iterador leve em vez de alocar imediatamente uma matriz de um milhão de elementos no heap. --- ## 13. Auditoria de ferramentas e ecossistemas - **`joss run` e `joss analyze`:** Eles funcionam perfeitamente, garantindo que nenhum código com erros de análise semântica seja executado. - **Extensão de código VS (`vscode-joss`):** Duplica a análise de caminho e a lógica de validação de sintaxe no TypeScript. Deve evoluir para um servidor de linguagem puro em Go, desenvolvido por `pkg/analyzer`. --- ## 14. Auditoria Oficial do Formatador - O arquivo `pkg/formatter/scanner.go` mantém uma tabela de tokens paralela que diverge de `pkg/parser/token.go`. - O formatador deve ser refatorado para operar diretamente na sequência de tokens gerada pelo lexer canônico, preservando quebras de linha intencionais e aplicando formatação canônica estrita à la `gofmt`. --- ## 15. Analisador e Auditoria Linter Recomenda-se segregar claramente os níveis de severidade: - **Analyzer:** Valida a correção do programa (tipos, símbolos, terminação de chamada, invariantes). Transmite exclusivamente `Error` e `Warning`. - **Linter:** Avalia estilo e boas práticas (convenção de nomenclatura, detecção de chave em código rígido `JOSS-SEC-001`). Edições `Warning` e `Info`. --- ## 16. Comparação seletiva com outros idiomas | Idioma | Abordagem de controle de fluxo | Gestão de Nulabilidade | Lição para Joss | |---|---|---|---| | **Dardo** | Mantém o tradicional `if/else` | Som Nulo Segurança com Promoção de Fluxo | Permite que você escreva código linear enquanto o compilador remove `null` após salvar. | | **Vá** | Declarações simples `if err != nil` | Ponteiros de erro explícitos e tuplas | O código linear é mais legível que as expressões aninhadas. | | **Rápido** | Frase obrigatória `guard cond else { return }` | Opcionais estritos (`T?`) | A cláusula `guard` garante a saída da função sem recuo de pirâmide. | | **Kotlin** | `if` é expressão; suporta *elencos inteligentes* | Tipos anuláveis ​​`T?` com operador Elvis `?:` | Se uma variável for verificada em `null`, o compilador deverá atualizar seu tipo automaticamente. | --- ## 17. Oportunidades de simplificação 1. **Remover aliases obsoletos das funções:** Descontinuar gradualmente os prefixos `str_` e `array_`. 2. **Desencorajar `let $x` em favor de `mixed $x`:** Remova a ambiguidade conceitual sobre a mutabilidade de tipo. 3. **Unifique Fachadas e Auxiliares:** Certifique-se de que `view()` e `View::render()` compartilhem o mesmo contrato formal de assinatura. --- ## 18. Recursos que NÃO devem ser implementados Recomenda-se rejeitar explicitamente: - ❌ **Gráficos de Importações Tradicionais:** Preservar a simplicidade do modelo Zero-Importações. - ❌ **Genéricos complexos de ordem superior:** Mantenha a parametrização limitada a `array<T>` e `map<K, V>`. - ❌ **Herança Múltipla ou Características Complexas:** Preserve a herança simples com interfaces limpas.- ❌ **Função de coloração em assíncrono:** Proíbe a exigência de palavras-chave `async func`. --- ## 19. Catálogo de Propostas Priorizadas (P1 a P8) Segue abaixo o catálogo técnico de propostas de evolução da linguagem Joss, priorizadas de acordo com seu impacto na redução da complexidade acidental, eliminação de código defensivo e melhoria direta da produtividade e segurança do desenvolvedor. ### Matriz de Decisão e Priorização | ID | Proposta | Problema Principal | Benefício | Complexidade | Risco | Compatibilidade | Prioridade | Subsistema | |:---|:---|:---|:---|:---:|:---:|:---:|:---:|:---| | **P1** | **Frase `guard`** | Aninhamento de pirâmides por validações anteriores | Fluxo de controle linear e leitura sequencial | Médio | Baixo | 100% compatível | **P0** | Analisador, Analisador, Avaliador | | **P2** | **Propagação de restrição de tipo** | Perda de refinamento da taxa após saídas antecipadas | Elimine afirmações e verificações redundantes | Médio | Baixo | 100% compatível | **P0** | Analisador (`flow.go`, `infer.go`) | | **P3** | **Métodos Fluidos em Primitivos** | Atrito devido à mistura entre PHP processual e classes estáticas | API moderna, preenchimento automático e encadeamento | Médio | Baixo | 100% compatível | **P1** | Avaliador, Stdlib, Analisador | | **P4** | **Preservação de Objetos em `catch`** | Exceções rebaixadas para `string` flat in catch | Tratamento de erros digitados, acesso a rastreamento e metadados | Baixo | Mínimo | 100% compatível | **P0** | Avaliador (`executor.go`) | | **P5** | **Segurança de coalescência `??`** | Silenciamento indiscriminado de pânicos reais | Detecção imediata de bugs lógicos e divisões por zero | Baixo | Baixo | Compatível (correção semântica) | **P1** | Avaliador (`evaluator_infix.go`) | | **P6** | **Desestruturação Declarativa** | 5 a 10 linhas de descompactação repetitivas por controlador | Redução de 60% na alocação padrão | Médio | Baixo | 100% compatível | **P1** | Analisador, Analisador, Avaliador | | **P7** | **Promoção de imóvel em `Init`** | Declaração quádrupla de propriedades em classes e DTOs | Definição concisa e declarativa de modelos | Médio | Mínimo | 100% compatível | **P1** | Analisador, Analisador, Núcleo (`classes.go`) | | **P8** | **Mecanismo AST de correção automática (`joss fix`)** | Dívida técnica e correções manuais de sintaxe | Migração e modernização automáticas com esforço 0 | Médio | Baixo | Ferramenta CLI externa | **P0** | CLI, fixador, formatador | --- ### P1: Declaração de guarda e controle de fluxo plano (`guard`) #### 1. Problema atual Nos controladores, middlewares e serviços Joss, os métodos exigem a verificação de múltiplas pré-condições (autenticação, existência de registros no banco de dados, validade de tokens, presença de arquivos carregados). Como os desenvolvedores costumam usar ternários com blocos ou condicionais aninhados para evitar retornos duplicados, o código entra em colapso na chamada "pirâmide da destruição". Cada validação adiciona um nível extra de recuo e encerra a lógica principal dentro do corpo da ramificação falsa ou verdadeira.#### 2. Evidência em código real - `ejemplos/Joss-Red-JosSecurity/app/controllers/vault/BackupController.joss`: Até 5 níveis de aninhamento ternários consecutivos para verificar `$user`, permissões de função, existência de backup e parâmetros de solicitação. - `ejemplos/Joss-Red-JosSecurity/app/controllers/api/ApiRepositoryController.joss` (linhas 60-80): Três ternários são aninhados para verificar se o arquivo existe, se o JSON está analisado e se os campos opcionais estão presentes. - `ejemplos/Joss-Red-JosSecurity/app/middleware/MiddlewareLoader.joss` (linhas 30-93): Blocos de validação defensivos que forçam saltos de leitura. #### 3. Impacto no desenvolvedor - **Legibilidade gravemente degradada:** A lógica de negócios feliz (*caminho feliz*) está oculta no nível mais profundo de recuo. - **Risco de erros na refatoração:** Modificar um bloco aninhado requer o ajuste de colchetes emparelhados com dezenas de linhas de distância. - **Dificuldade de auditoria:** À primeira vista, não é óbvio quais são os pré-requisitos para um endpoint executar sua lógica central. #### 4. Solução técnica proposta Incorpore a frase `guard (condición) else { ... }`. - **Semântica:** A condição deve ser avaliada como booleana. Se a condição for verdadeira, a execução continua imediatamente na próxima instrução no mesmo nível de indentação. Se for falso, o bloco `else` é executado. - **Invariante estático:** O analisador semântico (`pkg/analyzer`) requer exaustivamente que o bloco `else` de um `guard` encerre o fluxo da função (via `return`, `throw` ou saída do terminal). Se o bloco `else` não interromper o fluxo, o compilador emitirá um erro estático `JOSS-FLOW-005`. #### 5. Código Atual vs. Código Proposto```joss-snippet
// --- ACTUAL (Anidamiento y ramas de escape complejas) ---
public func download(mixed $id) {
    $item = GranDB::table("repos")->where("id", $id)->first()
    return (!$item) ? json({"error": "No encontrado"}, 404) : {
        $user = Auth::user()
        return (!$user) ? json({"error": "No autenticado"}, 401) : {
            $path = $item["file_path"]
            return (!file_exists($path)) ? json({"error": "Archivo perdido"}, 404) : {
                return Response::download($path)
            }
        }
    }
}

// --- PROPUESTO (Flujo plano y lectura lineal con guard) ---
public func download(mixed $id) {
    $item = GranDB::table("repos")->where("id", $id)->first()
    guard ($item != null) else {
        return json({"error": "No encontrado"}, 404)
    }

    $user = Auth::user()
    guard ($user != null) else {
        return json({"error": "No autenticado"}, 401)
    }

    $path = $item["file_path"]
    guard (file_exists($path)) else {
        return json({"error": "Archivo perdido"}, 404)
    }

    return Response::download($path)
}
```
#### 6. Subsistema afetado - `pkg/parser`: Nova palavra-chave `guard`, nó AST `GuardStatement`. - `pkg/analyzer`: validação de condição booleana e verificação de conclusão garantida no bloco `else` usando `flow.go:blockTerminatesCallable`. - `pkg/core`: Avaliação em `executor.go` (se `isTruthy(cond)`, continue; caso contrário, avalie `elseBlock`). #### 7. Estimativa e Compatibilidade - **Benefício:** Muito Alto (Elimina 80% de aninhamento acidental em controladores). - **Dificuldade:** Médio. - **Risco:** Baixo. - **Compatibilidade:** 100% compatível com versões anteriores (palavra-chave contextual ou reservada com verificação em `token.go`). - **Prioridade:** **P0**. --- ### P2: Propagação do Estreitamento de Tipos no Escopo Principal (*Smart Casts*) #### 2.1. Problema atual O sistema de tipos Joss implementa refinamento de tipo (`narrowScopeFromCondition` em `pkg/analyzer/infer.go`), mas apenas dentro do corpo interno de uma ramificação `if` ou no braço verdadeiro/falso de um operador ternário. Quando um desenvolvedor valida o nulo de uma variável no início de uma função e retorna imediatamente se for nulo, o analisador semântico **esquece o refinamento** nas linhas subsequentes do escopo principal. #### 2.2. Evidência no Código Real - `ejemplos/Joss-Red-JosSecurity/app/controllers/auth/ProfileController.joss` (linha 18-28):  ```joss-snippet
  $u = Auth::user() // Tipo inferido: User|null
  (!$u) ? { return redirect("/login") }
  // En las siguientes líneas, $u->email o $u->first_name siguen considerando User|null
  ```
- `pkg/analyzer/infer.go` (linhas 90-96): `narrowScopeFromCondition` gera dois escopos filhos isolados (`trueScope`, `falseScope`), mas nenhum deles retorna ao escopo pai `current`. #### 23. Impacto no desenvolvedor - O desenvolvedor é forçado a usar `mixed` em variáveis ​​para silenciar os avisos do analisador. - Causa desconfiança no sistema de tipos, incentivando verificações defensivas duplicadas (`if ($u != null && $u->email)`) em todo o mesmo método. #### 2.4. Solução técnica proposta Integrar análise de encerramento de fluxo (`flow.go`) com refinamento de escopo em `infer.go`: - Quando uma instrução condicional (`if`, `guard`, ou instrução ternária) demonstra de forma abrangente que seu escapar da ramificação encerra a execução da função (`return`, `throw`), o escopo pai subsequente assume o tipo refinado da ramificação sem escape. - Exemplo: se `$x` for `User|null` e você executar `if ($x == null) { return }`, o tipo de `$x` no escopo principal se tornará automaticamente `User`. #### 2.5. Código Atual vs. Código Proposto```joss-snippet
// --- ACTUAL: Analyzer reporta posible desreferencia nula en $user->email ---
public func getEmail(): string {
    User|null $user = Auth::user()
    if ($user == null) {
        return ""
    }
    // El analyzer todavía considera que $user puede ser null
    return $user->email // Genera fricción o exige let mixed
}

// --- PROPUESTO: Smart Cast automático en flujo secuencial ---
public func getEmail(): string {
    User|null $user = Auth::user()
    if ($user == null) {
        return ""
    }
    // Type Narrowing propagado al scope exterior: $user promovido a User estricto
    return $user->email // 100% tipado, autocompletado y validado
}
```
#### 2.6. Subsistema Afetado - `pkg/analyzer/flow.go`: Exponha `statementTerminatesCallable(stmt parser.Statement) bool`. - `pkg/analyzer/infer.go`: Em `inferStatement`, se um bloco condicional tiver escape, aplique as mutações de tipo de `narrowScopeFromCondition` em `currentScope`. #### 2.7. Estimativa e Compatibilidade - **Benefício:** Muito Alto (O sistema de tipos funciona a favor do desenvolvedor, não contra ele). - **Dificuldade:** Médio. - **Risco:** Baixo. - **Compatibilidade:** 100% compatível com versões anteriores (não invalida o código válido existente; apenas resolve falsos positivos). - **Prioridade:** **P0**. --- ### P3: Métodos de instância fluentes em primitivos (`string`, `array`, `map`) #### 3.1. Problema atual Atualmente existe uma dicotomia confusa na biblioteca padrão e nos tipos básicos: 1. Funções globais de estilo processual herdadas do PHP (`strlen`, `str_contains`, `substr`, `array_keys`, `array_push`, `json_encode`). 2. Classes nativas com métodos estáticos (`Str::contains`, `Str::length`, `Arr::has`, `JSON::encode`). O desenvolvedor deve memorizar constantemente quando chamar uma função global, quando usar uma classe estática e em que ordem os parâmetros vão (por exemplo, `$needle` vs `$haystack`). As transformações de dados encadeadas tornam-se ilegíveis pelo aninhamento de chamadas internas. ####3.2. Evidência no Código Real - `ejemplos/Joss-Red-JosSecurity/app/services/PageBuilderService.joss`:  ```joss-snippet
  $clean = Str::trim(Str::lower(Str::replace($input, " ", "-")))
  ```
A leitura é feita de dentro para fora, exigindo 3 chamadas estáticas para uma operação trivial em uma string. - `ShopController.joss`: Lista manipulações que combinam `count($items)`, `array_slice($items, ...)` e `Arr::map(...)`. ####3.3. Impacto do desenvolvedor – Atrito cognitivo contínuo causado pela alternância entre estilos sintáticos incompatíveis. - Falta de preenchimento automático natural no editor: Ao escrever `$cadena->`, o LSP não pode oferecer métodos de transformação fluidos porque os primitivos não expõem métodos de instância. ####3.4. Solução técnica proposta Permite a invocação de métodos de instância virtual diretamente em valores do tipo `string`, `array` e `map`: - `$cadena->trim()->lower()->replace(" ", "-")` - `$lista->map(fn($x) => $x * 2)->filter(fn($x) => $x > 10)->join(", ")` - `$mapa->keys()`, `$mapa->values()`, `$mapa->has("clave")` - **Implementação gratuita:** Em `pkg/core/evaluator_member.go`, se o receptor for um valor primitivo Go nativo (`string`, `[]interface{}`, `map[string]interface{}`), despacha internamente para os avaliadores já otimizados de `StrMethods` e `ArrMethods` sem criar objetos wrapper intermediários. ####3.5. Código Atual vs. Código Proposto```joss-snippet
// --- ACTUAL: Llamadas estáticas anidadas de adentro hacia afuera ---
public func slugify(string $title): string {
    return Str::lower(Str::trim(Str::replace($title, " ", "-")))
}

// --- PROPUESTO: Encadenamiento fluido natural de izquierda a derecha ---
public func slugify(string $title): string {
    return $title->trim()->replace(" ", "-")->lower()
}
```
####3.6. Subsistema Afetado - `pkg/core/evaluator_member.go`: Intercepta chamadas de membros em tipos e índices não instanciados em despachantes primitivos nativos. - `pkg/analyzer/infer.go`: Declare assinaturas de retorno para membros dos tipos `String`, `Array` e `Map`. - `tools/cataloggen`: Exporte métodos primitivos para o catálogo de preenchimento automático do VS Code. ####3.7. Estimativa e Compatibilidade - **Benefício:** Muito Alto (Moderniza drasticamente a ergonomia da linguagem). - **Dificuldade:** Médio. - **Risco:** Baixo. - **Compatibilidade:** 100% compatível. Funções globais e classes estáticas continuam a existir sem alterações. - **Prioridade:** **P1**. --- ### P4: Preservação de instâncias e exceções de objetos em `catch` #### 4.1. Problema atual Na implementação atual do tempo de execução do Joss (`pkg/core/executor.go`, linha 459), quando uma exceção é capturada usando um bloco `try / catch ($ex)`, o valor retornado é convertido à força em uma string de texto simples usando `fmt.Sprint(evalErr.Value)`. Se o desenvolvedor lançou uma instância de classe (`throw new ValidationException("Error", 422, $errores)`), o bloco `catch` recebe uma string vazia ou uma representação formatada inerte (`"Instance of ValidationException"`), em vez da instância ativa do objeto. #### 4.2. Evidência no Código Real - `pkg/core/executor.go` (linha 459):  ```go
  r.currentEnvironment.Set(node.Variable.Value, fmt.Sprint(evalErr.Value))
  ```
- `ejemplos/Joss-Red-JosSecurity/app/controllers/web/FlaskController.joss` (linhas 28-30):  ```joss-snippet
  } catch ($ex) {
      return json({"error": "Plugin error: " . $ex}, 500)
  }
  ```
Os desenvolvedores não podem acessar `$ex->getCode()` ou inspecionar propriedades específicas porque `$ex` é sempre uma string. #### 4.3. Impacto no desenvolvedor – Incapacidade de implementar tratamento granular de exceções por tipo (`if ($ex instanceof NotFoundException)`). - Perda irremediável do stack trace, códigos de status HTTP associados, metadados e contexto de falha estruturada. #### 4.4. Solução técnica proposta - Em `pkg/core/executor.go`, atribua diretamente o valor `evalErr.Value` (seja `*Instance`, `error`, mapa ou string) ao quadro do ambiente lexical do bloco `catch`. - Se o valor gerado for um erro genérico ou uma string, envolva-o de forma transparente em uma instância da classe base nativa `Exception` com os métodos `$ex->getMessage()` e `$ex->getFile()`. #### 4.5. Código Atual vs. Código Proposto```joss-snippet
// --- ACTUAL: $ex es forzado a string plano, sin métodos ni propiedades ---
try {
    PaymentGateway::charge($amount)
} catch ($ex) {
    // $ex es string: "CardDeclinedException"
    // $ex->getCode() provoca error de runtime
    return json({"error": $ex}, 500)
}

// --- PROPUESTO: $ex preserva la instancia original lanzada ---
try {
    PaymentGateway::charge($amount)
} catch ($ex) {
    if ($ex instanceof CardDeclinedException) {
        return json({"error": $ex->getMessage(), "decline_code": $ex->declineCode}, 402)
    }
    return json({"error": $ex->getMessage()}, 500)
}
```
#### 4.6. Subsistema Afetado - `pkg/core/executor.go`: Substitua a coerção `fmt.Sprint` por atribuição direta de valor e empacotamento consistente em `Exception`. - `pkg/core/classes.go`: certifique-se de que a classe base `Exception` oferece métodos canônicos `getMessage()`, `getCode()`, `getLine()`, `getFile()`. #### 4.7. Estimativa e Compatibilidade - **Benefício:** Crítico (restaura a integridade do paradigma orientado a objetos no gerenciamento de erros). - **Dificuldade:** Baixa. - **Risco:** Mínimo. - **Compatibilidade:** 100% compatível (concatenar `$ex` como string ainda funciona graças a `CoerceString`). - **Prioridade:** **P0** (Correção imediata). --- ### P5: Segurança no Operador Coalescente `??` e Resgate com Silenciador Sem Pânico #### 5.1. Problema atual O operador de coalescência nula `??` foi projetado para fornecer um valor alternativo quando uma expressão é avaliada como `null` ou acessa um índice/propriedade indefinido em uma coleção. No entanto, na implementação atual (`pkg/core/evaluator_infix.go`, linhas 88-94), a avaliação do operando esquerdo é envolvida em um `recover()` que captura qualquer pânico Go ou exceção Joss, silenciando erros graves de programação, como divisão por zero, tipos inválidos ou erros de lógica interna. #### 5.2. Evidência no Código Real - `pkg/core/evaluator_infix.go` (linhas 88-94):  ```go
  defer func() {
      if r := recover(); r != nil {
          result = r.evaluateExpression(ie.Right)
      }
  }()
  ```
- Se um desenvolvedor digitar `$val = ($total / $count) ?? 0` e `$count` for 0, em vez de alertar sobre a falha ou divisão proibida, o operador oculta silenciosamente o erro e retorna o valor correto. #### 5.3. Impacto no desenvolvedor - **Bugs silenciosos que são difíceis de depurar:** Defeitos críticos em algoritmos passam despercebidos porque `??` captura quaisquer exceções que ocorrem na avaliação de expressões complexas em seu lado esquerdo. - Viola o princípio da menor surpresa e as garantias de robustez de Joss. #### 5.4. Solução técnica proposta - Refatore o avaliador `??` para que ele recupere apenas erros de valor faltante (`UndefinedVariable`, `MissingKeyError` ou valor de retorno igual a `nil`/`NullValue`). - Erros fatais de tempo de execução (exceções lançadas explicitamente, erros de invocação de método inexistentes ou falhas de tipo estrito) não devem ser consumidos por `??` e devem ser propagados para o manipulador de erro superior ou bloquear `catch`. #### 5.5. Código Atual vs. Código Proposto```joss-snippet
// --- ACTUAL: Silenciamiento accidental de fallos de ejecución ---
// Si calculateDiscount() tiene un bug y arroja excepción, ?? lo oculta
$precioFinal = calculateDiscount($producto) ?? 0 // Devuelve 0 en silencio

// --- PROPUESTO: Coalescencia estricta solo ante null/no definido ---
// Si calculateDiscount() retorna null, asigna 0.
// Si calculateDiscount() arroja una excepción, la excepción se propaga y se diagnostica.
$precioFinal = calculateDiscount($producto) ?? 0
```
####5.6. Subsistema Afetado - `pkg/core/evaluator_infix.go`: Exclua o blind `recover()` em `evaluateNullCoalesceExpression` e avalie o valor verificando se é nulo ou índice não encontrado. ####5.7. Estimativa e Compatibilidade - **Benefício:** Alto (Evita falhas silenciosas na produção). - **Dificuldade:** Baixa. - **Risco:** Baixo. - **Compatibilidade:** Compatível (melhora a correção semântica sem quebrar o código idiomático). - **Prioridade:** **P1**. --- ### P6: Desestruturação Declarativa de Tuplas, Matrizes e Mapas #### 6.1. Problema atual Em aplicativos da web Joss (como o projeto JosSecurity real), os métodos de controlador e serviço recebem constantemente mapas ou tuplas com vários valores (dados de formulário, cabeçalhos, resultados de validação, segredos 2FA). Para extrair esses valores, o desenvolvedor é forçado a escrever de 5 a 10 mapeamentos repetitivos individuais linha por linha. #### 6.2. Evidência no Código Real - `ejemplos/Joss-Red-JosSecurity/app/controllers/auth/ProfileController.joss` (linhas 43-48):  ```joss-snippet
  $first_name = request("first_name")
  $last_name  = request("last_name")
  $phone      = request("phone")
  $password   = request("password")
  ```
- `AuthController.joss` (linhas 125-140): Descompactação manual de arrays retornados por autenticação e serviços 2FA (`$secret = $totp["secret"]`, `$qrCode = $totp["qr_url"]`). #### 6.3. Impacto no desenvolvedor – Grande volume de código cerimonial e repetitivo. - Aumento de erros de digitação na correspondência manual entre o nome da variável e a chave do mapa. #### 6.4. Solução técnica proposta Incorporar padrões de desestruturação (*atribuição de desestruturação*) em instruções de atribuição e declaração: 1. **Desestruturação de listas/tuplas por posição:** `[$id, $nombre, $rol] = $usuarioArray` 2. **Desestruturação de mapas por chave:** `{"email": $email, "password": $password} = request()` 3. **Valores padrão opcionais:** `{"role": $role = "cliente", "active": $active = true} = $data` #### 6.5. Código Atual vs. Código Proposto```joss-snippet
// --- ACTUAL: 6 líneas ceremoniales de extracción individual ---
public func registerUser() {
    $req = request()
    $name = $req["name"]
    $email = $req["email"]
    $password = $req["password"]
    $role = $req["role"] ?? "user"
    $terms = $req["terms"] ?? false
}

// --- PROPUESTO: 1 sola línea declarativa y expresiva ---
public func registerUser() {
    {"name": $name, "email": $email, "password": $password, "role": $role = "user", "terms": $terms = false} = request()
}
```
#### 6.6. Subsistema Afetado - `pkg/parser`: Suporte para padrões `ArrayPattern` e `MapPattern` no lado esquerdo das instruções de atribuição (`parser_statements.go`). - `pkg/analyzer`: Digitação e inferência de cada variável individual do tipo de contêiner (`infer.go`). - `pkg/core/executor.go`: Atribuição sequencial de slots do objeto iterado ou indexado. #### 6.7. Estimativa e Compatibilidade - **Benefício:** Muito Alto (Reduz o padrão do controlador em 60%). - **Dificuldade:** Médio. - **Risco:** Baixo. - **Compatibilidade:** 100% compatível (nova construção sintática sem conflitos lexicais). - **Prioridade:** **P1**. --- ### P7: Promoção de Imóveis no Builder (`Init`) #### 7.1. Problema Atual Para criar classes de domínio simples, DTOs (*Data Transfer Objects*), entidades ou serviços em Joss, o programador deve declarar o nome do campo em quatro locais diferentes: 1. Como uma propriedade de classe (`public string $name`). 2. Como parâmetro no construtor `Init(string $name)`. 3. Como uma atribuição para `$this` no corpo do construtor (`$this->name = $name`). 4. Na documentação ou tipos de retorno. #### 7.2. Evidência em código real - `ejemplos/Joss-Red-JosSecurity/app/services/LicenseService.joss`: Múltiplas classes de serviço com 5 ou mais propriedades atribuídas de forma idêntica no construtor. - `ejemplos/plugins/joss_ai/src/plugin.joss`: Repetição dos parâmetros de configuração atribuídos um a um a `$this->propiedad`. #### 7.3. Impacto do desenvolvedor - Resistência à criação de DTOs fortemente tipados devido à verbosidade cerimonial necessária para cada classe. - Refatorações lentas: renomear uma propriedade requer a modificação de vários pontos dentro do mesmo arquivo. #### 7.4. Solução técnica proposta Permite modificadores de visibilidade (`public`, `protected`, `private`) e constância (`const`) diretamente nos parâmetros da função construtora `Init`: - Ao declarar um parâmetro com visibilidade (por exemplo, `func Init(public string $titulo, private GranDB $db = new GranDB())`), o compilador e o tempo de execução declaram automaticamente a propriedade na classe e geram a atribuição `$this->titulo = $titulo` antes de executar o corpo de `Init`. #### 7.5. Código Atual vs. Código Proposto```joss-snippet
// --- ACTUAL: Cuádruple repetición del identificador ---
public class UserDTO {
    public int $id
    public string $email
    public string $role

    public func Init(int $id, string $email, string $role) {
        $this->id = $id
        $this->email = $email
        $this->role = $role
    }
}

// --- PROPUESTO: Constructor conciso con promoción de propiedades ---
public class UserDTO {
    public func Init(
        public int $id,
        public string $email,
        public string $role = "cliente"
    ) {}
}
```
#### 7.6. Subsistema afetado - `pkg/parser`: reconheça `public`, `protected`, `private` antes de digitar a lista de parâmetros de função `Init`. - `pkg/analyzer`: Sintetize as propriedades da classe a partir dos parâmetros promovidos. - `pkg/core/classes.go`: Instanciação automática de slots na construção do objeto. #### 7.7. Estimativa e compatibilidade - **Benefício:** Muito alto em ergonomia orientada a objetos e arquitetura limpa. - **Dificuldade:** Médio. - **Risco:** Mínimo. - **Compatibilidade:** 100% compatível. A sintaxe tradicional de `Init` ainda funciona exatamente da mesma forma. - **Prioridade:** **P1**. --- ### P8: Mecanismo de autofixação mecânica baseado em AST (`joss fix`) #### 8.1. Problema Atual A evolução de uma linguagem inevitavelmente gera dívida técnica em projetos reais quando os padrões são modernizados (como as 370 filiais vazias `: {}` que acabamos de limpar no JosSecurity, ou a exigência de visibilidade explícita em funções e métodos globais). Atualmente, os desenvolvedores contam com pesquisas manuais de expressões regulares, o que apresenta riscos de modificação de texto em strings literais ou comentários. ####8.2. Evidência em código real - A presença massiva de `: {}` ramificações em 65 arquivos no JosSecurity demonstrou que os programadores mantêm velhos hábitos sintáticos na ausência de uma ferramenta oficial de modernização. - Vários diagnósticos de compilador estável (`JOSS-VIS-001`, `JOSS-TYPE-009`, `JOSS-DEPR-001`) já calculam sugestões de soluções precisas (`Diagnostic.Suggestion`), mas hoje elas só são impressas no console sem poder ser aplicadas automaticamente na fonte código. ####8.3. Impacto no Desenvolvedor – Atrito e atraso na adoção de novas versões do Joss. - Medo de refatorar ou atualizar o compilador devido à carga de trabalho manual de correção de avisos ou descontinuações de estilo. ####8.4. Solução técnica proposta Consolidar o comando CLI `joss fix [directorio]` como um mecanismo de reescrita mecânica baseado diretamente na árvore de sintaxe abstrata (AST) e na tabela de diagnósticos: 1. **Detecção estruturada:** O analisador semântico identifica diagnósticos que possuem uma sugestão `FixAvailable` ou padronizada. 2. **Transformação segura:** Em vez de substituir o texto usando expressões regulares, o fixador substitui nós específicos no AST ou aplica deltas de texto delimitados por intervalos exatos de tokens (`Token.StartLine`, `Token.StartCol`, `Token.EndCol`). 3. **Formatação preservada:** Após a conclusão da aplicação das correções, invoca automaticamente o mecanismo `pkg/formatter` para garantir que o recuo e o estilo do projeto permaneçam consistentes. 4. **Regras iniciais suportadas em `joss fix`:** - Remoção automática de ramificações vazias redundantes `: {}`. - Inserção de modificadores automáticos de visibilidade `public` onde faltam. - Substituição de funções globais obsoletas por chamadas canônicas para classes estáticas (`str_contains` -> `Str::contains`).- Normalização de tipos históricos obsoletos (`list` -> `array`, `dynamic` -> `mixed`). ####8.5. Exemplo de fluxo de trabalho de terminal```bash
# Diagnosticar problemas corregibles automáticamente en el proyecto
joss check ./app

# Aplicar correcciones mecánicas automáticas con informe detallado
joss fix ./app

# Salida esperada:
# [joss fix] Analizando 65 archivos...
# [joss fix] Eliminadas 370 ramas ternarias vacías ': {}' innecesarias.
# [joss fix] Actualizadas 14 llamadas deprecadas a métodos canónicos de Str/Arr.
# [joss fix] Formateado completado satisfactoriamente. 0 errores restantes.
```
####8.6. Subsistema afetado - `cmd/joss/fix.go`: subcomando CLI e orquestrador de projeto. - `pkg/fixer/`: Mecanismo de reescrita de token delta no AST. - `pkg/diagnostics`: Incorporação do campo opcional `TextEdit` em `Diagnostic`. ####8.7. Estimativa e Compatibilidade - **Benefício:** Extraordinário para a saúde do ecossistema e fidelidade do desenvolvedor. - **Dificuldade:** Médio. - **Risco:** Baixo. - **Compatibilidade:** 100% compatível (ferramenta opt-in que não modifica a semântica da linguagem). - **Prioridade:** **P0** (Essencial para o ciclo de vida da linguagem). --- --- ## 20. Mudanças que exigiriam depreciação - Sinalize com aviso de diagnóstico `JOSS-DEPR-001` os nomes procedimentais de funções globais que duplicam nomes modernos (`str_contains`, `array_keys`, `file_get_contents`). - Ofereça autofix mecânico através de `joss fix`. --- ## 21. Possíveis alterações de ruptura justificadas 1. **Restrição de `null == ""` para `false`:** Nenhum tipo nulo deve ser avaliado como equivalente a uma string vazia sob igualdade ordinária. 2. **Propagação do Pânico em `??`:** Erradique a captura silenciosa de erros fatais no ramo esquerdo do operador. --- ## 22. Proposta e Arquitetura de `joss fix` Modernizar a ferramenta CLI para usar transformações de árvore de sintaxe (reescrita AST) em vez de regex: - Inserção automática de modificadores de visibilidade necessários. - Substituição de chamadas a funções obsoletas por seus equivalentes canônicos. - Remoção automática de filiais `: {}` redundantes e vazias. - Lançamento automático do formatador oficial após a conclusão do reparo. --- ## 23. Definição de "Código Joss Idiomático" O código Joss idiomático é definido pelas seguintes características: 1. **Digitação Declarativa e Segura:** Contratos públicos explícitos, inferência limpa em variáveis ​​locais e uso de `mixed` reservado para limites de entrada dinâmicos. 2. **Fluxo Linear sem Nesting:** Uso disciplinado de cláusulas de guarda com rescisão antecipada. 3. **Expressividade Fluida:** Uso do operador de pipeline `|>` e chamadas encadeadas em coleções. 4. **Tratamento de erros estruturado:** Exceções representadas como objetos de domínio, evitando strings de texto simples. --- ## 24. Roteiro Recomendado (Fases 0 a 5) | Fase | Título | Principais Iniciativas | Prioridade | Impacto | |---|---|---|:---:|---| | **Fase 0** | **Correções Imediatas** | Preservar instâncias em `catch ($e)`; não silencie pânicos fatais em `??`; relatar caracteres UTF-8 inválidos no lexer. | **P0** | Crítico | | **Fase 1** | **Vitórias rápidas** | Estreitamento sensível ao fluxo após retornos no analisador; correção de `isFalsy` (tratar `0.0` e `{}` como falso); coerção de chaves numéricas em mapas. | **P0** | Muito alto | | **Fase 2** | **Ferramentas Unificadas** | Refatore `pkg/formatter` usando o lexer canônico; modernize `joss fix` com reescrita AST. | **P1** | Alto || **Fase 3** | **Ergonomia das Coleções** | Métodos de instância nativa em strings e arrays; operador preguiçoso `..` (iterador lento). | **P1** | Alto | | **Fase 4** | **Evolução da linguagem** | Julgamento `guard (...) else { ... }`; suporte para `catch (TipoException $e)` e bloco `finally`. | **P2** | Muito alto | | **Fase 5** | **Arquitetura Futura** | Servidor LSP unificado nativo em Go; conexão gradual de `pkg/vm` para otimização de bytecode. | **P3** | Estratégico | --- *Fim do Documento de Auditoria 2026. Este relatório constitui a base de referência oficial para a tomada de decisões de design e evolução técnica do Joss.*