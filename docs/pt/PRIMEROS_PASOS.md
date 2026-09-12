# Primeiros passos: de um arquivo a um programa

Antes: [Índice](README.md). Depois: [Valores, variáveis ​​e operações](FUNDAMENTOS.md).
Referência rápida: [Linha de comando (CLI)](CLI.md).

---

## O que você vai aprender aqui?

Se você nunca escreveu uma única linha de código em sua vida, este é o lugar certo para começar. Você não precisa de conhecimento prévio de programação ou experiência em outras linguagens.

Neste guia você aprenderá:
1. O que é programação e o que são instruções para um computador.
2. O que é a linguagem **Joss** e quais componentes compõem seu ecossistema.
3. Como instalar o Joss no seu sistema operacional (Windows, Linux ou macOS).
4. Como escrever seu primeiro programa ("Hello World"), analisá-lo e executá-lo.
5. O que acontece internamente desde o momento em que você salva o arquivo texto até a tela mostrar o resultado.
6. Como resolver os erros mais comuns ao dar os primeiros passos.

---

## 1. O que é programação e o que é um programa?

Um computador é extraordinariamente rápido em fazer cálculos, mas não consegue descobrir o que você quer fazer. Você precisa de uma sequência de pedidos clara, ordenada e inequívoca. Esta sequência de instruções é chamada de **programa** ou **algoritmo**.

Imagine uma receita culinária:
1. Pese 200 gramas de farinha.
2. Adicione dois ovos.
3. Misture por cinco minutos.

Um programa de computador funciona sob a mesma lógica: executa tarefas passo a passo. O texto exato que você escreve para fornecer esses comandos ao computador é chamado **código-fonte**.

No **Joss**, o código-fonte é escrito em texto simples e salvo em arquivos cuja extensão termina em `.joss` (por exemplo, `hola.joss` ou `app.joss`). Você não deve usar processadores de texto avançados como Microsoft Word ou Google Docs, porque eles adicionam formatação invisível que o computador não entende; **editores de código** como o Visual Studio Code são usados.

---

## 2. O que é Joss?

**Joss** é uma linguagem de programação moderna projetada especialmente para aplicativos de back-end, desenvolvimento web, utilitários de linha de comando e serviços simultâneos de alto desempenho.

Sua filosofia de design é baseada em quatro pilares:

1. **Zero imports no código fonte (Zero Imports)**: Em muitas linguagens você deve escrever dezenas de linhas como `import X from Y` no início de cada arquivo. Joss descobre e organiza automaticamente os arquivos públicos, classes e funções do seu projeto e seus plugins, permitindo que você se concentre na lógica de negócios.
2. **Análise estática rigorosa e segura**: antes de executar uma única instrução, o **analisador semântico** de Joss analisa suas variáveis, tipos de dados, caminhos de retorno e visibilidades para detectar erros antes que eles cheguem à produção.
3. **Full Stack incluído**: Joss inclui nativamente servidor HTTP de alto desempenho, sistema de roteamento, mecanismo de modelagem HTML, ORM e gerador de esquema de banco de dados (GranDB / Schema), suporte para WebSockets em tempo real, hashing criptográfico e tarefas assíncronas.4. **Simultaneidade e canais limpos**: Você pode delegar tarefas pesadas para segundo plano de forma não-bloqueante com `async` e comunicar processos com canais seguros (`channel`).

### As camadas do ecossistema Joss

É importante distinguir três conceitos que por vezes são confundidos:```text
┌─────────────────────────────────────────────────────────────┐
│ 1. Tu código fuente (.joss)                                 │
│    El texto legible que tú escribes con tus instrucciones.  │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. El analizador semántico (Semantic Analyzer)              │
│    Revisa tipos, nombres, visibilidad y coherencia.          │
└──────────────────────────────┬──────────────────────────────┘
                               │ Si no hay errores
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. El motor de ejecución (Runtime de Joss en Go)             │
│    Ejecuta las instrucciones reales en tu máquina física.    │
└──────────────────────────────┴──────────────────────────────┘
```
- **A linguagem Joss**: As regras de escrita, palavras reservadas e sintaxe que você aprenderá.
- **A ferramenta CLI (`joss`)**: O executável de linha de comando que você usa em seu terminal para analisar, formatar, testar e iniciar seus projetos.
- **O Runtime**: O mecanismo que pega seu programa comprovado e executa as operações reais no processador e na memória do seu computador.

---

## 3. Instalação passo a passo

Um **terminal** (ou console) é uma janela de texto onde você se comunica com o sistema operacional digitando comandos em vez de clicar com o mouse.
- No **Windows**: você pode abrir o aplicativo **PowerShell** ou o **Windows Terminal** (procure-o no menu Iniciar).
- No **Linux ou macOS**: Abra o aplicativo chamado **Terminal**.

### Opção A: Instalador Automático Oficial (Recomendado)

No **Windows** (abra o PowerShell como usuário padrão ou administrador):```powershell
iwr -useb https://raw.githubusercontent.com/josprox/Joss-language/main/install/remote-install.ps1 | iex
```
No **Linux ou macOS** (abra seu Terminal):```bash
curl -fsSL https://raw.githubusercontent.com/josprox/Joss-language/main/install/remote-install.sh | bash
```
Este comando irá baixar o binário Joss compilado, colocá-lo em uma pasta padrão do sistema e registrá-lo em seu `PATH`.

> [!NOTA]
> **O que é `PATH`?**
> O `PATH` é uma lista interna do sistema operacional com os caminhos onde residem os programas que você pode invocar escrevendo apenas seus nomes. Se você adicionar Joss ao `PATH`, poderá digitar `joss` em qualquer pasta sem precisar digitar o caminho completo do executável. Se você acabou de instalá-lo e seu terminal não o reconhece, **feche o terminal e abra-o novamente**.

### Opção B: download manual das versões do GitHub

1. Vá para a seção de lançamentos: [Joss GitHub Releases](https://github.com/josprox/Joss-language/releases).
2. Baixe o pacote compactado `.zip` ou `.tar.gz` correspondente à sua arquitetura (`windows_amd64`, `linux_amd64`, `darwin_arm64`, etc.).
3. Descompacte o arquivo e coloque o executável `joss` (ou `joss.exe`) em uma pasta acessível em seu disco.
4. Adicione esta pasta às variáveis ​​de ambiente do sistema (`PATH`).

### Opção C: Compilar a partir do código-fonte com Go

Se você é um desenvolvedor e tem o [Go](https://go.dev) instalado (versão 1.22 ou superior), você pode clonar este repositório e construí-lo em segundos:```bash
git clone https://github.com/josprox/Joss-language.git
cd Joss-language
go build -o joss ./cmd/joss
```
No Windows será criado `joss.exe`; no Linux/macOS o binário executável `joss` será criado.

### Verifique se a instalação funciona

Abra um terminal e digite:```bash
joss version
```
Você deverá ver a versão instalada do Joss na tela (por exemplo, `Joss version 3.6.7.2`). Se você vir essa mensagem, seu ambiente está 100% pronto para programar!

---

## 4. Escreva e execute seu primeiro programa

Vamos criar o programa clássico que todo programador escreve quando começa: exibir uma saudação na tela.

### Etapa 1: Crie um portfólio

Crie uma pasta limpa para seus experimentos. No seu terminal:```bash
mkdir mi-primer-joss
cd mi-primer-joss
```
### Etapa 2: Crie o arquivo `hola.joss`

Abra seu editor de texto favorito (por exemplo, Visual Studio Code digitando `code .` nessa pasta) e crie um novo arquivo chamado:

`hola.joss`

> [!CUIDADO]
> Certifique-se de que o arquivo termine exatamente em `.joss`. No Windows, se as extensões conhecidas estiverem ocultas, o Bloco de Notas poderá salvá-las como `hola.joss.txt`, o que impedirá Joss de reconhecê-las como código-fonte.

Escreva exatamente a seguinte linha no arquivo:<!-- joss-run: ["Hola, Joss!"] -->
```joss
print("Hola, Joss!")
```
Salve o arquivo.

### Passo 3: Entenda cada parte dessa linha

Vamos ver o que cada personagem significa:

1. `print`: É o nome de uma **função nativa** incorporada ao Joss. Sua finalidade exclusiva é receber informações e gravá-las na tela (saída padrão), acrescentando uma quebra de linha no final para que a próxima instrução comece na linha abaixo.
2. `(` e `)`: Os parênteses informam a Joss que você está **chamando** (executando) a função `print`. Dentro dos parênteses você coloca os dados de entrada que a função precisa para funcionar. Esses dados de entrada são chamados de **argumentos**.
3. `"Hola, Joss!"`: É um valor do tipo **text** (tecnicamente chamado de `string` ou string de caracteres). As aspas duplas `"` servem para marcar exatamente onde o texto começa e termina. As aspas não são exibidas na tela, apenas delimitam o conteúdo.

### Etapa 4: Analise e execute o programa

Volte para o seu terminal, certifique-se de estar localizado na pasta `mi-primer-joss` e digite:```bash
joss run hola.joss
```
Você verá imediatamente a saída no terminal:```text
Hola, Joss!
```
Parabéns! Você acabou de escrever, processar e executar seu primeiro programa em Joss.

### Modo interativo para testes rápidos: `joss repl`

Se quiser experimentar operações matemáticas, variáveis ou pequenas funções sem criar um arquivo no disco, você pode abrir o Joss **console interativo (REPL)** digitando em seu terminal:```bash
joss repl
```
Você verá um cursor de boas-vindas:```text
Joss Interactive REPL (v3.7.0)
Escribe expresiones, sentencias o 'exit' / 'quit' para salir.
>>> $x = 10
>>> $x * 5
50
>>> print("Hola desde el REPL!")
Hola desde el REPL!
>>> exit
```
As variáveis ​​que você criar serão lembradas entre linhas enquanto a sessão estiver aberta. Para sair, basta digitar `exit` ou `quit`.

### Extensão do VS Code e formatação automática

Se você usar o Visual Studio Code com a extensão Joss oficial, poderá classificar e alinhar automaticamente seu código de acordo com as regras de estilo canônico a qualquer momento com o atalho padrão:
- Em **Windows e Linux**: `Shift + Alt + F`
- No **macOS**: `Shift + Option + F` (ou clique com o botão direito → *Formatar documento*)

---

## 5. O comando `analyze`: Sua rede de segurança

Antes de executar um programa grande ou implantá-lo em um servidor, Joss permite verificá-lo estaticamente usando o comando `analyze`:```bash
joss analyze hola.joss
```
Se o código estiver correto e seguro, o analisador será encerrado com sucesso:```text
[Analyzer] Análisis completado sin errores.
```
O que exatamente o analisador faz?
- **Verifique a sintaxe**: Verifique se você não esqueceu aspas, parênteses ou colchetes.
- **Validar nomes de variáveis ​​e funções**: Verifique se você não está chamando coisas que não existem.
- **Verifique os tipos de dados**: Se você disse que uma variável era um número inteiro, verifique se não tentou atribuir a ela uma lista de usuários.
- **Verificar caminhos de retorno**: Verifique se suas funções sempre retornam um valor consistente em qualquer caminho possível.

O comando `joss run` executa esta análise internamente antes de executar. Se houver um erro de bloqueio (`Severity: error`), Joss se recusará a executá-lo para proteger seu sistema contra comportamento errático.

---

## 6. Modifique o programa: use memória e variáveis

Um programa que exibe apenas texto fixo não é muito interativo. Os programas reais armazenam informações na memória do computador para serem recuperadas ou transformadas posteriormente. Para fazer isso são usadas **variáveis**.

Modifique seu arquivo `hola.joss` para conter:<!-- joss-run: ["Hola, Ada", "Bienvenida a Joss"] -->
```joss
$nombre = "Ada"
print("Hola, " . $nombre)
print("Bienvenida a Joss")
```
Salve e execute:```bash
joss run hola.joss
```
Saída:```text
Hola, Ada
Bienvenida a Joss
```
### O que mudou aqui?

1. `$nombre = "Ada"`:
   - O símbolo `$` no início indica que estamos declarando ou usando uma variável. Em Joss, **todas as variáveis ​​começam com `$`**.
   - O sinal `=` é chamado de **operador de atribuição**. Ele pega o valor da direita (`"Ada"`) e o armazena na "caixa" de memória identificada pelo nome `$nombre`.
   - Joss infere automaticamente que `$nombre` armazena texto (`string`).
2. `"Hola, " . $nombre`:
   - O ponto `.` é o **operador de concatenação**. É usado para unir dois trechos de texto em um. Aqui ele une `"Hola, "` com o conteúdo dentro de `$nombre` (`"Ada"`), produzindo o texto `"Hola, Ada"`.
3. Segunda chamada para `print`:
   - Exibe a próxima linha de forma independente.

Tente alterar `"Ada"` para seu próprio nome na primeira linha, salve o arquivo e execute `joss run hola.joss` novamente. Você verá como a saudação muda automaticamente.

---

---

## 7. Modo interativo: peça dados ao usuário com `cin >>`

Até agora, o programa mostra apenas dados que você já escreveu no código. Para criar programas divertidos e úteis, você precisa que o computador **ouça** você, espere pela sua resposta e reaja a ela.

O ciclo fundamental de todo programa é:```text
┌─────────────────────────┐       ┌─────────────────────────┐       ┌─────────────────────────┐
│     1. ENTRADA          │  ──>  │     2. PROCESO          │  ──>  │     3. SALIDA           │
│ El usuario escribe con  │       │ El programa calcula,    │       │ Se muestra el resultado │
│ cin >> $variable        │       │ une o toma decisiones   │       │ en pantalla con print() │
└─────────────────────────┘       └─────────────────────────┘       └─────────────────────────┘
```
Em Joss, ler os dados que o usuário digita em seu teclado é tão simples quanto usar `cin >>`:<!-- joss-check: lectura interactiva de datos por teclado -->
```joss
string $nombre = ""
int $edad = 0

print("¿Cómo te llamas?")
cin >> $nombre

print("¿Cuántos años tienes?")
cin >> $edad

print("¡Mucho gusto, " . $nombre . "! El próximo año tendrás " . ($edad + 1) . " años.")
```
### Como funciona `cin >>`?
1. `print(...)` exibe uma pergunta no terminal para que a pessoa saiba o que digitar.
2. `cin >> $nombre` pausa o programa e espera que o usuário digite seu nome e pressione a tecla **Enter**.
3. Tudo o que o usuário digita é automaticamente armazenado na variável `$nombre`.
4. Se os dados esperados forem um número (como idade), Joss os converte automaticamente para que você possa realizar operações matemáticas diretas como `$edad + 1`.

---

## 8. A "Folha de dicas essencial para iniciantes"

Mantenha este quadro por perto. Esses 8 padrões resolvem praticamente qualquer programa nas primeiras semanas de aprendizado:

| O que você deseja alcançar? | Como você escreve em Joss? | Exemplo mínimo |
|---|---|---|
| **Mostrar uma mensagem** | `print(...)` | `print("¡Hola mundo!")` |
| **Peça dados ao usuário** | `cin >> $variable` | `cin >> $ciudad` |
| **Salvar informações** | `$variable = valor` | `$precio = 25` |
| **Aumente ou adicione rapidamente** | `$variable += valor` | `$puntos += 10` |
| **Junte pedaços de texto** | `texto1 . texto2` | `"Hola " . $nombre` |
| **Tome uma decisão** | `(condicion) ? { si } : { no }` | `($edad >= 18) ? { print("Mayor") } : { print("Menor") }` |
| **Escolha entre diversas opções** | `match ($opcion) { caso => ... }` | `match ($color) { "rojo" => "Alto", default => "Sigue" }` |
| **Salve e percorra uma lista** | `foreach ($lista as $item)` | `foreach (["manzana", "pera"] as $f) { print($f) }` |

---

## 9. As três regras de ouro para iniciantes

Para evitar 99% de dúvidas e erros na hora de dar os primeiros passos:

1. **O dólar sagrado (`$`):** Toda variável em Joss necessariamente começa com `$`. Se você vir um erro de variável indefinida, verifique se esqueceu o `$`.
2. **As citações vão em pares (`"..."`):** Todo texto livre deve começar e terminar com aspas duplas. Os números (`42`) não possuem aspas; as palavras (`"Hola"`) fazem.
3. **Tudo que abre, fecha:** Os parênteses `()`, as chaves `{}` e os colchetes `[]` sempre vão em pares. Se você abrir uma chave `{`, certifique-se de ter um fechamento correspondente `}`.

---

## 10. Se algo não funcionar: diagnóstico rápido

Quando você está aprendendo a programar, cometer erros não é apenas normal, é a melhor forma de entender como o computador pensa!

Aqui está uma tabela com os problemas mais frequentes e como resolvê-los:

| Sintoma ou mensagem | Causa provável | Como consertar |
|---|---|---|
| `joss: command not found` ou `el término 'joss' no se reconoce` | O terminal não sabe onde está o executável `joss`. | Feche e reabra seu terminal. Se persistir, verifique se a pasta Joss está incluída na sua variável de ambiente `PATH`. |
| `Error: No se pudo leer el archivo 'hola.joss'` (`JOSS-IO-001`) | O terminal não encontra o arquivo na pasta atual. | Digite `ls` (no Linux/macOS ou PowerShell) ou `dir` (no Windows CMD) para visualizar os arquivos na pasta. Verifique se o nome está escrito corretamente ou se você está no diretório correto com `cd`. || `JOSS-PARSE-001: Error de sintaxis` | Há uma aspa de fechamento `"` ausente, um parêntese `)` ou um caractere inesperado. | Leia a linha e a coluna indicadas pela mensagem de erro. Verifique se todas as aspas e parênteses correspondem corretamente. |
| `JOSS-SYM-001: Variable no definida` | Você tentou ler uma variável que nunca foi criada ou seu nome contém um erro de digitação. | Lembre-se de sempre incluir o `$`. Check case (`$nombre` e `$Nombre` são duas variáveis completamente diferentes para Joss). |
| Dezenas de erros sobre arquivos que você não reconhece | Você está executando `joss analyze` ou `joss run` dentro de uma pasta que contém outros projetos ou arquivos `.joss` anteriores. | Sempre trabalhe em uma pasta vazia dedicada ao seu projeto. Joss descobre automaticamente todos os arquivos `.joss` na pasta atual e seus subdiretórios. |

---

## 11. Exercícios práticos para iniciantes

Para consolidar o que você acabou de aprender antes de continuar:

1. **Seu cartão de visita**: Escreva um programa `tarjeta.joss` que declare três variáveis: `$miNombre`, `$miPais` e `$miProfesion`. Em seguida, usando `print` e concatenação com `.`, exiba uma mensagem na tela que reúne um parágrafo apresentando você.
2. **Seu primeiro diálogo interativo**: Crie `conversacion.joss`, pergunte ao usuário seu nome e comida favorita com `cin >>` e responda com uma alegre recomendação culinária.
3. **Experimentando erros (The Code Breaking Game)**: Remova propositalmente a citação final do texto em `print("Hola)` e execute `joss analyze`. Observe como Joss informa exatamente o número da linha onde detectou o problema. Coloque a cotação de volta para que fique limpa.

---

## Próxima etapa

Agora que você já sabe o que é Joss, como criar um arquivo, como imprimir dados e como interagir com o usuário, é hora de aprender a fundo os tipos de dados que um computador manipula, como operar com números e como decidir que tipo de variável usar.

Continue com: [Valores, variáveis ​​e operações fundamentais](FUNDAMENTOS.md).