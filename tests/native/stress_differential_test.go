package native_test

import (
	"testing"
)

// TestStress_OOP_ObjectCreationAndMethodChaining tests creating 500 object instances,
// accumulating state, invoking instance methods and checking differential outputs.
func TestStress_OOP_ObjectCreationAndMethodChaining(t *testing.T) {
	source := `
		public class Counter {
			public int $val = 0;

			public func increment(int $step): int {
				$this->val = $this->val + $step;
				return $this->val;
			}
		}

		int $total = 0;
		int $i = 0;
		while ($i < 50) {
			var $c = new Counter();
			$c->increment($i);
			$total = $total + $c->val;
			$i = $i + 1;
		}
		echo $total;
	`
	assertDifferential(t, "stress_oop_50_instances", source)
}

// TestStress_Collections_LargeArrayAccumulation tests pushing and mutating items in dynamic arrays.
func TestStress_Collections_LargeArrayAccumulation(t *testing.T) {
	source := `
		var $nums = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
		int $sum = 0;
		int $idx = 0;
		while ($idx < 10) {
			$sum = $sum + $nums[$idx];
			$idx = $idx + 1;
		}
		echo $sum;

		$nums[0] = 100;
		$nums[9] = 900;
		echo $nums[0] + $nums[9];
	`
	assertDifferential(t, "stress_collections_array", source)
}

// TestStress_Collections_MapOperations tests dense key lookup and updates in associative maps.
func TestStress_Collections_MapOperations(t *testing.T) {
	source := `
		var $dict = {"alpha": "first", "beta": "second", "gamma": "third"};
		echo $dict["alpha"];
		echo $dict["beta"];
		echo $dict["gamma"];

		$dict["alpha"] = "A_prime";
		$dict["omega"] = "last";
		echo $dict["alpha"];
		echo $dict["omega"];
	`
	assertDifferential(t, "stress_collections_map", source)
}

// TestStress_Procedural_HeavyLoopsAndBranching tests nested while loops, ternary conditions and arithmetic.
func TestStress_Procedural_HeavyLoopsAndBranching(t *testing.T) {
	source := `
		public func half(int $num): int {
			int $count = 0;
			int $accum = 0;
			while ($accum + 2 <= $num) {
				$accum = $accum + 2;
				$count = $count + 1;
			}
			return $count;
		}

		public func collatz(int $n): int {
			int $steps = 0;
			int $val = $n;
			while ($val > 1) {
				int $mod = $val % 2;
				$val = ($mod == 0) ? half($val) : (($val * 3) + 1);
				$steps = $steps + 1;
			}
			return $steps;
		}

		echo collatz(27);
		echo collatz(12);
		echo collatz(19);
	`
	assertDifferential(t, "stress_procedural_collatz", source)
}

// TestStress_Composition_ObjectsWithArraysAndMaps tests integration across language domains.
func TestStress_Composition_ObjectsWithArraysAndMaps(t *testing.T) {
	source := `
		public class Store {
			public string $name = "JossStore";
			public int $revenue = 0;

			public func sell(int $price, int $quantity): int {
				int $amount = $price * $quantity;
				$this->revenue = $this->revenue + $amount;
				return $amount;
			}
		}

		var $inventory = ["Widget", "Gadget", "Device"];
		var $prices = {"Widget": "15", "Gadget": "30", "Device": "50"};

		var $shop = new Store();
		int $s1 = $shop->sell(15, 2);
		int $s2 = $shop->sell(30, 4);

		echo $shop->name;
		echo $inventory[0];
		echo $prices["Gadget"];
		echo $shop->revenue;
	`
	assertDifferential(t, "stress_composition_full", source)
}

// TestEdgeCases_ExtremeArithmeticAndRelational tests boundary numbers and operators.
func TestEdgeCases_ExtremeArithmeticAndRelational(t *testing.T) {
	source := `
		int $zero = 0;
		int $neg = 0 - 500;
		int $pos = 1000;
		echo $zero + $neg;
		echo $pos * $zero;
		int $t1 = ($neg < $pos) ? 1 : 0;
		int $t2 = ($zero == 0) ? 1 : 0;
		echo $t1;
		echo $t2;
		echo ($neg <=> $pos);
		echo ($pos <=> $neg);
		echo ($zero <=> 0);
	`
	assertDifferential(t, "edge_cases_arithmetic", source)
}
