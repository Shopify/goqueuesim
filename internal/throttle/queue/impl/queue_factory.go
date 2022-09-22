package impl

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/Shopify/goqueuesim/internal/common"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/throttle/queue"
	"github.com/Shopify/goqueuesim/internal/throttle/queue/impl/redis_queue"

	"github.com/go-redis/redis/v7"
)

func MakeNoopQueue() queue.Queue {
	return &NoopQueue{}
}

// Returns SortedSetQueue backed by Redis Sorted Sets with fixed windowSize.
func MakeSortedSetQueue(
	redisClient *redis.Client,
	windowSize int64,
) queue.Queue {
	ssq := &redis_queue.SortedSetQueue{
		RedisClient:   redisClient,
		MinWindowSize: windowSize,
		WindowSize:    windowSize,
	}
	metrics.Gauge("ssq.window_size", float64(ssq.WindowSize), nil)
	ssq.Clear()
	return ssq
}

func MakeCappedBinsQueue(
	windowSize int64,
) queue.Queue {
	metrics.Gauge("cbq.working_bin", 1, nil)
	return &CappedBinsQueue{
		bins:               make(map[int64]map[int]bool),
		curWindowDequeues:  make(map[int64]int64),
		binSize:            int64(math.Ceil(float64(windowSize) * 1.50)),
		queueBin:           1,
		workingBin:         1,
		totalQueuedClients: 0,
	}
}

func MakePollDrivenCappedBinsQueue(
	startSignalWaitGroup *sync.WaitGroup,
	maxCheckoutsPerWindow int64,
	windowDur time.Duration,
	maxTargetPollingUtil float64,
	utilUpdateInterval time.Duration,
	workingBinUpdateInterval time.Duration,
	latestPollingUtilWeight float64,
) queue.Queue {
	cps := maxCheckoutsPerWindow / int64(windowDur.Seconds())
	sbq := &PollDrivenCappedBinsQueue{
		startSignalWaitGroup: startSignalWaitGroup,
		lock:                 &sync.Mutex{},

		binSize:                  int(cps),
		checkoutsPerSecond:       int(cps),
		maxTargetPollingUtil:     maxTargetPollingUtil,
		utilUpdateInterval:       utilUpdateInterval,
		workingBinUpdateInterval: workingBinUpdateInterval,
		latestPollingUtilWeight:  latestPollingUtilWeight,

		pollsPerSecTracker: &pollsPerSecondTracker{lock: &sync.Mutex{}},
		workingBin:         0,
	}
	go sbq.RoutinelyUpdatePollingUtil()
	return sbq
}

func MakeIntervalBinsQueue(
	ctx context.Context,
	startSignalWaitGroup *sync.WaitGroup,
	windowDur time.Duration,
	maxCheckoutsPerWindow int64,
	maxUnfairMilliseconds time.Duration,
) queue.Queue {
	metrics.Gauge("ibq.max_considered_bin_idx", 1, nil)
	ibq := &IntervalBinsQueue{
		ctx:                      ctx,
		startSignalWaitGroup:     startSignalWaitGroup,
		consideredBinLastUpdated: time.Now(),
		windowDur:                windowDur,
		maxCheckoutsPerWindow:    maxCheckoutsPerWindow,
		maxUnfairMilliseconds:    maxUnfairMilliseconds,
		binCounts:                make(map[int64]int64),
		latestBinIdx:             0,
		maxConsideredBinIdx:      0,
		totalQueuedClients:       0,
	}
	go ibq.RoutinelyUpdateLatestBin()
	go ibq.RoutinelyMaximizeFairThroughput()
	return ibq
}

func MakeLuaDrivenBinsQueue(
	client *redis.Client,
	methodToLuaShaMap map[string]string,
	shopScopePrefix string,
	methodToShopScopePrefixedKeysMap map[string][]string,
	methodToClientScopeNonPrefixedKeysMap map[string][]string,
	luaMethodToConstantArgsMap map[string][]string,
	keysBuilder redis_queue.KeysPreprocessor,
	argsBuilder redis_queue.ArgsPreprocessor,
	postprocessor redis_queue.LuaResultPostprocessor,
	globalInventoryCounter *common.AtomicCounter,
) queue.Queue {
	ldbq := &redis_queue.LuaDrivenQueue{
		RedisClient:                           client,
		MethodToLuaShaMap:                     methodToLuaShaMap,
		ShopScopePrefix:                       shopScopePrefix,
		MethodToShopScopePrefixedKeysMap:      methodToShopScopePrefixedKeysMap,
		MethodToClientScopeNonPrefixedKeysMap: methodToClientScopeNonPrefixedKeysMap,
		MethodToConstantArgsMap:               luaMethodToConstantArgsMap,
		KeysBuilder:                           keysBuilder,
		ArgsBuilder:                           argsBuilder,
		Postprocessor:                         postprocessor,
		GlobalInventoryCounter:                globalInventoryCounter,
	}
	return ldbq
}
