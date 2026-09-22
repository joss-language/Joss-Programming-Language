package core

import (
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestClassMetadataCachingAndInvalidation(t *testing.T) {
	r := NewRuntime()
	defer r.Free()

	src := `
public class Item {
    public int $id = 1
    public string $name = "test"
    public func describe(): string {
        return $this->name
    }
}
`
	p := parser.NewParser(parser.NewLexer(src))
	prog := p.ParseProgram()
	r.Execute(prog)

	meta1 := r.lookupClassMetadata("Item")
	if meta1 == nil {
		t.Fatal("expected class metadata to be found")
	}

	meta2 := r.lookupClassMetadata("Item")
	if meta1 != meta2 {
		t.Fatal("expected second lookup to return identical cached metadata pointer")
	}

	// Invalidate cache
	r.InvalidateClassMetadata("Item")
	if _, ok := r.classMetadataCache["Item"]; ok {
		t.Fatal("expected 'Item' to be purged from metadata cache")
	}

	meta3 := r.lookupClassMetadata("Item")
	if meta3 == nil {
		t.Fatal("expected lookup to rebuild metadata after invalidation")
	}
}
