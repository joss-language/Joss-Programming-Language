//go:build ignore

package main

import (
	"fmt"
	"time"
)

type Node struct {
	left  *Node
	right *Node
	item  int
}

func bottomUpTree(item, depth int) *Node {
	if depth > 0 {
		return &Node{
			left:  bottomUpTree(2*item-1, depth-1),
			right: bottomUpTree(2*item, depth-1),
			item:  item,
		}
	}
	return &Node{item: item}
}

func itemCheck(n *Node) int {
	if n.left == nil {
		return n.item
	}
	return n.item + itemCheck(n.left) - itemCheck(n.right)
}

func main() {
	const maxDepth = 14
	const minDepth = 4
	const stretchDepth = maxDepth + 1

	start := time.Now()

	stretchTree := bottomUpTree(0, stretchDepth)
	stretchCheck := itemCheck(stretchTree)

	longLivedTree := bottomUpTree(0, maxDepth)

	totalChecks := 0
	for depth := minDepth; depth <= maxDepth; depth += 2 {
		iterations := 1 << (maxDepth - depth + minDepth)
		check := 0
		for i := 1; i <= iterations; i++ {
			t1 := bottomUpTree(i, depth)
			t2 := bottomUpTree(-i, depth)
			check += itemCheck(t1) + itemCheck(t2)
		}
		totalChecks += check
	}

	longLivedCheck := itemCheck(longLivedTree)

	elapsedMs := float64(time.Since(start).Nanoseconds()) / 1e6

	fmt.Printf("stretch tree depth %d check: %d\n", stretchDepth, stretchCheck)
	fmt.Printf("long lived tree depth %d check: %d\n", maxDepth, longLivedCheck)
	fmt.Printf("total checks: %d\n", totalChecks)
	fmt.Printf("Binary trees (depth %d) completed in %.4f ms\n", maxDepth, elapsedMs)
}
