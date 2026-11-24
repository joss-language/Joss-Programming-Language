package core

import (
	"strings"
	"testing"
)

func TestStripSCSSLineCommentsPreservesURLs(t *testing.T) {
	input := "// Main SCSS Entry Point\n  // Indented comment\nbody {\n  background: url(\"https://example.com/bg.png\");\n}\n"
	got := stripSCSSLineComments(input)
	if strings.Contains(got, "// Main SCSS") || strings.Contains(got, "// Indented") {
		t.Fatalf("expected standalone // comments to be stripped, got:\n%s", got)
	}
	if !strings.Contains(got, `https://example.com/bg.png`) {
		t.Fatalf("expected URL with // to be preserved, got:\n%s", got)
	}
}
