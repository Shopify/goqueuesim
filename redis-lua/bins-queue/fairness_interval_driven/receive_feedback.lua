-- important to enforce command determinism (e.g. avoid passing in time as param) in Redis versions <=5 --
-- https://redis.io/commands/EVAL#replicating-commands-instead-of-scripts --
redis.replicate_commands()

local checkoutUtil = tonumber(ARGV[1])
local pollingUtil = tonumber(ARGV[2])
local msg = string.format(
  "called receive_feedback with: checkoutUtil=%.2f,pollingUtil=%.2f\n", checkoutUtil, pollingUtil
)
return { true, msg }
