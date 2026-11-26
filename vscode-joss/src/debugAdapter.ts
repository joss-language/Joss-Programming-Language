import * as vscode from 'vscode';
import { ChildProcess, spawn } from 'child_process';

interface DapMessage {
    seq: number;
    type: 'request' | 'response' | 'event';
    command?: string;
    arguments?: Record<string, unknown>;
    request_seq?: number;
    success?: boolean;
    body?: Record<string, unknown>;
    event?: string;
}

export interface JossDebugConfig extends vscode.DebugConfiguration {
    args: string[];
    cwd?: string;
    jossBin?: string;
}

/**
 * Inline Debug Adapter for Joss CLI sessions (`joss run`, `joss server start`,
 * `joss program start`, `joss build`). Streams stdout/stderr directly into the
 * VS Code Debug Console ("Consola de depuración") and provides the native
 * floating Stop / Restart toolbar just like Flutter.
 */
class JossInlineDebugAdapter implements vscode.DebugAdapter {
    private sendMessage = new vscode.EventEmitter<vscode.DebugProtocolMessage>();
    readonly onDidSendMessage: vscode.Event<vscode.DebugProtocolMessage> = this.sendMessage.event;

    private seq = 1;
    private child: ChildProcess | undefined;
    private terminated = false;

    handleMessage(raw: vscode.DebugProtocolMessage): void {
        const msg = raw as unknown as DapMessage;
        if (msg.type !== 'request' || !msg.command) {
            return;
        }

        switch (msg.command) {
            case 'initialize':
                this.sendResponse(msg, {
                    supportsConfigurationDoneRequest: true,
                    supportsTerminateRequest: true,
                    supportsRestartRequest: false
                });
                this.sendEvent('initialized');
                break;

            case 'launch':
                this.sendResponse(msg, {});
                this.launchProcess(msg.arguments as unknown as JossDebugConfig);
                break;

            case 'configurationDone':
                this.sendResponse(msg, {});
                break;

            case 'threads':
                this.sendResponse(msg, {
                    threads: [{ id: 1, name: 'Joss Native Runtime' }]
                });
                break;

            case 'terminate':
            case 'disconnect':
                this.stopProcess();
                this.sendResponse(msg, {});
                break;

            default:
                this.sendResponse(msg, {});
                break;
        }
    }

    private launchProcess(config: JossDebugConfig): void {
        const jossBin = config.jossBin
            || vscode.workspace.getConfiguration('joss').get<string>('executablePath', 'joss')
            || 'joss';
        const args = Array.isArray(config.args) ? config.args : [];
        const cwd = config.cwd
            || vscode.workspace.workspaceFolders?.[0]?.uri.fsPath
            || process.cwd();

        this.output(`⚡ Ejecutando: ${jossBin} ${args.join(' ')}\n`, 'console');

        try {
            const child = spawn(jossBin, args, {
                cwd,
                windowsHide: true,
                env: { ...process.env }
            });
            this.child = child;

            child.stdout?.on('data', (chunk: Buffer) => {
                this.output(chunk.toString('utf-8'), 'stdout');
            });

            child.stderr?.on('data', (chunk: Buffer) => {
                this.output(chunk.toString('utf-8'), 'stderr');
            });

            child.on('error', (err: Error) => {
                this.output(`❌ Error ejecutando ${jossBin}: ${err.message}\n`, 'stderr');
                this.finish();
            });

            child.on('close', (code: number | null, signal: NodeJS.Signals | null) => {
                if (!this.terminated) {
                    if (code === 0) {
                        this.output(`\n✓ Proceso finalizado correctamente (código 0).\n`, 'console');
                    } else if (signal) {
                        this.output(`\n⏹ Proceso detenido (${signal}).\n`, 'console');
                    } else {
                        this.output(`\n✗ Proceso finalizado con código ${code ?? 1}.\n`, 'stderr');
                    }
                }
                this.child = undefined;
                this.finish();
            });
        } catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            this.output(`❌ Error iniciando proceso Joss: ${message}\n`, 'stderr');
            this.finish();
        }
    }

    private stopProcess(): void {
        this.terminated = true;
        const proc = this.child;
        if (!proc || proc.killed || !proc.pid) {
            this.finish();
            return;
        }
        this.output('\n⏹ Deteniendo proceso Joss...\n', 'console');
        if (process.platform === 'win32') {
            try {
                spawn('taskkill', ['/F', '/T', '/PID', String(proc.pid)], { windowsHide: true });
            } catch {
                proc.kill();
            }
        } else {
            try {
                proc.kill('SIGTERM');
            } catch {
                // ignore
            }
        }
    }

    private finish(): void {
        this.sendEvent('terminated');
    }

    private output(text: string, category: 'console' | 'stdout' | 'stderr'): void {
        this.sendEvent('output', {
            category,
            output: text
        });
    }

    private sendResponse(request: DapMessage, body: Record<string, unknown>): void {
        this.sendMessage.fire({
            seq: this.seq++,
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: request.command,
            body
        });
    }

    private sendEvent(event: string, body?: Record<string, unknown>): void {
        this.sendMessage.fire({
            seq: this.seq++,
            type: 'event',
            event,
            body
        });
    }

    dispose(): void {
        this.stopProcess();
        this.sendMessage.dispose();
    }
}

export class JossDebugAdapterFactory implements vscode.DebugAdapterDescriptorFactory {
    createDebugAdapterDescriptor(
        _session: vscode.DebugSession
    ): vscode.ProviderResult<vscode.DebugAdapterDescriptor> {
        return new vscode.DebugAdapterInlineImplementation(new JossInlineDebugAdapter());
    }
}

export class JossDebugConfigurationProvider implements vscode.DebugConfigurationProvider {
    resolveDebugConfiguration(
        folder: vscode.WorkspaceFolder | undefined,
        config: vscode.DebugConfiguration
    ): vscode.ProviderResult<vscode.DebugConfiguration> {
        if (!config.type && !config.request && !config.name) {
            const editor = vscode.window.activeTextEditor;
            if (editor && editor.document.languageId === 'joss') {
                return {
                    type: 'joss',
                    request: 'launch',
                    name: 'Joss: Run Current File',
                    args: ['run', editor.document.uri.fsPath],
                    cwd: folder?.uri.fsPath
                };
            }
        }
        return config;
    }
}

/**
 * Starts a Joss CLI command inside VS Code's Debug Console with the floating
 * Stop / Restart toolbar (Flutter-style) instead of opening a terminal.
 */
export async function startJossDebugSession(name: string, args: string[], cwd?: string): Promise<void> {
    const folder = vscode.workspace.workspaceFolders?.[0];
    const jossBin = vscode.workspace.getConfiguration('joss').get<string>('executablePath', 'joss') || 'joss';
    const config: JossDebugConfig = {
        type: 'joss',
        request: 'launch',
        name,
        args,
        cwd: cwd || folder?.uri.fsPath,
        jossBin,
        internalConsoleOptions: 'openOnSessionStart'
    };
    await vscode.debug.startDebugging(folder, config);
    void vscode.commands.executeCommand('workbench.debug.action.focusRepl');
}
