import { Diagnostic, DiagnosticSeverity } from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { spawn } from 'child_process';
import * as path from 'path';
import { URI } from 'vscode-uri';
import { connection, documents, routeParser, indexer, securityAnalyzer, getDocumentSettings } from '../server';

interface CompilerLintIssue {
    rule_id?: string;
    category?: string;
    severity?: string;
    file?: string;
    line?: number;
    column?: number;
    message?: string;
    explanation?: string;
    suggestion?: string;
    auto_fixable?: boolean;
}

const debounceTimers = new Map<string, NodeJS.Timeout>();
const compilerDiagnosticsCache = new Map<string, Diagnostic[]>();

export function setupDiagnostics() {
    documents.onDidOpen(event => {
        void validateTextDocument(event.document, true).catch(error => {
            connection.console.error(`Error validating document ${event.document.uri}: ${error}`);
        });
    });

    documents.onDidSave(event => {
        void validateTextDocument(event.document, true).catch(error => {
            connection.console.error(`Error validating document on save ${event.document.uri}: ${error}`);
        });
    });

    documents.onDidChangeContent(change => {
        const uri = change.document.uri;
        const existing = debounceTimers.get(uri);
        if (existing) clearTimeout(existing);
        debounceTimers.set(uri, setTimeout(() => {
            debounceTimers.delete(uri);
            const doc = documents.get(uri);
            if (doc) {
                void validateTextDocument(doc, false).catch(error => {
                    connection.console.error(`Error validating document ${uri}: ${error}`);
                });
            }
        }, 200));
    });

    documents.onDidClose(event => {
        const existing = debounceTimers.get(event.document.uri);
        if (existing) clearTimeout(existing);
        debounceTimers.delete(event.document.uri);
        compilerDiagnosticsCache.delete(event.document.uri);
        void connection.sendDiagnostics({ uri: event.document.uri, diagnostics: [] });
    });
}

function runCompilerLint(jossBin: string, filePath: string): Promise<CompilerLintIssue[]> {
    return new Promise((resolve) => {
        try {
            const proc = spawn(jossBin, ['lint', '--json', filePath], {
                windowsHide: true
            });
            let stdout = '';
            proc.stdout.on('data', (chunk: Buffer) => {
                stdout += chunk.toString('utf-8');
            });
            proc.on('close', () => {
                const trimmed = stdout.trim();
                if (!trimmed || !trimmed.startsWith('[')) {
                    resolve([]);
                    return;
                }
                try {
                    const parsed = JSON.parse(trimmed) as CompilerLintIssue[];
                    resolve(Array.isArray(parsed) ? parsed : []);
                } catch {
                    resolve([]);
                }
            });
            proc.on('error', () => resolve([]));
        } catch {
            resolve([]);
        }
    });
}

function mapSeverity(sev?: string): DiagnosticSeverity {
    switch ((sev || '').toLowerCase()) {
        case 'error':
            return DiagnosticSeverity.Error;
        case 'warning':
        case 'warn':
            return DiagnosticSeverity.Warning;
        case 'info':
            return DiagnosticSeverity.Information;
        default:
            return DiagnosticSeverity.Hint;
    }
}

async function validateTextDocument(textDocument: TextDocument, runCompiler: boolean): Promise<void> {
    const settings = await getDocumentSettings(textDocument.uri);
    const text = textDocument.getText();
    const diagnostics: Diagnostic[] = [];

    // 1. Validate obsolete syntax (use, import, @import)
    const lines = text.split('\n');
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const match = line.match(/^\s*(use|import|@import)\b/);
        if (match) {
            const keyword = match[1];
            const startCol = line.indexOf(keyword);
            diagnostics.push({
                severity: DiagnosticSeverity.Error,
                code: 'JOSS-SYNTAX-IMPORT',
                range: {
                    start: { line: i, character: startCol },
                    end: { line: i, character: line.length }
                },
                message: `La instrucción '${keyword}' es obsoleta y fue eliminada en Joss. Los paquetes y plugins declarados en joss.yaml se cargan automáticamente.`,
                source: 'joss-compiler'
            });
        }
    }

    // 2. Parse routes and validate controllers/methods
    const routes = routeParser.parseDocument(text);
    for (const route of routes) {
        const controller = await indexer.findController(route.controller);
        if (!controller) {
            diagnostics.push({
                severity: DiagnosticSeverity.Error,
                range: route.controllerRange,
                message: `Controller '${route.controller}' not found`,
                source: 'joss'
            });
        } else {
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

    // 3. JosSecurity static analysis
    if (settings.enableJosSecurity !== false) {
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

    // 4. Real Joss compiler & linter diagnostics (`joss lint --json <file>`)
    if (settings.diagnostics?.compiler?.enable !== false && textDocument.uri.startsWith('file:')) {
        if (runCompiler) {
            const fsPath = URI.parse(textDocument.uri).fsPath;
            const jossBin = settings.executablePath || 'joss';
            const issues = await runCompilerLint(jossBin, fsPath);
            const compilerDiags: Diagnostic[] = [];

            const normalizedTarget = path.resolve(fsPath).toLowerCase();
            for (const issue of issues) {
                if (issue.file) {
                    const normalizedIssueFile = path.resolve(issue.file).toLowerCase();
                    if (normalizedIssueFile !== normalizedTarget) {
                        continue;
                    }
                }
                const lineIdx = Math.max(0, (issue.line ?? 1) - 1);
                const colIdx = Math.max(0, (issue.column ?? 1) - 1);
                const lineLen = (lines[lineIdx] ?? '').length;
                let msg = issue.message || 'Joss diagnostic';
                if (issue.explanation) msg += `\n${issue.explanation}`;
                if (issue.suggestion) msg += `\nSuggestion: ${issue.suggestion}`;

                compilerDiags.push({
                    severity: mapSeverity(issue.severity),
                    code: issue.rule_id || 'JOSS-LINT',
                    range: {
                        start: { line: lineIdx, character: Math.min(colIdx, lineLen) },
                        end: { line: lineIdx, character: lineLen }
                    },
                    message: msg,
                    source: 'joss-compiler',
                    data: { autoFixable: !!issue.auto_fixable }
                });
            }
            compilerDiagnosticsCache.set(textDocument.uri, compilerDiags);
        }

        const cached = compilerDiagnosticsCache.get(textDocument.uri);
        if (cached && cached.length > 0) {
            diagnostics.push(...cached);
        }
    }

    await connection.sendDiagnostics({ uri: textDocument.uri, diagnostics });
}
