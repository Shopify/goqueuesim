-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

-- scope-namespaced key strings --
local latest_queue_bin_key = KEYS[1]
local bin_counts_dict_key = KEYS[2]
local working_bin_idx_key = KEYS[3]
local working_bin_update_timer_key = KEYS[4]

-- constant config args --
local max_unfair_ms = tonumber(ARGV[1])
local window_dur_ms = tonumber(ARGV[2])
local max_checkouts_per_window = tonumber(ARGV[3])

-- variable input args --
local client_bin_idx = tonumber(ARGV[4])
local client_queue_pos = tonumber(ARGV[5])

-- * snippet starting here could be moved to run only on shared cache miss / expiry of working_bin_update_timer --

local max_considered_bin_idx = tonumber(redis.call('GET', working_bin_idx_key))
local latest_queue_bin = tonumber(redis.call('GET', latest_queue_bin_key))

-- (working_bin == nil) || (working_bin >= latest_queue_bin) => (working_bin_update_ms = max_unfair_ms) --
local working_bin_update_ms = max_unfair_ms

if max_considered_bin_idx == nil then
  max_considered_bin_idx = -1
elseif max_considered_bin_idx < latest_queue_bin then
  local bin_sz_to_cpw_ratio = redis.call("HGET", bin_counts_dict_key, max_considered_bin_idx) / max_checkouts_per_window
  working_bin_update_ms = math.max(max_unfair_ms, math.floor(bin_sz_to_cpw_ratio * window_dur_ms))
end

if redis.call('SET', working_bin_update_timer_key, 1, 'PX', working_bin_update_ms, 'NX') then
  if max_considered_bin_idx < latest_queue_bin then
    max_considered_bin_idx = redis.call('INCR', working_bin_idx_key)
  end
end

-- * snippet ending here could be moved to run only on shared cache miss / expiry of working_bin_update_timer --

if client_bin_idx > max_considered_bin_idx then
  -- another cool thing: could advertise redis.call('PTTL', working_bin_update_timer) as client's advised POLL_AFTER --
  return { true, "poll rejected\n", "reject" }
end

local msg = string.format(
  "passed poll with: binIdx=%d,queuePos=%d\n", client_bin_idx, client_queue_pos
)
return { true, msg, "pass" }
