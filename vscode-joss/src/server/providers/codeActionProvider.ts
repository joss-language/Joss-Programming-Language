import {
    CodeAction,
    CodeActionKind,
    CodeActionParams,
    Range,
    TextEdit
} from 'vscode-languageserver/node';
import { connection, documents } from '../server';

export function setupCodeActionProvider() {
    connection.onCodeAction((params: CodeActionParams): CodeAction[] => {
        const document = documents.get(params.textDocument.uri);
        if (!document) {
            return [];
        }

        const actions: CodeAction[] = [];
        const lines = document.getText().split('\n');

        for (const diag of params.context.diagnostics) {
            const lineIdx = diag.range.start.line;
            const lineText = lines[lineIdx] ?? '';
            const code = typeof diag.code === 'string' ? diag.code : '';

            // 1. Remove obsolete use / import / @import statement
            if (/^\s*(use|import|@import)\b/.test(lineText)) {
                const removeRange = Range.create(
                    { line: lineIdx, character: 0 },
                    { line: Math.min(lineIdx + 1, lines.length), character: 0 }
                );
                actions.push({
                    title: 'Remove obsolete import/use statement (auto-loaded via joss.yaml)',
                    kind: CodeActionKind.QuickFix,
                    diagnostics: [diag],
                    isPreferred: true,
                    edit: {
                        changes: {
                            [params.textDocument.uri]: [TextEdit.del(removeRange)]
                        }
                    }
                });
            }

            // 2. Migrate deprecated `let` -> `var` (JOSS-DECL-006)
            if (code === 'JOSS-DECL-006' || /\blet\s+\$/.test(lineText)) {
                const letMatch = lineText.match(/^(\s*)let(\s+\$)/);
                if (letMatch) {
                    const startChar = letMatch[1].length;
                    actions.push({
                        title: "Replace deprecated 'let' with 'var'",
                        kind: CodeActionKind.QuickFix,
                        diagnostics: [diag],
                        isPreferred: true,
                        edit: {
                            changes: {
                                [params.textDocument.uri]: [
                                    TextEdit.replace(
                                        Range.create(
                                            { line: lineIdx, character: startChar },
                                            { line: lineIdx, character: startChar + 3 }
                                        ),
                                        'var'
                                    )
                                ]
                            }
                        }
                    });
                }
            }

            // 3. Migrate deprecated `nil` -> `null` (JOSS-DECL-008)
            if (code === 'JOSS-DECL-008' || /\bnil\b/.test(lineText)) {
                const nilIdx = lineText.search(/\bnil\b/);
                if (nilIdx >= 0) {
                    actions.push({
                        title: "Replace deprecated 'nil' with 'null'",
                        kind: CodeActionKind.QuickFix,
                        diagnostics: [diag],
                        isPreferred: true,
                        edit: {
                            changes: {
                                [params.textDocument.uri]: [
                                    TextEdit.replace(
                                        Range.create(
                                            { line: lineIdx, character: nilIdx },
                                            { line: lineIdx, character: nilIdx + 3 }
                                        ),
                                        'null'
                                    )
                                ]
                            }
                        }
                    });
                }
            }

            // 4. Offer `joss fix` for any compiler auto-fixable diagnostic
            const data = diag.data as { autoFixable?: boolean } | undefined;
            if (data?.autoFixable) {
                actions.push({
                    title: `Run 'joss fix' for ${code || 'this issue'}`,
                    kind: CodeActionKind.QuickFix,
                    diagnostics: [diag],
                    command: {
                        title: 'Run joss fix',
                        command: 'joss.fix'
                    }
                });
            }
        }

        // Source action: fix all auto-fixable issues in file
        if (params.context.only?.includes(CodeActionKind.SourceFixAll) || actions.length > 0) {
            actions.push({
                title: 'Joss: Fix all auto-fixable issues',
                kind: CodeActionKind.SourceFixAll,
                command: {
                    title: 'Fix all Joss issues',
                    command: 'joss.fix'
                }
            });
        }

        return actions;
    });
}
