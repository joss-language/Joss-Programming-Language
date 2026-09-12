# Extensão Joss para VS Code

[Índice](README.md)

A distribuição Joss inclui o VSIX oficial no artefato `jossecurity-vscode.zip`. Instale-o do VS Code com **Extensões: Instale do VSIX...**.

A extensão reconhece `.joss` e `.joss.html`. Indexa classes, propriedades, funções e métodos; oferece preenchimento automático, ajuda com assinatura, foco, símbolos, definição e referências. Valida rotas `Controller@method` e aplica três heurísticas de segurança baseadas em texto: uso de `eval`, SQL interpolado com `DB::query` e baixo custo de bcrypt. Não substitui o analisador ou uma auditoria de segurança.

Para um plugin JP v2, ele lê `META-INF/joss-symbols.json` do pacote e adiciona classes, métodos, parâmetros e retorna ao IntelliSense e ao analisador. JPs mais antigos sem esse arquivo podem ser executados, mas não oferecem sua API ao editor.

Depois de instalar um plugin com `joss pub install`, recarregue a janela do VS Code se seus símbolos não aparecerem imediatamente. Para criar um plugin que ofereça uma boa ajuda de assinatura, declare parâmetros e tipos na API pública antes de executar `joss build package .`.