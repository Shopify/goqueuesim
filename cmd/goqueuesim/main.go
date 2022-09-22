package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/Shopify/goqueuesim/internal/common"
	"github.com/Shopify/goqueuesim/internal/lua_config"

	"github.com/rs/zerolog/log"
)

func main() {
	// Prepare background context configured to listen for cancelling.
	ctx, cancel := context.WithCancel(context.Background())

	// Configure channel to receive terminal interrupt.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	// Start goroutine to listen for interrupt signal => if received, cancel running context.
	go func() { <-sig; log.Info().Msg("shutting down simulator"); cancel() }()

	var startSignalWaitGroup sync.WaitGroup
	startSignalWaitGroup.Add(1)

	validateParams(maxSkpdLowCheckoutUtilCount)
	setLogging(logLevel)
	configureExperiment()
	clientsConfig, actualNumClients := loadClientDistributionConfig(clientDistributionJsonPath)
	networkParams := prepareNetworkParams(ctx, actualNumClients)

	// Prepare throttle simulation configured for target params.
	checkoutClients := makeMockCheckoutClients(clientsConfig, networkParams, shouldRandomizeClientOrder)
	clientRepo := makeClientRepo(clientRepoType)

	luaQueueConstants := prepareLuaQueueConstants()
	luaQueueParams := lua_config.SetDefaultLuaQueueParams()
	redisClient, redisErr := makeRedisClient(redisAddr)
	if luaQueueConstants.QueueType == "lua_driven_bins_queue" {
		if redisErr != nil {
			panic(fmt.Errorf("Redis is required for Lua queues but failed to respond a ping request"))
		}
		luaQueueParams = lua_config.ConfigureRedisLua(luaQueueDirPath, redisClient, luaQueueConstants)
	}

	globalInventoryCounter := common.AtomicCounter{Count: inventoryStockTotal}
	userQueue := makeUserQueue(ctx, &startSignalWaitGroup, redisClient, &globalInventoryCounter, luaQueueParams)
	userQueue.Clear()

	rateTracker := makeRateTracker(ctx, &startSignalWaitGroup, windowDuration, maxCheckoutsAllowedPerWindow)

	checkoutThrottleDriver := makeCheckoutThrottleDriver(
		ctx, cancel, &startSignalWaitGroup, userQueue, rateTracker, &globalInventoryCounter,
	)

	simDriver := makeSimulator(
		ctx,
		cancel,
		checkoutThrottleDriver,
		checkoutClients,
		numServerWorkers,
		inventoryStockTotal,
		networkParams.RequestTargetChannel,
		networkParams.ResponseChannelsMap,
		maxUnfairnessToleranceSeconds,
		networkParams.ClientsFinishedWaitGroup,
		clientRepo,
		&startSignalWaitGroup,
	)

	simDriver.StartSimulation()
}
