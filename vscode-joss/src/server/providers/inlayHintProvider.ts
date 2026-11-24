import { InlayHint, InlayHintKind, InlayHintParams } from 'vscode-languageserver/node';
import { connection, documents, getDocumentSettings } from '../server';
import { findNativeCallable } from '../nativeCatalog';

function inferSimpleExprType(rhs: string): string | null {
    const trimmed = rhs.trim().replace(/;$/, '').trim();
    if (!trimmed) return null;
    if (/^"[^"]*"$/.test(trimmed) || /^'[^']*'$/.test(trimmed)) return 'string';
    if (/^(true|false)$/.test(trimmed)) return 'bool';
    if (/^-?\d+$/.test(trimmed)) return 'int';
    if (/^-?\d+\.\d+$/.test(trimmed)) return 'float';
    if (/^\[.*\]$/.test(trimmed)) return 'array';
    if (/^\{.*\}$/.test(trimmed)) return 'map';
    const newMatch = trimmed.match(/^new\s+([A-Z]\w*)\b/);
    if (newMatch) return newMatch[1];
    const staticMatch = trimmed.match(/^([A-Z]\w*)::(\w+)\s*\(/);
    if (staticMatch) {
        const found = findNativeCallable(`${staticMatch[1]}::${staticMatch[2]}`);
        if (found?.returnType && found.returnType !== 'void' && found.returnType !== 'mixed') {
            return found.returnType;
        }
    }
    return null;
}

function splitTopLevelArgs(argsText: string): Array<{ text: string; offset: number }> {
    const result: Array<{ text: string; offset: number }> = [];
    let depth = 0;
    let inStr: string | null = null;
    let start = 0;

    for (let i = 0; i < argsText.length; i++) {
        const ch = argsText[i];
        if (inStr) {
            if (ch === '\\') {
                i++;
                continue;
            }
            if (ch === inStr) inStr = null;
            continue;
        }
        if (ch === '"' || ch === "'") {
            inStr = ch;
        } else if (ch === '(' || ch === '[' || ch === '{') {
            depth++;
        } else if (ch === ')' || ch === ']' || ch === '}') {
            depth = Math.max(0, depth - 1);
        } else if (ch === ',' && depth === 0) {
            const slice = argsText.slice(start, i);
            const leadingSpaces = slice.length - slice.trimStart().length;
            result.push({ text: slice.trim(), offset: start + leadingSpaces });
            start = i + 1;
        }
    }
    const tail = argsText.slice(start);
    if (tail.trim().length > 0) {
        const leadingSpaces = tail.length - tail.trimStart().length;
        result.push({ text: tail.trim(), offset: start + leadingSpaces });
    }
    return result;
}

export function setupInlayHintProvider() {
    connection.languages.inlayHint.on(async (params: InlayHintParams): Promise<InlayHint[]> => {
        const settings = await getDocumentSettings(params.textDocument.uri);
        if (settings.inlayHints?.enable === false) {
            return [];
        }

        const document = documents.get(params.textDocument.uri);
        if (!document) {
            return [];
        }

        const hints: InlayHint[] = [];
        const lines = document.getText().split('\n');
        const startLine = Math.max(0, params.range.start.line);
        const endLine = Math.min(lines.length - 1, params.range.end.line);

        for (let lineIdx = startLine; lineIdx <= endLine; lineIdx++) {
            const line = lines[lineIdx];

            // 1. Inferred type hints for `var $x = <expr>`
            const varMatch = line.match(/^(\s*var\s+\$[A-Za-z_]\w*)\s*=\s*(.+)$/);
            if (varMatch) {
                const inferred = inferSimpleExprType(varMatch[2]);
                if (inferred) {
                    hints.push({
                        position: { line: lineIdx, character: varMatch[1].length },
                        label: `: ${inferred}`,
                        kind: InlayHintKind.Type,
                        paddingLeft: false,
                        paddingRight: true
                    });
                }
            }

            // 2. Parameter name hints for `Class::method(...)` and global function calls
            const callRegex = /\b(?:([A-Z]\w*)::)?([a-zA-Z_]\w*)\s*\(([^()]*)\)/g;
            let match: RegExpExecArray | null;
            while ((match = callRegex.exec(line)) !== null) {
                const className = match[1];
                const fnName = match[2];
                const argsStr = match[3];
                if (!argsStr.trim()) continue;

                const def = findNativeCallable(className ? `${className}::${fnName}` : fnName);
                if (!def || !def.parameters || def.parameters.length === 0) continue;

                const openParenOffset = match.index + match[0].indexOf('(') + 1;
                const args = splitTopLevelArgs(argsStr);

                for (let idx = 0; idx < args.length && idx < def.parameters.length; idx++) {
                    const paramName = def.parameters[idx].name.replace(/^\$/, '');
                    if (!paramName || args[idx].text === `$${paramName}`) continue;
                    hints.push({
                        position: { line: lineIdx, character: openParenOffset + args[idx].offset },
                        label: `${paramName}:`,
                        kind: InlayHintKind.Parameter,
                        paddingRight: true
                    });
                }
            }
        }

        return hints;
    });
}
