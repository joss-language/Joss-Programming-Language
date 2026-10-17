# Primeiros passos: de um arquivo a um programa

Antes: [Índice](README.md). Depois: [Valores, variáveis e operações](FUNDAMENTOS.md).
Consulta rápida: [Linha de comando (CLI)](CLI.md).

---

## O que você vai aprender aqui?

Se você nunca escreveu uma única linha de código na vida, este é o lugar certo para começar. Você não precisa de conhecimentos prévios de programação nem de experiência em outras linguagens.

Neste guia você aprenderá:
1. O que é programar e o que é uma instrução para um computador.
2. O que é a linguagem **Joss** e quais componentes formam seu ecossistema.
3. Como instalar o Joss no seu sistema operacional (Windows, Linux ou macOS).
4. Como escrever seu primeiro programa ("Olá Mundo"), analisá-lo e executá-lo.
5. O que acontece internamente desde o momento em que você salva o arquivo de texto até a tela mostrar o resultado.
6. Como solucionar os erros mais comuns ao dar os primeiros passos.

---

## 1. O que é programar e o que é um programa?

Um computador é extraordinariamente rápido realizando cálculos, mas não consegue adivinhar o que você quer fazer. Ele precisa de uma sequência clara, ordenada e sem ambiguidades de ordens. Essa sequência de instruções é chamada de **programa** ou **algoritmo**.

Imagine uma receita de culinária:
1. Pese 200 gramas de farinha.
2. Adicione dois ovos.
3. Misture durante cinco minutos.

Um programa de computador funciona sob a mesma lógica: realiza tarefas passo a passo. Ao texto exato que você escreve para dar essas ordens ao computador dá-se o nome de **código-fonte**.

No **Joss**, o código-fonte é redigido em texto simples e salvo em arquivos cuja extensão termina em `.joss` (por exemplo, `hola.joss` ou `app.joss`). Você não deve usar processadores de texto rico como Microsoft Word ou Google Docs, porque eles adicionam formatações invisíveis que o computador não compreende; utilizam-se **editores de código** como o Visual Studio Code.

---

## 2. O que é o Joss?

**Joss** é uma linguagem de programação moderna projetada especialmente para aplicações backend, desenvolvimento web, utilitários de linha de comando e serviços concorrentes de alto desempenho.

Sua filosofia de design apoia-se em quatro pilares:

1. **Zero imports no código-fonte (Zero Imports)**: Em muitas linguagens você precisa escrever dezenas de linhas como `import X from Y` no início de cada arquivo. O Joss descobre e organiza automaticamente os arquivos, classes e funções públicas do seu projeto e de seus plugins, permitindo que você se concentre na lógica de negócio.
2. **Segurança e análise estática rigorosa**: Antes de executar uma única instrução, o **analisador semântico** do Joss revisa suas variáveis, tipos de dados, rotas de retorno e visibilidades para detectar erros antes que cheguem à produção.
3. **Pilha completa incluída (Batteries-Included)**: O Joss inclui nativamente servidor HTTP de alto desempenho, sistema de rotas, motor de templates HTML, ORM e gerador de esquemas para bancos de dados (GranDB / Schema), suporte para WebSockets em tempo real, hashing criptográfico e tarefas assíncronas.
4. **Concorrência limpa e canais**: Você pode delegar tarefas pesadas para segundo plano de forma não bloqueante com `async` e comunicar processos com canais seguros (`channel`).

#### Uma só linguagem, duas formas de executá-la: `joss run` e `joss build`

O código que você escreve no Joss é definido uma única vez e tem exatamente o mesmo significado em todos os modos:

```text
┌─────────────────────────────────────────────────────────────────┐
│ 1. Tu código fuente (.joss)                                     │
│    El texto legible que tú escribes con tus instrucciones.      │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. El analizador semántico (Semantic Analyzer)                  │
│    Revisa tipos, nombres, visibilidad y coherencia del proyecto. │
└───────────────┬───────────────────────────────┬─────────────────┘
                │                               │
                ▼                               ▼
┌───────────────────────────────┐ ┌───────────────────────────────┐
│ A. Desarrollo inmediato       │ │ B. Distribución nativa        │
│    `joss run main.joss`       │ │    `joss build main.joss`     │
│    Ejecuta al instante en     │ │    Compila el mismo código a  │
│    tu terminal.               │ │    un ejecutable (.exe / ELF).│
└───────────────────────────────┘ └───────────────────────────────┘
```

- **A linguagem Joss**: As regras de escrita, palavras reservadas e sintaxe que você aprenderá neste curso.
- **A ferramenta CLI (`joss`)**: O programa de linha de comando que você utiliza a partir do seu terminal para criar projetos, analisá-los, executá-los instantaneamente (`joss run`) ou compilá-los em um arquivo executável independente (`joss build`).
- **O Runtime / Compilador**: O motor que recebe seu programa já verificado e executa as operações reais no processador e na memória do seu equipamento.

---

## 3. Instalação passo a passo

Um **terminal** (ou console) é uma janela de texto onde você se comunica com o sistema operacional digitando ordens em vez de clicar com o mouse.
- No **Windows**: Abra o **PowerShell** ou o aplicativo **Terminal do Windows** (procure-o no menu Iniciar).
- No **Linux ou macOS**: Abra o aplicativo chamado **Terminal**.

### Opção A: Instalador automático oficial (Recomendado)

No **Windows** (abra o PowerShell):

```powershell
iwr -useb https://astriom.org/install | iex
```

No **Linux, macOS ou Android (Termux)** (abra seu Terminal):

```bash
curl -fsSL https://astriom.org/install.sh | bash
```

Este comando baixará o executável do Joss, o colocará em uma pasta padrão do sistema e o registrará no seu `PATH`.

> [!NOTE]
> **O que é o `PATH`?**
> O `PATH` é uma lista interna do sistema operacional com as pastas onde ficam os programas que você pode invocar digitando apenas o nome deles. Graças a isso você pode digitar `joss` a partir de qualquer pasta. Se você acabou de instalá-lo e seu terminal não o reconhece, **feche o terminal e abra-o novamente**.

### Opção B: Download manual pelo GitHub Releases

1. Acesse a seção de lançamentos: [GitHub Releases do Joss](https://github.com/joss-language/Joss-Programming-Language/releases).
2. Baixe o arquivo compactado correspondente ao seu sistema (`windows_amd64`, `linux_amd64`, `darwin_arm64`, etc.).
3. Descompacte o arquivo, coloque `joss` (ou `joss.exe`) em uma pasta do seu disco e adicione-a ao `PATH`.

### Opção C: Compilar a partir do código-fonte com Go

Se você já tem o [Go](https://go.dev) instalado (versão 1.22 ou superior):

```bash
git clone https://github.com/joss-language/Joss-Programming-Language.git
cd Joss-Programming-Language
go build -o joss ./cmd/joss
```

### Verificar se a instalação funciona

Abra seu terminal e digite:

```bash
joss version
```

Você deverá ver na tela a versão instalada do Joss:

```text
Joss v3.6.8.0 (powerbuilder Joss 3.6.8.0)
```

Se você vir essa mensagem, seu ambiente está 100% pronto!

---

## 4. Criar e executar seu primeiro projeto com `joss new console`

Em vez de criar pastas e arquivos vazios manualmente, o Joss inclui um gerador oficial que prepara todo o seu espaço de trabalho em um segundo.

### Passo 1: Gerar o projeto de console

No seu terminal, digite o seguinte comando e pressione **Enter**:

```bash
joss new console mi-primer-joss
```

Você verá que o Joss confirma a criação do projeto:

```text
Creando proyecto Joss Console en: mi-primer-joss

✓ Proyecto de consola creado exitosamente

Estructura creada:
  - main.joss           (Entry point)
  - env.joss            (Variables de entorno)
  - config/reglas.joss  (Constantes globales)
  - app/controllers/    (Lógica de negocio)
  - app/models/         (Acceso a datos)
  - app/libs/           (Librerías)
  - app/database/migrations/

Para ejecutar:
  cd mi-primer-joss
  joss run main.joss
```

### Passo 2: Entrar na pasta do projeto e executá-lo

Entre na pasta recém-criada com o comando `cd` (*change directory*, mudar de diretório) e execute o arquivo principal `main.joss`:

```bash
cd mi-primer-joss
joss run main.joss
```

Saída no seu terminal:

```text
=== Aplicación de Consola Joss ===

¡Hola desde Joss Console!

Este es un proyecto de consola backend-only.
Puedes agregar tu lógica en este archivo main.joss

Aplicación finalizada correctamente.
```

Parabéns! Você acabou de criar e executar seu primeiro projeto completo em Joss.

---

## 5. O que o Joss acabou de criar? Entendendo `main.joss`

Antes de modificar o programa, vamos entender três conceitos essenciais:

1. **O que é um projeto?**
   É uma pasta organizada (`mi-primer-joss`) onde vivem seu código-fonte, sua configuração (`env.joss`) e as subpastas (`app/`) para quando seu programa crescer.
2. **O que é `main.joss`?**
   É a **porta principal** (ponto de entrada ou *entry point*) do seu programa. Quando você executa `joss run main.joss` ou compila seu projeto com `joss build`, o Joss começa a ler por este arquivo.
3. **O que há dentro de `main.joss`?**
   Abra a pasta `mi-primer-joss` no seu editor de código (por exemplo, digitando `code .` se usar o Visual Studio Code) e abra o arquivo `main.joss`. Você verá este conteúdo:

<!-- joss-run: ["=== Aplicación de Consola Joss ===", "", "¡Hola desde Joss Console!", "", "Aplicación finalizada correctamente."] -->
```joss
public class Main {
    Init main() {
        print("=== Aplicación de Consola Joss ===")
        print("")
        print("¡Hola desde Joss Console!")
        print("")
        print("Aplicación finalizada correctamente.")
    }
}
```

### O que significam `public class Main` e `Init main()`?

Mais adiante (no capítulo de [Classes e objetos](CLASES.md)) você aprenderá em detalhes o que é uma classe. Por enquanto, você só precisa saber uma regra muito simples:

- `public class Main { Init main() { ... } }` é o **"botão de ligar"** oficial do seu projeto.
- Tudo o que você escrever **dentro das chaves `{` e `}` de `Init main()`** são as ordens que o computador executará em ordem, de cima para baixo, quando você iniciar o programa.
- As linhas que começam com `//` são **comentários**: anotações para você que o computador ignora por completo.

### Escrever sua própria saudação mínima

Substitua o conteúdo de `main.joss` por este exemplo mínimo de uma única instrução dentro de `Init main()`:

<!-- joss-run: ["Hola, Joss!"] -->
```joss
public class Main {
    Init main() {
        print("Hola, Joss!")
    }
}
```

> [!TIP]
> **Sabia que você também pode escrever instruções diretas?**
> Se para um experimento rápido você escrever apenas `print("Hola, Joss!")` em `main.joss` (sem envolvê-lo em `public class Main`), o Joss também o executará. Ao longo do curso veremos trechos curtos assim para ir direto ao ponto, mas nos seus projetos criados com `joss new console` você pode colocar todo o seu código dentro de `Init main() { ... }`.

Salve o arquivo e execute novamente no seu terminal:

```bash
joss run main.joss
```

Saída:

```text
Hola, Joss!
```

### O que faz cada parte de `print("Hola, Joss!")`?

1. `print`: É uma **função integrada** no Joss. Seu trabalho é receber um dado e mostrá-lo na tela do seu terminal, adicionando uma quebra de linha ao final.
2. `(` e `)`: Os parênteses indicam ao Joss que você quer **executar** `print`. O que você coloca dentro dos parênteses é o dado que entrega para ela trabalhar (chamado de **argumento**).
3. `"Hola, Joss!"`: É um valor do tipo **texto** (`string`). As aspas duplas `"` marcam onde começa e onde termina o texto. As aspas não são impressas na tela; apenas delimitam a mensagem.

### Testar ideias instantaneamente sem editar arquivos: `joss repl`

Se a qualquer momento você quiser testar uma soma ou uma linha de código ao vivo, digite no seu terminal:

```bash
joss repl
```

Você verá o console interativo do Joss:

```text
Joss REPL v3.6.8.0
Escribe 'exit' para salir.
joss> var $x = 10
joss> $x * 5
50
joss> print("Hola desde el REPL!")
Hola desde el REPL!
joss> exit
```

---

## 6. O comando `analyze`: Sua rede de segurança antes de executar

No Joss você não precisa esperar que um programa falhe no meio do caminho para descobrir um erro. Você pode pedir ao **analisador semântico** que revise seu projeto digitando:

```bash
joss analyze main.joss
```

Se o seu código cumprir todas as regras, você verá:

```text
[Analyzer] Análisis completado sin errores.
```

O que `joss analyze` revisa para você?
- **A pontuação (sintaxe)**: Que você não tenha esquecido de fechar aspas `"`, um parêntese `)` ou uma chave `}`.
- **Os nomes**: Que você não tente usar uma variável ou função que nunca foi criada.
- **Os tipos de dados**: Que você não tente guardar um texto onde prometeu guardar um número inteiro.

De fato, toda vez que você executa `joss run main.joss` ou compila com `joss build main.joss`, o Joss passa primeiro por esse filtro de segurança automaticamente.

---

## 7. Guardar informações na memória: Sua primeira variável

Um programa que só imprime texto fixo sempre faz a mesma coisa. Para lembrar um nome, uma idade ou um resultado, usamos **variáveis**.

Imagine uma variável como uma **caixa etiquetada na memória** do computador onde você pode guardar um dado para usá-lo depois.

Atualize seu arquivo `main.joss` com o seguinte código:

<!-- joss-run: ["Hola, Ada", "Bienvenida a Joss"] -->
```joss
public class Main {
    Init main() {
        var $nombre = "Ada"
        print("Hola, " . $nombre)
        print("Bienvenida a Joss")
    }
}
```

Salve e execute `joss run main.joss`:

```text
Hola, Ada
Bienvenida a Joss
```

### Explicação linha por linha:

1. `var $nombre = "Ada"`:
   - `var`: Diz ao Joss *"vou criar uma variável nova; deduza seu tipo a partir do valor inicial"*.
   - `$nombre`: É o nome da variável. **No Joss, todas as variáveis começam obrigatoriamente com o sinal `$`**.
   - `=`: É o **operador de atribuição**. Pega o valor da direita (`"Ada"`) e o guarda dentro de `$nombre`.
2. `print("Hola, " . $nombre)`:
   - O ponto `.` é o **operador de concatenação de texto**. Une o texto `"Hola, "` com o conteúdo guardado em `$nombre` (`"Ada"`), formando `"Hola, Ada"`.
   - Você também pode incorporar a variável diretamente dentro das aspas duplas escrevendo `print("Hola, ${nombre}")`.

> [!IMPORTANT]
> **Criar uma variável (`var $nombre = ...`) vs Alterar seu valor (`$nombre = ...`)**
> No Joss, na **primeira vez** em que você cria uma variável, deve declará-la com `var` (ou indicando seu tipo, como `string $nombre = "Ada"`). Se tentar escrever `$nombre = "Ada"` pela primeira vez sem tê-la declarado antes, o Joss avisará com o erro `JOSS-SYM-001` (*Variable is not declared*). Uma vez declarada, você pode alterar seu valor mais adiante escrevendo `$nombre = "Carlos"`.

---

## 8. Dialogar com o usuário: Entrada pelo teclado com `cin >>`

Agora vamos fazer com que o programa nos faça perguntas, espere que digitemos no teclado e nos responda. Todo programa interativo segue três passos:

```text
┌─────────────────────────┐       ┌─────────────────────────┐       ┌─────────────────────────┐
│     1. ENTRADA          │  ──>  │     2. PROCESO          │  ──>  │     3. SALIDA           │
│ El usuario escribe con  │       │ El programa calcula o   │       │ Se muestra el resultado │
│ cin >> $variable        │       │ prepara la respuesta    │       │ en pantalla con print() │
└─────────────────────────┘       └─────────────────────────┘       └─────────────────────────┘
```

Escreva no seu `main.joss`:

<!-- joss-check: lectura interactiva de datos por teclado -->
```joss
public class Main {
    Init main() {
        string $nombre = ""
        int $edad = 0

        print("¿Cómo te llamas?")
        cin >> $nombre

        print("¿Cuántos años tienes?")
        cin >> $edad

        print("¡Mucho gusto, " . $nombre . "! El próximo año tendrás " . ($edad + 1) . " años.")
    }
}
```

### Como funciona este programa quando você o executa com `joss run main.joss`?

1. `string $nombre = ""` e `int $edad = 0`: Criamos duas variáveis prontas para receber dados: uma para texto (`string`) inicialmente vazia `""`, e outra para um número inteiro (`int`) inicialmente em `0`.
2. `cin >> $nombre`: Pausa o programa e espera que você digite seu nome no terminal e pressione **Enter**. O que você digitar é guardado em `$nombre`.
3. `cin >> $edad`: Espera que você digite sua idade e a converte automaticamente em um número inteiro dentro de `$edad`.
4. `($edad + 1)`: Soma `1` à idade que você digitou e mostra a saudação completa.

Exemplo de sessão real no seu terminal:

```text
¿Cómo te llamas?
Ada
¿Cuántos años tienes?
20
¡Mucho gusto, Ada! El próximo año tendrás 21 años.
```

---

## 9. Resumo rápido do seu primeiro capítulo

Guarde esta tabela como referência do que você já domina:

| O que você quer fazer? | Comando ou código em Joss | Exemplo real |
|---|---|---|
| **Criar um projeto de console** | `joss new console <nombre>` | `joss new console mi-primer-joss` |
| **Executar seu projeto** | `joss run main.joss` | `joss run main.joss` |
| **Revisar erros sem executar** | `joss analyze main.joss` | `joss analyze main.joss` |
| **Compilar para executável nativo** | `joss build main.joss` | Gera `build/main.exe` (ou `build/main`) |
| **Mostrar uma mensagem na tela** | `print(...)` | `print("¡Hola mundo!")` |
| **Criar uma variável nova** | `var $variable = valor` | `var $ciudad = "Madrid"` |
| **Alterar uma variável já criada** | `$variable = nuevo_valor` | `$ciudad = "Lima"` |
| **Unir textos** | `texto1 . texto2` ou `"${variable}"` | `"Hola, " . $nombre` |
| **Ler o que o usuário digita** | `cin >> $variable` | `cin >> $nombre` |

### As Três Regras de Ouro para começar sem tropeços

1. **Toda variável começa com `$`**: Escreva sempre `$nombre`, `$edad`, `$total`.
2. **Declare antes de usar**: Use `var $x = ...` (ou o tipo como `int $x = ...`) na primeira vez que criar uma variável.
3. **Tudo o que abre, fecha**: Aspas `"..."`, parênteses `(...)` e chaves `{...}` vão sempre em pares.

---

## 10. Se algo não funcionar: Diagnóstico rápido

| Sintoma ou mensagem | Causa provável | Como solucionar |
|---|---|---|
| `joss: command not found` ou `el término 'joss' no se reconoce` | O terminal ainda não recarregou o `PATH` após instalar o Joss. | Feche a janela do terminal, abra-a novamente e teste `joss version`. |
| `No se encontró el archivo 'main.joss'` | Você não entrou na pasta do projeto que criou. | Execute `cd mi-primer-joss` antes de rodar `joss run main.joss`. |
| `JOSS-PARSE-001` (Erro de sintaxe) | Falta fechar uma aspa `"`, um parêntese `)` ou uma chave `}`. | Observe o número da linha indicado pelo Joss e verifique se cada aspa e chave tem o seu par. |
| `JOSS-SYM-001: Variable '$x' is not declared` | Você escreveu `$x = valor` sem ter criado antes a variável, ou escreveu seu nome errado. | Adicione `var` ao criá-la pela primeira vez (`var $x = valor`) e revise maiúsculas/minúsculas (`$edad` e `$Edad` são diferentes). |

---

## 11. Prática do capítulo

Coloque à prova o que aprendeu diretamente no arquivo `main.joss` do seu projeto `mi-primer-joss`:

### 1. Teste rápido
No exemplo da seção 7:
1. Mude `"Ada"` para o seu próprio nome.
2. Adicione uma terceira linha `print("Estoy aprendiendo a programar")` antes de fechar a chave `}` de `Init main()`.
3. Execute `joss run main.joss` e comprove que as três linhas aparecem em ordem.

### 2. Exercício guiado: Seu cartão de visitas
Modifique `Init main()` seguindo estes 4 passos:
1. Declare uma variável com seu nome: `var $nombre = "Tu Nombre"`.
2. Declare uma variável com seu país: `var $pais = "Tu País"`.
3. Declare uma variável com sua meta: `var $meta = "Crear aplicaciones en Joss"`.
4. Use `print` e o operador `.` (ou `${...}`) para mostrar um resumo de três linhas no terminal.

### 3. Desafio: Calculadora do dobro de um número
1. Declare uma variável inteira `int $numero = 0`.
2. Mostre uma mensagem com `print("Escribe un número entero:")`.
3. Leia o número do teclado com `cin >> $numero`.
4. Mostre na tela: `"El doble de tu número es: " . ($numero * 2)`.
5. *(Opcional)* Execute `joss build main.joss` e observe como o Joss cria seu próprio programa executável dentro da pasta `build/`.

---

## Próximo passo

Agora que você já sabe criar um projeto com `joss new console`, entender `main.joss`, mostrar mensagens, criar variáveis e pedir dados pelo teclado, é hora de conhecer todos os tipos de dados que o Joss manipula (números exatos para dinheiro, decimais, booleanos e texto) e como realizar operações com eles.

Continue com: [Valores, variáveis e operações fundamentais](FUNDAMENTOS.md).