use std::time::Instant;

struct Node {
    left: Option<Box<Node>>,
    right: Option<Box<Node>>,
    item: i32,
}

fn bottom_up_tree(item: i32, depth: i32) -> Box<Node> {
    if depth > 0 {
        Box::new(Node {
            left: Some(bottom_up_tree(2 * item - 1, depth - 1)),
            right: Some(bottom_up_tree(2 * item, depth - 1)),
            item,
        })
    } else {
        Box::new(Node {
            left: None,
            right: None,
            item,
        })
    }
}

fn item_check(tree: &Node) -> i32 {
    match (&tree.left, &tree.right) {
        (Some(l), Some(r)) => tree.item + item_check(l) - item_check(r),
        _ => tree.item,
    }
}

fn main() {
    let max_depth = 14;
    let min_depth = 4;
    let stretch_depth = max_depth + 1;

    let start = Instant::now();

    let stretch_tree = bottom_up_tree(0, stretch_depth);
    let stretch_check = item_check(&stretch_tree);
    drop(stretch_tree);

    let long_lived_tree = bottom_up_tree(0, max_depth);

    let mut total_checks = 0;
    let mut depth = min_depth;
    while depth <= max_depth {
        let iterations = 1 << (max_depth - depth + min_depth);
        let mut check = 0;
        for i in 1..=iterations {
            let t1 = bottom_up_tree(i, depth);
            let t2 = bottom_up_tree(-i, depth);
            check += item_check(&t1) + item_check(&t2);
        }
        total_checks += check;
        depth += 2;
    }

    let long_lived_check = item_check(&long_lived_tree);
    drop(long_lived_tree);

    let elapsed_ms = start.elapsed().as_secs_f64() * 1000.0;

    println!("stretch tree depth {} check: {}", stretch_depth, stretch_check);
    println!("long lived tree depth {} check: {}", max_depth, long_lived_check);
    println!("total checks: {}", total_checks);
    println!("Binary trees (depth {}) completed in {:.4} ms", max_depth, elapsed_ms);
}
