import {
    RenameParams,
    PrepareRenameParams,
    WorkspaceEdit,
    TextEdit,
    Range,
    ResponseError,
    ErrorCodes
} from 'vscode-languageserver/node';
import { connection, documents, indexer } from '../server';
import { referenceAtPosition } from '../utils/callContext';

// A valid Joss identifier: starts with optional $ for variables, or a plain
// letter/underscore for functions, classes, methods and enum cases.
const JOSS_IDENTIFIER = /^(\$?[A-Za-z_]\w*)$/;

export function setupRenameProvider() {
    // prepareRename: validate that the symbol under the cursor is renameable
    // and return the range that covers the current name so the editor can
    // pre-populate the input field.
    connection.onPrepareRename(async (params: PrepareRenameParams) => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return null;

        const word = referenceAtPosition(document, params.position);
        if (!word || !JOSS_IDENTIFIER.test(word)) {
            throw new ResponseError(
                ErrorCodes.InvalidRequest,
                'The symbol under the cursor cannot be renamed.'
            );
        }

        // Build the range of the word at the cursor position.
        const line = document.getText({
            start: { line: params.position.line, character: 0 },
            end: { line: params.position.line, character: 10000 }
        });
        const col = params.position.character;
        let start = col;
        let end = col;
        // Expand left — include leading $ for variables.
        while (start > 0 && /[\w$]/.test(line[start - 1])) start--;
        // Expand right.
        while (end < line.length && /\w/.test(line[end])) end++;
        const tokenInRange = line.substring(start, end);
        if (!tokenInRange) return null;

        const range: Range = {
            start: { line: params.position.line, character: start },
            end: { line: params.position.line, character: end }
        };
        return { range, placeholder: tokenInRange };
    });

    // onRenameRequest: collect all reference locations and build workspace
    // edits that replace the old name with the new one.
    connection.onRenameRequest(async (params: RenameParams): Promise<WorkspaceEdit | null> => {
        const document = documents.get(params.textDocument.uri);
        if (!document) return null;

        const newName = params.newName.trim();
        if (!newName || !JOSS_IDENTIFIER.test(newName)) return null;

        const word = referenceAtPosition(document, params.position);
        if (!word) return null;

        // Resolve the base name: strip $ prefix for variable lookups so the
        // reference search in the indexer matches both declarations and usages.
        const baseName = word.replace(/^\$/, '');

        // Use the indexer's findReferences to locate every usage in the
        // workspace. Because the indexer stores sources and uses a regex scan,
        // this covers all open and indexed files.
        const locations = await indexer.findReferences(word);

        if (!locations.length) return null;

        // Determine whether we need to preserve the $ sigil.  The indexer
        // strips it when searching, so each location points at the plain base
        // name; we need to know whether the original token had a $ to construct
        // the right replacement.
        const hasSigil = word.startsWith('$');
        const replacement = hasSigil
            ? (newName.startsWith('$') ? newName : '$' + newName)
            : newName.replace(/^\$/, '');

        // Group edits by file URI.
        const changeMap: { [uri: string]: TextEdit[] } = {};
        for (const location of locations) {
            const edits = changeMap[location.uri] || (changeMap[location.uri] = []);
            edits.push(TextEdit.replace(location.range, replacement));
        }

        return { changes: changeMap };
    });
}
