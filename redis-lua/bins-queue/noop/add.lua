-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local dummyBinIdx = 1
local dummyQueuePos = 2
local dummyNextPollMs = 5
return {true, "called add\n", dummyBinIdx, dummyQueuePos, dummyNextPollMs}
