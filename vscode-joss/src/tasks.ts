import * as vscode from 'vscode';

interface JossTaskDefinition extends vscode.TaskDefinition {
    command: string;
    args?: string[];
}

export class JossTaskProvider implements vscode.TaskProvider {
    static readonly JossType = 'joss';

    private getBin(): string {
        return vscode.workspace.getConfiguration('joss').get<string>('executablePath', 'joss') || 'joss';
    }

    provideTasks(): vscode.Task[] {
        const folder = vscode.workspace.workspaceFolders?.[0];
        if (!folder) {
            return [];
        }

        const bin = this.getBin();
        const defs: Array<{
            label: string;
            command: string;
            args: string[];
            group?: vscode.TaskGroup;
        }> = [
            { label: 'build: native release', command: 'build', args: ['main.joss', '--release'], group: vscode.TaskGroup.Build },
            { label: 'build: native GUI app', command: 'build', args: ['main.joss', '--gui', '--release'], group: vscode.TaskGroup.Build },
            { label: 'test: run suite', command: 'test', args: [], group: vscode.TaskGroup.Test },
            { label: 'check: syntax & types', command: 'check', args: ['.'] },
            { label: 'lint: workspace', command: 'lint', args: ['.'] },
            { label: 'fix: auto-fix issues', command: 'fix', args: ['.'] },
            { label: 'format: workspace', command: 'format', args: ['.', '--write'] }
        ];

        return defs.map(d => {
            const def: JossTaskDefinition = {
                type: JossTaskProvider.JossType,
                command: d.command,
                args: d.args
            };
            const exec = new vscode.ShellExecution(bin, [d.command, ...d.args]);
            const task = new vscode.Task(def, folder, d.label, 'joss', exec, '$joss');
            if (d.group) {
                task.group = d.group;
            }
            return task;
        });
    }

    resolveTask(task: vscode.Task): vscode.Task | undefined {
        const def = task.definition as JossTaskDefinition;
        if (!def.command) {
            return undefined;
        }
        const bin = this.getBin();
        const exec = new vscode.ShellExecution(bin, [def.command, ...(def.args ?? [])]);
        return new vscode.Task(def, task.scope ?? vscode.TaskScope.Workspace, task.name, 'joss', exec, '$joss');
    }
}
