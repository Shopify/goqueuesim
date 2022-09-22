-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local len = ARGV[1] or 0
for i=len,1,-1 do redis.call('DEL', KEYS[i], 0) end

return { true, "called clear\n" }
