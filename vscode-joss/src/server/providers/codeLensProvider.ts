import { CodeLens, CodeLensParams, Range } from 'vscode-languageserver/node';
import { connection, documents, getDocumentSettings } from '../server';

export function setupCodeLensProvider() {
    connection.onCodeLens(async (params: CodeLensParams): Promise<CodeLens[]> => {
        const settings = await getDocumentSettings(params.textDocument.uri);
        if (settings.codeLens?.enable === false) {
            return [];
        }

        const document = documents.get(params.textDocument.uri);
        if (!document) {
            return [];
        }

        const lenses: CodeLens[] = [];
        const lines = document.getText().split('\n');
        const isTestFile = /_test\.joss$/i.test(params.textDocument.uri);
        let hasMainOrWindow = false;

        for (let i = 0; i < lines.length; i++) {
            const line = lines[i];

            // Main entry point lens
            if (/^\s*(public\s+)?func\s+main\s*\(/.test(line)) {
                hasMainOrWindow = true;
                const range = Range.create({ line: i, character: 0 }, { line: i, character: line.length });
                lenses.push(
                    {
                        range,
                        command: { title: '$(play) Run', command: 'joss.runFile' }
                    },
                    {
                        range,
                        command: { title: '$(zap) Run Native', command: 'joss.runFileNative' }
                    },
                    {
                        range,
                        command: { title: '$(package) Build Native', command: 'joss.buildNative' }
                    }
                );
            }

            // Native GUI Window lens
            if (/\bWindow::open\s*\(/.test(line)) {
                hasMainOrWindow = true;
                const range = Range.create({ line: i, character: 0 }, { line: i, character: line.length });
                lenses.push(
                    {
                        range,
                        command: { title: '$(window) Run Native GUI', command: 'joss.programStart' }
                    },
                    {
                        range,
                        command: { title: '$(package) Build GUI App', command: 'joss.buildGUI' }
                    }
                );
            }

            // Test function lens in *_test.joss files
            if (isTestFile && /^\s*(public\s+)?func\s+test\w*\s*\(/i.test(line)) {
                const range = Range.create({ line: i, character: 0 }, { line: i, character: line.length });
                lenses.push({
                    range,
                    command: { title: '$(beaker) Run Tests', command: 'joss.test' }
                });
            }
        }

        // Top-level script file without explicit main()
        if (!hasMainOrWindow && !isTestFile && /main\.joss$/i.test(params.textDocument.uri) && lines.length > 0) {
            const range = Range.create({ line: 0, character: 0 }, { line: 0, character: 0 });
            lenses.push(
                {
                    range,
                    command: { title: '$(play) Run', command: 'joss.runFile' }
                },
                {
                    range,
                    command: { title: '$(zap) Run Native', command: 'joss.runFileNative' }
                }
            );
        }

        return lenses;
    });
}
