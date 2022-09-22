package redis_queue

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/common"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"

	"github.com/go-redis/redis/v7"
)

const (
	LuaUserBinKey = "luaUserBinId"
	LuaUserPosKey = "luaUserQueuePos"
)

type LuaDrivenQueue struct {
	RedisClient       *redis.Client
	MethodToLuaShaMap map[string]string

	ShopScopePrefix string

	MethodToShopScopePrefixedKeysMap      map[string][]string
	MethodToClientScopeNonPrefixedKeysMap map[string][]string

	MethodToConstantArgsMap map[string][]string

	// Slightly inconvenient...these two interfaces allow customizing keys (e.g. embedding time) dynamically
	KeysBuilder KeysPreprocessor
	ArgsBuilder ArgsPreprocessor

	Postprocessor LuaResultPostprocessor

	// TODO: enforce this in our Lua scripts to ensure atomicity via Redis.
	GlobalInventoryCounter *common.AtomicCounter

	// Only to track & print local state.
	totalPollsCount int64
}

func (ldq *LuaDrivenQueue) Add(c client.Client) {
	ldq.GlobalInventoryCounter.Lock()
	defer ldq.GlobalInventoryCounter.Unlock()
	remInventory, _ := ldq.GlobalInventoryCounter.AtomicRead()
	if remInventory <= 0 {
		return
	}
	methodKey := "add"
	defer metrics.BenchmarkMethod(time.Now(), "add", nil)

	redisLuaSha := ldq.MethodToLuaShaMap[methodKey]

	clientScopedKeys := ldq.withClientScopedKeys(methodKey, string(c.ID()))
	keys := ldq.KeysBuilder.PreprocessKeys(methodKey, clientScopedKeys)

	constantArgs := ldq.MethodToConstantArgsMap[methodKey]
	args := ldq.ArgsBuilder.PreprocessArgs(methodKey, constantArgs)

	resultTuple := ldq.redisLuaResultOrDie(redisLuaSha, keys, args)

	resultParams := PostprocessParams{ResultTuple: resultTuple, C: c}
	ldq.Postprocessor.PostprocessLuaResult(methodKey, resultParams)
}

func (ldq *LuaDrivenQueue) Remove(c client.Client) {
	ldq.GlobalInventoryCounter.Lock()
	defer ldq.GlobalInventoryCounter.Unlock()

	remInventory, _ := ldq.GlobalInventoryCounter.AtomicRead()
	if remInventory <= -1 {
		// Note: allow inventory=0 minimum to be set before removing last client.
		return
	}

	methodKey := "remove"
	defer metrics.BenchmarkMethod(time.Now(), methodKey, nil)

	redisLuaSha := ldq.MethodToLuaShaMap[methodKey]

	clientScopedKeys := ldq.withClientScopedKeys(methodKey, string(c.ID()))
	keys := ldq.KeysBuilder.PreprocessKeys(methodKey, clientScopedKeys)

	argsWithClientData := ldq.argsWithClientData(methodKey, c)
	args := ldq.ArgsBuilder.PreprocessArgs(methodKey, argsWithClientData)

	resultTuple := ldq.redisLuaResultOrDie(redisLuaSha, keys, args)

	resultParams := PostprocessParams{ResultTuple: resultTuple, C: c}
	ldq.Postprocessor.PostprocessLuaResult(methodKey, resultParams)
}

func (ldq *LuaDrivenQueue) IsCandidateToProceed(c client.Client) bool {
	ldq.totalPollsCount++
	ldq.GlobalInventoryCounter.Lock()
	defer ldq.GlobalInventoryCounter.Unlock()
	remInventory, _ := ldq.GlobalInventoryCounter.AtomicRead()
	if remInventory <= 0 {
		return false
	}

	methodKey := "poll"
	defer metrics.BenchmarkMethod(time.Now(), methodKey, nil)

	redisLuaSha := ldq.MethodToLuaShaMap[methodKey]

	clientScopedKeys := ldq.withClientScopedKeys(methodKey, string(c.ID()))
	keys := ldq.KeysBuilder.PreprocessKeys(methodKey, clientScopedKeys)

	argsWithClientData := ldq.argsWithClientData(methodKey, c)
	args := ldq.ArgsBuilder.PreprocessArgs(methodKey, argsWithClientData)

	resultTuple := ldq.redisLuaResultOrDie(redisLuaSha, keys, args)

	resultParams := PostprocessParams{ResultTuple: resultTuple, C: c}
	isCandidateToProceed := ldq.Postprocessor.PostprocessLuaResult(methodKey, resultParams).PollResult
	return isCandidateToProceed
}

func (ldq *LuaDrivenQueue) ReceiveTrackerFeedback(feedback tracker.Feedback) {
	methodKey := "receive_feedback"
	defer metrics.BenchmarkMethod(time.Now(), methodKey, nil)
	fmt.Printf(
		"checkoutUtil=%.2f pollingUtil=%.2f queuedClients=%d totalPolls=%d\n",
		feedback.CheckoutUtil, feedback.PollingUtil, ldq.Size(), ldq.totalPollsCount,
	)

	redisLuaSha := ldq.MethodToLuaShaMap[methodKey]

	shopScopedKeys := ldq.MethodToShopScopePrefixedKeysMap[methodKey]
	keys := ldq.KeysBuilder.PreprocessKeys(methodKey, shopScopedKeys)

	checkoutUtilStr := strconv.FormatFloat(feedback.CheckoutUtil, 'f', -1, 64)
	pollingUtilStr := strconv.FormatFloat(feedback.PollingUtil, 'f', -1, 64)
	feedbackArgs := append(ldq.MethodToConstantArgsMap[methodKey], checkoutUtilStr, pollingUtilStr)
	args := ldq.ArgsBuilder.PreprocessArgs(methodKey, feedbackArgs)

	resultTuple := ldq.redisLuaResultOrDie(redisLuaSha, keys, args)

	resultParams := PostprocessParams{ResultTuple: resultTuple, feedback: feedback}
	ldq.Postprocessor.PostprocessLuaResult(methodKey, resultParams)
}

func (ldq *LuaDrivenQueue) Size() int64 {
	methodKey := "size"
	defer metrics.BenchmarkMethod(time.Now(), methodKey, nil)

	redisLuaSha := ldq.MethodToLuaShaMap[methodKey]

	shopScopedKeys := ldq.MethodToShopScopePrefixedKeysMap[methodKey]
	keys := ldq.KeysBuilder.PreprocessKeys(methodKey, shopScopedKeys)

	constantArgs := ldq.MethodToConstantArgsMap[methodKey]
	args := ldq.ArgsBuilder.PreprocessArgs(methodKey, constantArgs)

	resultTuple := ldq.redisLuaResultOrDie(redisLuaSha, keys, args)

	resultParams := PostprocessParams{ResultTuple: resultTuple}
	totalClients := ldq.Postprocessor.PostprocessLuaResult(methodKey, resultParams).SizeResult

	return totalClients
}

func (ldq *LuaDrivenQueue) Clear() {
	ldq.GlobalInventoryCounter.Lock()
	defer ldq.GlobalInventoryCounter.Unlock()

	methodKey := "clear"
	defer metrics.BenchmarkMethod(time.Now(), methodKey, nil)

	redisLuaSha := ldq.MethodToLuaShaMap[methodKey]

	shopScopedKeys := ldq.MethodToShopScopePrefixedKeysMap[methodKey]
	keys := ldq.KeysBuilder.PreprocessKeys(methodKey, shopScopedKeys)

	constantArgs := ldq.MethodToConstantArgsMap[methodKey]
	args := ldq.ArgsBuilder.PreprocessArgs(methodKey, constantArgs)

	numKeysToClear := strconv.FormatInt(int64(len(keys)), 10)
	argsWithKeysCount := append(args, numKeysToClear)

	ldq.totalPollsCount = 0
	resultTuple := ldq.redisLuaResultOrDie(redisLuaSha, keys, argsWithKeysCount)

	resultParams := PostprocessParams{ResultTuple: resultTuple}
	ldq.Postprocessor.PostprocessLuaResult(methodKey, resultParams)

	//testVal := resultTuple[1].(string)
	//fmt.Printf("%s", testVal)
}

func (ldq *LuaDrivenQueue) redisLuaResultOrDie(luaSha1 string, keys []string, args []string) []interface{} {
	var result interface{}
	var err error
	if result, err = ldq.RedisClient.EvalSha(luaSha1, keys, args).Result(); err != nil {
		panic(err)
	}
	resultTuple := result.([]interface{})
	failed := resultTuple[0] == nil
	if failed {
		panic(resultTuple[1].(string))
	}
	return result.([]interface{})
}

func (ldq *LuaDrivenQueue) prependPrefix(prefix string, strList []string) []string {
	result := make([]string, len(strList))
	for i, str := range strList {
		result[i] = prefix + str
	}
	return result
}

func (ldq *LuaDrivenQueue) withClientScopedKeys(methodKey string, clientId string) []string {
	shopScopePrefixedKeys := ldq.MethodToShopScopePrefixedKeysMap[methodKey]

	clientScopePrefix := ldq.ShopScopePrefix + ":client_id:" + clientId + ":"
	clientScopeNonPrefixedKeys := ldq.MethodToClientScopeNonPrefixedKeysMap[methodKey]
	clientScopedKeys := ldq.prependPrefix(clientScopePrefix, clientScopeNonPrefixedKeys)

	return append(shopScopePrefixedKeys, clientScopedKeys...)
}
func (ldq *LuaDrivenQueue) getUserBinIdx(c client.Client) (string, bool) {
	if binIdxStr, ok := c.GetThrottleCookieVal(LuaUserBinKey); ok {
		return binIdxStr, true
	}
	return "NaN", false
}

func (ibq *LuaDrivenQueue) getUserQueuePos(c client.Client) (string, bool) {
	if queuePosStr, ok := c.GetThrottleCookieVal(LuaUserPosKey); ok {
		return queuePosStr, true
	}
	return "NaN", false
}

func (ldq *LuaDrivenQueue) argsWithClientData(methodName string, c client.Client) []string {
	args := ldq.MethodToConstantArgsMap[methodName]

	if binIdx, ok := ldq.getUserBinIdx(c); ok {
		args = append(args, binIdx)
	}
	if queuePos, ok := ldq.getUserQueuePos(c); ok {
		args = append(args, queuePos)
	}

	return args
}
