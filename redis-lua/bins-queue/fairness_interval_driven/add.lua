-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local total_clients_key = KEYS[1]
local latest_queue_bin_timer_key = KEYS[2]
local latest_queue_bin_key = KEYS[3]
local bin_counts_dict_key = KEYS[4]

-- to preserve fairness guarantees, we require (max_unfair_ms = latest_queue_bin_update_ms) < window_dur_ms
local max_unfair_ms = tonumber(ARGV[1])
local window_dur_ms = tonumber(ARGV[2])
local max_checkouts_per_window = tonumber(ARGV[3])

-- default unnecessary b/c (latest_queue_bin == nil) => (latest_queue_bin_timer == nil) => (latest_queue_bin -> 0) --
local latest_queue_bin = tonumber(redis.call('GET', latest_queue_bin_key))

if redis.call('SET', latest_queue_bin_timer_key, 1, 'PX', max_unfair_ms, 'NX') then
  latest_queue_bin = redis.call('INCR', latest_queue_bin_key)
end

redis.call("HINCRBY", bin_counts_dict_key, latest_queue_bin, 1)
local total_clients = redis.call("INCR", total_clients_key)

local rem_millisecs_to_poll = (total_clients / max_checkouts_per_window) * window_dur_ms

return{ true, "called add\n", latest_queue_bin, total_clients, rem_millisecs_to_poll }
