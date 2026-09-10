import { Hover, HoverParams, MarkupKind } from 'vscode-languageserver/node';
import { connection, documents, indexer } from '../server';
import { findNativeCallable, nativeClasses, nativeSignature } from '../nativeCatalog';
import { referenceAtPosition } from '../utils/callContext';

export function setupHoverProvider() {
    connection.onHover(async (params: HoverParams): Promise<Hover | null> => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return null;

        const pipeHover = findPipelineAtPosition(document, params.position);
        if (pipeHover) return pipeHover;

        const reference = referenceAtPosition(document, params.position);

        if (['use', 'import', '@import'].includes(reference.toLowerCase())) {
            return markdown(`⚠️ **Sintaxis Obsoleta en Joss**\n\nLas instrucciones \`use\` e \`import\` fueron eliminadas del lenguaje. Los plugins y paquetes declarados en \`joss.yaml\` se cargan e indexan automáticamente en el espacio de nombres global.`);
        }

        if (reference.toLowerCase() === 'cin') {
            return markdown(`\`\`\`joss\ncin >> $variable\n\`\`\`\n\n📥 **Flujo de Entrada Estándar (Console Input)**\n\nLee datos introducidos por el usuario desde la terminal:\n- **Conversión automática**: Si la variable destino es numérica (\`int\` o \`float\`), convierte el texto automáticamente al número correspondiente.\n- **Lectura completa**: Si es \`string\`, captura la línea completa de texto con espacios.\n- **Encadenamiento múltiple**: Permite leer varias variables consecutivas: \`cin >> $a >> $b\`.`);
        }

        if (reference.toLowerCase() === 'cout') {
            return markdown(`\`\`\`joss\ncout << $expr << endl;\ncout("texto", ...);\n\`\`\`\n\n📤 **Flujo de Salida Estándar (Console Output)**\n\nImprime datos en la terminal sin salto de línea automático (a diferencia de \`print\`).\n- **Operador de flujo**: Permite encadenamiento fluido con \`<<\`, números, cadenas y variables.\n- **Salto de línea**: Combina con \`endl\` para emitir un salto de línea.\n- **Invocación directa**: También puede ser llamada como función: \`cout("hola ", $nombre)\`.`);
        }

        if (reference.toLowerCase() === 'cerr') {
            return markdown(`\`\`\`joss\ncerr << $error << endl;\ncerr("mensaje de error");\n\`\`\`\n\n⚠️ **Flujo de Error Estándar (Standard Error Output)**\n\nImprime mensajes directamente en el canal de errores estándar (\`stderr\`).`);
        }

        if (reference.toLowerCase() === 'endl') {
            return markdown(`\`\`\`joss\nconst endl = "\\n"\n\`\`\`\n\n↵ **Manipulador de Salto de Línea (End Line)**\n\nRepresenta un salto de línea estándar (\`"\\n"\`) para flujos \`cout\` y \`cerr\`.`);
        }

        if (reference.toLowerCase() === 'defer') {
            return markdown(`\`\`\`joss\ndefer { ... }\n\`\`\`\n\n🛡️ **Limpieza Garantizada de Recursos (defer)**\n\nPospone la ejecución del bloque o sentencia hasta que la función, método o archivo actual termine, incluso ante retornos tempranos. Múltiples sentencias \`defer\` se ejecutan en orden **LIFO** (Last In, First Out).`);
        }

        if (reference.toLowerCase() === 'interface') {
            return markdown(`\`\`\`joss\npublic interface NombreInterfaz [extends OtraInterfaz] {\n    public func metodo(Tipo $arg): TipoRetorno;\n}\n\`\`\`\n\n📐 **Definición de Interfaz (Contrato)**\n\nDefine un contrato estricto de métodos que una o más clases deben implementar. Las interfaces solo declaran firmas de métodos sin cuerpo.`);
        }

        if (reference.toLowerCase() === 'implements') {
            return markdown(`\`\`\`joss\npublic class MiClase [extends ClaseBase] implements Interfaz1, Interfaz2 {\n    ...\n}\n\`\`\`\n\n🧩 **Implementación de Interfaz**\n\nDeclara que la clase cumple con los contratos establecidos por una o varias interfaces.`);
        }

        const native = findNativeCallable(reference);
        if (native) {
            return markdown(`\`\`\`joss\n${nativeSignature(native)}\n\`\`\`\n\n${native.documentation}${parameterDocs(native.parameters)}`);
        }
        if (nativeClasses.includes(reference)) {
            return markdown(`**${reference}**\n\nClase nativa de Joss. Escribe \`${reference}::\` para ver sus métodos.`);
        }

        let symbol = await indexer.findSymbol(reference);
        if (!symbol) {
            symbol = (await indexer.findSymbolsBySimpleName(reference.replace(/^\$/, '')))[0] || null;
        }
        if (!symbol) return null;
        const location = symbol.location.uri.replace('file:///', '');
        const signature = symbol.signature || symbol.qualifiedName;

        let originBadge = '';
        if (symbol.origin === 'plugin' || symbol.packageName) {
            const pkgName = symbol.packageName || 'plugin';
            const verStr = symbol.packageVersion ? ` (v${symbol.packageVersion})` : '';
            originBadge = `\n\n📦 **Plugin Originario**: \`${pkgName}\`${verStr}\n*Origen: Paquete autocontenido \`${pkgName}.jp\`*`;
        }

        return markdown(`\`\`\`joss\n${signature}\n\`\`\`${originBadge}\n\n${symbol.docstring || `Símbolo ${symbol.kind} del proyecto.`}${parameterDocs(symbol.parameters)}\n\n_${location}:${symbol.location.range.start.line + 1}_`);
    });
}

function parameterDocs(parameters: Array<{ label: string; documentation?: string }>): string {
    if (!parameters.length) return '';
    return `\n\n**Parámetros**\n${parameters.map(parameter => `- \`${parameter.label}\`${parameter.documentation ? ` — ${parameter.documentation}` : ''}`).join('\n')}`;
}

function markdown(value: string): Hover {
    return { contents: { kind: MarkupKind.Markdown, value } };
}

function findPipelineAtPosition(document: { getText(range?: any): string }, position: { line: number; character: number }): Hover | null {
    const lineText = document.getText({
        start: { line: position.line, character: 0 },
        end: { line: position.line, character: 1000 }
    });
    const col = position.character;

    if (!lineText.includes('|>')) return null;

    const parts = lineText.split('|>');
    let currentOffset = 0;
    for (let i = 0; i < parts.length - 1; i++) {
        const pipePos = lineText.indexOf('|>', currentOffset);
        if (pipePos === -1) break;
        currentOffset = pipePos + 2;

        const leftSegment = parts.slice(0, i + 1).map(p => p.trim()).join(' |> ');
        const simpleLeft = parts[i].trim().replace(/^[;{}()\s]+/, '');
        const rightPart = parts[i + 1].trim();

        const callMatch = rightPart.match(/^([A-Za-z_$][\w$]*(?:::|->)?[\w$]*)(?:\((.*?)\))?/);
        if (!callMatch) continue;

        const fnName = callMatch[1];
        const argsStr = callMatch[2];

        let desugared = '';
        if (argsStr !== undefined) {
            const cleanArgs = argsStr.trim();
            if (cleanArgs.length > 0) {
                desugared = `${fnName}(${simpleLeft}, ${cleanArgs})`;
            } else {
                desugared = `${fnName}(${simpleLeft})`;
            }
        } else {
            desugared = `${fnName}(${simpleLeft})`;
        }

        const rightStart = lineText.indexOf(fnName, pipePos);
        const rightEnd = rightStart + (callMatch[0]?.length || fnName.length);

        const onPipe = col >= pipePos && col <= pipePos + 2;
        const onRight = col >= rightStart && col <= rightEnd;

        if (onPipe || onRight) {
            return markdown(`\`\`\`joss\n${desugared}\n\`\`\`\n\n🚀 **Operador Pipeline (\`|>\`)**\n\n**Ejecución desazucarada:**\nEn tiempo de ejecución, Joss inyecta automáticamente el valor de la izquierda (\`${simpleLeft}\`) como primer argumento de la llamada:\n\`\`\`joss\n${desugared}\n\`\`\``);
        }
    }
    return null;
}
