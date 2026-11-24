import * as path from 'path';
import * as vscode from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';

import { registerCommands } from './commands';
import { getCompletionItemProvider } from './completion';
import { JossTaskProvider } from './tasks';
import { JossExplorerProvider } from './views/jossExplorer';

let client: LanguageClient;

export function activate(context: vscode.ExtensionContext) {
    console.log('Joss Language Extension v3.8 is now active!');

    try {
        // Start Language Server
        client = startLanguageServer(context);

        // Register Commands
        registerCommands(context, client);

        // View templates (.joss.html) keep their controller-variable suggestions.
        context.subscriptions.push(getCompletionItemProvider());

        // Built-in VS Code Task Provider (Ctrl+Shift+B Build Native / GUI / Test / Lint)
        context.subscriptions.push(
            vscode.tasks.registerTaskProvider(JossTaskProvider.JossType, new JossTaskProvider())
        );

        // Joss Explorer Sidebar TreeView (Quick Actions, Routes, Models, Controllers)
        const explorerProvider = new JossExplorerProvider(client);
        context.subscriptions.push(
            vscode.window.registerTreeDataProvider('jossExplorer', explorerProvider),
            vscode.commands.registerCommand('joss.refreshExplorer', () => explorerProvider.refresh()),
            vscode.commands.registerCommand('joss.openRouteTarget', async (route: unknown) => {
                const location = await client.sendRequest<{ uri: string; range: { start: { line: number; character: number } } }>('joss/resolveRoute', route);
                if (location) {
                    const uri = vscode.Uri.parse(location.uri);
                    const pos = new vscode.Position(location.range.start.line, location.range.start.character);
                    await vscode.window.showTextDocument(uri, { selection: new vscode.Range(pos, pos) });
                }
            })
        );

        // Auto-refresh explorer when .joss files are created/deleted/saved
        context.subscriptions.push(
            vscode.workspace.onDidSaveTextDocument(doc => {
                if (doc.languageId === 'joss') explorerProvider.refresh();
            })
        );

        // Interactive Status Bar Item
        const statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
        statusBarItem.text = '$(zap) Joss';
        statusBarItem.tooltip = 'Joss Language Server — Click for Quick Actions (Run, Build Native, Build GUI, Test, Lint)';
        statusBarItem.command = 'joss.quickActions';
        statusBarItem.show();
        context.subscriptions.push(statusBarItem);
    } catch (e) {
        vscode.window.showErrorMessage(`Joss Language Extension failed to activate: ${e}`);
        console.error('Activation failed:', e);
    }
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    return client.stop();
}

function startLanguageServer(context: vscode.ExtensionContext): LanguageClient {
    const serverModule = context.asAbsolutePath(
        path.join('out', 'server', 'server.js')
    );

    const serverOptions: ServerOptions = {
        run: { module: serverModule, transport: TransportKind.ipc },
        debug: {
            module: serverModule,
            transport: TransportKind.ipc,
            options: { execArgv: ['--nolazy', '--inspect=6009'] }
        }
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [{ scheme: 'file', language: 'joss' }],
        synchronize: {
            fileEvents: [
                vscode.workspace.createFileSystemWatcher('**/*.joss'),
                vscode.workspace.createFileSystemWatcher('**/*.jp'),
                vscode.workspace.createFileSystemWatcher('**/joss.yaml')
            ]
        }
    };

    const client = new LanguageClient(
        'jossLanguageServer',
        'Joss Language Server',
        serverOptions,
        clientOptions
    );

    void client.start();

    return client;
}
