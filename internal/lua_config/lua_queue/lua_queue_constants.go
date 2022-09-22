package lua_queue

import "time"

type LuaQueueConstants struct {
	QueueType                    string
	ShopScopePrefix              string
	MaxCheckoutsAllowedPerWindow int64
	WindowDuration               time.Duration

	PollDrivenMaxTargetPollingUtil     float64
	PollDrivenUtilUpdateInterval       time.Duration
	PollDrivenWorkingBinUpdateInterval time.Duration
	PollDrivenLatestPollingUtilWeight  float64
}
