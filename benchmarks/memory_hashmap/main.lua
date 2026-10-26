local map = {}
local n = 50000

local t0 = os.clock()
for i = 1, n do
    map["user_" .. i] = i
end

local sum = 0
for i = 1, n do
    sum = sum + map["user_" .. i]
end
local t1 = os.clock()
local ms = (t1 - t0) * 1000

io.write(string.format("SUM=%d TIME_MS=%.4f\n", sum, ms))
