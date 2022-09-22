-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local throttle_fields = redis.call("HMGET", KEYS[1], "size")
local size = tonumber(throttle_fields[1]) or 0

return { true, "called size\n", size }
