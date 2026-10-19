public class Main {
    static class Node {
        Node left;
        Node right;
        int item;

        Node(Node left, Node right, int item) {
            this.left = left;
            this.right = right;
            this.item = item;
        }

        int check() {
            if (left == null) return item;
            return item + left.check() - right.check();
        }
    }

    static Node bottomUpTree(int item, int depth) {
        if (depth > 0) {
            return new Node(
                bottomUpTree(2 * item - 1, depth - 1),
                bottomUpTree(2 * item, depth - 1),
                item
            );
        }
        return new Node(null, null, item);
    }

    public static void main(String[] args) {
        int maxDepth = 14;
        int minDepth = 4;
        int stretchDepth = maxDepth + 1;

        long start = System.nanoTime();

        Node stretchTree = bottomUpTree(0, stretchDepth);
        int stretchCheck = stretchTree.check();

        Node longLivedTree = bottomUpTree(0, maxDepth);

        int totalChecks = 0;
        for (int depth = minDepth; depth <= maxDepth; depth += 2) {
            int iterations = 1 << (maxDepth - depth + minDepth);
            int check = 0;
            for (int i = 1; i <= iterations; i++) {
                Node t1 = bottomUpTree(i, depth);
                Node t2 = bottomUpTree(-i, depth);
                check += t1.check() + t2.check();
            }
            totalChecks += check;
        }

        int longLivedCheck = longLivedTree.check();
        long end = System.nanoTime();

        double elapsedMs = (end - start) / 1_000_000.0;
        System.out.printf("stretch tree depth %d check: %d\n", stretchDepth, stretchCheck);
        System.out.printf("long lived tree depth %d check: %d\n", maxDepth, longLivedCheck);
        System.out.printf("total checks: %d\n", totalChecks);
        System.out.printf("Binary trees (depth %d) completed in %.4f ms\n", maxDepth, elapsedMs);
    }
}
