import * as path from 'path';
import * as vscode from 'vscode';
import { LanguageClient } from 'vscode-languageclient/node';

type NodeKind = 'section' | 'action' | 'route' | 'file';

interface ExplorerItemData {
    kind: NodeKind;
    section?: 'actions' | 'routes' | 'models' | 'controllers';
    command?: vscode.Command;
    fileUri?: vscode.Uri;
}

export class JossExplorerItem extends vscode.TreeItem {
    constructor(
        label: string,
        collapsibleState: vscode.TreeItemCollapsibleState,
        public readonly data: ExplorerItemData,
        description?: string,
        iconId?: string
    ) {
        super(label, collapsibleState);
        this.description = description;
        if (iconId) {
            this.iconPath = new vscode.ThemeIcon(iconId);
        }
        if (data.command) {
            this.command = data.command;
        }
    }
}

export class JossExplorerProvider implements vscode.TreeDataProvider<JossExplorerItem> {
    private readonly _onDidChangeTreeData = new vscode.EventEmitter<JossExplorerItem | undefined | void>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    constructor(private readonly client: LanguageClient) {}

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    getTreeItem(element: JossExplorerItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: JossExplorerItem): Promise<JossExplorerItem[]> {
        if (!element) {
            return [
                new JossExplorerItem('Quick Actions', vscode.TreeItemCollapsibleState.Expanded, { kind: 'section', section: 'actions' }, undefined, 'zap'),
                new JossExplorerItem('Routes', vscode.TreeItemCollapsibleState.Collapsed, { kind: 'section', section: 'routes' }, undefined, 'globe'),
                new JossExplorerItem('Models', vscode.TreeItemCollapsibleState.Collapsed, { kind: 'section', section: 'models' }, undefined, 'database'),
                new JossExplorerItem('Controllers', vscode.TreeItemCollapsibleState.Collapsed, { kind: 'section', section: 'controllers' }, undefined, 'symbol-class')
            ];
        }

        switch (element.data.section) {
            case 'actions':
                return [
                    this.actionItem('Run Current File', 'joss.runFile', 'play'),
                    this.actionItem('Start Dev Server', 'joss.serverStart', 'server'),
                    this.actionItem('Build Executable', 'joss.buildNative', 'package'),
                    this.actionItem('Build GUI App (--gui)', 'joss.buildGUI', 'window'),
                    this.actionItem('Launch Desktop App', 'joss.programStart', 'browser'),
                    this.actionItem('Run Tests', 'joss.test', 'beaker'),
                    this.actionItem('Fix Auto-Fixable Issues', 'joss.fix', 'wrench')
                ];

            case 'routes':
                return this.getRouteItems();

            case 'models':
                return this.getFileItems('app/models/**/*.joss', 'symbol-struct');

            case 'controllers':
                return this.getFileItems('app/controllers/**/*.joss', 'symbol-method');

            default:
                return [];
        }
    }

    private actionItem(label: string, commandId: string, icon: string): JossExplorerItem {
        return new JossExplorerItem(
            label,
            vscode.TreeItemCollapsibleState.None,
            { kind: 'action', command: { title: label, command: commandId } },
            undefined,
            icon
        );
    }

    private async getRouteItems(): Promise<JossExplorerItem[]> {
        try {
            const routes = await this.client.sendRequest<Array<{
                method: string;
                path: string;
                controller: string;
                methods: string[];
            }>>('joss/getRoutes');
            if (!routes || routes.length === 0) {
                return [new JossExplorerItem('No routes indexed', vscode.TreeItemCollapsibleState.None, { kind: 'route' }, undefined, 'info')];
            }
            return routes.map(r => new JossExplorerItem(
                `${r.method} ${r.path}`,
                vscode.TreeItemCollapsibleState.None,
                {
                    kind: 'route',
                    command: {
                        title: 'Open Route',
                        command: 'joss.openRouteTarget',
                        arguments: [r]
                    }
                },
                `${r.controller}@${r.methods.join('@')}`,
                'arrow-right'
            ));
        } catch {
            return [new JossExplorerItem('Language server starting...', vscode.TreeItemCollapsibleState.None, { kind: 'route' }, undefined, 'sync')];
        }
    }

    private async getFileItems(glob: string, icon: string): Promise<JossExplorerItem[]> {
        const files = await vscode.workspace.findFiles(glob, '**/node_modules/**', 100);
        if (files.length === 0) {
            return [new JossExplorerItem('None found', vscode.TreeItemCollapsibleState.None, { kind: 'file' }, undefined, 'info')];
        }
        files.sort((a, b) => a.fsPath.localeCompare(b.fsPath));
        return files.map(uri => new JossExplorerItem(
            path.basename(uri.fsPath, '.joss'),
            vscode.TreeItemCollapsibleState.None,
            {
                kind: 'file',
                fileUri: uri,
                command: {
                    title: 'Open File',
                    command: 'vscode.open',
                    arguments: [uri]
                }
            },
            vscode.workspace.asRelativePath(uri, false),
            icon
        ));
    }
}
