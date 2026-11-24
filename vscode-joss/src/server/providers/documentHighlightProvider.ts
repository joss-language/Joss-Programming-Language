import {
    DocumentHighlight,
    DocumentHighlightKind,
    DocumentHighlightParams,
    Range
} from 'vscode-languageserver/node';
import { connection, documents } from '../server';

function extractSymbolAt(line: string, character: number): string | null {
    const isIdent = (ch: string | undefined) => !!ch && /[\w$]/.test(ch);
    let start = character;
    if (!isIdent(line[start]) && start > 0 && isIdent(line[start - 1])) {
        start--;
    }
    if (!isIdent(line[start])) return null;
    while (start > 0 && isIdent(line[start - 1])) start--;
    let end = start;
    while (end < line.length && isIdent(line[end])) end++;
    const word = line.slice(start, end);
    if (!word || /^\d+$/.test(word)) return null;
    return word;
}

export function setupDocumentHighlightProvider() {
    connection.onDocumentHighlight((params: DocumentHighlightParams): DocumentHighlight[] => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return [];

        const lines = document.getText().split('\n');
        const currentLine = lines[params.position.line];
        if (!currentLine) return [];

        const target = extractSymbolAt(currentLine, params.position.character);
        if (!target) return [];

        const escaped = target.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const pattern = target.startsWith('$')
            ? new RegExp(`${escaped}(?![A-Za-z0-9_])`, 'g')
            : new RegExp(`(?<![A-Za-z0-9_$])${escaped}(?![A-Za-z0-9_])`, 'g');

        const highlights: DocumentHighlight[] = [];
        for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            const commentIdx = line.indexOf('//');
            const searchable = commentIdx >= 0 ? line.slice(0, commentIdx) : line;

            let m: RegExpExecArray | null;
            pattern.lastIndex = 0;
            while ((m = pattern.exec(searchable)) !== null) {
                const startChar = m.index;
                const endChar = startChar + target.length;
                const after = searchable.slice(endChar);
                const isWrite = /^\s*(?:=(?!=)|\+=|-=|\*=|\/=|\?\?=|\+\+|--)/.test(after);

                highlights.push({
                    range: Range.create({ line: i, character: startChar }, { line: i, character: endChar }),
                    kind: isWrite ? DocumentHighlightKind.Write : DocumentHighlightKind.Read
                });
            }
        }

        return highlights;
    });
}
