import {
    CompletionItem,
    CompletionItemKind,
    InsertTextFormat,
    TextDocumentPositionParams
} from 'vscode-languageserver/node';
import { connection, documents, indexer } from '../server';
import { nativeCallables, nativeClasses, nativeSignature, NativeCallable } from '../nativeCatalog';
import languageCatalog from '../generated/languageCatalog.json';
import { JossSymbol } from '../languageSymbols';
import { inferReceiverClass } from '../utils/callContext';

const keywords = Array.from(new Set([...languageCatalog.keywords, ...languageCatalog.types]));

export function setupCompletionProvider() {
    connection.onCompletion(async (params: TextDocumentPositionParams): Promise<CompletionItem[]> => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return [];
        const text = document.getText();
        const offset = document.offsetAt(params.position);
        const prefix = text.substring(Math.max(0, offset - 300), offset);

        const staticMatch = prefix.match(/([A-Za-z_]\w*)::$/);
        if (staticMatch) {
            return deduplicate([
                ...nativeCallables.filter(item => item.owner === staticMatch[1] && item.static !== false).map(nativeCompletion),
                ...(await indexer.getClassMembers(staticMatch[1])).filter(item => item.kind === 'method').map(symbolCompletion)
            ]);
        }

        const instanceMatch = prefix.match(/(\$[A-Za-z_]\w*|\$this)->$/);
        if (instanceMatch) {
            let className = inferReceiverClass(text, offset, instanceMatch[1]);
            if (!className && instanceMatch[1] === '$this') {
                className = await indexer.getClassAtPosition(params.textDocument.uri, params.position);
            }
            if (className) {
                return deduplicate([
                    ...nativeCallables.filter(item => item.owner === className && item.static === false).map(nativeCompletion),
                    ...(await indexer.getClassMembers(className)).map(symbolCompletion)
                ]);
            }
            return deduplicate([
                ...nativeCallables.filter(item => item.static === false).map(nativeCompletion),
                ...(await indexer.getAllSymbols()).filter(item => item.kind === 'method' || item.kind === 'property').map(symbolCompletion)
            ]);
        }

        const workspaceSymbols = await indexer.getAllSymbols();
        return deduplicate([
            ...nativeClasses.map(name => ({ label: name, kind: CompletionItemKind.Class, detail: 'Clase nativa de Joss' })),
            ...nativeCallables.filter(item => !item.owner).map(nativeCompletion),
            ...workspaceSymbols.filter(item => item.kind === 'class' || item.kind === 'function').map(symbolCompletion),
            ...keywords.map(label => ({ label, kind: CompletionItemKind.Keyword, detail: 'Palabra reservada de Joss' })),
            {
                label: 'cin',
                kind: CompletionItemKind.Keyword,
                detail: 'cin >> $variable',
                documentation: 'Flujo de entrada estándar interactivo. Captura datos desde la terminal y los asigna automáticamente a una o más variables.',
                insertTextFormat: InsertTextFormat.Snippet,
                insertText: 'cin >> $${1:variable}'
            },
            {
                label: 'cout',
                kind: CompletionItemKind.Keyword,
                detail: 'cout << $expr << endl;',
                documentation: 'Flujo de salida estándar. Imprime datos en la consola mediante encadenamiento << sin salto de línea implícito.',
                insertTextFormat: InsertTextFormat.Snippet,
                insertText: 'cout << ${1:expr} << endl;'
            },
            {
                label: 'cerr',
                kind: CompletionItemKind.Keyword,
                detail: 'cerr << $expr << endl;',
                documentation: 'Flujo de error estándar (stderr). Imprime mensajes directamente en el canal de errores.',
                insertTextFormat: InsertTextFormat.Snippet,
                insertText: 'cerr << ${1:error} << endl;'
            },
            {
                label: 'endl',
                kind: CompletionItemKind.Constant,
                detail: 'const endl = "\\n"',
                documentation: 'Salto de línea estándar para usar con cout y cerr.',
                insertTextFormat: InsertTextFormat.PlainText,
                insertText: 'endl'
            }
        ]);
    });

    connection.onCompletionResolve((item: CompletionItem): CompletionItem => item);
}

function nativeCompletion(item: NativeCallable): CompletionItem {
    return {
        label: item.name,
        kind: item.owner ? CompletionItemKind.Method : CompletionItemKind.Function,
        detail: nativeSignature(item),
        documentation: item.documentation,
        insertTextFormat: InsertTextFormat.Snippet,
        insertText: snippet(item.name, item.parameters.map(parameter => parameter.name))
    };
}

function symbolCompletion(symbol: JossSymbol): CompletionItem {
    const kind = symbol.kind === 'class' ? CompletionItemKind.Class
        : symbol.kind === 'property' ? CompletionItemKind.Property
            : symbol.kind === 'function' ? CompletionItemKind.Function : CompletionItemKind.Method;
    const detailPrefix = symbol.origin === 'plugin' && symbol.packageName ? `[Plugin: ${symbol.packageName}] ` : '';
    return {
        label: symbol.name,
        kind,
        detail: `${detailPrefix}${symbol.signature || symbol.name}`,
        documentation: symbol.origin === 'plugin' && symbol.packageName
            ? `📦 Exportado por el plugin ${symbol.packageName} (v${symbol.packageVersion || '1.0.0'}).\n${symbol.docstring || ''}`
            : (symbol.docstring || `${symbol.kind} de ${symbol.containerName || 'este proyecto'}`),
        insertTextFormat: symbol.kind === 'method' || symbol.kind === 'function' ? InsertTextFormat.Snippet : InsertTextFormat.PlainText,
        insertText: symbol.kind === 'method' || symbol.kind === 'function'
            ? snippet(symbol.name, symbol.parameters.map(parameter => parameter.name))
            : symbol.name
    };
}

function snippet(name: string, parameters: string[]): string {
    if (!parameters.length) return `${name}()`;
    return `${name}(${parameters.map((parameter, index) => `\${${index + 1}:\$${parameter}}`).join(', ')})`;
}

function deduplicate(items: CompletionItem[]): CompletionItem[] {
    const seen = new Set<string>();
    return items.filter(item => {
        const key = `${item.kind}:${item.label}`;
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
    });
}
