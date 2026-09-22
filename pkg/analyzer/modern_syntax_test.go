package analyzer

import (
	"testing"
)

func TestModernSyntaxAnalysis(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantWarn int
		wantErr  int
	}{
		{
			name: "destructuring tuple assignment",
			source: `
public func test(): int {
    let ($a, $b) = [10, 20];
    return $a + $b;
}
`,
			wantErr: 0,
		},
		{
			name: "generic class declaration with type parameter T",
			source: `
public class Container<T> {
    public T $item;
    public func getItem(): T {
        return $this->item;
    }
}
`,
			wantErr: 0,
		},
		{
			name: "record declaration",
			source: `
public record Point(int $x, int $y);

public func test(): Point {
    return new Point(1, 2);
}
`,
			wantErr: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := analyzeSource(t, tt.source, Environment{})
			errors := 0
			for _, d := range diags {
				if d.Severity == "error" {
					errors++
					t.Logf("unexpected error: %s: %s", d.Code, d.Message)
				}
			}
			if errors != tt.wantErr {
				t.Fatalf("got %d errors, want %d", errors, tt.wantErr)
			}
		})
	}
}
