import {
    DocumentSymbolParams,
    DocumentSymbol,
    SymbolInformation,
    SymbolKind,
    Range
} from 'vscode-languageserver/node';
import { connection, documents, indexer } from '../server';

function findBlockEndLine(lines: string[], startLine: number): number {
    let depth = 0;
    let foundOpen = false;
    for (let i = startLine; i < lines.length; i++) {
        const line = lines[i];
        let inStr: string | null = null;
        for (let c = 0; c < line.length; c++) {
            const ch = line[c];
            if (inStr) {
                if (ch === '\\') { c++; continue; }
                if (ch === inStr) inStr = null;
                continue;
            }
            if (ch === '/' && line[c + 1] === '/') break;
            if (ch === '"' || ch === "'") {
                inStr = ch;
            } else if (ch === '{') {
                depth++;
                foundOpen = true;
            } else if (ch === '}') {
                depth--;
                if (foundOpen && depth <= 0) {
                    return i;
                }
            }
        }
    }
    return startLine;
}

export function setupDocumentSymbolProvider() {
    connection.onDocumentSymbol(async (params: DocumentSymbolParams): Promise<DocumentSymbol[]> => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return [];

        const rootSymbols: DocumentSymbol[] = [];
        const lines = document.getText().split('\n');

        interface ContainerCtx {
            symbol: DocumentSymbol;
            endLine: number;
            kind: 'class' | 'interface' | 'enum';
        }
        let activeContainer: ContainerCtx | null = null;

        for (let i = 0; i < lines.length; i++) {
            if (activeContainer && i > activeContainer.endLine) {
                activeContainer = null;
            }

            const line = lines[i];
            const trimmed = line.trim();
            if (!trimmed || trimmed.startsWith('//') || trimmed.startsWith('/*') || trimmed.startsWith('*')) {
                continue;
            }

            // 1. Record declaration: `public record Point(int $x, int $y)`
            const recordMatch = line.match(/^\s*(?:public|private)?\s*record\s+(\w+)/);
            if (recordMatch) {
                const name = recordMatch[1];
                const col = line.indexOf(name);
                const endLine = line.includes('{') ? findBlockEndLine(lines, i) : i;
                const sym: DocumentSymbol = {
                    name,
                    detail: 'record',
                    kind: SymbolKind.Struct,
                    range: Range.create({ line: i, character: 0 }, { line: endLine, character: lines[endLine].length }),
                    selectionRange: Range.create({ line: i, character: col }, { line: i, character: col + name.length }),
                    children: []
                };
                rootSymbols.push(sym);
                if (endLine > i) {
                    activeContainer = { symbol: sym, endLine, kind: 'class' };
                }
                continue;
            }

            // 2. Class / Abstract Class / Interface / Enum declaration
            const typeMatch = line.match(/^\s*(?:public|private)?\s*(?:abstract\s+)?(class|interface|enum)\s+(\w+)/);
            if (typeMatch) {
                const kw = typeMatch[1] as 'class' | 'interface' | 'enum';
                const name = typeMatch[2];
                const col = line.indexOf(name);
                const endLine = findBlockEndLine(lines, i);
                const kind = kw === 'interface'
                    ? SymbolKind.Interface
                    : kw === 'enum'
                        ? SymbolKind.Enum
                        : SymbolKind.Class;
                const sym: DocumentSymbol = {
                    name,
                    detail: kw,
                    kind,
                    range: Range.create({ line: i, character: 0 }, { line: endLine, character: lines[endLine].length }),
                    selectionRange: Range.create({ line: i, character: col }, { line: i, character: col + name.length }),
                    children: []
                };
                rootSymbols.push(sym);
                activeContainer = { symbol: sym, endLine, kind: kw };
                continue;
            }

            // 3. Enum case inside enum
            if (activeContainer?.kind === 'enum') {
                const caseMatch = line.match(/^\s*case\s+(\w+)/);
                if (caseMatch) {
                    const name = caseMatch[1];
                    const col = line.indexOf(name);
                    activeContainer.symbol.children!.push({
                        name,
                        kind: SymbolKind.EnumMember,
                        range: Range.create({ line: i, character: 0 }, { line: i, character: line.length }),
                        selectionRange: Range.create({ line: i, character: col }, { line: i, character: col + name.length })
                    });
                    continue;
                }
            }

            // 4. Constructor: `Init(...)` or `Init main(...)`
            const initMatch = line.match(/^\s*Init(?:\s+(\w+))?\s*\(/);
            if (initMatch) {
                const name = initMatch[1] ? `Init ${initMatch[1]}` : 'Init';
                const col = line.indexOf('Init');
                const endLine = findBlockEndLine(lines, i);
                const sym: DocumentSymbol = {
                    name,
                    detail: 'constructor',
                    kind: SymbolKind.Constructor,
                    range: Range.create({ line: i, character: 0 }, { line: endLine, character: lines[endLine].length }),
                    selectionRange: Range.create({ line: i, character: col }, { line: i, character: col + name.length })
                };
                if (activeContainer) {
                    activeContainer.symbol.children!.push(sym);
                } else {
                    rootSymbols.push(sym);
                }
                continue;
            }

            // 5. Method or Function: `[public|protected|private] [static] [abstract] [async] func name(...)`
            const funcMatch = line.match(/^\s*(?:(?:public|protected|private|static|abstract|async)\s+)*func\s+(\w+)\s*\(([^)]*)\)(?:\s*:\s*([^\s{]+))?/);
            if (funcMatch) {
                const name = funcMatch[1];
                const retType = funcMatch[3] ? `: ${funcMatch[3]}` : '';
                const col = line.indexOf(name);
                const endLine = line.includes('{') ? findBlockEndLine(lines, i) : i;
                const sym: DocumentSymbol = {
                    name,
                    detail: `(${funcMatch[2].trim()})${retType}`,
                    kind: activeContainer ? SymbolKind.Method : SymbolKind.Function,
                    range: Range.create({ line: i, character: 0 }, { line: endLine, character: lines[endLine].length }),
                    selectionRange: Range.create({ line: i, character: col }, { line: i, character: col + name.length })
                };
                if (activeContainer) {
                    activeContainer.symbol.children!.push(sym);
                } else {
                    rootSymbols.push(sym);
                }
                continue;
            }

            // 6. Class property or top-level constant
            if (activeContainer) {
                const propMatch = line.match(/^\s*(?:public|protected|private)\s+(?:static\s+)?(?:const\s+)?(?:[\w|?<>]+\s+)?(\$[A-Za-z_]\w*)/);
                if (propMatch) {
                    const name = propMatch[1];
                    const col = line.indexOf(name);
                    const isConst = /\bconst\b/.test(line);
                    activeContainer.symbol.children!.push({
                        name,
                        kind: isConst ? SymbolKind.Constant : SymbolKind.Property,
                        range: Range.create({ line: i, character: 0 }, { line: i, character: line.length }),
                        selectionRange: Range.create({ line: i, character: col }, { line: i, character: col + name.length })
                    });
                    continue;
                }
            } else {
                const constMatch = line.match(/^\s*const\s+(?:[\w|?<>]+\s+)?(\$[A-Za-z_]\w*)/);
                if (constMatch) {
                    const name = constMatch[1];
                    const col = line.indexOf(name);
                    rootSymbols.push({
                        name,
                        kind: SymbolKind.Constant,
                        range: Range.create({ line: i, character: 0 }, { line: i, character: line.length }),
                        selectionRange: Range.create({ line: i, character: col }, { line: i, character: col + name.length })
                    });
                }
            }
        }

        return rootSymbols;
    });

    connection.onWorkspaceSymbol(async params => {
        const query = params.query.toLowerCase();
        return (await indexer.getAllSymbols())
            .filter(symbol => !query || symbol.name.toLowerCase().includes(query) || symbol.qualifiedName.toLowerCase().includes(query))
            .map(symbol => ({
                name: symbol.qualifiedName,
                kind: symbol.kind === 'class' ? SymbolKind.Class
                    : symbol.kind === 'property' ? SymbolKind.Property
                        : symbol.kind === 'method' ? SymbolKind.Method : SymbolKind.Function,
                location: symbol.location,
                containerName: symbol.containerName
            } as SymbolInformation));
    });
}
