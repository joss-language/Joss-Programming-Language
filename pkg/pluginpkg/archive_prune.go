package pluginpkg

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"

	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
)

// PruneArchive removes unreferenced functions and classes from a JP v2 archive.
// usedSymbols is the set of symbol names referenced by the caller.
// If usedSymbols is nil or empty, or contains "*", the archive is left untouched.
// Returns the pruned archive bytes, a boolean indicating if pruning was applied, and any error.
func PruneArchive(jpBytes []byte, usedSymbols map[string]bool) ([]byte, bool, error) {
	if len(jpBytes) == 0 {
		return jpBytes, false, nil
	}
	if len(usedSymbols) == 0 || usedSymbols["*"] {
		return jpBytes, false, nil
	}

	archive, err := Read(jpBytes)
	if err != nil {
		return jpBytes, false, err
	}

	bcFile := archive.Metadata.Bytecode
	if bcFile == "" {
		return jpBytes, false, nil
	}

	bcBytes, exists := archive.Files[bcFile]
	if !exists || !bytecode.IsBytecode(bcBytes) {
		return jpBytes, false, nil
	}

	prog, err := bytecode.Decode(bcBytes)
	if err != nil {
		return jpBytes, false, err
	}
	if prog == nil || len(prog.Statements) == 0 {
		return jpBytes, false, nil
	}

	// 1. Index declared functions and classes
	funcDecls := make(map[string]*parser.MethodStatement)
	classDecls := make(map[string]*parser.ClassStatement)

	for _, stmt := range prog.Statements {
		switch s := stmt.(type) {
		case *parser.MethodStatement:
			if s.Name != nil {
				funcDecls[s.Name.Value] = s
			}
		case *parser.ClassStatement:
			if s.Name != nil {
				classDecls[s.Name.Value] = s
			}
		}
	}

	// If no functions or classes to prune, return
	if len(funcDecls) == 0 && len(classDecls) == 0 {
		return jpBytes, false, nil
	}

	// 2. Identify initial live symbols
	liveFuncs := make(map[string]bool)
	liveClasses := make(map[string]bool)

	for sym := range usedSymbols {
		if _, ok := funcDecls[sym]; ok {
			liveFuncs[sym] = true
		}
		if _, ok := classDecls[sym]; ok {
			liveClasses[sym] = true
		}
	}

	// Scan top-level non-callable statements (variable initializations, etc.)
	for _, stmt := range prog.Statements {
		switch stmt.(type) {
		case *parser.MethodStatement, *parser.ClassStatement:
			// Handled separately
		default:
			scanASTDependencies(stmt, liveFuncs, liveClasses)
		}
	}

	// 3. Transitive expansion until fixpoint
	scannedFuncs := make(map[string]bool)
	scannedClasses := make(map[string]bool)

	for {
		newWork := false

		for fnName := range liveFuncs {
			if scannedFuncs[fnName] {
				continue
			}
			scannedFuncs[fnName] = true
			if decl, ok := funcDecls[fnName]; ok && decl.Body != nil {
				newWork = true
				scanASTDependencies(decl.Body, liveFuncs, liveClasses)
			}
		}

		for clsName := range liveClasses {
			if scannedClasses[clsName] {
				continue
			}
			scannedClasses[clsName] = true
			if decl, ok := classDecls[clsName]; ok {
				newWork = true
				if decl.SuperClass != nil && classDecls[decl.SuperClass.Value] != nil {
					liveClasses[decl.SuperClass.Value] = true
				}
				if decl.Body != nil {
					scanASTDependencies(decl.Body, liveFuncs, liveClasses)
				}
			}
		}

		if !newWork {
			break
		}
	}

	// 4. Prune statements
	var retainedStmts []parser.Statement
	prunedCount := 0

	for _, stmt := range prog.Statements {
		switch s := stmt.(type) {
		case *parser.MethodStatement:
			if s.Name != nil && !liveFuncs[s.Name.Value] {
				prunedCount++
				continue
			}
			retainedStmts = append(retainedStmts, s)

		case *parser.ClassStatement:
			if s.Name != nil && !liveClasses[s.Name.Value] {
				prunedCount++
				continue
			}
			retainedStmts = append(retainedStmts, s)

		default:
			retainedStmts = append(retainedStmts, stmt)
		}
	}

	if prunedCount == 0 {
		return jpBytes, false, nil
	}

	prog.Statements = retainedStmts

	// 5. Re-encode bytecode
	newBc, err := bytecode.Encode(prog)
	if err != nil {
		return jpBytes, false, err
	}
	archive.Files[bcFile] = newBc

	// 6. Update symbol index if present
	if archive.Metadata.Symbols != "" && archive.Files[archive.Metadata.Symbols] != nil {
		newIndex := BuildSymbolIndex(prog, archive.Metadata.Name, archive.Metadata.Version)
		if idxJSON, jsonErr := json.MarshalIndent(newIndex, "", "  "); jsonErr == nil {
			archive.Files[archive.Metadata.Symbols] = idxJSON
		}
	}

	// 7. Update exports
	if len(archive.Metadata.Exports) > 0 {
		var newExports []string
		for _, exp := range archive.Metadata.Exports {
			if liveFuncs[exp] || liveClasses[exp] {
				newExports = append(newExports, exp)
			}
		}
		archive.Metadata.Exports = newExports
	}

	// 8. Re-sign archive
	var signingKey ed25519.PrivateKey
	if k, _, err := LoadOrCreateSigningKey(archive.Metadata.Name); err == nil && len(k) == ed25519.PrivateKeySize {
		signingKey = k
	} else {
		_, priv, genErr := ed25519.GenerateKey(rand.Reader)
		if genErr != nil {
			return jpBytes, false, genErr
		}
		signingKey = priv
	}

	rebuilt, err := BuildSigned(archive.Metadata, archive.Files, signingKey)
	if err != nil {
		return jpBytes, false, err
	}

	return rebuilt, true, nil
}

func scanASTDependencies(node parser.Node, liveFuncs map[string]bool, liveClasses map[string]bool) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parser.Program:
		for _, s := range n.Statements {
			scanASTDependencies(s, liveFuncs, liveClasses)
		}
	case *parser.BlockStatement:
		for _, s := range n.Statements {
			scanASTDependencies(s, liveFuncs, liveClasses)
		}
	case *parser.ExpressionStatement:
		scanASTDependencies(n.Expression, liveFuncs, liveClasses)
	case *parser.LetStatement:
		scanASTDependencies(n.Value, liveFuncs, liveClasses)
	case *parser.MultiLetStatement:
		for _, d := range n.Declarations {
			scanASTDependencies(d.Value, liveFuncs, liveClasses)
		}
	case *parser.ReturnStatement:
		scanASTDependencies(n.ReturnValue, liveFuncs, liveClasses)
	case *parser.EchoStatement:
		scanASTDependencies(n.Value, liveFuncs, liveClasses)
	case *parser.GuardStatement:
		scanASTDependencies(n.Condition, liveFuncs, liveClasses)
		scanASTDependencies(n.Body, liveFuncs, liveClasses)
	case *parser.ForeachStatement:
		scanASTDependencies(n.Iterable, liveFuncs, liveClasses)
		scanASTDependencies(n.Body, liveFuncs, liveClasses)
	case *parser.WhileStatement:
		scanASTDependencies(n.Condition, liveFuncs, liveClasses)
		scanASTDependencies(n.Body, liveFuncs, liveClasses)
	case *parser.DoWhileStatement:
		scanASTDependencies(n.Condition, liveFuncs, liveClasses)
		scanASTDependencies(n.Body, liveFuncs, liveClasses)
	case *parser.TryCatchStatement:
		scanASTDependencies(n.TryBlock, liveFuncs, liveClasses)
		scanASTDependencies(n.CatchBlock, liveFuncs, liveClasses)
	case *parser.ThrowStatement:
		scanASTDependencies(n.Value, liveFuncs, liveClasses)
	case *parser.DeferStatement:
		scanASTDependencies(n.Body, liveFuncs, liveClasses)
	case *parser.DestructureStatement:
		scanASTDependencies(n.Value, liveFuncs, liveClasses)
	case *parser.InitStatement:
		scanASTDependencies(n.Body, liveFuncs, liveClasses)
	case *parser.MethodStatement:
		scanASTDependencies(n.Body, liveFuncs, liveClasses)
	case *parser.AssignExpression:
		scanASTDependencies(n.Left, liveFuncs, liveClasses)
		scanASTDependencies(n.Value, liveFuncs, liveClasses)
	case *parser.CallExpression:
		if ident, ok := n.Function.(*parser.Identifier); ok {
			liveFuncs[ident.Value] = true
		} else if member, ok := n.Function.(*parser.MemberExpression); ok {
			if ident, ok := member.Left.(*parser.Identifier); ok {
				liveClasses[ident.Value] = true
			}
			scanASTDependencies(member.Left, liveFuncs, liveClasses)
		} else {
			scanASTDependencies(n.Function, liveFuncs, liveClasses)
		}
		for _, arg := range n.Arguments {
			scanASTDependencies(arg, liveFuncs, liveClasses)
		}
	case *parser.NewExpression:
		if n.Class != nil {
			liveClasses[n.Class.Value] = true
		}
		for _, arg := range n.Arguments {
			scanASTDependencies(arg, liveFuncs, liveClasses)
		}
	case *parser.MemberExpression:
		if ident, ok := n.Left.(*parser.Identifier); ok {
			liveClasses[ident.Value] = true
		}
		scanASTDependencies(n.Left, liveFuncs, liveClasses)
	case *parser.IndexExpression:
		scanASTDependencies(n.Left, liveFuncs, liveClasses)
		scanASTDependencies(n.Index, liveFuncs, liveClasses)
	case *parser.InfixExpression:
		scanASTDependencies(n.Left, liveFuncs, liveClasses)
		scanASTDependencies(n.Right, liveFuncs, liveClasses)
	case *parser.PrefixExpression:
		scanASTDependencies(n.Right, liveFuncs, liveClasses)
	case *parser.PostfixExpression:
		scanASTDependencies(n.Left, liveFuncs, liveClasses)
	case *parser.TernaryExpression:
		scanASTDependencies(n.Condition, liveFuncs, liveClasses)
		scanASTDependencies(n.True, liveFuncs, liveClasses)
		scanASTDependencies(n.False, liveFuncs, liveClasses)
	case *parser.MatchExpression:
		scanASTDependencies(n.Subject, liveFuncs, liveClasses)
		for _, arm := range n.Arms {
			for _, k := range arm.Keys {
				scanASTDependencies(k, liveFuncs, liveClasses)
			}
			scanASTDependencies(arm.Value, liveFuncs, liveClasses)
		}
	case *parser.ArrayLiteral:
		for _, el := range n.Elements {
			scanASTDependencies(el, liveFuncs, liveClasses)
		}
	case *parser.MapLiteral:
		for k, v := range n.Pairs {
			scanASTDependencies(k, liveFuncs, liveClasses)
			scanASTDependencies(v, liveFuncs, liveClasses)
		}
	case *parser.FunctionLiteral:
		scanASTDependencies(n.Body, liveFuncs, liveClasses)
	case *parser.BlockExpression:
		scanASTDependencies(n.Block, liveFuncs, liveClasses)
	}
}
