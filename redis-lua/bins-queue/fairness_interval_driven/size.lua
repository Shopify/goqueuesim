-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local total_clients_key = KEYS[1]

local size = redis.call("GET", total_clients_key)
if not size then
  redis.call("SET", total_clients_key, 0)
  size = "0"
end

return { true, "called size\n", tonumber(size) }
