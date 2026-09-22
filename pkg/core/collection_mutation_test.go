package core

import "testing"

func TestRuntimeTypedCollectionIndexAssignmentChecksElement(t *testing.T) {
	mustPanicWithType(t, `array<int> $items = [1]
$items[0] = "wrong"`, "TypeError")
	mustNotPanic(t, `array<int> $items = [1]
$items[0] = 2`)

	mustPanicWithType(t, `array<array<int>> $matrix = [[1]]
$matrix[0][0] = "bad"`, "TypeError")
	mustNotPanic(t, `array<array<int>> $matrix = [[1]]
$matrix[0][0] = 99`)
}
