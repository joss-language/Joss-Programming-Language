# Autenticação e segundo fator

[Índice](README.md) · Antes: [HTTP](CONTROLADORES.md) , [modelos](MODELOS.md) · Depois: middleware [MIDDLEWARE.md](MIDDLEWARE.md)

**Autenticar** é verificar quem faz uma solicitação. **Autorizar** é decidir
o que essa pessoa pode fazer. Auth integra usuários, senhas e JWT, mas uma chamada
como update(id,data) não substitui as verificações de permissões do driver
. As operações

requerem conexão, tabelas de autenticação e segredos configurados.
Um JWT é um token assinado; não deve ser confundido com uma senha ou
publicado em logs. Esta referência descreve o código local, ela não certifica a segurança
de um aplicativo implementado.

## Consulta o fragmento

do usuário atual para um controlador com sessão já validada:

<!-- joss-check: requiere contexto autenticado -->
```joss
$usuario = Auth::user()
$usuario != null ? {
    print($usuario->email)
} : {
    print("Sin sesion")
}
```

user() retorna **instância ou nulo**, não mapa ou JSON. Utilize `->` ; para um ID
prefere `Auth::id()` . Passa os campos escalares selecionados para visualizações em vez de
em toda a instância.

## Contratos de autenticação

| Assinatura | Retorno e efeito |
|---|---|
| `hash(contraseña)` | Hash bcrypt como string ou nulo em caso de falha. |
| `create(mapaDatos)` | Token do usuário ou falso dependendo da inserção; requer dados de esquema. Não é envio automático. |
| `attempt(email,contraseña)` | JWT ou falso; valida credenciais de acordo com as regras da tabela. Ele próprio não incorpora todo o fluxo de fluido MFA. |
| `login(email,contraseña)` | AuthLoginResult ou null para argumentos insuficientes. |
| `check()` , `guest()` | Bool no contexto da sessão. |
| `user()` , `id()` | Instância/ID atual ou nulo. |
| `hasRole(nombre)` | Bool; não verifica a propriedade de um recurso. |
| `validateToken(token)` | Bool; verifica o JWT e preenche novamente o contexto da sessão. |
| `refresh(id)` | JWT renovado ou falso/nulo; autoriza primeiro quem pode solicitá-lo. |
| `update(id,mapa)` , `delete(id)` | Bool ou nulo dependendo dos argumentos/falha; modificar usuários. |
| `logout()` | Limpe o contexto e retorne verdadeiro. Isso não implica revogação global de todos os JWT emitidos. |
| `verify(token)` | Verifique o token de confirmação do e-mail, retorne bool. |
| `verificationStatus(email)` | not_found, verificado ou não verificado. |
| `resendVerification(email)` | token de verificação ou falso; a entrega da mensagem fica por conta do aplicativo. |
| `forgotPassword(email)` | Token de recuperação ou falso; Não é confirmação de envio SMTP. |
| `resetPassword(token,nueva)` | texto verdadeiro ou de erro: invalid_token, fraca_password, database_error, used_token, expired_token. Compare com a verdade, não apenas com a veracidade do texto. |
| `verify2FAChallenge(token,codigo)` | JWT final ou falso após verificar o desafio e o código. |
| `complete2FA(id)` | Gera JWT após busca pelo usuário; **não verifica um código TOTP em si**. Não exponha como um endpoint público com um ID fornecido pelo cliente. |

## Resultado suave e retornos de chamada

`AuthLoginResult` preserva sucesso, erro, usuário e resposta. Cada método
retorna a mesma instância, exceto response():

| Método | Quando ligar para retorno de chamada | Parâmetro |
|---|---|---|
| `require2FA()` | Verifique os métodos de MFA ativos e verifique os requisitos. | Sem retorno de chamada. |
| `onSuccess(callback)` | Credenciais corretas e nenhum desafio necessário. | JWT. |
| `onChallenge(callback)` | Credenciais corretas e desafio necessário. | Desafie o JWT temporário. |
| `onFail(callback)` | Credenciais incorretas. | Mensagem de erro. |
| `response()` | Retorna o resultado do retorno de chamada executado ou nulo. | Nenhum. |

Os retornos de chamada de origem devem digitar seus parâmetros, por exemplo
`func(mixed $token) { return Response::json({"token": $token}) }` .
Registra require2FA antes de onSuccess quando o fluxo requer o segundo fator.
Não mostre um token final antes de terminar o desafio.

## MFA e TwoFactor

| Assinatura | Contrato |
|---|---|
| `MFA::generateTOTP()` | Segredo do mapa, qr_uri e qr_url. qr_uri é URI codificado; qr_url aponta para um serviço QR externo e inclui o segredo. Mostrar esse URL enviaria o segredo para esse serviço. |
| `MFA::verifyTOTP(secreto,codigo)` | Bool de acordo com a janela de tempo implementada. |
| `MFA::generateRecoveryCodes()` | Matriz de código. A persistência e sua associação ao usuário exigem o fluxo da aplicação. |
| `MFA::verifyRecoveryCode(id,codigo)` | Consulte os códigos salvos, verifique e consuma aquele que corresponde. |
| `TwoFactor::required(usuario)` | Bool sobre instâncias e registros de MFA. |
| `TwoFactor::verify(id,codigo)` | Bool; Obtém segredo dos métodos MFA do usuário. |

O gerador TOTP nesta implementação usa matemática/rand e constrói uma URL
externa com o segredo. Estas são descobertas que necessitam de revisão de segurança;
não são apresentados como garantias criptográficas do framework.

## Mail

`SmtpClient` é uma classe nativa separada. `auth(usuario,contraseña)` ,
`secure(bool)` e `timeout(segundos)` configuram e retornam sua instância;
`send(destinatario,asunto,cuerpo)` retorna bool e `lastError()` retorna
o último erro textual. Requer servidor SMTP e configuração MAIL_* de acordo com
[smtp_native.go](../../pkg/core/smtp_native.go) . A criação de um token não envia esse email.

Fontes: [Auth](../../pkg/core/auth.go) , [JWT](../../pkg/core/auth_jwt.go) ,
[Fluxo MFA](../../pkg/core/auth_fluent.go) , Tabelas [../pkg/core/auth_tables.go](../../pkg/core/auth_tables.go) .
