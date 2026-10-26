local function make_tree(depth)
    if depth <= 0 then
        return {nil, nil}
    end
    return {make_tree(depth - 1), make_tree(depth - 1)}
end

local function check_tree(node)
    if node[1] == nil then
        return 1
    end
    return 1 + check_tree(node[1]) + check_tree(node[2])
end

local max_depth = 14
local min_depth = 4
local stretch_depth = max_depth + 1

local t0 = os.clock()

local stretch_tree = make_tree(stretch_depth)
local stretch_check = check_tree(stretch_tree)

local long_lived_tree = make_tree(max_depth)

local iterations = 1
for d = max_depth, min_depth, -2 do
    iterations = iterations * 4
end

local total_checks = 0
for depth = min_depth, max_depth, 2 do
    local iters = 2 ^ (max_depth - depth + min_depth)
    local check = 0
    for i = 1, iters do
        local a = make_tree(depth)
        check = check + check_tree(a)
    end
    total_checks = total_checks + check
end

local long_lived_check = check_tree(long_lived_tree)
local t1 = os.clock()
local ms = (t1 - t0) * 1000

io.write(string.format("CHECKSUM=%d TIME_MS=%.4f\n", total_checks, ms))
