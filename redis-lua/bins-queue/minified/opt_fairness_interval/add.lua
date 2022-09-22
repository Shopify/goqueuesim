-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local shop_throttle_key = KEYS[1]

local throttle_fields = redis.call("HMGET", shop_throttle_key, "latest_queue_bin", "size")
local latest_queue_bin = tonumber(throttle_fields[1]) or 0
local size = tonumber(throttle_fields[2]) or 0
local updated_bin = false
size = size + 1

-- routinely_update_latest_bin: (KEYS[3]==update_timer), (ARGV[1]==max_unfair_ms==update_ms < window_dur_ms) --
if redis.call('SET', KEYS[3], 1, 'PX', tonumber(ARGV[1]), 'NX') then
  latest_queue_bin = latest_queue_bin + 1
  updated_bin = true
end

-- incr bins count: (KEYS[2]==bins_count_dict) --
redis.call("HINCRBY", KEYS[2], latest_queue_bin, 1)
if updated_bin then
  redis.call("HMSET", shop_throttle_key, "latest_queue_bin", latest_queue_bin, "size", size)
else
 redis.call("HSET", shop_throttle_key, "size", size)
end

return{ true, "called add\n", latest_queue_bin, size }
