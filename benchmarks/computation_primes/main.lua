local ffi = require("ffi")

local function count_primes(limit)
    if limit < 2 then return 0 end
    local sieve = ffi.new("uint8_t[?]", limit + 1)
    for i = 0, limit do sieve[i] = 1 end
    sieve[0] = 0
    sieve[1] = 0
    local p = 2
    while p * p <= limit do
        if sieve[p] == 1 then
            local i = p * p
            while i <= limit do
                sieve[i] = 0
                i = i + p
            end
        end
        p = p + 1
    end
    local count = 0
    for i = 2, limit do
        if sieve[i] == 1 then count = count + 1 end
    end
    return count
end

local limit = tonumber(arg[1]) or 1000000
local t0 = os.clock()
local count = count_primes(limit)
local t1 = os.clock()
local ms = (t1 - t0) * 1000
io.write(string.format("LIMIT=%d COUNT=%d TIME_MS=%.4f\n", limit, count, ms))
