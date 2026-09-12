# Glossário

[Índice](README.md) · [Primeiros passos](PRIMEROS_PASOS.md) · [Referência](SINTAXIS.md)

Essas palavras também são explicadas onde são introduzidas. Você pode voltar aqui
sem interromper a jornada de aprendizagem.

| Prazo | Significado nesta documentação |
|---|---|
| Programa | Instruções salvas em arquivos para um computador executar uma tarefa. |
| Valor | Uma informação específica: `12`, `"Ana"`, `true`. |
| Variável | Nome que permite guardar e consultar um valor; por exemplo `$edad`. |
| Encadernação | Associação entre um nome e seu valor. Uma constante protege essa associação, não necessariamente o conteúdo de uma coleção. |
| Tipo | Tipo de dado que uma operação ou variável admite. `int` representa inteiros. |
| Inferência | Dedução de um tipo do valor, sem escrevê-lo explicitamente. |
| Conversão/elenco | Obtenção de um valor em outro tipo utilizando regras específicas; você pode perder informações. |
| `mixed` | Decisão explícita de aceitar valores de diferentes tipos. |
| `unknown` | Falta de informações do analisador; Não é um tipo de origem para declarar variáveis. |
| `null` / `nil` | Ausência de valor; ambas as formas produzem o mesmo valor nulo. |
| Anulável | Tipo que também permite `null`, como `string|null` ou `string?`. |
| Expressão | Código que produz um valor: `2 + 3`. |
| Frase | Instrução que executa uma etapa: declarar, retornar ou repetir. |
| Bloco | Grupo de declarações entre colchetes em um contexto que espera um corpo. |
| Função | Operação reutilizável que recebe dados e pode retornar um resultado. |
| Parâmetro | Nome e tipo de dado recebido na declaração de uma função. |
| Argumento | Valor entregue quando a função é chamada. |
| Retorno | Resultado que uma função retorna com `return`. |
| Exigível | Valor que o runtime pode invocar: função, método ou encerramento, entre outros. Não é uma palavra-chave de origem. |
| Escopo/escopo | Região onde um nome pode ser resolvido. |
| Encerramento | Função anônima que preserva um ambiente de variáveis ​​de sua criação. |
| Recursão | Função que chama a si mesma, direta ou indiretamente. |
| Classe | Declaração que agrupa propriedades e métodos. |
| Instância | Objeto criado a partir de uma classe usando `new`. |
| Propriedade | Dados salvos em uma instância. |
| Método | Função associada a uma classe; chamado com `->` em uma instância ou `::` em contexto estático. |
| Construtor | Inicialização ao criar uma instância; Joss suporta `Init` em seu contrato de classe. |
| Herança | Reutilizando uma classe base usando `extends`. |
| Encapsulamento | Controle de acesso com `public`, `protected` ou `private`. |
| Matriz | Coleção ordenada de elementos, indexados do zero; “lista” é uma explicação, não o alias de tipo `list`. |
| Mapa | Coleta de chaves e valores; Também é chamado de dicionário. |
| Referência | Acesso ao mesmo armazenamento. `ref` também é uma capacidade temporária e restrita de modificar uma variável de chamada. |
| Mutabilidade | Possibilidade de alterar um valor ou o conteúdo de uma estrutura. || Cópia superficial | Cópia do container que pode continuar compartilhando elementos interiores. |
| Erro/diagnóstico | Problema detectado; um diagnóstico inclui localização, código e explicação. |
| Exceção | Falha que interrompe o fluxo e pode ser recuperada com `try/catch`. |
| Pilha/pilha de chamadas | Sequência de funções que aguardam o término de outra chamada. Não deve ser confundido com a classe wrapper `Stack`. |
| Pilha | Memória para objetos cuja vida não se limita a uma chamada; É gerenciado pelo Go, não manualmente pelo programa Joss. |
| Sincronia | Uma operação termina antes de continuar com a próxima. |
| Simultaneidade | Várias tarefas progridem durante períodos sobrepostos; não garante execução simultânea. |
| Assincronia | Uma operação permite obter seu resultado posteriormente. |
| Futuro | Objeto de tempo de execução representando um resultado pendente de `async`; Não é uma palavra-chave ou tipo de fonte canônica. |
| `await` | O bloqueio aguarda o resultado de um Future na execução atual. |
| Canal | Canal para enviar dados entre tarefas e coordená-las. |
| Tempo de execução | Motor que executa o programa e oferece serviços integrados. |
| Lexer | Componente que converte caracteres em tokens. |
| Ficha | Unidade reconhecida: um nome, um número, um operador, etc. |
| Analisador | Componente que organiza tokens de acordo com a sintaxe. |
| AST | Árvore que representa a estrutura do programa. |
| Analisador / analisador | Componente que verifica nomes, tipos e fluxo antes de executar. |
| Interpretação | Execução através da leitura de uma representação do programa, como o seu AST. |
| Compilação | Transformação para outra representação. Em Joss isso não implica necessariamente código de máquina. |
| Bytecódigo | Formato intermediário. Joss main contém AST compactado; JPBC possui instruções para plug-ins. |
| Módulo | Capacidade integrada ou organização física; Joss não possui módulos de origem com importações. |
| Pacote | Unidade distribuível com metadados e arquivos. |
| Plug-ins | Extensão carregada em tempo de execução a partir de um pacote. |
| CLI | Programa que é controlado digitando comandos em um terminal. |
| Terminais | Janela para executar comandos e visualizar sua saída. |
| Rota HTTP | Associação entre uma URL, um método HTTP e um código de resposta. |
| Controlador | Classe que prepara a resposta a uma solicitação web. |
| Middleware | Verificação ou transformação em torno de uma solicitação. |
| Ver | Modelo que converte dados em HTML. |
| Migração | Alteração versionada da estrutura de um banco de dados. |
| Construtor de consultas | Objeto que constrói uma consulta antes de executá-la. |
| API | Um conjunto de operações que outro programa ou componente pode usar. |