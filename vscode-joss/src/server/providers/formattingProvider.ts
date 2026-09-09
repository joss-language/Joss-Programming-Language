import { DocumentFormattingParams, TextEdit, Range } from 'vscode-languageserver/node';
import { connection, documents } from '../server';
import { spawnSync } from 'child_process';

export function setupFormattingProvider() {
    connection.onDocumentFormatting((params: DocumentFormattingParams): TextEdit[] => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return [];

        const text = document.getText();
        try {
            const proc = spawnSync('joss', ['format', '-'], {
                input: text,
                encoding: 'utf-8',
                windowsHide: true
            });

            if (proc.status === 0 && proc.stdout && proc.stdout.length > 0) {
                const formatted = proc.stdout;
                if (formatted !== text) {
                    const fullRange = Range.create(
                        document.positionAt(0),
                        document.positionAt(text.length)
                    );
                    return [TextEdit.replace(fullRange, formatted)];
                }
            }
        } catch {
            // Fallback gracefully
        }

        return [];
    });
}
