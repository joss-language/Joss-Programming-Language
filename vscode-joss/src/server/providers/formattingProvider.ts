import { DocumentFormattingParams, TextEdit, Range } from 'vscode-languageserver/node';
import { connection, documents, getDocumentSettings } from '../server';
import { spawn } from 'child_process';

export function setupFormattingProvider() {
    connection.onDocumentFormatting(async (params: DocumentFormattingParams): Promise<TextEdit[]> => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return [];

        const settings = await getDocumentSettings(params.textDocument.uri);
        if (settings.format?.enable === false) {
            return [];
        }

        const jossBin = settings.executablePath || 'joss';
        const text = document.getText();

        return new Promise((resolve) => {
            try {
                const proc = spawn(jossBin, ['format', '-'], {
                    windowsHide: true
                });

                let stdout = '';
                proc.stdout.on('data', (chunk: Buffer) => {
                    stdout += chunk.toString('utf-8');
                });

                proc.on('close', (code) => {
                    if (code === 0 && stdout.length > 0 && stdout !== text) {
                        const fullRange = Range.create(
                            document.positionAt(0),
                            document.positionAt(text.length)
                        );
                        resolve([TextEdit.replace(fullRange, stdout)]);
                    } else {
                        resolve([]);
                    }
                });

                proc.on('error', () => resolve([]));
                proc.stdin.write(text);
                proc.stdin.end();
            } catch {
                resolve([]);
            }
        });
    });
}
