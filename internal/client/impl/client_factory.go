package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/network_mock"
)

const (
	defaultPollIntervalSeconds = 5
)

type NetworkParams struct {
	Ctx                      context.Context
	ClientsFinishedWaitGroup *sync.WaitGroup
	RequestTargetChannel     chan *network_mock.MockRequest
	ResponseChannelsMap      map[int]chan *network_mock.MockResponse
}

func MakeClientFromConfig(
	config client.ClientConfig,
	networkParams NetworkParams,
	id int,
) client.Client {
	baseClient := MakeBaseClient(config, networkParams, id)
	switch config.ClientType {
	case "routinely_polling_client":
		return &baseClient
	case "fully_disappearing_client":
		return MakeFullyDisappearingClient(config, &baseClient)
	case "lazy_polling_client":
		return MakeLazyPollingClient(config, &baseClient)
	case "exponential_backoff_client":
		return MakeExponentialBackoffClient(config, &baseClient)
	case "jit_greedy_poller":
		return MakeJitGreedyPoller(config, &baseClient)
	default:
		panic(fmt.Errorf("client type must be one of: {routinely_polling_client}"))
	}
}

func MakeBaseClient(
	config client.ClientConfig,
	networkParams NetworkParams,
	id int,
) BaseClient {
	pollIntervalSeconds, found := config.CustomIntProperties["dflt_poll_interval_seconds"]
	if !found || pollIntervalSeconds <= 0 {
		pollIntervalSeconds = defaultPollIntervalSeconds
	}
	return BaseClient{
		Ctx:                      networkParams.Ctx,
		Id:                       id,
		label:                    config.HumanizedLabel,
		ClientsFinishedWaitGroup: networkParams.ClientsFinishedWaitGroup,
		RequestTargetChannel:     networkParams.RequestTargetChannel,
		HitCheckoutStep:          false,
		DefaultPollInterval:      time.Duration(pollIntervalSeconds) * time.Second,
		MaxInitialDelayMs:        config.MaxInitialDelayMs,
		MaxNetworkJitterMs:       config.MaxNetworkJitterMs,
		ObeysPollAfter:           config.ObeysServerPollAfter,
		StartedPolling:           false,
		state:                    client.Initial,
	}
}

func MakeFullyDisappearingClient(
	config client.ClientConfig,
	baseClient *BaseClient,
) client.Client {
	pollsBeforeDisappearing, found := config.CustomIntProperties["polls_before_disappearing"]
	if !found || pollsBeforeDisappearing < 0 {
		pollsBeforeDisappearing = 0
	}
	return &FullyDisappearingClient{
		BaseClient:              baseClient,
		PollsBeforeDisappearing: pollsBeforeDisappearing,
	}
}

func MakeLazyPollingClient(
	config client.ClientConfig,
	baseClient *BaseClient,
) client.Client {
	skipPollingRoundProbabilityPct, found := config.CustomFloatProperties["skip_polling_probability_pct"]
	if !found || skipPollingRoundProbabilityPct <= 0 || skipPollingRoundProbabilityPct >= 1 {
		skipPollingRoundProbabilityPct = 0.05
	}
	maxSleepMs, found := config.CustomIntProperties["max_ms_sleep_dur"]
	if !found || maxSleepMs <= 0 {
		maxSleepMs = 10000
	}
	return &LazyPollingClient{
		BaseClient:                     baseClient,
		SkipPollingRoundProbabilityPct: skipPollingRoundProbabilityPct,
		MaxSleepMs:                     maxSleepMs,
	}
}

func MakeExponentialBackoffClient(
	config client.ClientConfig,
	baseClient *BaseClient,
) client.Client {
	maximumBackoffMs, found := config.CustomIntProperties["maximum_backoff_ms"]
	if !found || maximumBackoffMs <= 0 {
		maximumBackoffMs = 1
	}
	maximumBackoff := time.Duration(maximumBackoffMs) * time.Millisecond
	return &ExponentialBackoffClient{
		BaseClient:     baseClient,
		MaximumBackoff: maximumBackoff,
	}
}

func MakeJitGreedyPoller(
	config client.ClientConfig,
	baseClient *BaseClient,
) client.Client {
	windowCheaterOnly, found := config.CustomBoolProperties["window_cheater_only"]
	if !found {
		windowCheaterOnly = false
	}
	greedyPollsCount, found := config.CustomIntProperties["greedy_polls_count"]
	if !found || greedyPollsCount <= 0 {
		greedyPollsCount = 1
	}
	earlyPollLeadTimeMs, found := config.CustomIntProperties["early_poll_lead_time_ms"]
	if !found || earlyPollLeadTimeMs <= 0 {
		earlyPollLeadTimeMs = 1
	}
	greedyPollsDelayMs, found := config.CustomIntProperties["greedy_polls_delay_ms"]
	if !found || greedyPollsDelayMs <= 0 {
		greedyPollsDelayMs = 1
	}
	windowDurMs, found := config.CustomIntProperties["window_dur_ms"]
	if !found || windowDurMs <= 0 {
		windowDurMs = 1
	}
	earlyPollLeadTime := time.Duration(earlyPollLeadTimeMs) * time.Millisecond
	greedyPollsDelay := time.Duration(greedyPollsDelayMs) * time.Millisecond
	windowDur := time.Duration(windowDurMs) * time.Millisecond
	return &JitGreedyPoller{
		BaseClient:        baseClient,
		windowCheaterOnly: windowCheaterOnly,
		greedyPollsCount:  greedyPollsCount,
		earlyPollLeadTime: earlyPollLeadTime,
		greedyPollsDelay:  greedyPollsDelay,
		windowDur:         windowDur,
	}
}
