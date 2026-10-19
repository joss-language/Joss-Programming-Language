package ir

import (
	"testing"
)

func TestHoistLoopInvariantLoads(t *testing.T) {
	fn := NewFunction("test_loop", TypeI64)
	p0 := fn.AddParam("high", TypeI64)

	entry := fn.EntryBlock
	highPtr := fn.NewTemp(PtrType(TypeI64), "high_ptr")
	entry.AddInstruction(&AllocaInst{Dest: highPtr, AllocType: TypeI64})
	entry.AddInstruction(&StoreInst{Val: p0, Ptr: highPtr})

	jPtr := fn.NewTemp(PtrType(TypeI64), "j_ptr")
	entry.AddInstruction(&AllocaInst{Dest: jPtr, AllocType: TypeI64})
	entry.AddInstruction(&StoreInst{Val: NewConstInt(0, TypeI64), Ptr: jPtr})

	condBlock := fn.NewBlock("cond")
	bodyBlock := fn.NewBlock("body")
	exitBlock := fn.NewBlock("exit")

	entry.SetTerminator(&JumpTerminator{Target: condBlock})

	// In condBlock: load j, load high, cmp, branch
	jVal := fn.NewTemp(TypeI64, "j_val")
	condBlock.AddInstruction(&LoadInst{Dest: jVal, Ptr: jPtr})
	highVal := fn.NewTemp(TypeI64, "high_val")
	condBlock.AddInstruction(&LoadInst{Dest: highVal, Ptr: highPtr})
	cmpVal := fn.NewTemp(TypeBool, "cmp")
	condBlock.AddInstruction(&CompareInst{Dest: cmpVal, Op: OpCmpLt, Left: jVal, Right: highVal})
	condBlock.SetTerminator(&BranchTerminator{Cond: cmpVal, TrueBlock: bodyBlock, FalseBlock: exitBlock})

	// In bodyBlock: j = j + 1, jump cond
	newJ := fn.NewTemp(TypeI64, "new_j")
	bodyBlock.AddInstruction(&BinaryInst{Dest: newJ, Op: OpAdd, Left: jVal, Right: NewConstInt(1, TypeI64)})
	bodyBlock.AddInstruction(&StoreInst{Val: newJ, Ptr: jPtr})
	bodyBlock.SetTerminator(&JumpTerminator{Target: condBlock})

	exitBlock.SetTerminator(&ReturnTerminator{Val: NewConstInt(0, TypeI64)})

	t.Logf("Before Hoist:")
	for _, b := range fn.Blocks {
		t.Logf("Block %s: %d insts", b.Label, len(b.Instructions))
		for _, i := range b.Instructions {
			t.Logf("   %s", i.String())
		}
	}

	HoistLoopInvariantLoads(fn)

	t.Logf("After Hoist:")
	for _, b := range fn.Blocks {
		t.Logf("Block %s: %d insts", b.Label, len(b.Instructions))
		for _, i := range b.Instructions {
			t.Logf("   %s", i.String())
		}
	}

	// Verify that highVal load was hoisted to entry
	foundInEntry := false
	for _, i := range entry.Instructions {
		if load, ok := i.(*LoadInst); ok && load.Dest.ID == highVal.ID {
			foundInEntry = true
		}
	}
	if !foundInEntry {
		t.Errorf("Expected high_val load to be hoisted to entry block!")
	}
}
