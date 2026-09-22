package core

import (
	"testing"
)

func TestModernSyntaxRuntime(t *testing.T) {
	t.Run("destructure statement execution", func(t *testing.T) {
		r := executeCode(t, `
let ($a, $b) = [15, 25];
$res = $a + $b;
`)
		if r.Variables["res"] != int64(40) {
			t.Fatalf("expected res=40, got %v", r.Variables["res"])
		}
	})

	t.Run("record creation and field access", func(t *testing.T) {
		r := executeCode(t, `
public record Point(int $x, int $y);

$p = new Point(10, 20);
$res = $p->x + $p->y;
`)
		if r.Variables["res"] != int64(30) {
			t.Fatalf("expected res=30, got %v", r.Variables["res"])
		}
	})

	t.Run("first-class await expression", func(t *testing.T) {
		r := executeCode(t, `
$task = async {
    return 100;
};
$val = await $task;
$res = $val + 1;
`)
		if r.Variables["res"] != int64(101) {
			t.Fatalf("expected res=101, got %v", r.Variables["res"])
		}
	})

	t.Run("record fields are immutable after construction", func(t *testing.T) {
		defer func() {
			if rec := recover(); rec == nil {
				t.Fatalf("expected panic modifying immutable record field")
			}
		}()
		executeCode(t, `
public record Point(int $x, int $y);

$p = new Point(10, 20);
$p->x = 99;
`)
	})
}
