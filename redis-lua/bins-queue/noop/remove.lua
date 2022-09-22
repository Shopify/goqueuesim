-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local binIdx = tonumber(ARGV[1])
local queuePos = tonumber(ARGV[2])
local msg = string.format(
  "called remove with: binIdx=%d,queuePos=%d\n", binIdx, queuePos
)
return { true, msg }
