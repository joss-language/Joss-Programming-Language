class TreeNode(val left: TreeNode?, val right: TreeNode?, val item: Int) {
    fun check(): Int {
        return if (left == null) item else item + left.check() - (right?.check() ?: 0)
    }
}

fun bottomUpTree(item: Int, depth: Int): TreeNode {
    return if (depth > 0) {
        TreeNode(
            bottomUpTree(2 * item - 1, depth - 1),
            bottomUpTree(2 * item, depth - 1),
            item
        )
    } else {
        TreeNode(null, null, item)
    }
}

fun main() {
    val maxDepth = 14
    val minDepth = 4
    val stretchDepth = maxDepth + 1

    val start = System.nanoTime()

    val stretchTree = bottomUpTree(0, stretchDepth)
    val stretchCheck = stretchTree.check()

    val longLivedTree = bottomUpTree(0, maxDepth)

    var totalChecks = 0
    var depth = minDepth
    while (depth <= maxDepth) {
        val iterations = 1 shl (maxDepth - depth + minDepth)
        var check = 0
        for (i in 1..iterations) {
            val t1 = bottomUpTree(i, depth)
            val t2 = bottomUpTree(-i, depth)
            check += t1.check() + t2.check()
        }
        totalChecks += check
        depth += 2
    }

    val longLivedCheck = longLivedTree.check()
    val end = System.nanoTime()

    val elapsedMs = (end - start) / 1_000_000.0
    println("stretch tree depth $stretchDepth check: $stretchCheck")
    println("long lived tree depth $maxDepth check: $longLivedCheck")
    println("total checks: $totalChecks")
    println("Binary trees (depth $maxDepth) completed in $elapsedMs ms")
}
