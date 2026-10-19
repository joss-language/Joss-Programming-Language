import { Diagnostic, DiagnosticSeverity } from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { connection, documents, routeParser, indexer, securityAnalyzer } from '../server';
import { getDocumentSettings } from '../server';

export function setupDiagnostics() {
    documents.onDidChangeContent(change => {
        void validateTextDocument(change.document).catch(error => {
            connection.console.error(`Error validating document ${change.document.uri}: ${error}`);
        });
    });
}

async function validateTextDocument(textDocument: TextDocument): Promise<void> {
    const settings = await getDocumentSettings(textDocument.uri);
    const text = textDocument.getText();
    const diagnostics: Diagnostic[] = [];

    // Validar sintaxis obsoleta (use, import, @import)
    const lines = text.split('\n');
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const match = line.match(/^\s*(use|import|@import)\b/);
        if (match) {
            const keyword = match[1];
            const startCol = line.indexOf(keyword);
            diagnostics.push({
                severity: DiagnosticSeverity.Error,
                range: {
                    start: { line: i, character: startCol },
                    end: { line: i, character: line.length }
                },
                message: `La instrucción '${keyword}' es obsoleta y fue eliminada en Joss. Los paquetes y plugins declarados en joss.yaml se cargan automáticamente.`,
                source: 'joss-compiler'
            });
        }
    }

    // Parse routes and validate
    const routes = routeParser.parseDocument(text);

    for (const route of routes) {
        // Validate controller exists
        const controller = await indexer.findController(route.controller);
        if (!controller) {
            diagnostics.push({
                severity: DiagnosticSeverity.Error,
                range: route.controllerRange,
                message: `Controller '${route.controller}' not found`,
                source: 'joss'
            });
        } else {
            // Validate methods exist
            for (const method of route.methods) {
                const methodSymbol = await indexer.findMethod(route.controller, method.name);
                if (!methodSymbol) {
                    const suggestions = await indexer.fuzzyFindMethod(route.controller, method.name);
                    const suggestionText = suggestions.length > 0
                        ? `\nDid you mean '${suggestions[0]}'?`
                        : '';

                    diagnostics.push({
                        severity: DiagnosticSeverity.Error,
                        range: method.range,
                        message: `Method '${method.name}' not found in ${route.controller}${suggestionText}`,
                        source: 'joss'
                    });
                }
            }
        }
    }

    // Security analysis
    if (settings.enableJosSecurity) {
        const securityIssues = await securityAnalyzer.analyze(text);
        for (const issue of securityIssues) {
            diagnostics.push({
                severity: issue.severity === 'error'
                    ? DiagnosticSeverity.Error
                    : DiagnosticSeverity.Warning,
                range: issue.range,
                message: issue.message,
                source: 'joss-security'
            });
        }
    }

    await connection.sendDiagnostics({ uri: textDocument.uri, diagnostics });
}
