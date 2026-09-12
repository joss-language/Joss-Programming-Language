# Funções globais

[Índice](README.md) · Antes: [funções](FUNCIONES.md) · [Classes nativas](MODULOS_NATIVOS.md)

Essas funções são integradas ao runtime, sem importação ou instalação.
A assinatura nesta página descreve o uso suportado pelo despachante; nem todos
os parâmetros são publicados no analisador. `[x]` indica um argumento
opcional e `...` vários argumentos. Não são personagens que você deva copiar.

As famílias compartilham exemplos no final. Aliases de funções como
`doubleval` ainda existe; isso não restaura o tipo de fonte excluído
`double`. Os retornos declarados para ferramentais podem ser consultados no
[catálogo gerado](CATALOGO_NATIVO.md); O resultado do tempo de execução é descrito abaixo.

## Conversão e tipo de consultas

| Assinatura | Resultado e limites | Exemplo |
|---|---|---|
| `intval(valor)` | Todo; truncar float/decimal, bool definido como 0/1; inválido ou ausente dá 0. | `intval("12")` → 12 |
| `floatval(valor)`, `doubleval(valor)` | Flutuação aproximada; inválido dá 0. | `floatval("1.5")` |
| `decimal([valor])` | Decimal de número/texto/bool; inválido ou nulo dá zero. Os sufixos textuais m/M/d/D são cortados. | `decimal("1.25")` |
| `strval(valor)` | Formato Text by Go, null fornece texto vazio. Não é JSON. | `strval(12)` |
| `boolval(valor)` | Bool de acordo com a verdade do tempo de execução. Zero float e mapa vazio têm diferenças de outras linguagens. | `boolval("")` → falso |
| `is_numeric(valor)` | Bool: número ou texto interpretável como float/decimal. Não valida um domínio como “idade”. | `is_numeric("12")` |
| `is_int(valor)`, `is_integer(valor)` | Bool; inteiro, sem converter strings. | `is_int("12")` → falso |
| `is_float(valor)`, `is_double(valor)` | Bool; flutuar, não convertido. | `is_float(1.5)` |
| `is_decimal(valor)` | Bool; decimal concreto. | `is_decimal(1m)` |
| `is_string(valor)` | Bool; texto específico. | `is_string("a")` |
| `is_array(valor)` | Bool; matriz/fatia, não mapa. | `is_array([1])` |
| `is_null(valor)` | Bool; ausência de valor; nenhum argumento também é verdadeiro. | `is_null(nil)` |
| `isset(expresiones...)` | Forma especial do analisador: consulta de existência sem erro por falta de variável. Uma variável vinculada a nulo conta como existente; Não serve como uma verificação geral das chaves do mapa. | `isset($nombre)` |
| `empty(expresion)` | Forma especial: verdadeiro se não existir ou se o valor for falso de acordo com as regras de tempo de execução. | `empty($ausente)` |

As conversões explícitas são permissivas; Eles não substituem a validação.
Para condições exatas compare com `0`, `null` ou `""` dependendo do que
você precisa ver [tipos](SISTEMA_TIPOS.md) e [sintaxe](SINTAXIS.md).

## Matrizes e mapas

| Assinatura | Resultado, mutação e erros | Exemplo |
|---|---|---|
| `len(valor)`, `count(valor)` | comprimento da matriz/mapa; bytes de string; outros valores dão 0. | `count([2,3])` → 2 |
| `keys(map)`, `array_keys(map)` | Matriz de chaves sem ordem garantida; inválido dá []. | `keys({"a":1})` |
| `values(map)`, `array_values(map)` | Matriz de valores sem pedido garantido; inválido dá []. | `values({"a":1})` |
| `explode(separador, texto)` | Matriz de segmentos; requer duas strings, inválido fornece nulo. | `explode(",", "a,b")` || `append(array, elemento)` | Retornar array expandido; reatribui o resultado. Inválido dá nulo. | `$a = append($a, 2)` |
| `merge(array1, array2)` | Novo contêiner com ambas as sequências; inválido dá nulo. | `merge([1],[2])` |
| `array_merge(primero, otros...)` | Matriz concatenada ou mapa combinado de acordo com o primeiro argumento; as últimas chaves vencem. Ignore o seguimento de outros tipos; inválido dá []. | `array_merge({"a":1},{"a":2})` |
| `array_push(array, elementos...)` | Matriz Expandida; não retorna comprimento. Reatribuir. Inválido dá nulo. | `$a = array_push($a,2,3)` |
| `end(array)`, `array_pop(array)` | Último elemento ou nulo. **Eles não reduzem a matriz**. | `array_pop([1,2])` → 2 |
| `array_shift(array)` | Primeiro elemento ou nulo. **Não reduz a matriz**. | `array_shift([1,2])` → 1 |
| `array_slice(array,inicio,[longitud])` | Vista de superfície; negativos em relação ao final; sai dá []. Compartilhe armazenamento. | `array_slice([1,2,3],1)` |
| `array_unique(array)` | Nova matriz, igualdade por representação textual; você pode mesclar valores de diferentes tipos. | `array_unique([1,1,2])` |
| `array_reverse(array)` | nova matriz invertida; inválido dá []. | `array_reverse([1,2])` |
| `array_column(array,clave)` | Valores dessa chave nos elementos do mapa; omite outros elementos/chaves ausentes. | `array_column([{"id":1}],"id")` |
| `in_array(valor,array)` | Bool; em matrizes Joss, ele suporta igualdade profunda ou textual. | `in_array(1,["1"])` → verdadeiro |
| `array_key_exists(clave,map)` | existência bool; não deve ser confundido com valor não nulo. | `array_key_exists("a",{"a":null})` |

## Texto e formatação

| Assinatura | Resultado e limites | Exemplo |
|---|---|---|
| `print(valores...)`, `echo(valores...)` | Eles imprimem cada argumento com uma quebra de linha; retornar nulo. | `print("Hola")` |
| `printf(formato,valores...)` | Formato Go, sem salto implícito; retornar nulo. | `printf("%s: %d\\n","Edad",20)` |
| `strlen(texto)` | Número de pontos Unicode; nulo dá 0. | `strlen("é")` → 1 |
| `str_contains(texto,parte)`, `contains(texto,parte)` | Bool; argumentos são convertidos em texto. | `contains("casa","as")` |
| `str_starts_with(texto,prefijo)`, `starts_with(...)` | Prefixo bool. | `starts_with("abc","a")` |
| `str_ends_with(texto,sufijo)`, `ends_with(...)` | Postfix bool. | `ends_with("abc","c")` |
| `str_replace(buscar,reemplazo,texto)` | Substitui todas as correspondências. A ordem difere de Str::replace. | `str_replace("a","o","casa")` |
| `strtolower(texto)`, `to_lower(texto)` | Unicode minúscula. | `to_lower("HOLA")` |
| `strtoupper(texto)`, `to_upper(texto)` | Unicode maiúscula. | `to_upper("hola")` |
| `ucfirst(texto)`, `lcfirst(texto)` | Altere o primeiro ponto Unicode para maiúsculas/minúsculas. | `ucfirst("hola")` |
| `ucwords(texto)` | Capitalização de acordo com Go strings.Title; não análise linguística. | `ucwords("hola mundo")` |
| `trim(texto,[conjunto])` | Remove espaços ou caracteres Unicode do conjunto nas extremidades. | `trim(" hola ")` |
| `ltrim(texto,[conjunto])`, `rtrim(texto,[conjunto])` | Apare em uma extremidade; espaços ASCII padrão. | `ltrim(" hola")` || `substr(texto,inicio,[longitud])` | Pontos Unicode; negativos em relação ao final; sai dá "". | `substr("abc",-2)` → bc |
| `strpos(texto,parte)` | Posição em pontos Unicode ou falso se estiver faltando. Zero é uma correspondência válida. | `strpos("abc","a")` → 0 |
| `implode(separador,array)`, `join(separador,array)` | Converta elementos em texto e junte; inválido dá "". | `join("-",[1,2])` |
| `str_pad(texto,longitud,[relleno])` | Adiciona preenchimento à direita em **bytes**, espaço padrão. O enchimento vazio pode causar pânico; não use. | `str_pad("7",3,"0")` → 700 |
| `str_repeat(texto,cantidad)` | Texto repetido; Negativo é igual a zero. | `str_repeat("a",3)` |
| `html_escape(valor)` | Escapes &, <, > e aspas para HTML; nulo dá "". | `html_escape("<b>")` |
| `md5(texto)`, `sha1(texto)`, `sha256(texto)` | resumo hexadecimal de bytes; sem criptografia ou hash de senha. | `sha256("dato")` |
| `base64_encode(texto)` | Texto Base64, sem segredo criptográfico. | `base64_encode("a")` → YQ== |
| `base64_decode(texto)` | String de bytes ou falso se for inválido. | `base64_decode("YQ==")` |

Para senhas use o contrato Auth. Para personagens percebidos
como emojis compostos, consulte [unidades de texto](COLECCIONES.md).

## Números e hora

| Assinatura | Resultado e limites | Exemplo |
|---|---|---|
| `round(numero,[precision])` | Flutuador arredondado; Precisão padrão 0. Não processa decimal.Decimal. | `round(1.25,1)` |
| `floor(numero)`, `ceil(numero)` | Número inteiro para baixo/para cima; Eles aceitam float/int, não decimal. | `floor(1.9)` → 1 |
| `abs(numero)` | magnitude int/float. O int64 mínimo pode estourar sem diagnóstico aqui. | `abs(-2)` |
| `min(valores...)`, `max(valores...)` | Eles também aceitam um array não vazio; sem argumentos nulos. Uma única matriz vazia é retornada como tal. | `max([1,3,2])` |
| `rand([min,max])` | inteiro inclusivo; sem dois limites use 0..MaxInt32. max<=min retorna min. Não criptográfico. | `rand(1,6)` |
| `time()` | Segundos Unix como número inteiro. | `time()` |
| `microtime([comoFloat])` | Com true, flutue em segundos; por padrão, texto "fração de segundos". | `microtime(true)` |
| `date([formato,timestamp])` | Texto; por padrão Y-m-d H:i:s e hora atual. O carimbo de data/hora oferece suporte a número, texto ou hora do host.Valor de hora. | `date("Y-m-d",0)` |
| `now([formato])` | Texto da data atual; mesmo formato padrão. | `now("H:i:s")` |
| `strtotime(texto,[base])` | Inteiro Unix ou nulo se o texto não for reconhecido. Analisador limitado de datas e deslocamentos. | `strtotime("+1 day",0)` |
| `sleep(segundos)`, `usleep(microsegundos)` | Eles bloqueiam a tarefa atual; retornar nulo. sleep suporta frações flutuantes. | `sleep(0.01)` |

Os tokens de data suportados e expressões relativas são implementados em
[date_utils.go](../../pkg/core/date_utils.go). Nem toda gramática é incorporada
de datas PHP. O fuso horário depende do processo; evite saídas de data
local em testes que devem ser idênticos em qualquer máquina.

## Arquivos, serialização e processos

| Assinatura | Resultado e erros | Exemplo |
|---|---|---|
| `file_exists(ruta)` | Bool; inclui diretórios; false em erro de estatística. | `file_exists("datos.json")` || `is_dir(ruta)`, `is_file(ruta)` | Bool; is_file significa existente e não diretório. | `is_dir("storage")` |
| `file_get_contents(ruta)` | String de bytes ou nulo em caso de falha. Somente arquivos locais. | `file_get_contents("datos.json")` |
| `file_put_contents(ruta,texto)` | Substituir/criar arquivo; verdadeiro ou falso. Não crie pais. | `file_put_contents("nota.txt","Hola")` |
| `unlink(ruta)`, `file_delete(ruta)` | Exclua arquivo ou diretório vazio; sucesso bool. | `unlink("nota.txt")` |
| `mkdir(ruta)` | Crie pai incluindo diretórios; bool. | `mkdir("storage/informes")` |
| `json_encode(valor)` | JSON com recuo conforme JsonEncode; "" em caso de erro. | `json_encode({"ok":true})` |
| `json_decode(texto)` | Valor nativo ou nulo em caso de erro (também JSON nulo). Os números são decodificados para flutuar. | `json_decode("[1,2]")` |
| `json_verify(texto)` | Sintaxe JSON bool, não estrutura de negócios bool. | `json_verify("{}")` |
| `toon_encode(valor)` | Formato textual simplificado de registros; fuga não binária e não completa. | `toon_encode([{"a":"b"}])` |
| `toon_decode(texto)` | Matriz de registros do subconjunto ou nulo. Valores textuais. | Use saída de codificador compatível. |
| `toon_verify(texto)` | Verificação limitada de cabeçalho/estrutura; não integridade criptográfica. | Não use para validar dados hostis. |
| `hive_read_box(ruta)` | Matriz de entradas ou nulo; erro de impressão. Leitor de formato Hive parcial. | Requer arquivo Hive compatível. |
| `run(ruta,args...)` | Execute .py com python ou .php com php; saída combinada. Requer ALLOW_SYSTEM_RUN=true e executável instalado. Falha na impressão e retorno de saída/""; Ele nem sempre joga. | `run("script.py","dato")` |

Os caminhos são relativos ao diretório de trabalho. As operações não têm
caixa de areia de arquivo. O exemplo completo está em [relatório de compras](PROYECTO_CONSOLA.md).

## Contexto da Web e simultaneidade

| Assinatura | Contrato e disponibilidade |
|---|---|
| `env(clave,[default])`, `config(clave,[default])` | Consulta r.Env; retorna string ou padrão arbitrário se estiver ausente/vazio. Ele não lê .env novamente. |
| `view(nombre,[datos])` | Delegar Visualização::render; contexto do modelo de projeto. |
| `json(datos,[status])` | Delega Response::json e retorna WebResponse; **não é serialização para string**. |
| `response(cuerpo,[status,mime,headers])` | Resposta dos delegados::raw. |
| `redirect(url,[status])`, `back()` | redirecionar WebResponse; back usa contexto de solicitação. |
| `request([clave,default])` | Sem chave você obtém Request::all; com a chave Request::input. |
| `session([clave])` | Nenhuma chave de objeto de sessão ou nula; com a chave Session::get. Não incorpora uma sessão de console. |
| `__(clave)` | Tradução usando localidade atual e gerenciador i18n. |
| `csrf_field()` | _token campo HTML usando sessão; sem ele token vazio. |
| `async { instrucciones }` | Sintaxe que cria Future e executa um encerramento em fork/goroutine. |
| `await(future)` | Espera e retorna resultado; propaga o fracasso. |
| `make_chan([capacidad])` | Canal; capacidade 0 sincroniza transmissor e receptor. |
| `send(channel,valor)` | Enviar; pode bloquear, canal fechado causa falha. Também canal << valor. || `recv(channel)` | Receber; bloqueia até dados/fechamento; o fechamento esgotado produz nulo. |
| `close(channel)` | Fechar; fechar duas vezes ou enviar após falhar. |

Consulte [HTTP e classes nativas](MODULOS_NATIVOS.md), [views](VISTAS.md) e
[concurrency](CONCURRENCIA.md) para exemplos com o contexto necessário.

## Exemplo executável de utilitários<!-- joss-run: ["Ana", "3", "a-b", "true", "Hola", "2"] -->
```joss
print(ucfirst(trim(" ana ")))
print(strlen("sol"))
print(join("-", ["a", "b"]))
print(array_key_exists("id", {"id": null}))
print(base64_decode(base64_encode("Hola")))
print(floor(2.8))
```
## Diferenças entre metadados e implementação

Retornos postados incompletos observados: boolval anuncia int mas
retornar bool; json anuncia string mas retorna WebResponse; microtempo
anuncia float embora sem true retorne string; strpos e base64_decode podem
retornar falso; array_merge também retorna mapa. Não atribua esses erros
ao usuário ou forçar anotações incompatíveis para satisfazer o catálogo.
O [relatório de auditoria](DOCUMENTATION_AUDIT.md) registra esta dívida.

Fontes: [catálogo](../../pkg/core/builtins.go),
[matrizes e conversões](../../pkg/core/builtins_array.go),
[textos](../../pkg/core/builtins_string.go), [E/S](../../pkg/core/builtins_io.go),
[hora](../../pkg/core/builtins_date.go), [async](../../pkg/core/builtins_async.go).