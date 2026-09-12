# Análise estática (`joss analyze`)

[Índice](README.md)

`joss analyze [archivo.joss]` analisa os arquivos de entrada e `.joss` em `app/` sem executar o aplicativo. O pipeline carrega cada arquivo como uma unidade de origem, registra declarações globais e, em seguida, analisa cada chamada com seu próprio escopo.```bash
joss analyze
joss analyze main.joss
```
O comando retorna um código de saída diferente de zero se houver erros. Os avisos são exibidos, mas não bloqueiam. `joss run` aplica a mesma análise antes de executar e só continua se não houver erros.

## Verificações atuais

- Variáveis indefinidas, redeclarações e premissas não utilizadas.
- Funções, classes, superclasses e métodos que não existem quando o seu receptor é conhecido.
- Escopo de parâmetros, funções, métodos, `Init` e encerramentos.
- Corrigida inferência na compatibilidade da primeira atribuição e reatribuição.
- Inicializadores digitados, uniões/anuláveis, constantes, padrões, argumentos, retornos, caminhos exaustivos e aridade de funções Joss.
- Tipos de classes inexistentes em variáveis, parâmetros e retornos.
- Operadores e índices incompatíveis.
- Código após um `return` incondicional.
- Declarações duplicadas ao nível do projeto.
- Símbolos de classes nativas e plugins JP v2 carregados pelo projeto.

## Evidências e informações desconhecidas

O analisador diferencia `unknown` de inválido e `mixed`. Os retornos de todas as APIs principais possuem metadados explícitos, mas algumas entradas não correspondem a todos os resultados do tempo de execução (consulte [auditoria](DOCUMENTATION_AUDIT.md)). `mixed` representa polimorfismo intencional. Uma API nativa sem metadados de parâmetro não produz um erro de aridade especulativo; um receptor dinâmico não produz erro de membro; `isset` e `empty` podem consultar uma variável ausente.

## Saída

Os diagnósticos usam o modelo `pkg/diagnostics`:```text
error[JOSS-TYPE-001] app/example.joss:3:2: Cannot use `string` as assignment for `$age` of type `int`.
  suggestion: Convert the value explicitly or use `let $name` only when dynamic typing is intentional.
```
Consulte [Diagnostics](DIAGNOSTICOS.md) para códigos e gravidades, e [Type System](SISTEMA_TIPOS.md) para regras de inferência.

## Arquitetura

`pkg/analyzer` não depende de `pkg/core`. O adaptador `pkg/core/analyzer.go` constrói seu `Environment` a partir dos registros internos, classes nativas e plug-ins reais. A CLI usa `analyzer.LoadProject`, portanto preserva arquivo, linha e coluna em vez de concatenar ASTs.

## Limites conhecidos

- Testes de retorno abrangentes abrangem blocos, ternários, `match` com `default` e `try/catch`; ainda não demonstra a terminação matemática do loop.
- Os parâmetros de muitas APIs nativas permanecem variáveis/desconhecidos para evitar erros de aridade; Seus retornos são explícitos.
- Não há análise de refinamento sensível à ramificação, contaminação/escape formal ou contratos de esquema de banco de dados.
- Joss não terá gráfico de importação de origem: o projeto utiliza carregamento automático e um único espaço de declaração.
- A recuperação do analisador ainda pode emitir mais de um diagnóstico derivado após um token inválido; cada descoberta já preserva a linha e a coluna estruturadas a partir do token original.

Essas limitações não são relatadas como erros do usuário.