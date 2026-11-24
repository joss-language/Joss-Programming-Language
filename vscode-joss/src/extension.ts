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

let client: LanguageClient;

export function activate(context: vscode.ExtensionContext) {
    console.log('Joss Language Extension v3.8 is now active!');

    try {
        // Start Language Server (handles formatting, diagnostics, quick fixes, CodeLens, inlay hints, folding, etc.)
        client = startLanguageServer(context);

        // Register Commands
        registerCommands(context, client);

        // View templates (.joss.html) keep their controller-variable suggestions.
        // Source files (.joss) use the richer LSP completion provider.
        context.subscriptions.push(getCompletionItemProvider());

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
