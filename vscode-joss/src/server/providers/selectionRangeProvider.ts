import { SelectionRange, SelectionRangeParams, Range } from 'vscode-languageserver/node';
import { connection, documents } from '../server';

export function setupSelectionRangeProvider() {
    connection.onSelectionRanges((params: SelectionRangeParams): SelectionRange[] => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return [];

        const text = document.getText();
        const lines = text.split('\n');
        const fullRange = Range.create(
            { line: 0, character: 0 },
            { line: Math.max(0, lines.length - 1), character: lines[lines.length - 1]?.length ?? 0 }
        );

        return params.positions.map(pos => {
            const line = lines[pos.line] ?? '';
            let start = pos.character;
            let end = pos.character;

            while (start > 0 && /[\w$]/.test(line[start - 1])) start--;
            while (end < line.length && /[\w$]/.test(line[end])) end++;

            const wordRange = Range.create({ line: pos.line, character: start }, { line: pos.line, character: end });
            const trimmedStart = line.length - line.trimStart().length;
            const lineRange = Range.create({ line: pos.line, character: trimmedStart }, { line: pos.line, character: line.length });

            // Find enclosing { ... } block
            let blockRange: Range | null = null;
            let depth = 0;
            let openLine = -1;
            let openChar = 0;
            for (let l = pos.line; l >= 0; l--) {
                const ln = lines[l];
                const cEnd = l === pos.line ? pos.character : ln.length - 1;
                for (let c = cEnd; c >= 0; c--) {
                    if (ln[c] === '}') depth++;
                    else if (ln[c] === '{') {
                        if (depth === 0) {
                            openLine = l;
                            openChar = c;
                            break;
                        }
                        depth--;
                    }
                }
                if (openLine !== -1) break;
            }
            if (openLine !== -1) {
                let closeDepth = 0;
                for (let l = openLine; l < lines.length; l++) {
                    const ln = lines[l];
                    const cStart = l === openLine ? openChar : 0;
                    for (let c = cStart; c < ln.length; c++) {
                        if (ln[c] === '{') closeDepth++;
                        else if (ln[c] === '}') {
                            closeDepth--;
                            if (closeDepth === 0) {
                                blockRange = Range.create({ line: openLine, character: openChar }, { line: l, character: c + 1 });
                                break;
                            }
                        }
                    }
                    if (blockRange) break;
                }
            }

            let root: SelectionRange = { range: fullRange };
            if (blockRange) {
                root = { range: blockRange, parent: root };
            }
            root = { range: lineRange, parent: root };
            if (start < end) {
                root = { range: wordRange, parent: root };
            }
            return root;
        });
    });
}
