<?php

class TreeNode {
    public ?TreeNode $left;
    public ?TreeNode $right;
    public int $item;

    public function __construct(?TreeNode $left, ?TreeNode $right, int $item) {
        $this->left = $left;
        $this->right = $right;
        $this->item = $item;
    }

    public function check(): int {
        return ($this->left === null)
            ? $this->item
            : ($this->item + $this->left->check() - $this->right->check());
    }
}

function bottomUpTree(int $item, int $depth): TreeNode {
    return ($depth > 0)
        ? new TreeNode(bottomUpTree(2 * $item - 1, $depth - 1), bottomUpTree(2 * $item, $depth - 1), $item)
        : new TreeNode(null, null, $item);
}

$maxDepth = 14;
$minDepth = 4;
$stretchDepth = $maxDepth + 1;

$start = microtime(true);

$stretchTree = bottomUpTree(0, $stretchDepth);
$stretchCheck = $stretchTree->check();
unset($stretchTree);

$longLivedTree = bottomUpTree(0, $maxDepth);

$totalChecks = 0;
for ($depth = $minDepth; $depth <= $maxDepth; $depth += 2) {
    $iterations = 1 << ($maxDepth - $depth + $minDepth);
    $check = 0;
    for ($i = 1; $i <= $iterations; ++$i) {
        $t1 = bottomUpTree($i, $depth);
        $t2 = bottomUpTree(-$i, $depth);
        $check += $t1->check() + $t2->check();
    }
    $totalChecks += $check;
}

$longLivedCheck = $longLivedTree->check();
unset($longLivedTree);

$end = microtime(true);
$elapsedMs = ($end - $start) * 1000.0;

echo "stretch tree depth $stretchDepth check: $stretchCheck\n";
echo "long lived tree depth $maxDepth check: $longLivedCheck\n";
echo "total checks: $totalChecks\n";
printf("Binary trees (depth %d) completed in %.4f ms\n", $maxDepth, $elapsedMs);
