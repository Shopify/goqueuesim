-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local shop_throttle_key = KEYS[1]

local throttle_fields = redis.call("HMGET", shop_throttle_key, "latest_queue_bin", "working_bin_idx")
local latest_queue_bin = tonumber(throttle_fields[1]) or 0
local working_bin = tonumber(throttle_fields[2]) or -1

-- (working_bin == -1) || (working_bin >= latest_queue_bin) => (working_bin_update_ms = ARGV[1] = max_unfair_ms) --
local working_bin_update_ms = tonumber(ARGV[1])
if working_bin >= 0 and working_bin < latest_queue_bin then
  -- (KEYS[2]==bins_counts_dict), (ARGV[2]==window_dur_ms), (ARGV[3]==max_checkouts_per_window) --
  local bin_sz_to_cpw_ratio = tonumber(redis.call("HGET", KEYS[2], working_bin)) / tonumber(ARGV[3])
  working_bin_update_ms = math.max(working_bin_update_ms, math.floor(bin_sz_to_cpw_ratio * tonumber(ARGV[2])))
end

-- routinely_update_working_bin: (KEYS[3]==working_bin_timer) --
if redis.call('SET', KEYS[3], 1, 'PX', working_bin_update_ms, 'NX') then
  if working_bin < latest_queue_bin then
    -- note: at most, working_bin is the only hash field updated by this algorithm on calling poll --
    working_bin = redis.call('HINCRBY', shop_throttle_key, "working_bin_idx", 1)
  end
end

-- reject unless (ARGV[4]==client_bin_idx) <= working_bin
if tonumber(ARGV[4]) > working_bin then
  -- another cool thing: could advertise redis.call('PTTL', working_bin_update_timer) as client's advised POLL_AFTER --
  return { true, "poll rejected\n", "reject" }
end

return { true, "poll passed\n", "pass" }
