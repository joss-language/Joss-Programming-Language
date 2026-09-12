# Classes nativas e serviços integrados

[Índice](README.md) · [Funções globais](FUNCIONES_GLOBALES.md) · [Catálogo completo](CATALOGO_NATIVO.md)

Uma classe nativa fornece operações implementadas em Go. Registrado ao preparar
o tempo de execução; não requer importações. Uma fachada usa `Clase::metodo(...)`; um objeto
com estado use `$objeto->metodo(...)`. O catálogo lista **cada nome registrado,
seu retorno publicado e o manipulador de origem**. Este guia explica os contratos e o contexto;
Os aliases de catálogo herdam o contrato de seu nome principal, a menos que seja indicada dívida.

Os parâmetros entre colchetes são opcionais na notação de referência.
Muitas APIs retornam nulo/falso ou imprimem um erro em vez de lançar uma exceção.
O fato de o analisador aceitar uma chamada nativa não demonstra aridade correta: faltando
metadados de parâmetro em parte da biblioteca.

## Utilitários sem serviços externos

| Classe e assinatura | Resultado, erros e exemplo |
|---|---|
| `Math::random(min,max)` | inteiro inclusivo; requer dois inteiros e intervalos válidos; Faixa invertida pode causar pânico. Não criptográfico. Ex.: Matemática::aleatório(1,6). |
| `Math::floor(n)`, `ceil(n)`, `abs(n)` | resultado flutuante para entradas numéricas conversíveis; Eles não são equivalentes em tipo aos auxiliares globais de piso/teto. |
| `Str::length(texto)` | Bytes UTF-8, 0 para argumento inválido. |
| `Str::random([longitud])` | Texto alfanumérico, padrão 16, matemática/rand; não use como token secreto. O comprimento negativo falha. |
| `Str::startsWith(texto,prefijo)`, `contains(texto,parte)` | Bool; requer cordas. |
| `Str::substring(texto,inicio,[longitud])` | Pontos Unicode; o início negativo é definido como zero. O comprimento negativo pode falhar. |
| `Str::indexOf(texto,parte)` | Índice em pontos Unicode ou -1. |
| `Str::trim(texto)`, `replace(texto,buscar,nuevo)` | Corda. A ordem de substituição difere de str_replace. |
| `UUID::generate()`, `v4()` | Identificador UUID textual. |
| `JSON::parse(texto)`, `decode(texto)` | Valor decodificado ou nulo; Os números JSON são flutuantes, e não números inteiros arbitrariamente precisos. |
| `JSON::stringify(valor)`, `encode(valor)` | JSON compacto ou "" em caso de falha. |
| `Markdown::toHtml(texto)`, `readFile(ruta)` | HTML renderizado; readFile requer arquivo local. Não substitui a autorização de rota ou a higienização de conteúdo não confiável. |
| `new Stack()` → `push(valor)`, `pop()`, `peek()` | Pilha: último a entrar, primeiro a sair. pop remove; vazio retorna nulo. |
| `new Queue()` → `enqueue(valor)`, `dequeue()`, `peek()` | Fila: primeiro a entrar, primeiro a sair. desenfileirar remove; nulo nulo. |
| `new Exception(mensaje,[codigo])` → `getMessage()`, `getCode()` | Objeto de erro com campos; código padrão 0. Não gera um diagnóstico JOSS. |<!-- joss-run: ["abc", "1", "dos", "uno"] -->
```joss
print(Str::trim(" abc "))
print(Str::indexOf("casa", "a"))
$pila = new Stack()
$pila->push("uno")
$pila->push("dos")
print($pila->pop())
print($pila->peek())
```
## HTTP de saída

Um cliente HTTP solicita informações de outro servidor; O roteador responde às solicitações
que chegam à sua aplicação. Não os confunda.

| Assinatura HTTP | Retorno e comportamento |
|---|---|
| `get(url,[headers])`, `delete(url,[headers])` | Corda corporal; o erro pode ser semelhante a "". Tempo limite de 15 segundos. |
| `post/put/patch(url,[datos,headers])` | Corpo de corda. O mapa é serializado para JSON ou formulário, se Content-Type indicar. |
| `head(url,[headers])`, `options(url,[headers])` | Mapa dos primeiros cabeçalhos por nome; vazio diante de certos erros. |
| `json(metodo,url,[datos,headers])` | Valor JSON decodificado; erros podem ser convertidos para mapear error.message. Tempo limite de 30 segundos. Um HTTP 4xx com JSON válido ainda retorna esse JSON. |
| `request(metodo,url,[opciones])` | mapa de status, status_text, corpo, cabeçalhos, sucesso; pode incluir json ou erro. o sucesso requer 2xx. |

Opções de solicitação: `headers` e `query` como mapas; órgão prioritário
`body`, depois `json`, depois `form`; `timeout` em segundos (15 se <=0);
`follow_redirects` bool, verdadeiro por padrão. A chave json é decodificada
automaticamente somente quando o corpo começa com { ou [. Uma falha de rede
fornece status 0 e erro, não uma resposta HTTP bem-sucedida.

Fragmento Contextual: Requer um servidor escutando nesse endereço.<!-- joss-check: necesita servicio HTTP local -->
```joss
$respuesta = Http::request("GET", "http://127.0.0.1:8080/saludo/Ana", {"timeout": 3})
$respuesta["success"] ? {
    print($respuesta["body"])
} : {
    print("No se pudo consultar el servicio")
}
```
`Http::query` possui código interno mas **não está registrado**. Usar
`Http::request("QUERY", url, opciones)` se o servidor suportar esse método.

## Servidor, solicitação e resposta

| Classe | Contratos |
|---|---|
| `Router` | get/post/put/patch/delete/head/options/query(caminho,manipulador); qualquer(caminho,manipulador); match(métodos, caminho, manipulador); api(caminho,manipulador); ws(caminho, manipulador). Cadastrar rotas; não faz solicitações de saída. group(name,callback), middleware(name), RegisterMiddleware(name,callback), end() gerencia middleware. Consulte [HTTP](CONTROLADORES.md) e [middleware](MIDDLEWARE.md). |
| `Request` | input/post(key,[default]) obtém dados combinados; all() e except(arrayClaves) retornam um mapa filtrado por uma lista específica de campos internos. Eles não são uma validação ou uma lista de campos permitidos. |
| `Request` | arquivo(chave) → mapear com conteúdo ou nulo; hasFile/hasfile(chave) → bool; has(key) requer valor diferente de null e "". |
| `Request` | cookie(chave,[padrão]), cabeçalho(chave), root(), método(), isMethod/ismethod(verbo), path(), url(), ip(), userAgent/useragent(), bearerToken/bearertoken(). url atualmente prioriza _path; não promete URL absoluto. uri não está registrado. Sem contexto, vários retornam valores padrão. |
| `Response` | json(dados,[status=200]), erro(mensagem,[status=400]), redirecionamento(url,[status=302]), back(), raw(corpo,[status=200,mime,cabeçalhos]), stream(retorno de chamada), download(caminho,[nome]). retornar WebResponse; download e stream são resolvidos ao despachar HTTP. |
| `Redirect` | to(url,[status=302]) → WebResponse. |
| `WebResponse` | com(chave,valor) adicione flash; withCookie(nome,valor), withHeader(nome,valor), status(código) sofrem mutação e retornam a mesma resposta. |
| `Session` | get(chave), put(chave,valor), has(chave), esquecer(chave), todos(). Nenhuma sessão injetada retorna nulo; não faz login chamando a fachada. |
| `View` | render(nome,[mapa]), existe(nome), compartilhar(chave,valor) ou compartilhar(mapa). Consulte [visualizações](VISTAS.md). |
| `Stream` | Objeto recebido pelo retorno de chamada SSE: send(data) ou send(type, data), close(). Sem um escritor válido não funciona. |
| `WebSocket` | enviar(mensagem), onMessage(retorno de chamada), onClose(retorno de chamada), inscrever-se(canal), cancelar(canal), publicar(canal, mensagem), subscriberCount(canal), transmitir(mensagem), fechar(). Hub local para o processo; [contrato completo](WEBSOCKETS.md). |
| `Server` | start() solicita modo servidor do host; Não é um novo ouvinte independente em nenhum contexto. spawn(nome, comando, porta) inicia o processo auxiliar e registra o proxy de acordo com a configuração; precisa de permissão de execução. |
| `Middleware`, `Migration` | Classes base registradas sem métodos nativos públicos; Convenções de estrutura, não tipo de interfaces de sistema. |

Nos uploads use `$archivo["content"]`. Para binários use raw com MIME e
Disposição de conteúdo adequada ou download para arquivo. O corpo HTML normal
pode receber recarga a quente em desenvolvimento.

## Dados, estado e armazenamento

| Classe e assinaturas | Retorno, contexto e erros |
|---|---|
| `GranDB` | Operações de construtor e SQL: [referência completa](MODELOS.md). Requer banco de dados configurado. || `Schema`, `Blueprint` | Alterações de tabela/coluna: [Construtor de esquema](SCHEMA_BUILDER.md). Nomes como inteiro, duplo e booleano são métodos SQL válidos, não aliases do tipo Joss. |
| `SQLite::open(ruta)`, `query(sql,[bindings])`, `close()` | Conexão SQLite nativa; abrir/fechar bool, coleção de consultas ou impressão nula e de erros. Mantenha a conexão com a instância. |
| `Cache::put(clave,valor,[segundos=60])` | Cache global na memória do processo; verdadeiro ou nulo para argumentos inválidos. Não persistente. |
| `Cache::get(clave,[default])`, `has(clave)`, `forget(clave)` | get retorna valor/padrão/nulo. A entrada expirada retorna nulo mesmo que o padrão tenha sido entregue. tem bool; esqueça verdadeiro/nulo. |
| `Redis::connect(host,[password,db])` | Configurar cliente; também se conecta automaticamente com REDIS_URL ou REDIS_HOST. Requer Redis externo. |
| `Redis::set(clave,valor,[ttl])`, `get(clave)`, `has(clave)` | Escrita, leitura ou existência; ausência/erros podem produzir nulo/falso. Verifique a serialização antes de assumir o tipo recuperado. |
| `Redis::del(clave)`, `forget(clave)`, `ttl(clave)`, `flush()` | Limpo, TTL em segundos (-1 sem expiração, -2 ausente); flush libera o banco de dados Redis atual. |
| `UserStorage::put(token,nombre,contenido)` | Bool; armazenamento local/OCI selecionado por ambiente; requer configuração e tabelas internas. |
| `UserStorage::get(token,nombre)`, `getToFile(token,nombre,destino)`, `delete(token,nombre)` | String/nulo, bool e bool respectivamente. O token pode ser um usuário com user_token. |
| `UserStorage::path([ruta])` | Rota local em armazenamento; não baixa objetos OCI. |
| `Zip::extract(archivo,destino)` | Bool; extraia arquivos com verificação de caminho. Você pode escrever antes de encontrar um erro posterior: não é uma operação atômica. |

Não use um cache como única cópia de informações insubstituíveis. Um mapa salvo
no Cache ainda pode compartilhar seu conteúdo: o contêiner simultâneo não
torna automaticamente todos os valores armazenados seguros.

## Sistema, plugins e tarefas

| Assinatura | Contrato |
|---|---|
| `System::env(clave,[default])` | Valor de r.Env ou padrão. |
| `System::Run(comando,[arrayArgs])` | Executa processo externo e retorna saída; requer ALLOW_SYSTEM_RUN=true/1. Não avalia o código Joss. |
| `System::load_driver(ruta,[nombre])` | Bool ao carregar DLL/SO/dylib ABI C v1; depende da plataforma/construção. |
| `System::driver_call(nombre,metodo,[args])` | Resultado JSON decodificado, texto ou nulo em caso de erro. |
| `System::log(mensaje)`, `sleep(segundos)`, `now([dias])` | Log, espera inteira e data textual com deslocamento de dias. System::now não é formatado pelo auxiliar now. |
| `Plugin::platform()`, `path(nombre,ruta)` | plataforma os-arch e caminho de recursos; Rotas inválidas podem ser iniciadas. |
| `Plugin::call(nombre,metodo,[args])`, `stream(nombre,metodo,[args,callback])` | Ponte para plugin; resultados polimórficos e erro de mapa/nulo dependendo da rota. [Plugins](PLUGINS.md). || `new Process(comando,[arrayArgs])` | Preparar processo; requer permissão. start() → bool; wait() → código de saída ou -1; matar() → bool; pid() → inteiro; stdin(texto) → instância; stdout_chan()/stderr_chan() → canais. Drene as saídas para evitar bloqueios. |
| `Cron::schedule(nombre,expresion,callback)` | Registrar trabalhos de casa; cron de minuto com gramática e estado local limitados. |
| `Task::on_request(nombre,intervalo,callback)` | Atualmente inicia goroutine ao ligar; intervalo não é usado. Não promete execução para cada solicitação. |

Para coordenação e fechamento de canais leia [concurrency](CONCURRENCIA.md).
Os recursos externos não são serializáveis ​​como os dados da linguagem comum.

## Identidade, tradução e publicação

Auth, AuthLoginResult, MFA e TwoFactor são explicados em
[autenticação](AUTENTICACION.md): precisa de contexto, tabelas e uma política
autorização do pedido.

`Lang::get(clave,[reemplazos])` tradução de retorno; `set(locale)` altera o local;
`locale()` consulta os atuais e `locales()` lista os disponíveis. Os arquivos
idioma são carregados pela infraestrutura i18n. A tradução não foi obtida
da rede automaticamente.

`SEO::title(texto)`, `description(texto)`, `keywords(textoOArray)`,
`canonical(url)`, `og(propiedad,contenido)` e `meta(nombre,contenido)`
atualizar metadados; `render()` produz HTML. **SEO::twitter não está registrado**:
use meta para nomes do Twitter:*.

`Sitemap::add(url,[lastmod,changefreq,priority])` ou `add(mapa)`, `exclude(rutaOArray)`,
`generate()`, `xsl()` gera XML/XSL. `provider(callback)` está registrado,
mas o manipulador aceita apenas FunctionLiteral e um fechamento de origem é avaliado como
CapturedFunction: Seu registro não é garantido. Use adição explícita enquanto
essa borda é corrigida. Consulte [SEO e mapa do site](SEO_SITEMAP.md).

A lista fechada do [catálogo](CATALOGO_NATIVO.md) permite verificar quais APIs
estão disponíveis. A presença de uma função Go, um comentário ou sugestão
do editor não é suficiente para torná-lo um método público.