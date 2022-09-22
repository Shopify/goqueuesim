-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local total_clients_key = KEYS[1]
local bin_counts_dict_key = KEYS[2]

local bin_idx = tonumber(ARGV[1])

-- warning: statement below can decrement bin count < 0 if called erroneously --
redis.call("HINCRBY", bin_counts_dict_key, bin_idx, -1)

redis.call("DECR", total_clients_key)

return { true, "called remove\n" }
