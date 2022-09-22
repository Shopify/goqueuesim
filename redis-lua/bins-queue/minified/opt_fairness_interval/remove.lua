-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

-- KEYS = {shop_throttle_dict, bin_counts_dict}, ARGV = {bin_idx}
redis.call("HINCRBY", KEYS[1], "size", -1)
redis.call("HINCRBY", KEYS[2], ARGV[1], -1)

return { true, "called remove\n" }
