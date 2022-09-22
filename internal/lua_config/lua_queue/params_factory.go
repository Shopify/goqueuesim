package lua_queue

import (
	"strconv"

	"github.com/Shopify/goqueuesim/internal/lua_config/lua_queue/param_builders"
	"github.com/Shopify/goqueuesim/internal/lua_config/lua_queue/postprocessors"
	"github.com/Shopify/goqueuesim/internal/throttle/queue/impl/redis_queue"
)

func prefixedKeys(prefix string, keys []string) []string {
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = prefix + k
	}
	return result
}

var defaultLuaMethodMap = map[string][]string{
	"add":              {},
	"remove":           {},
	"poll":             {},
	"receive_feedback": {},
	"size":             {},
	"clear":            {},
}

var defaultLuaQueueParams = LuaQueueParams{
	ShopScopePrefix:                       "",
	LuaMethodToShopScopePrefixedKeys:      defaultLuaMethodMap,
	LuaMethodToClientScopeNonPrefixedKeys: defaultLuaMethodMap,
	LuaMethodToConstantArgsMap:            defaultLuaMethodMap,
	LuaMethodToShaMap:                     map[string]string{},
	KeysBuilder:                           &redis_queue.NoopLuaPreprocessor{},
	ArgsBuilder:                           &redis_queue.NoopLuaPreprocessor{},
	Postprocessor:                         &redis_queue.DefaultLuaResultPostprocessor{},
}

func MakeNoopQueueParams(luaMethodToShaMap map[string]string) *LuaQueueParams {
	dup := defaultLuaQueueParams
	dup.LuaMethodToShaMap = luaMethodToShaMap
	return &dup
}

func MakePollingFeedbackBinsQueueParams(
	constants LuaQueueConstants,
	luaMethodToShaMap map[string]string,
) *LuaQueueParams {
	shopScopeNonPrefixedKeys := []string{
		"latest_queue_bin", "latest_queue_bin_pos", "total_clients", "working_bin",
		"prev_poll_unix_second", "prev_unix_second_poll_count", "cur_second_poll_count",
		"poll_util_update_timer", "poll_util", "working_bin_update_timer",
	}
	shopScopePrefixedKeys := prefixedKeys(constants.ShopScopePrefix+":", shopScopeNonPrefixedKeys)

	cps := constants.MaxCheckoutsAllowedPerWindow / int64(constants.WindowDuration.Seconds())
	binSize := strconv.FormatInt(cps, 10)
	maxCps := strconv.FormatInt(cps, 10)
	maxPollingUtil := strconv.FormatFloat(constants.PollDrivenMaxTargetPollingUtil, 'f', -1, 64)
	pollingUtilUpdateMs := strconv.FormatInt(constants.PollDrivenUtilUpdateInterval.Milliseconds(), 10)
	workingBinUpdateMs := strconv.FormatInt(constants.PollDrivenWorkingBinUpdateInterval.Milliseconds(), 10)
	latestPollingUtilWeight := strconv.FormatFloat(constants.PollDrivenLatestPollingUtilWeight, 'f', -1, 64)
	constantArgs := []string{
		binSize, maxCps, maxPollingUtil,
		pollingUtilUpdateMs, workingBinUpdateMs, latestPollingUtilWeight,
	}

	luaMethodToShopScopePrefixedKeys := map[string][]string{
		"add":              shopScopePrefixedKeys[0:3],
		"remove":           shopScopePrefixedKeys[2:3],
		"poll":             shopScopePrefixedKeys[3:10],
		"receive_feedback": {},
		"size":             shopScopePrefixedKeys[2:3],
		"clear":            shopScopePrefixedKeys,
	}
	luaMethodToConstantArgsMap := map[string][]string{
		"add":              constantArgs[0:2],
		"remove":           {},
		"poll":             constantArgs[1:6],
		"receive_feedback": {},
		"size":             {},
		"clear":            {},
	}
	queueParams := &LuaQueueParams{
		LuaMethodToShopScopePrefixedKeys:      luaMethodToShopScopePrefixedKeys,
		LuaMethodToClientScopeNonPrefixedKeys: defaultLuaMethodMap,
		LuaMethodToConstantArgsMap:            luaMethodToConstantArgsMap,
		LuaMethodToShaMap:                     luaMethodToShaMap,
		KeysBuilder:                           &redis_queue.NoopLuaPreprocessor{},
		ArgsBuilder:                           &redis_queue.NoopLuaPreprocessor{},
		Postprocessor:                         &redis_queue.DefaultLuaResultPostprocessor{},
	}
	return queueParams
}

func MakeFairnessIntervalDrivenQueueParams(
	constants LuaQueueConstants,
	luaMethodToShaMap map[string]string,
) *LuaQueueParams {
	shopScopeNonPrefixedKeys := []string{
		"total_clients", "latest_queue_bin_update_timer", "latest_queue_bin",
		"bin_counts_dict", "working_bin", "working_bin_update_timer",
	}
	shopScopePrefixedKeys := prefixedKeys(constants.ShopScopePrefix+":", shopScopeNonPrefixedKeys)

	// TODO: parametrize maxUnfairMs in config.go constants
	maxUnfairMs := "2000"
	maxCpw := strconv.FormatInt(constants.MaxCheckoutsAllowedPerWindow, 10)
	windowMs := strconv.FormatInt(constants.WindowDuration.Milliseconds(), 10)
	constantArgs := []string{maxUnfairMs, windowMs, maxCpw}

	luaMethodToShopScopePrefixedKeys := map[string][]string{
		"add":              shopScopePrefixedKeys[0:4],
		"remove":           {shopScopePrefixedKeys[0], shopScopePrefixedKeys[3]},
		"poll":             shopScopePrefixedKeys[2:6],
		"receive_feedback": {},
		"size":             shopScopePrefixedKeys[0:1],
		"clear":            shopScopePrefixedKeys,
	}
	luaMethodToConstantArgsMap := map[string][]string{
		"add":              constantArgs,
		"remove":           {},
		"poll":             constantArgs,
		"receive_feedback": {},
		"size":             {},
		"clear":            {},
	}
	queueParams := &LuaQueueParams{
		LuaMethodToShopScopePrefixedKeys:      luaMethodToShopScopePrefixedKeys,
		LuaMethodToClientScopeNonPrefixedKeys: defaultLuaMethodMap,
		LuaMethodToConstantArgsMap:            luaMethodToConstantArgsMap,
		LuaMethodToShaMap:                     luaMethodToShaMap,
		KeysBuilder:                           &redis_queue.NoopLuaPreprocessor{},
		ArgsBuilder:                           &redis_queue.NoopLuaPreprocessor{},
		Postprocessor:                         &redis_queue.DefaultLuaResultPostprocessor{},
	}
	return queueParams
}

func MakeOptNotimersPollDrivenParams(
	constants LuaQueueConstants,
	luaMethodToShaMap map[string]string,
) *LuaQueueParams {
	shopScopeNonPrefixedKeys := []string{"checkout_throttle_dict"}
	shopScopePrefixedKeys := prefixedKeys(constants.ShopScopePrefix+":", shopScopeNonPrefixedKeys)

	cps := constants.MaxCheckoutsAllowedPerWindow / int64(constants.WindowDuration.Seconds())
	binSize := strconv.FormatInt(cps, 10)
	maxCps := strconv.FormatInt(cps, 10)
	maxPollingUtil := strconv.FormatFloat(constants.PollDrivenMaxTargetPollingUtil, 'f', -1, 64)
	pollingUtilUpdateMs := strconv.FormatInt(constants.PollDrivenUtilUpdateInterval.Milliseconds(), 10)
	workingBinUpdateMs := strconv.FormatInt(constants.PollDrivenWorkingBinUpdateInterval.Milliseconds(), 10)
	latestPollingUtilWeight := strconv.FormatFloat(constants.PollDrivenLatestPollingUtilWeight, 'f', -1, 64)
	constantArgs := []string{
		binSize, maxCps, maxPollingUtil, pollingUtilUpdateMs, workingBinUpdateMs, latestPollingUtilWeight,
	}

	luaMethodToShopScopePrefixedKeys := map[string][]string{
		"add":              shopScopePrefixedKeys,
		"remove":           shopScopePrefixedKeys,
		"poll":             shopScopePrefixedKeys,
		"receive_feedback": {},
		"size":             shopScopePrefixedKeys,
		"clear":            shopScopePrefixedKeys,
	}
	luaMethodToConstantArgsMap := map[string][]string{
		"add":              constantArgs[0:2],
		"remove":           {},
		"poll":             constantArgs[1:6],
		"receive_feedback": {},
		"size":             {},
		"clear":            {},
	}
	queueParams := &LuaQueueParams{
		LuaMethodToShopScopePrefixedKeys:      luaMethodToShopScopePrefixedKeys,
		LuaMethodToClientScopeNonPrefixedKeys: defaultLuaMethodMap,
		LuaMethodToConstantArgsMap:            luaMethodToConstantArgsMap,
		LuaMethodToShaMap:                     luaMethodToShaMap,
		KeysBuilder:                           &param_builders.OptPollDrivenNotimersLuaKeysBuilder{},
		ArgsBuilder:                           &param_builders.OptPollDrivenNotimersLuaArgsBuilder{},
		Postprocessor:                         &postprocessors.MinimalistPostprocessor{cps},
	}
	return queueParams
}

func MakeOptKTimersPollDrivenParams(
	constants LuaQueueConstants,
	luaMethodToShaMap map[string]string,
) *LuaQueueParams {
	shopScopeNonPrefixedKeys := []string{
		"checkout_throttle_dict", "poll_util_update_timer", "working_bin_update_timer",
	}
	shopScopePrefixedKeys := prefixedKeys(constants.ShopScopePrefix+":", shopScopeNonPrefixedKeys)

	cps := constants.MaxCheckoutsAllowedPerWindow / int64(constants.WindowDuration.Seconds())
	binSize := strconv.FormatInt(cps, 10)
	maxCps := strconv.FormatInt(cps, 10)
	maxPollingUtil := strconv.FormatFloat(constants.PollDrivenMaxTargetPollingUtil, 'f', -1, 64)
	pollingUtilUpdateMs := strconv.FormatInt(constants.PollDrivenUtilUpdateInterval.Milliseconds(), 10)
	workingBinUpdateMs := strconv.FormatInt(constants.PollDrivenWorkingBinUpdateInterval.Milliseconds(), 10)
	latestPollingUtilWeight := strconv.FormatFloat(constants.PollDrivenLatestPollingUtilWeight, 'f', -1, 64)
	constantArgs := []string{
		binSize, maxCps, maxPollingUtil, pollingUtilUpdateMs, workingBinUpdateMs, latestPollingUtilWeight,
	}

	luaMethodToShopScopePrefixedKeys := map[string][]string{
		"add":              shopScopePrefixedKeys[0:1],
		"remove":           shopScopePrefixedKeys[0:1],
		"poll":             shopScopePrefixedKeys,
		"receive_feedback": {},
		"size":             shopScopePrefixedKeys[0:1],
		"clear":            shopScopePrefixedKeys,
	}
	luaMethodToConstantArgsMap := map[string][]string{
		"add":              constantArgs[0:2],
		"remove":           {},
		"poll":             constantArgs[1:6],
		"receive_feedback": {},
		"size":             {},
		"clear":            {},
	}
	queueParams := &LuaQueueParams{
		LuaMethodToShopScopePrefixedKeys:      luaMethodToShopScopePrefixedKeys,
		LuaMethodToClientScopeNonPrefixedKeys: defaultLuaMethodMap,
		LuaMethodToConstantArgsMap:            luaMethodToConstantArgsMap,
		LuaMethodToShaMap:                     luaMethodToShaMap,
		KeysBuilder:                           &param_builders.OptPollDrivenKTimersLuaKeysBuilder{},
		ArgsBuilder:                           &param_builders.OptPollDrivenKTimersLuaArgsBuilder{},
		Postprocessor:                         &postprocessors.MinimalistPostprocessor{cps},
	}
	return queueParams
}

func MakeOptFairnessIntervalParams(
	constants LuaQueueConstants,
	luaMethodToShaMap map[string]string,
) *LuaQueueParams {
	shopScopeNonPrefixedKeys := []string{
		"checkout_throttle_dict", "bin_counts", "latest_bin_update_timer", "working_bin_update_timer",
	}
	shopScopePrefixedKeys := prefixedKeys(constants.ShopScopePrefix+":", shopScopeNonPrefixedKeys)

	maxUnfairMs := "2000"
	maxCpw := strconv.FormatInt(constants.MaxCheckoutsAllowedPerWindow, 10)
	windowMs := strconv.FormatInt(constants.WindowDuration.Milliseconds(), 10)
	maxCps := float64(constants.MaxCheckoutsAllowedPerWindow) / constants.WindowDuration.Seconds()
	constantArgs := []string{maxUnfairMs, windowMs, maxCpw}

	luaMethodToShopScopePrefixedKeys := map[string][]string{
		"add":              shopScopePrefixedKeys[0:3],
		"remove":           shopScopePrefixedKeys[0:2],
		"poll":             append(shopScopePrefixedKeys[0:2], shopScopePrefixedKeys[3]),
		"receive_feedback": {},
		"size":             shopScopePrefixedKeys[0:1],
		"clear":            shopScopePrefixedKeys,
	}
	luaMethodToConstantArgsMap := map[string][]string{
		"add":              constantArgs,
		"remove":           {},
		"poll":             constantArgs,
		"receive_feedback": {},
		"size":             {},
		"clear":            {},
	}
	queueParams := &LuaQueueParams{
		LuaMethodToShopScopePrefixedKeys:      luaMethodToShopScopePrefixedKeys,
		LuaMethodToClientScopeNonPrefixedKeys: defaultLuaMethodMap,
		LuaMethodToConstantArgsMap:            luaMethodToConstantArgsMap,
		LuaMethodToShaMap:                     luaMethodToShaMap,
		KeysBuilder:                           &redis_queue.NoopLuaPreprocessor{},
		ArgsBuilder:                           &redis_queue.NoopLuaPreprocessor{},
		Postprocessor:                         &postprocessors.MinimalistPostprocessor{MaxCps: int64(maxCps)},
	}
	return queueParams
}

func MakeOptPollDrivenNobinsParams(
	constants LuaQueueConstants,
	luaMethodToShaMap map[string]string,
) *LuaQueueParams {
	shopScopeNonPrefixedKeys := []string{"checkout_throttle_dict"}
	shopScopePrefixedKeys := prefixedKeys(constants.ShopScopePrefix + ":", shopScopeNonPrefixedKeys)

	cps := constants.MaxCheckoutsAllowedPerWindow / int64(constants.WindowDuration.Seconds())
	maxCps := strconv.FormatInt(cps, 10)
	maxPollingUtil := strconv.FormatFloat(constants.PollDrivenMaxTargetPollingUtil, 'f', -1, 64)
	pollingUtilUpdateMs := strconv.FormatInt(constants.PollDrivenUtilUpdateInterval.Milliseconds(), 10)
	workingBinUpdateMs := strconv.FormatInt(constants.PollDrivenWorkingBinUpdateInterval.Milliseconds(), 10)
	latestPollingUtilWeight := strconv.FormatFloat(constants.PollDrivenLatestPollingUtilWeight, 'f', -1, 64)
	constantArgs := []string{
		maxCps, maxPollingUtil, pollingUtilUpdateMs, workingBinUpdateMs, latestPollingUtilWeight,
	}

	luaMethodToShopScopePrefixedKeys := map[string][]string{
		"add":              shopScopePrefixedKeys,
		"remove":           shopScopePrefixedKeys,
		"poll":             shopScopePrefixedKeys,
		"receive_feedback": {},
		"size":             shopScopePrefixedKeys,
		"clear":            shopScopePrefixedKeys,
	}
	luaMethodToConstantArgsMap := map[string][]string{
		"add":              {},
		"remove":           {},
		"poll":             constantArgs,
		"receive_feedback": {},
		"size":             {},
		"clear":            {},
	}
	argsBuilder := &param_builders.OptPollDrivenNobinsLuaArgsBuilder{DefaultWorkingPosIncr: maxCps}
	queueParams := &LuaQueueParams{
		LuaMethodToShopScopePrefixedKeys:      luaMethodToShopScopePrefixedKeys,
		LuaMethodToClientScopeNonPrefixedKeys: defaultLuaMethodMap,
		LuaMethodToConstantArgsMap:            luaMethodToConstantArgsMap,
		LuaMethodToShaMap:                     luaMethodToShaMap,
		KeysBuilder:                           &param_builders.OptPollDrivenNobinsLuaKeysBuilder{},
		ArgsBuilder:                           argsBuilder ,
		Postprocessor:                         &postprocessors.NobinsPostprocessor{cps},
	}
	return queueParams
}
