package impl

import (
	"context"
	"sync"
	"time"

	"github.com/Shopify/goqueuesim/internal/throttle/tracker"
)

// Comparable to https://github.com/Shopify/nginx-routing-modules/blob/master/lua/shopify/util/rate.lua.

// FixedWindowTracker implements an in-memory fixed window throttler.
// There are two limitations to this approach:
// - window counters do not expire
// - each window can only store up to the maximum value of uint64.
type FixedWindowTracker struct {
	Ctx context.Context

	startSignalWaitGroup *sync.WaitGroup

	// Minimum length of time before rolling over to a new window.
	fixedWindowDuration time.Duration

	// Maximum number of entries to allow in each window.
	maxCheckoutsPerWindow uint64

	curWindowIndex int

	// Mutex-guarded dict for { windowIndex -> { clientId -> hasPolled } }
	windowedPollingUtil struct {
		sync.Mutex
		dict map[int]map[int]bool
	}

	// Mutex-guarded dict for { windowIndex -> { clientId -> reachedCheckout } }
	windowedCheckoutUtil struct {
		sync.Mutex
		dict map[int]map[int]bool
	}

	// Channel to buffer emitted Tracker Feedback messages.
	TrackerFeedbackChannel chan tracker.Feedback
}

func (t *FixedWindowTracker) GetFeedbackChannel() chan tracker.Feedback {
	return t.TrackerFeedbackChannel
}

// Returns polling utilization rate for window with index windowIndex.
func (t *FixedWindowTracker) PollingUtilization(windowIndex int) float64 {
	observedPollingClients, ok := t.windowedPollingUtil.dict[windowIndex]
	if !ok {
		return float64(0)
	}
	return float64(float64(len(observedPollingClients)) / float64(t.maxCheckoutsPerWindow))
}

// Returns actual checkout utilization rate for window with index windowIndex.
func (t *FixedWindowTracker) CheckoutUtilization(windowIndex int) float64 {
	reachedCheckoutClients, ok := t.windowedCheckoutUtil.dict[windowIndex]
	if !ok {
		return float64(0)
	}
	return float64(float64(len(reachedCheckoutClients)) / float64(t.maxCheckoutsPerWindow))
}

// Returns true if sufficient capacity to permit event, else false.
func (t *FixedWindowTracker) ShouldProceed(clientId int) bool {
	t.windowedPollingUtil.Lock()
	t.windowedCheckoutUtil.Lock()
	defer t.windowedPollingUtil.Unlock()
	defer t.windowedCheckoutUtil.Unlock()

	windowIndex := t.curWindowIndex

	if _, ok := t.windowedPollingUtil.dict[windowIndex]; !ok {
		t.windowedPollingUtil.dict[windowIndex] = make(map[int]bool)
	}
	if _, ok := t.windowedCheckoutUtil.dict[windowIndex]; !ok {
		t.windowedCheckoutUtil.dict[windowIndex] = make(map[int]bool)
	}

	t.windowedPollingUtil.dict[windowIndex][clientId] = true
	canPass := uint64(len(t.windowedCheckoutUtil.dict[windowIndex])) <= t.maxCheckoutsPerWindow
	if canPass {
		t.windowedCheckoutUtil.dict[windowIndex][clientId] = true
	}

	return canPass
}

// Checks utilization once per tracker window -> emits to feedback channels.
func (t *FixedWindowTracker) MonitorAndEmitFeedback(
	lowUtilMaxThresholdPct float64,
	maxSkpdLowPollingUtilCount int,
) {
	lowPollingUtilConsecCount := 0
	t.startSignalWaitGroup.Wait()
	sleepTimer := time.NewTimer(t.fixedWindowDuration)
	for { // loop infinitely
		select { // block until timeout or termination received

		case <-t.Ctx.Done():
			sleepTimer.Stop()
			return

		case <-sleepTimer.C:
			sleepTimer.Reset(t.fixedWindowDuration)

			t.windowedPollingUtil.Lock()
			t.windowedCheckoutUtil.Lock()
			trackerPollingUtil := t.PollingUtilization(t.curWindowIndex)
			trackerCheckoutUtil := t.CheckoutUtilization(t.curWindowIndex)
			t.curWindowIndex++
			t.windowedCheckoutUtil.Unlock()
			t.windowedPollingUtil.Unlock()

			// Skip notifying queue (let clients poll) while (low util events seq.) <= (max skipped)
			if trackerPollingUtil < lowUtilMaxThresholdPct {
				lowPollingUtilConsecCount++
			} else {
				lowPollingUtilConsecCount = 0
			}

			feedback := tracker.Feedback{PollingUtil: trackerPollingUtil, CheckoutUtil: trackerCheckoutUtil}
			feedback.CustomFeedback = make(map[string]interface{})
			feedback.CustomFeedback["should_ignore_poll_util"] = true
			if lowPollingUtilConsecCount == 0 || lowPollingUtilConsecCount >= maxSkpdLowPollingUtilCount {
				feedback.CustomFeedback["should_ignore_poll_util"] = false
			}

			select {
			case t.TrackerFeedbackChannel <- feedback:
			default:
				// no-op if channel is full (buffer drained quickly => rarely stale feedback)
			}
		}
	}
}
