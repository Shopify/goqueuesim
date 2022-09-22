package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Shopify/goqueuesim/internal/throttle/tracker"
)

// TODO: add Redis-backed FixedWindowTracker -> expire stale entries
// Returns FixedWindowTracker allowing maxCheckoutsPerWindow per interval of size `fixedWindowDuration`.
func MakeFixedWindowTracker(
	ctx context.Context,
	startSignalWaitGroup *sync.WaitGroup,
	windowDuration time.Duration,
	maxEventsPerWindow uint64,
) tracker.Tracker {
	if windowDuration.Milliseconds() <= 0 {
		panic(fmt.Errorf("Failed instantiating FixedWindowTracker: invalid window duration"))
	}
	t := &FixedWindowTracker{
		Ctx:                    ctx,
		startSignalWaitGroup:   startSignalWaitGroup,
		fixedWindowDuration:    windowDuration,
		maxCheckoutsPerWindow:  maxEventsPerWindow,
		curWindowIndex:         0,
		TrackerFeedbackChannel: make(chan tracker.Feedback),
	}
	t.windowedPollingUtil.dict = make(map[int]map[int]bool)
	t.windowedCheckoutUtil.dict = make(map[int]map[int]bool)
	return t
}
