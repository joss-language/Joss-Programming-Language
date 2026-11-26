import * as vscode from 'vscode';
import * as cp from 'child_process';
import * as path from 'path';
import { LanguageClient } from 'vscode-languageclient/node';
import { startJossDebugSession } from './debugAdapter';

export function registerCommands(context: vscode.ExtensionContext, client: LanguageClient) {
    // Index Workspace
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.indexWorkspace', async () => {
            await vscode.window.withProgress({
                location: vscode.ProgressLocation.Notification,
                title: 'Indexing JosSecurity workspace...',
                cancellable: false
            }, async (progress) => {
                progress.report({ increment: 0 });

                // Send command to server
                await client.sendRequest('workspace/executeCommand', {
                    command: 'joss.indexWorkspace'
                });

                progress.report({ increment: 100 });
                vscode.window.showInformationMessage('Workspace indexed successfully!');
            });
        })
    );

    // Go to Route
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.goToRoute', async () => {
            const routes = await client.sendRequest<any[]>('joss/getRoutes');

            if (!routes || routes.length === 0) {
                vscode.window.showInformationMessage('No routes found');
                return;
            }

            const items = routes.map(r => ({
                label: `${r.method} ${r.path}`,
                description: `${r.controller}@${r.methods.join('@')}`,
                route: r
            }));

            const selected = await vscode.window.showQuickPick(items, {
                placeHolder: 'Select a route to navigate to'
            });

            if (selected) {
                const location = await client.sendRequest<any>('joss/resolveRoute', selected.route);
                if (location) {
                    const uri = vscode.Uri.parse(location.uri);
                    const position = new vscode.Position(location.range.start.line, location.range.start.character);
                    await vscode.window.showTextDocument(uri, { selection: new vscode.Range(position, position) });
                }
            }
        })
    );

    // Create Controller
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.createController', async () => {
            const name = await vscode.window.showInputBox({
                prompt: 'Controller name (e.g., UserController)',
                placeHolder: 'UserController',
                validateInput: (value) => {
                    if (!value) return 'Controller name is required';
                    if (!value.endsWith('Controller')) return 'Controller name should end with "Controller"';
                    return null;
                }
            });

            if (name) {
                const result = await client.sendRequest<any>('joss/createController', { name });
                if (result.success) {
                    vscode.window.showInformationMessage(`Controller ${name} created successfully!`);
                    const uri = vscode.Uri.file(result.path);
                    await vscode.window.showTextDocument(uri);
                } else {
                    vscode.window.showErrorMessage(`Failed to create controller: ${result.error}`);
                }
            }
        })
    );

    // Run Security Check
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.runSecurityCheck', async () => {
            await vscode.window.withProgress({
                location: vscode.ProgressLocation.Notification,
                title: 'Running JosSecurity analysis...',
                cancellable: false
            }, async (progress) => {
                progress.report({ increment: 0 });

                const results = await client.sendRequest<any>('joss/securityCheck');

                progress.report({ increment: 100 });

                if (results.issues.length === 0) {
                    vscode.window.showInformationMessage('✓ No security issues found!');
                } else {
                    vscode.window.showWarningMessage(`Found ${results.issues.length} security issue(s)`);
                }
            });
        })
    );

    // Open Definition
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.openDefinition', async () => {
            await vscode.commands.executeCommand('editor.action.revealDefinition');
        })
    );

    // Restart Server
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.restartServer', async () => {
            await client.stop();
            await client.start();
            void vscode.window.showInformationMessage('Language server restarted');
        })
    );

    // Change DB Prefix
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.changeDbPrefix', async () => {
            const prefix = await vscode.window.showInputBox({
                prompt: 'Enter new database prefix (e.g., app_)',
                placeHolder: 'app_',
                validateInput: (value) => {
                    if (!value) return 'Prefix is required';
                    return null;
                }
            });

            if (prefix) {
                const workspaceFolders = vscode.workspace.workspaceFolders;
                if (!workspaceFolders) {
                    vscode.window.showErrorMessage('No workspace open');
                    return;
                }
                const rootPath = workspaceFolders[0].uri.fsPath;

                vscode.window.withProgress({
                    location: vscode.ProgressLocation.Notification,
                    title: `Changing DB prefix to '${prefix}'...`,
                    cancellable: false
                }, async (progress) => {
                    return new Promise<void>((resolve, reject) => {
                        const jossBin = vscode.workspace.getConfiguration('joss').get<string>('executablePath', 'joss') || 'joss';
                        cp.execFile(jossBin, ['change', 'db', 'prefix', prefix], { cwd: rootPath }, (err, _stdout, stderr) => {
                            if (err) {
                                vscode.window.showErrorMessage(`Failed to change prefix: ${stderr || err.message}`);
                                resolve();
                            } else {
                                vscode.window.showInformationMessage(`Database prefix changed to '${prefix}' successfully!`);
                                resolve();
                            }
                        });
                    });
                });
            }
        })
    );

    // Start Server
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.serverStart', async () => {
            await vscode.window.activeTextEditor?.document.save();
            await startJossDebugSession('Joss Server (Hot-Reload)', ['server', 'start']);
        })
    );

    // New Project
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.newProject', async () => {
            const type = await vscode.window.showQuickPick(['web', 'console'], {
                placeHolder: 'Select project type'
            });

            if (!type) return;

            const folderUri = await vscode.window.showOpenDialog({
                canSelectFiles: false,
                canSelectFolders: true,
                canSelectMany: false,
                openLabel: 'Select Project Location'
            });

            if (folderUri && folderUri[0]) {
                const projectPath = folderUri[0].fsPath;
                const terminal = vscode.window.createTerminal('Joss New Project');
                terminal.show();
                terminal.sendText(`joss new ${type} "${projectPath}"`);
            }
        })
    );

    // Make Model
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.makeModel', async () => {
            const name = await vscode.window.showInputBox({
                prompt: 'Model name (e.g., User)',
                placeHolder: 'User'
            });

            if (name) {
                const terminal = vscode.window.createTerminal('Joss Make Model');
                terminal.show();
                terminal.sendText(`joss make:model ${name}`);
            }
        })
    );

    // Make CRUD
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.makeCRUD', async () => {
            const table = await vscode.window.showInputBox({
                prompt: 'Table name (e.g., users)',
                placeHolder: 'users'
            });

            if (table) {
                const terminal = vscode.window.createTerminal('Joss Make CRUD');
                terminal.show();
                terminal.sendText(`joss make:crud ${table}`);
            }
        })
    );

    // Remove CRUD
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.removeCRUD', async () => {
            const table = await vscode.window.showInputBox({
                prompt: 'Table name (e.g., users)',
                placeHolder: 'users'
            });

            if (table) {
                const terminal = vscode.window.createTerminal('Joss Remove CRUD');
                terminal.show();
                terminal.sendText(`joss remove:crud ${table}`);
            }
        })
    );

    // Make Migration
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.makeMigration', async () => {
            const name = await vscode.window.showInputBox({
                prompt: 'Migration name (e.g., create_users_table)',
                placeHolder: 'create_users_table'
            });

            if (name) {
                const terminal = vscode.window.createTerminal('Joss Make Migration');
                terminal.show();
                terminal.sendText(`joss make:migration ${name}`);
            }
        })
    );

    // Run Migrations
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.migrate', async () => {
            const terminal = vscode.window.createTerminal('Joss Migrate');
            terminal.show();
            terminal.sendText('joss migrate');
        })
    );

    // Change DB Engine
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.changeDbEngine', async () => {
            const engine = await vscode.window.showQuickPick(['mysql', 'sqlite'], {
                placeHolder: 'Select database engine'
            });

            if (engine) {
                const workspaceFolders = vscode.workspace.workspaceFolders;
                if (!workspaceFolders) return;
                const terminal = vscode.window.createTerminal('Joss Change DB');
                terminal.show();
                terminal.sendText(`joss change db ${engine}`);
            }
        })
    );

    // User Storage
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.userstorage', async () => {
            const driver = await vscode.window.showQuickPick(['local', 'OCI'], {
                placeHolder: 'Select storage driver'
            });

            if (driver) {
                const terminal = vscode.window.createTerminal('Joss User Storage');
                terminal.show();
                terminal.sendText(`joss userstorage ${driver}`);
            }
        })
    );

    // Make View
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.makeView', async () => {
            const name = await vscode.window.showInputBox({
                prompt: 'View name (e.g., users/index)',
                placeHolder: 'users/index'
            });

            if (name) {
                const terminal = vscode.window.createTerminal('Joss Make View');
                terminal.show();
                terminal.sendText(`joss make:view ${name}`);
            }
        })
    );

    // Make MVC
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.makeMVC', async () => {
            const name = await vscode.window.showInputBox({
                prompt: 'Resource name (e.g., Product)',
                placeHolder: 'Product'
            });

            if (name) {
                const terminal = vscode.window.createTerminal('Joss Make MVC');
                terminal.show();
                terminal.sendText(`joss make:mvc ${name}`);
            }
        })
    );

    // Format Document
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.formatDocument', async () => {
            await vscode.commands.executeCommand('editor.action.formatDocument');
        })
    );

    // Format Workspace
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.formatWorkspace', async () => {
            const terminal = vscode.window.createTerminal('Joss Format');
            terminal.show();
            terminal.sendText('joss format --write .');
        })
    );

    // Run Linter
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.lint', async () => {
            const terminal = vscode.window.createTerminal('Joss Lint');
            terminal.show();
            terminal.sendText('joss lint');
        })
    );

    // Fix Code Issues Automatically
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.fix', async () => {
            const terminal = vscode.window.createTerminal('Joss Fix');
            terminal.show();
            terminal.sendText('joss fix');
        })
    );

    // Run Tests
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.test', async () => {
            const terminal = vscode.window.createTerminal('Joss Test');
            terminal.show();
            terminal.sendText('joss test');
        })
    );

    // Check Syntax & Types
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.checkSyntax', async () => {
            const terminal = vscode.window.createTerminal('Joss Check');
            terminal.show();
            terminal.sendText('joss check');
        })
    );

    // Build Web Application
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.buildWeb', async () => {
            await vscode.window.activeTextEditor?.document.save();
            await startJossDebugSession('Joss Build', ['build']);
        })
    );

    // Build Native Executable
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.buildNative', async () => {
            await vscode.window.activeTextEditor?.document.save();
            await startJossDebugSession('Joss Build (Release)', ['build', '--release']);
        })
    );

    // Fresh Migrations with Seeders
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.migrateFresh', async () => {
            const terminal = vscode.window.createTerminal('Joss Migrate Fresh');
            terminal.show();
            terminal.sendText('joss migrate:fresh');
        })
    );

    // Run Database Seeders
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.dbSeed', async () => {
            const terminal = vscode.window.createTerminal('Joss DB Seed');
            terminal.show();
            terminal.sendText('joss db:seed');
        })
    );

    // Make Middleware
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.makeMiddleware', async () => {
            const name = await vscode.window.showInputBox({
                prompt: 'Middleware name (e.g., AuthGuard)',
                placeHolder: 'AuthGuard'
            });

            if (name) {
                const terminal = vscode.window.createTerminal('Joss Make Middleware');
                terminal.show();
                terminal.sendText(`joss make:middleware ${name}`);
            }
        })
    );

    // Plugin Compile
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.pluginCompile', async () => {
            const terminal = vscode.window.createTerminal('Joss Plugin Compile');
            terminal.show();
            terminal.sendText('joss plugin compile');
        })
    );

    // Plugin Validate
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.pluginValidate', async () => {
            const terminal = vscode.window.createTerminal('Joss Plugin Validate');
            terminal.show();
            terminal.sendText('joss plugin validate');
        })
    );

    const getJossBin = (): string =>
        vscode.workspace.getConfiguration('joss').get<string>('executablePath', 'joss') || 'joss';

    const getSharedTerminal = (): vscode.Terminal => {
        const existing = vscode.window.terminals.find(t => t.name === 'Joss');
        return existing ?? vscode.window.createTerminal('Joss');
    };

    const getActiveJossFile = (): string | undefined => {
        const editor = vscode.window.activeTextEditor;
        if (!editor || editor.document.languageId !== 'joss') {
            return undefined;
        }
        return editor.document.uri.fsPath;
    };

    // Run Current File
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.runFile', async () => {
            const file = getActiveJossFile();
            if (!file) {
                vscode.window.showWarningMessage('Open a .joss file to run it.');
                return;
            }
            await vscode.window.activeTextEditor?.document.save();
            await startJossDebugSession(`Joss Run: ${path.basename(file)}`, ['run', file]);
        })
    );

    // Build GUI Executable
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.buildGUI', async () => {
            const file = getActiveJossFile() || 'main.joss';
            await vscode.window.activeTextEditor?.document.save();
            await startJossDebugSession('Joss Build GUI', ['build', file, '--gui', '--release']);
        })
    );

    // Launch Desktop App (joss program start)
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.programStart', async () => {
            await vscode.window.activeTextEditor?.document.save();
            await startJossDebugSession('Joss Desktop App', ['program', 'start']);
        })
    );

    // Inspect Native IR (emit-ir)
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.emitIR', async () => {
            const file = getActiveJossFile();
            if (!file) {
                vscode.window.showWarningMessage('Open a .joss file to inspect its Native IR.');
                return;
            }
            await vscode.window.activeTextEditor?.document.save();
            const term = getSharedTerminal();
            term.show();
            term.sendText(`${getJossBin()} emit-ir "${file}"`);
        })
    );

    // Status Bar Quick Actions Menu
    context.subscriptions.push(
        vscode.commands.registerCommand('joss.quickActions', async () => {
            const picks = [
                { label: '$(play) Run Current File', command: 'joss.runFile' },
                { label: '$(server) Start Dev Server (joss server start)', command: 'joss.serverStart' },
                { label: '$(package) Build Executable (joss build)', command: 'joss.buildNative' },
                { label: '$(window) Build GUI Executable (--gui)', command: 'joss.buildGUI' },
                { label: '$(browser) Launch Desktop App (joss program start)', command: 'joss.programStart' },
                { label: '$(beaker) Run Tests (joss test)', command: 'joss.test' },
                { label: '$(check) Check Syntax & Types (joss check)', command: 'joss.checkSyntax' },
                { label: '$(wrench) Fix Auto-Fixable Issues (joss fix)', command: 'joss.fix' },
                { label: '$(file-code) Inspect Native IR (joss emit-ir)', command: 'joss.emitIR' },
                { label: '$(refresh) Restart Joss Language Server', command: 'joss.restartServer' }
            ];
            const choice = await vscode.window.showQuickPick(picks, {
                placeHolder: 'Select a Joss action'
            });
            if (choice) {
                await vscode.commands.executeCommand(choice.command);
            }
        })
    );
}
