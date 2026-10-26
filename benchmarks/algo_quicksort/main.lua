local function partition(arr, low, high)
    local pivot = arr[high]
    local i = low - 1
    for j = low, high - 1 do
        if arr[j] <= pivot then
            i = i + 1
            arr[i], arr[j] = arr[j], arr[i]
        end
    end
    arr[i + 1], arr[high] = arr[high], arr[i + 1]
    return i + 1
end

local function quicksort(arr, low, high)
    while low < high do
        local pi = partition(arr, low, high)
        quicksort(arr, low, pi - 1)
        low = pi + 1
    end
end

local N = tonumber(arg[1]) or 50000
local arr = {}
local state = 123456789
for i = 1, N do
    state = (state * 1664525 + 1013904223) % 2147483648
    arr[i] = state
end

local t0 = os.clock()
quicksort(arr, 1, N)
local t1 = os.clock()
local ms = (t1 - t0) * 1000

for i = 1, N - 1 do
    if arr[i] > arr[i + 1] then
        error("Not sorted!")
    end
end

io.write(string.format("Quicksort sorted %d elements in %.4f ms\n", N, ms))
