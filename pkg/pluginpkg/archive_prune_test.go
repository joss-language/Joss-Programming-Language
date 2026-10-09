package pluginpkg

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"

	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/parser"
)

func TestPruneArchiveTreeShaking(t *testing.T) {
	source := `
public func func1() {
	return "one"
}

public func func2() {
	return helper()
}

private func helper() {
	return "helper"
}

public func func3() {
	$obj = new UnusedClass()
	return $obj->action()
}

public func func4() {
	return "four"
}

public func func5() {
	return "five"
}

public class UnusedClass {
	public func action() {
		return "unused"
	}
}
`
	l := parser.NewLexer(source)
	p := parser.NewParser(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	bcBytes, err := bytecode.Encode(prog)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	metadata := Metadata{
		Format:   FormatVersion,
		Name:     "math_lib",
		Version:  "1.0.0",
		Bytecode: "plugin.jbc",
		Symbols:  SymbolsPath,
		Exports:  []string{"func1", "func2", "func3", "func4", "func5"},
	}

	symIndex := BuildSymbolIndex(prog, metadata.Name, metadata.Version)
	symBytes, _ := json.MarshalIndent(symIndex, "", "  ")

	files := map[string][]byte{
		"plugin.jbc": bcBytes,
		SymbolsPath:  symBytes,
	}

	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	archiveBytes, err := BuildSigned(metadata, files, privKey)
	if err != nil {
		t.Fatalf("BuildSigned error: %v", err)
	}

	// Now prune keeping only "func2"
	// "func2" calls "helper()", so func2 and helper must be kept, while func1, func3, func4, func5 and UnusedClass are pruned.
	used := map[string]bool{"func2": true}
	prunedBytes, pruned, err := PruneArchive(archiveBytes, used)
	if err != nil {
		t.Fatalf("PruneArchive error: %v", err)
	}
	if !pruned {
		t.Fatalf("expected PruneArchive to prune symbols")
	}

	// Verify the pruned archive
	verifiedArch, err := ReadVerified(prunedBytes)
	if err != nil {
		t.Fatalf("ReadVerified failed on pruned archive: %v", err)
	}

	// Check exports
	if len(verifiedArch.Metadata.Exports) != 1 || verifiedArch.Metadata.Exports[0] != "func2" {
		t.Fatalf("unexpected exports: %v, expected [func2]", verifiedArch.Metadata.Exports)
	}

	// Decode pruned program and verify statements
	prunedProg, err := bytecode.Decode(verifiedArch.Files["plugin.jbc"])
	if err != nil {
		t.Fatalf("failed to decode pruned bytecode: %v", err)
	}

	fnNames := make(map[string]bool)
	classNames := make(map[string]bool)
	for _, stmt := range prunedProg.Statements {
		if ms, ok := stmt.(*parser.MethodStatement); ok && ms.Name != nil {
			fnNames[ms.Name.Value] = true
		}
		if cs, ok := stmt.(*parser.ClassStatement); ok && cs.Name != nil {
			classNames[cs.Name.Value] = true
		}
	}

	if !fnNames["func2"] {
		t.Errorf("expected func2 to be retained")
	}
	if !fnNames["helper"] {
		t.Errorf("expected helper to be retained (transitive call from func2)")
	}
	if fnNames["func1"] || fnNames["func3"] || fnNames["func4"] || fnNames["func5"] {
		t.Errorf("dead functions were not pruned: %v", fnNames)
	}
	if classNames["UnusedClass"] {
		t.Errorf("UnusedClass was not pruned")
	}
}
