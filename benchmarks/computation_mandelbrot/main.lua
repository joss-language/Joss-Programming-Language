local width = 500
local height = 500
local max_iter = 50
local x_min, x_max = -2.0, 0.5
local y_min, y_max = -1.25, 1.25

local t0 = os.clock()
local total = 0
for py = 0, height - 1 do
    local y0 = y_min + (py / height) * (y_max - y_min)
    for px = 0, width - 1 do
        local x0 = x_min + (px / width) * (x_max - x_min)
        local x, y = 0.0, 0.0
        local iter = 0
        while x * x + y * y <= 4.0 and iter < max_iter do
            local xtemp = x * x - y * y + x0
            y = 2.0 * x * y + y0
            x = xtemp
            iter = iter + 1
        end
        total = total + iter
    end
end
local t1 = os.clock()
local ms = (t1 - t0) * 1000

io.write(string.format("CHECKSUM=%d TIME_MS=%.4f\n", total, ms))
