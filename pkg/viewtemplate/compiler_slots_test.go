package viewtemplate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompileHTMLToJossWithSlotsPreservesStructureHashOnStaticEdit(t *testing.T) {
	htmlV1 := "<h1>holas</h1><p>{{ $name }}</p>"
	htmlV2 := "<h1>hola</h1><p class=\"lead\">{{ $name }}</p>"
	htmlV3 := "<h1>hola</h1><p>{{ $name }} - {{ $role }}</p>"

	code1, chunks1, hash1, err := CompileHTMLToJossWithSlots("home", htmlV1)
	if err != nil {
		t.Fatalf("v1 compile failed: %v", err)
	}
	_, chunks2, hash2, err := CompileHTMLToJossWithSlots("home", htmlV2)
	if err != nil {
		t.Fatalf("v2 compile failed: %v", err)
	}
	_, _, hash3, err := CompileHTMLToJossWithSlots("home", htmlV3)
	if err != nil {
		t.Fatalf("v3 compile failed: %v", err)
	}

	if hash1 == "" || hash1 != hash2 {
		t.Fatalf("expected identical structureHash on static HTML edit, got v1=%q v2=%q", hash1, hash2)
	}
	if hash1 == hash3 {
		t.Fatalf("expected different structureHash when dynamic expression added, both=%q", hash1)
	}
	if len(chunks1) != 2 || chunks1[0] != "<h1>holas</h1><p>" || chunks2[0] != "<h1>hola</h1><p class=\"lead\">" {
		t.Fatalf("unexpected static chunks: v1=%#v v2=%#v", chunks1, chunks2)
	}
	if code1 == "" {
		t.Fatal("expected generated Joss code")
	}
}

func TestSyncProjectViewChunksWritesBinaryChunks(t *testing.T) {
	tmp := t.TempDir()
	viewsDir := filepath.Join(tmp, "app", "views")
	cacheDir := filepath.Join(tmp, "assets", ".joss_cache", "views")
	if err := os.MkdirAll(viewsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewsDir, "welcome.joss.html"), []byte("<h1>holas</h1>"), 0644); err != nil {
		t.Fatal(err)
	}

	hashes1, err := SyncProjectViewChunks(viewsDir, cacheDir)
	if err != nil {
		t.Fatalf("SyncProjectViewChunks failed: %v", err)
	}
	chunkFile := filepath.Join(cacheDir, "welcome.chunks")
	if _, err := os.Stat(chunkFile); err != nil {
		t.Fatalf("expected chunk file %s to exist: %v", chunkFile, err)
	}

	if err := os.WriteFile(filepath.Join(viewsDir, "welcome.joss.html"), []byte("<h1>hola</h1>"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes2, err := SyncProjectViewChunks(viewsDir, cacheDir)
	if err != nil {
		t.Fatalf("second SyncProjectViewChunks failed: %v", err)
	}
	if hashes1["welcome"] != hashes2["welcome"] {
		t.Fatalf("expected same structure hash after static text edit: %q vs %q", hashes1["welcome"], hashes2["welcome"])
	}
}
