-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

for i=ARGV[1],1,-1 do redis.call('DEL', KEYS[i]) end

return { true, "called clear\n" }
