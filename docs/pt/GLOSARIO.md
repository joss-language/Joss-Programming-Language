# Glossário

[Índice](README.md) · [Primeiros passos](PRIMEROS_PASOS.md) · [Referência](SINTAXIS.md)

Estas palavras também são explicadas onde são introduzidas. Você pode voltar aqui
sem interromper o percurso de aprendizado.

| Termo | Significado nesta documentação |
|---|---|
| Programa | Instruções salvas em arquivos para que um computador realize uma tarefa. |
| Projeto | Pasta organizada criada com `joss new` ou `joss new console` que contém seu código, configuração e estrutura pronta para trabalhar. |
| `main.joss` | Arquivo principal de entrada de todo projeto Joss. É o primeiro arquivo que o Joss procura ao executar ou compilar sua aplicação. |
| `Init main()` | Bloco de inicialização dentro de `public class Main` em `main.joss`. Tudo o que você escrever dentro de suas chaves `{ ... }` é executado ao iniciar o programa. |
| Valor | Um dado concreto: `12`, `"Ana"`, `true`. |
| Variável | Nome com prefixo `$` que permite guardar, consultar ou alterar um valor na memória; por exemplo `$edad`. |
| Declaração | Ação de criar uma variável pela primeira vez indicando `var` (`var $x = 1`), um tipo explícito (`int $x = 1`), `mixed` ou `const`. |
| Reatribuição | Ação de alterar o valor de uma variável que já foi declarada anteriormente (`$x = 2`). |
| Binding | Associação entre um nome e seu valor. Uma constante (`const`) protege essa associação para que não possa ser reatribuída. |
| Tipo | Categoria de dados que uma operação ou variável admite. Por exemplo, `int` representa números inteiros e `decimal` valores monetários exatos. |
| Inferência | Dedução automática do tipo de uma variável a partir do seu valor inicial (como em `var $edad = 20`, onde o Joss deduz `int`). |
| Conversão / casting | Obtenção de um valor em outro tipo mediante funções como `intval(...)`, `decimal(...)` ou `strval(...)`. |
| Interpolação | Incorporar variáveis ou cálculos dentro de um texto com aspas duplas usando `${nombre}` ou `${$precio * 2}`. |
| `mixed` | Decisão explícita de permitir que uma variável ou parâmetro aceite valores de diferentes tipos ao longo do tempo. |
| `void` | Tipo de retorno que indica explicitamente que uma função realiza ações, mas não devolve nenhum valor. |
| `unknown` | Estado interno do analisador quando ainda não conhece um tipo; não é uma palavra para declarar variáveis. |
| `null` / `nil` | Ausência de valor; `null` é a forma canônica recomendada e `nil` é aceito como sinônimo de compatibilidade. |
| Nullable | Tipo que permite um valor concreto ou `null`, escrito como `string|null` ou com o atalho `string?`. |
| Expressão | Fragmento de código que é avaliado e produz um valor: `2 + 3`, `"Hola " . $nombre` ou `($edad >= 18) ? "Mayor" : "Menor"`. |
| Instrução | Ordem que realiza um passo dentro do programa: declarar uma variável, mostrar uma mensagem, repetir um laço ou retornar. |
| Bloco | Grupo de instruções delimitadas entre chaves `{` e `}`. |
| Função | Máquina reutilizável de código (`public func` ou `private func`) que recebe dados de entrada e pode devolver um resultado. |
| Parâmetro | Variável tipada declarada entre os parênteses de uma função (`int $a`) que define qual dado espera receber. |
| Argumento | Valor real entregue entre os parênteses quando você chama uma função (`sumar(5, 10)`). |
| Retorno | Resultado que uma função devolve a quem a chamou mediante a palavra `return`. |
| `guard` | Cláusula de guarda (`guard (condicion) : { return ... }`) que exige que uma condição seja cumprida ou sai imediatamente da função. |
| `defer` | Instrução que agenda um bloco para ser executado logo antes que termine a função ou arquivo atual, útil para fechar recursos. |
| Callable | Qualquer valor que pode ser invocado com parênteses `()`: uma função, um método ou uma closure. |
| Escopo / scope | Área do programa onde uma variável existe e pode ser utilizada. Cada chamada de função tem seu próprio escopo isolado. |
| Closure | Função anônima (`func(...) { ... }`) que pode ser guardada em uma variável e lembrar variáveis do ambiente onde foi criada. |
| Recursão | Técnica na qual uma função chama a si mesma para resolver um problema em passos menores. |
| Classe | Molde ou planta (`public class Nombre`) que agrupa dados (propriedades) e comportamentos (métodos). |
| `record` | Forma concisa (`public record Nombre(...)`) de declarar uma classe cujos dados ficam protegidos como constantes após serem criados. |
| Instância / Objeto | Entidade concreta criada na memória a partir de uma classe usando `new Nombre(...)`. |
| Propriedade | Variável que pertence a uma classe ou instância (acessada com `$this->propiedad` ou `$objeto->propiedad`). |
| Método | Função que pertence a uma classe; é invocado com `->` sobre uma instância ou com `::` se for um método estático da classe. |
| Construtor | Método especial (`public func constructor(...)` ou `Init(...)`) que prepara os dados iniciais ao criar um objeto com `new`. |
| Herança | Capacidade de uma classe para reutilizar e ampliar outra classe base mediante `extends`. |
| Interface | Contrato formal (`public interface`) que enumera quais métodos públicos uma classe deve implementar mediante `implements`. |
| Encapsulamento | Proteção dos membros de uma classe usando `public` (aberto), `protected` (classe e filhas) ou `private` (apenas a mesma classe). |
| Array | Lista ordenada de elementos entre colchetes `[10, 20]`, cujas posições (índices) começam a contar do `0`. |
| Map | Dicionário de pares chave → valor escrito como `{"clave": valor}` ou `["clave" => valor]`. |
| Referência | Acesso compartilhado ao mesmo dado. Com `ref` (`incrementar(ref $x)`), permite que uma função modifique temporariamente uma variável de quem a chamou. |
| Mutabilidade | Possibilidade de modificar o valor de uma variável ou os elementos internos de uma coleção. |
| Cópia superficial | Duplicata do contêiner externo de uma coleção cujos elementos internos complexos continuam compartilhados. |
| Erro / Diagnóstico | Aviso estruturado (`JOSS-...`) com arquivo, linha, explicação e sugestão que aponta um problema no código. |
| Exceção | Falha em tempo de execução que é lançada com `throw` e pode ser interceptada com `try { ... } catch ($e) { ... }`. |
| Pilha de chamadas / stack | Cadeia de funções em espera enquanto se executa uma chamada interna. Não confundir com a classe de estrutura de dados `Stack`. |
| Heap | Memória gerenciada automaticamente onde vivem os objetos, arrays e maps criados durante a execução. |
| Sincronia | Execução passo a passo na qual cada instrução espera que a anterior termine. |
| Concorrência | Avanço coordenado de várias tarefas ao mesmo tempo. |
| Assincronia | Delegar uma tarefa com `async { ... }` para que seja executada sem bloquear o programa principal. |
| Future | Objeto devolvido por um bloco `async` que representa o resultado futuro dessa tarefa. |
| `await` | Palavra-chave (`await $tarea` ou `await($tarea)`) que espera e obtém o valor final de um `Future`. |
| Channel | Canal tipado (`channel<T>` / `make_chan()`) para enviar (`<<`) e receber (`<-`) mensagens seguras entre tarefas concorrentes. |
| Runtime | Motor de execução do Joss que gerencia a memória, a concorrência e as classes nativas integradas. |
| Lexer | Primeiro passo do processador do Joss: converte o texto do arquivo `.joss` em peças indivisíveis chamadas tokens. |
| Token | Unidade léxica reconhecida: uma palavra-chave, uma variável, um número ou um operador. |
| Parser | Segundo passo do processador: organiza os tokens em uma árvore sintática verificando as regras de escrita. |
| AST | Árvore de Sintaxe Abstrata que representa a estrutura lógica do seu código. |
| Analisador / analyzer | Rede de segurança estática que verifica tipos, nomes, contratos e retornos antes de executar ou compilar, gerando um `PreparedProgram`. |
| Interpretação (`joss run`) | Execução imediata do programa validado diretamente a partir do terminal. |
| Compilação nativa (`joss build`) | Transformação do mesmo programa Joss em um arquivo executável nativo independente (`.exe` no Windows ou binário no Linux/macOS/Android). |
| Bytecode (`.jp`) | Representação binária assinada criptograficamente utilizada para distribuir pacotes e plugins do Joss. |
| Módulo | Organização automática de arquivos dentro do projeto (`app/`); no Joss as declarações públicas são descobertas sem escrever `import`. |
| Pacote | Biblioteca reutilizável distribuída e gerenciada com `joss pub`. |
| Plugin | Extensão compilada no formato `.jp` que adiciona capacidades ou comandos ao projeto. |
| CLI | Ferramenta de linha de comando (`joss`) que você controla digitando ordens no terminal. |
| Terminal / Console | Janela do sistema operacional (como PowerShell ou Terminal) onde você digita comandos de texto e vê os resultados. |
| Rota HTTP | Regra em `routes.joss` ou `api.joss` que conecta um endereço web (URL) com o controlador encarregado de respondê-lo. |
| Controlador | Classe localizada em `app/controllers/` que recebe a requisição do usuário, processa a lógica e devolve uma view ou JSON. |
| Middleware | Filtro intermediário que revisa ou protege uma requisição web antes que ela chegue ao controlador (por exemplo, verificar se iniciou sessão). |
| View | Template HTML dinâmico (`.joss.html`) em `app/views/` que mostra dados ao usuário escapando código malicioso automaticamente. |
| Migração | Arquivo em `app/database/migrations/` que cria ou atualiza tabelas do banco de dados de forma ordenada com `joss migrate`. |
| Query builder / ORM | Sistema integrado (`Model` e `GranDB`) para consultar e salvar dados em bancos de dados sem concatenar SQL inseguro. |
| API | Conjunto de funções, métodos ou rotas web que um programa oferece para se comunicar com outro. |