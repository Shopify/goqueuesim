package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/goqueuesim/internal/client"
	clientfactory "github.com/Shopify/goqueuesim/internal/client/impl"
	"github.com/Shopify/goqueuesim/internal/common"
	"github.com/Shopify/goqueuesim/internal/lua_config/lua_queue"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/network_mock"
	"github.com/Shopify/goqueuesim/internal/simulator"
	"github.com/Shopify/goqueuesim/internal/throttle"
	"github.com/Shopify/goqueuesim/internal/throttle/queue"
	queuefactory "github.com/Shopify/goqueuesim/internal/throttle/queue/impl"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"
	trackerfactory "github.com/Shopify/goqueuesim/internal/throttle/tracker/impl"

	"github.com/go-redis/redis/v7"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	logLevel = "disabled"

	dashboardUrl = "https://datadoghq.com/dashboard/path/to/dashboard"

	// Json file configuring distribution of Checkout clients for simulation.
	clientDistributionJsonPath = "config/simulation/client_distributions/plausible_best_case_scenario.json"

	// Type of UserQueue strategy to execute.
	queueType = "capped_bins_queue"

	// Dir with action scripts backing lua driven redis queue.
	luaQueueDirPath = "redis-lua/bins-queue/noop"

	// Type of RateTracker strategy to execute.
	trackerType = "fixed_window"

	// Type of ClientRepo backing our simulator.
	clientRepoType = "simple_client_repo"

	// Time window enacted by rate tracker.
	windowDuration = 2 * time.Second

	// Max checkouts allowed per rate tracker window.
	maxCheckoutsAllowedPerWindow = 200

	// Threshold >= to which we deem intolerable unfairness.
	maxUnfairnessToleranceSeconds = 15.0

	// The maximum number of queued requests or responses.
	maxNetworkIOBacklogSize = 2000

	// Number of Checkout clients generated.
	targetNumClients = 2200

	// Max available inventory (simulation terminates when depleted).
	inventoryStockTotal = 9200

	// Flag to enforce random client order (even on special case files).
	forceRandomClientOrder = false

	// Number of server workers generated.
	numServerWorkers = 2000

	// Redis called to back user queue.
	redisAddr = "localhost:6379"

	// Specific Model parameters follow:

	// Threshold pct <= to which we deem low CPM utilization.
	lowCheckoutUtilMaxThresholdPct = 0.25

	// Number of observed low util windows skipped before notifying queue.
	maxSkpdLowCheckoutUtilCount = 15

	// Prefix used only to demo shop-scoped namespacing where relevant to API.
	shopScopePrefixDemo = "shop_id:1"

	// PollDrivenCappedBinsQueue params:
	polldrivenMaxTargetPollingUtil     = 2.5
	polldrivenUtilUpdateInterval       = 100 * time.Millisecond
	polldrivenWorkingBinUpdateInterval = 1 * time.Second
	polldrivenLatestPollingUtilWeight  = 0.2
)

type Simulator = simulator.SimulationDriver
type CheckoutThrottleDriver = throttle.CheckoutThrottleDriver
type ClientConfig = client.ClientConfig
type NetworkParams = clientfactory.NetworkParams

// Flag to toggle whether we default (barring special case files) to random client order.
var shouldRandomizeClientOrder = true

func validateParams(skippedLowUtilWindows int) {
	// FIXME: should validate other params eventually
	if skippedLowUtilWindows < 0 {
		panic(fmt.Errorf("maxSkpdLowCheckoutUtilCount should be >= 0 but found %d", skippedLowUtilWindows))
	}
}

func setLogging(logLevel string) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	switch logLevel {
	case "disabled":
		zerolog.SetGlobalLevel(zerolog.Disabled)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		panic(fmt.Errorf("log level must be one of: {disabled, info}"))
	}
}

func isNonrandomClientsConfig(distributionFilename string) bool {
	switch distributionFilename {
	case
		"plausible_very_unfair",
		"polldriven_throughput_edge_case_1",
		"polldriven_throughput_edge_case_2":
		return true
	}
	return false
}

func configureExperiment() {
	distributionStrSlice := strings.Split(clientDistributionJsonPath, "/")
	distributionFilename := strings.TrimSuffix(distributionStrSlice[len(distributionStrSlice)-1], ".json")
	if isNonrandomClientsConfig(distributionFilename) {
		shouldRandomizeClientOrder = false
	}
	if forceRandomClientOrder {
		shouldRandomizeClientOrder = true
	}

	luaStrSlice := strings.Split(luaQueueDirPath, "/")
	luaDirName := luaStrSlice[len(luaStrSlice)-1]

	clientDistributionTag := fmt.Sprintf("client_distribution:%s", distributionFilename)
	luaDirTag := fmt.Sprintf("lua_queue_dir:%s", luaDirName)
	queueTag := fmt.Sprintf("queue_type:%s", queueType)
	drainRateTrackerTag := fmt.Sprintf("drain_rate_tracker_type:%s", trackerType)
	clientRepoTag := fmt.Sprintf("client_repo_type:%s", clientRepoType)
	windowDurSecondsTag := fmt.Sprintf("window_duration_seconds:%.2f", windowDuration.Seconds())
	maxCheckoutsPerWindowTag := fmt.Sprintf("max_checkouts_per_window:%d", maxCheckoutsAllowedPerWindow)
	randomizedClientsTag := fmt.Sprintf("client_order_randomized:%t", shouldRandomizeClientOrder)
	numClientsTag := fmt.Sprintf("num_clients:%d", targetNumClients)
	numServerWorkersTag := fmt.Sprintf("num_server_workers:%d", numServerWorkers)
	unfairnessToleranceTag := fmt.Sprintf("unfairness_tolerance_seconds:%.2f", maxUnfairnessToleranceSeconds)
	tags := []string{
		clientDistributionTag, luaDirTag, queueTag, drainRateTrackerTag, clientRepoTag,
		windowDurSecondsTag, maxCheckoutsPerWindowTag, randomizedClientsTag,
		numClientsTag, numServerWorkersTag, unfairnessToleranceTag,
	}
	metrics.AddGlobalTags(tags)
	metrics.Gauge("window_duration_seconds", windowDuration.Seconds(), nil)
	metrics.Gauge("max_checkouts_per_window", float64(maxCheckoutsAllowedPerWindow), nil)
	metrics.Gauge("num_clients", float64(targetNumClients), nil)
	metrics.Gauge("num_server_workers", float64(numServerWorkers), nil)
	metrics.Gauge("unfairness_tolerance_seconds", maxUnfairnessToleranceSeconds, nil)
	fmt.Printf(
		"\nExecuting with:\n\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n",
		clientDistributionTag, luaDirTag, queueTag, drainRateTrackerTag, clientRepoTag,
		numClientsTag, maxCheckoutsPerWindowTag, windowDurSecondsTag,
		unfairnessToleranceTag, randomizedClientsTag,
	)
	fmt.Printf("\nSee dashboard at: %s\n\n", dashboardUrl)
}

func loadClientDistributionConfig(filepath string) ([]ClientConfig, int) {
	var clientDistributionConfig []ClientConfig
	clientDistributionJson, err := os.Open(filepath)
	defer clientDistributionJson.Close()
	if err != nil {
		panic(fmt.Errorf("failed opening client distribution json at path '%s'", filepath))
	}

	jsonParser := json.NewDecoder(clientDistributionJson)
	err = jsonParser.Decode(&clientDistributionConfig)
	if err != nil {
		panic(fmt.Errorf("failed parsing client distribution json with error '%s'", err.Error()))
	}
	log.Debug().Msg(fmt.Sprintf("ClientDistributionConfig: %v", clientDistributionConfig))
	actualNumClients := 0
	for _, clientConfig := range clientDistributionConfig {
		pct := clientConfig.RepresentationPercent
		actualNumClients += int(math.Floor(pct * targetNumClients))
	}
	if actualNumClients > targetNumClients || actualNumClients < targetNumClients-1 {
		panic("Client config representationPercent values must sum to 1.0")
	}
	return clientDistributionConfig, actualNumClients
}

func prepareNetworkParams(ctx context.Context, numClients int) NetworkParams {
	var clientsFinishedWaitGroup sync.WaitGroup
	clientsFinishedWaitGroup.Add(numClients)
	return NetworkParams{
		Ctx:                      ctx,
		ClientsFinishedWaitGroup: &clientsFinishedWaitGroup,
		RequestTargetChannel:     make(chan *network_mock.MockRequest, maxNetworkIOBacklogSize),
		ResponseChannelsMap:      make(map[int]chan *network_mock.MockResponse),
	}
}

func makeMockCheckoutClients(
	clientDistributionConfig []ClientConfig,
	networkParams NetworkParams,
	shouldRandomizeOrder bool,
) []client.Client {
	checkoutClients := make([]client.Client, 0, targetNumClients)
	numClientsFloat := float64(targetNumClients)
	id := 1
	for _, clientConfig := range clientDistributionConfig {
		numToGenerate := int(math.Floor(clientConfig.RepresentationPercent * numClientsFloat))
		for j := 0; j < numToGenerate; j++ {
			c := clientfactory.MakeClientFromConfig(clientConfig, networkParams, id)
			checkoutClients = append(checkoutClients, c)
			networkParams.ResponseChannelsMap[id] =
				make(chan *network_mock.MockResponse, maxNetworkIOBacklogSize)
			id++
		}
	}
	if shouldRandomizeOrder {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(checkoutClients), func(i, j int) {
			checkoutClients[i], checkoutClients[j] = checkoutClients[j], checkoutClients[i]
		})
	}
	return checkoutClients
}

func makeRedisClient(redisAddr string) (*redis.Client, error) {
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	_, pingErr := redisClient.Ping().Result()
	return redisClient, pingErr
}

func prepareLuaQueueConstants() lua_queue.LuaQueueConstants {
	return lua_queue.LuaQueueConstants{
		QueueType:                    queueType,
		ShopScopePrefix:              shopScopePrefixDemo,
		MaxCheckoutsAllowedPerWindow: maxCheckoutsAllowedPerWindow,
		WindowDuration:               windowDuration,

		PollDrivenMaxTargetPollingUtil:     polldrivenMaxTargetPollingUtil,
		PollDrivenUtilUpdateInterval:       polldrivenUtilUpdateInterval,
		PollDrivenWorkingBinUpdateInterval: polldrivenWorkingBinUpdateInterval,
		PollDrivenLatestPollingUtilWeight:  polldrivenLatestPollingUtilWeight,
	}
}

func makeUserQueue(
	ctx context.Context,
	startSignalWaitGroup *sync.WaitGroup,
	redisClient *redis.Client,
	globalInventoryCounter *common.AtomicCounter,
	luaQueueParams *lua_queue.LuaQueueParams,
) queue.Queue {
	switch queueType {
	case "noop_queue":
		return queuefactory.MakeNoopQueue()
	case "sorted_set":
		return queuefactory.MakeSortedSetQueue(redisClient, int64(maxCheckoutsAllowedPerWindow))
	case "capped_bins_queue":
		return queuefactory.MakeCappedBinsQueue(int64(maxCheckoutsAllowedPerWindow))
	case "interval_bins_queue":
		return queuefactory.MakeIntervalBinsQueue(
			ctx, startSignalWaitGroup, windowDuration,
			maxCheckoutsAllowedPerWindow, 2000*time.Millisecond,
		)
	case "polldriven_capped_bins_queue":
		return queuefactory.MakePollDrivenCappedBinsQueue(
			startSignalWaitGroup,
			maxCheckoutsAllowedPerWindow,
			windowDuration,
			polldrivenMaxTargetPollingUtil,
			polldrivenUtilUpdateInterval,
			polldrivenWorkingBinUpdateInterval,
			polldrivenLatestPollingUtilWeight,
		)
	case "lua_driven_bins_queue":
		return queuefactory.MakeLuaDrivenBinsQueue(
			redisClient,
			luaQueueParams.LuaMethodToShaMap,
			luaQueueParams.ShopScopePrefix,
			luaQueueParams.LuaMethodToShopScopePrefixedKeys,
			luaQueueParams.LuaMethodToClientScopeNonPrefixedKeys,
			luaQueueParams.LuaMethodToConstantArgsMap,
			luaQueueParams.KeysBuilder,
			luaQueueParams.ArgsBuilder,
			luaQueueParams.Postprocessor,
			globalInventoryCounter,
		)
	default:
		panic(fmt.Errorf("queue type must be one of: {sorted_set, capped_bins_queue}"))
	}
}

func makeRateTracker(
	ctx context.Context,
	startSignalWaitGroup *sync.WaitGroup,
	windowDuration time.Duration,
	maxAllowed uint64,
) tracker.Tracker {
	switch trackerType {
	case "fixed_window":
		t := trackerfactory.MakeFixedWindowTracker(ctx, startSignalWaitGroup, windowDuration, maxAllowed)
		go t.MonitorAndEmitFeedback(lowCheckoutUtilMaxThresholdPct, maxSkpdLowCheckoutUtilCount)
		return t
	default:
		panic(fmt.Errorf("tracker type must be one of: {fixed_window}"))
	}
}

func makeCheckoutThrottleDriver(
	ctx context.Context,
	ctxCancelFunc context.CancelFunc,
	startSignalWaitGroup *sync.WaitGroup,
	userQueue queue.Queue,
	rateTracker tracker.Tracker,
	globalInventoryCounter *common.AtomicCounter,
) *CheckoutThrottleDriver {
	t := &CheckoutThrottleDriver{
		Ctx:                    ctx,
		CtxCancelFunc:          ctxCancelFunc,
		StartSignalWaitGroup:   startSignalWaitGroup,
		ThrottleQueue:          userQueue,
		RateTracker:            rateTracker,
		GlobalInventoryCounter: globalInventoryCounter,
	}
	// Drive utilization tracking via background goroutine rather than rely on client polling.
	go t.MonitorUtilAndNotifyQueue()
	return t
}

func makeClientRepo(clientRepoType string) simulator.ClientRepo {
	switch clientRepoType {
	case "simple_client_repo":
		return simulator.MakeSimpleClientRepo(make(map[int]client.Client))
	default:
		panic(fmt.Errorf("clientRepo type must be one of: {simple_client_repo}"))
	}
}

func makeSimulator(
	ctx context.Context,
	ctxCancelFunc context.CancelFunc,
	throttleDriver *throttle.CheckoutThrottleDriver,
	checkoutClients []client.Client,
	numServerWorkers int,
	inventoryStockTotal int,
	requestTargetChannel chan *network_mock.MockRequest,
	responseChannelsMap map[int]chan *network_mock.MockResponse,
	maxUnfairnessToleranceSeconds float64,
	clientsFinishedWaitGroup *sync.WaitGroup,
	clientRepo simulator.ClientRepo,
	startSignalWaitGroup *sync.WaitGroup,
) *Simulator {
	simDriver := &Simulator{
		Ctx:                                ctx,
		CtxCancelFunc:                      ctxCancelFunc,
		CheckoutThrottleDriver:             throttleDriver,
		Clients:                            checkoutClients,
		NumServerWorkers:                   numServerWorkers,
		InventoryStockTotal:                inventoryStockTotal,
		RequestTargetChannel:               requestTargetChannel,
		ResponseChannelsMap:                responseChannelsMap,
		SimulationCompletedListenerChannel: make(chan struct{}),
		StartSignalWaitGroup:               startSignalWaitGroup,
		ClientsFinishedWaitGroup:           clientsFinishedWaitGroup,
		ClientRepo:                         clientRepo,
		MaxUnfairnessToleranceSeconds:      maxUnfairnessToleranceSeconds,
	}
	return simDriver
}
