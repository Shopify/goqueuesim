package throttle

import (
	"context"
	"fmt"
	"sync"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/common"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/throttle/queue"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"
)

type CheckoutThrottleDriver struct {
	Ctx                    context.Context
	CtxCancelFunc          context.CancelFunc
	StartSignalWaitGroup   *sync.WaitGroup
	ThrottleQueue          queue.Queue
	RateTracker            tracker.Tracker
	GlobalInventoryCounter *common.AtomicCounter
}

// Returns true if a state change has occurred, else false.
func (t *CheckoutThrottleDriver) TryThrottleStateTransition(c client.Client) (client.ThrottleState, bool) {
	state := c.ThrottleState()
	switch state {
	case client.Initial:
		// Client first observed => track & enqueue.
		err := c.MarkQueued()
		if err != nil {
			return state, false
		}
		t.ThrottleQueue.Add(c)

		// Optional: poll-free attempt to progress immediately.
		latestTransitionState, _ := t.TryThrottleStateTransition(c)

		return latestTransitionState, true
	case client.Queued:
		if t.ThrottleQueue.IsCandidateToProceed(c) && t.RateTracker.ShouldProceed(c.ID()) {
			t.GlobalInventoryCounter.Lock()
			remInventory, _ := t.GlobalInventoryCounter.AtomicRead()
			if remInventory < 0 {
				t.GlobalInventoryCounter.Unlock()
				return state, false
			}
			err := c.MarkInCheckout()
			if err != nil {
				t.GlobalInventoryCounter.Unlock()
				return state, false
			}
			remInventory, _ = t.GlobalInventoryCounter.AtomicAdd(-1)
			t.GlobalInventoryCounter.Unlock()
			t.ThrottleQueue.Remove(c)
			if remInventory <= 0 {
				t.ThrottleQueue.Clear()
				t.CtxCancelFunc()
			}
			return client.InCheckout, true
		}
		return client.Queued, false
	case client.InCheckout:
		fallthrough
	case client.Exited:
		return state, false
	default:
		panic("Encountered unrecognized state set on client")
	}
}

// Blocking routine to monitor rateTrackerPollingUtil feedback -> emit to queue once per tracker window.
func (t *CheckoutThrottleDriver) MonitorUtilAndNotifyQueue() {
	trackerFeedbackChannel := t.RateTracker.GetFeedbackChannel()
	t.StartSignalWaitGroup.Wait()
	for { // loop infinitely
		select { // block until next util feedback or termination received
		case <-t.Ctx.Done():
			return
		case nextFeedbackMsg := <-trackerFeedbackChannel:
			fmt.Printf("Checkout Util=%.2f \n", nextFeedbackMsg.CheckoutUtil)
			metrics.Gauge("polling_util", nextFeedbackMsg.PollingUtil, nil)
			metrics.Gauge("reached_checkout_util", nextFeedbackMsg.CheckoutUtil, nil)
			t.ThrottleQueue.ReceiveTrackerFeedback(nextFeedbackMsg)
		}
	}
}
