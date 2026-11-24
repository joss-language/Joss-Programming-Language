import { FoldingRange, FoldingRangeKind, FoldingRangeParams } from 'vscode-languageserver/node';
import { connection, documents } from '../server';

export function setupFoldingRangeProvider() {
    connection.onFoldingRanges((params: FoldingRangeParams): FoldingRange[] => {
        const document = documents.get(params.textDocument.uri);
        if (!document) {
            return [];
        }

        const lines = document.getText().split('\n');
        const ranges: FoldingRange[] = [];
        const braceStack: number[] = [];
        const regionStack: number[] = [];
        let blockCommentStart = -1;

        for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            const trimmed = line.trim();

            // Multi-line comment folding
            if (blockCommentStart === -1 && trimmed.startsWith('/*') && !trimmed.includes('*/')) {
                blockCommentStart = i;
                continue;
            }
            if (blockCommentStart !== -1) {
                if (trimmed.includes('*/')) {
                    if (i > blockCommentStart) {
                        ranges.push({
                            startLine: blockCommentStart,
                            endLine: i,
                            kind: FoldingRangeKind.Comment
                        });
                    }
                    blockCommentStart = -1;
                }
                continue;
            }

            // #region / // region markers
            if (/^\/\/\s*#?region\b/i.test(trimmed)) {
                regionStack.push(i);
                continue;
            }
            if (/^\/\/\s*#?endregion\b/i.test(trimmed)) {
                const start = regionStack.pop();
                if (start !== undefined && i > start) {
                    ranges.push({
                        startLine: start,
                        endLine: i,
                        kind: FoldingRangeKind.Region
                    });
                }
                continue;
            }

            // Brace-delimited blocks (classes, functions, control structures, maps)
            let inStr: string | null = null;
            for (let c = 0; c < line.length; c++) {
                const ch = line[c];
                if (inStr) {
                    if (ch === '\\') { c++; continue; }
                    if (ch === inStr) inStr = null;
                    continue;
                }
                if (ch === '/' && line[c + 1] === '/') break;
                if (ch === '"' || ch === "'") {
                    inStr = ch;
                } else if (ch === '{') {
                    braceStack.push(i);
                } else if (ch === '}') {
                    const start = braceStack.pop();
                    if (start !== undefined && i > start) {
                        ranges.push({ startLine: start, endLine: i });
                    }
                }
            }
        }

        return ranges;
    });
}
