import {
    createConnection,
    TextDocuments,
    ProposedFeatures,
    InitializeParams,
    DidChangeConfigurationNotification,
    TextDocumentSyncKind,
    InitializeResult
} from 'vscode-languageserver/node';

import { TextDocument } from 'vscode-languageserver-textdocument';
import { Indexer } from './indexer/indexer';
import { RouteParser } from './parser/routeParser';
import { SecurityAnalyzer } from './analyzer/securityAnalyzer';
import { setupHoverProvider } from './providers/hoverProvider';
import { setupDefinitionProvider } from './providers/definitionProvider';
import { setupCompletionProvider } from './providers/completionProvider';
import { setupSignatureHelpProvider } from './providers/signatureHelpProvider';
import { setupDiagnostics } from './providers/diagnosticsProvider';
import { setupCustomRequests } from './providers/customRequests';
import { setupDocumentSymbolProvider } from './providers/documentSymbolProvider';
import { setupReferencesProvider } from './providers/referencesProvider';
import { setupFormattingProvider } from './providers/formattingProvider';
import { JossSettings, getDefaultSettings } from './config/settings';
import { URI } from 'vscode-uri';

// Create connection
export const connection = createConnection(ProposedFeatures.all);

// Create document manager
export const documents: TextDocuments<TextDocument> = new TextDocuments(TextDocument);

// Create core services
export const indexer = new Indexer();
export const routeParser = new RouteParser();
export const securityAnalyzer = new SecurityAnalyzer();

// Capabilities
let hasConfigurationCapability = false;
let hasWorkspaceFolderCapability = false;

connection.onInitialize((params: InitializeParams) => {
    const capabilities = params.capabilities;

    hasConfigurationCapability = !!(
        capabilities.workspace && !!capabilities.workspace.configuration
    );
    hasWorkspaceFolderCapability = !!(
        capabilities.workspace && !!capabilities.workspace.workspaceFolders
    );

    const result: InitializeResult = {
        capabilities: {
            textDocumentSync: TextDocumentSyncKind.Incremental,
            completionProvider: {
                resolveProvider: true,
                triggerCharacters: [':', '@', '$', '.', '>']
            },
            signatureHelpProvider: {
                triggerCharacters: ['(', ','],
                retriggerCharacters: [',']
            },
            hoverProvider: true,
            definitionProvider: true,
            referencesProvider: true,
            documentSymbolProvider: true,
            workspaceSymbolProvider: true,
            documentFormattingProvider: true
        }
    };

    if (hasWorkspaceFolderCapability) {
        result.capabilities.workspace = {
            workspaceFolders: {
                supported: true
            }
        };
    }

    return result;
});

connection.onInitialized(async () => {
    if (hasConfigurationCapability) {
        connection.client.register(DidChangeConfigurationNotification.type, undefined);
    }

    connection.console.log('JosSecurity Language Server initialized');

    // Index workspace on startup
    const workspaceFolders = await connection.workspace.getWorkspaceFolders();
    if (workspaceFolders && workspaceFolders.length > 0) {
        const workspaceRoot = URI.parse(workspaceFolders[0].uri).fsPath;
        indexer.setWorkspaceRoot(workspaceRoot);

        connection.console.log(`Indexing workspace: ${workspaceRoot}`);
        await indexer.indexWorkspace();
        connection.console.log('Workspace indexed successfully');
    }
});

// Setup all providers
setupHoverProvider();
setupDefinitionProvider();
setupCompletionProvider();
setupSignatureHelpProvider();
setupDiagnostics();
setupCustomRequests();
setupDocumentSymbolProvider();
setupReferencesProvider();
setupFormattingProvider();

// Configuration management
export let globalSettings: JossSettings = getDefaultSettings();
const documentSettings: Map<string, Thenable<JossSettings>> = new Map();

export function getDocumentSettings(resource: string): Thenable<JossSettings> {
    if (!hasConfigurationCapability) {
        return Promise.resolve(globalSettings);
    }
    let result = documentSettings.get(resource);
    if (!result) {
        result = connection.workspace.getConfiguration({
            scopeUri: resource,
            section: 'joss'
        });
        documentSettings.set(resource, result);
    }
    return result;
}

connection.onDidChangeConfiguration(change => {
    if (hasConfigurationCapability) {
        documentSettings.clear();
    } else {
        globalSettings = <JossSettings>(
            (change.settings.joss || getDefaultSettings())
        );
    }
    documents.all().forEach(doc => {
        // Re-validate all documents
        connection.sendDiagnostics({ uri: doc.uri, diagnostics: [] });
    });
});

documents.onDidClose(e => {
    documentSettings.delete(e.document.uri);
});

documents.onDidOpen(event => {
    void indexer.indexDocument(event.document.uri, event.document.getText());
});

documents.onDidChangeContent(event => {
    void indexer.indexDocument(event.document.uri, event.document.getText());
});

let reindexTimer: NodeJS.Timeout | undefined;
connection.onDidChangeWatchedFiles(() => {
    if (reindexTimer) clearTimeout(reindexTimer);
    reindexTimer = setTimeout(() => void indexer.indexWorkspace(), 250);
});

// Listen on the connection
documents.listen(connection);
connection.listen();
