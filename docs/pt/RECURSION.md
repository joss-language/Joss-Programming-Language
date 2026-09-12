# Funções recursivas

[Índice](README.md)

Joss suporta recursão direta e mútua. Para uma verificação estática útil, declare o tipo de retorno:```joss
public func factorial(int $n): int {
    ($n <= 1) ? {
        return 1
    } : {
        return $n * factorial($n - 1)
    }
}
```
As declarações de funções e suas assinaturas são registradas antes da análise dos órgãos. É por isso que uma função pode referir-se a si mesma ou a outra função declarada posteriormente no projeto.

Cada chamada cria um quadro separado para parâmetros, localidades, tipos inferidos e constantes. Uma função nomeada não pode ler acidentalmente os locais de seu chamador ou de variáveis ​​de origem de nível superior; os dados devem viajar por parâmetros. As ligações nativas e de plug-in são visíveis. Instâncias, mapas e arrays passados ​​como valores mantêm a semântica de referência atual; Isolar a ligação local não transforma esses objetos em cópias profundas. Os fechamentos preservam um ambiente capturado separado.

O tempo de execução usa `Runtime.MaxCallDepth` e aplica 1.024 quadros por padrão. Excedê-lo produz `RecursionLimit` em vez de deixar a pilha Go crescer incontrolavelmente. Este limite protege a execução, mas não substitui um caso base correto.

O analisador valida os tipos de argumentos, cada `return` explícito, e que todo caminho provável de uma função anotada termina em `return` ou `throw`. Reconhece blocos, ambos os braços de ternários, `match` com `default` e ambos os braços de `try/catch`; não assume que um loop arbitrário termine.