package impl

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"
)

const (
	cappedUserBinKey = "cappedUserBinId"
)

type CappedBinsQueue struct {
	bins               map[int64]map[int]bool
	curWindowDequeues  map[int64]int64
	binSize            int64
	queueBin           int64
	workingBin         int64
	totalQueuedClients int64
	mu                 sync.Mutex
}

func (cbq *CappedBinsQueue) Add(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "add", nil)
	cbq.mu.Lock()
	defer cbq.mu.Unlock()

	bins := cbq.bins
	binSize := cbq.binSize

	if _, ok := bins[cbq.queueBin]; !ok {
		bins[cbq.queueBin] = make(map[int]bool, binSize)
	}

	if int64(len(bins[cbq.queueBin])) >= cbq.binSize {
		cbq.queueBin++
		bins[cbq.queueBin] = make(map[int]bool, binSize)
	}
	bins[cbq.queueBin][c.ID()] = true
	cbq.totalQueuedClients++
	c.SetThrottleCookieVal(cappedUserBinKey, strconv.FormatInt(cbq.queueBin, 10))
}

func (cbq *CappedBinsQueue) Remove(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "remove", nil)
	cbq.mu.Lock()
	defer cbq.mu.Unlock()
	cbq.totalQueuedClients--
	cbq.curWindowDequeues[cbq.workingBin]++
}

func (cbq *CappedBinsQueue) Size() int64 {
	return cbq.totalQueuedClients
}

func (cbq *CappedBinsQueue) IsCandidateToProceed(c client.Client) bool {
	if ub, e := cbq.getUserBin(c); !e {
		return ub <= cbq.workingBin
	}
	return false
}

// Received once per window => working bin incremented at most once per time window.
func (cbq *CappedBinsQueue) ReceiveTrackerFeedback(feedback tracker.Feedback) {
	if earlyExit, ok := feedback.CustomFeedback["should_ignore_poll_util"]; ok && earlyExit == true {
		return
	}

	checkoutUtil := feedback.CheckoutUtil

	allowedInCurrentWindow := cbq.binSize * cbq.workingBin
	consideredClientUtil := float64(allowedInCurrentWindow) / float64(cbq.totalQueuedClients)
	allowedWindowDrainUtil := float64(cbq.curWindowDequeues[cbq.workingBin]) / float64(cbq.binSize)

	fmt.Printf(
		"workingBin=%d checkoutUtil=%.2f consideredClientUtil=%.2f allowedWindowDrainUtil=%.2f\n",
		cbq.workingBin, checkoutUtil, consideredClientUtil, allowedWindowDrainUtil,
	)
	metrics.Gauge("cbq.working_bin", float64(cbq.workingBin), nil)
	metrics.Gauge("cbq.considered_client_util", consideredClientUtil, nil)
	metrics.Gauge("cbq.allowed_window_drain_util", allowedWindowDrainUtil, nil)

	if checkoutUtil <= 0.2 {
		cbq.workingBin += cbq.workingBin + 2
	} else if checkoutUtil > 0.2 && checkoutUtil <= 0.9 {
		cbq.workingBin++
	} else {
		// if checkoutUtil > 90% then we consider allowedWindowDrainUtil as well
		// and increment only when allowedWindowDrainUtil is > 0.5 (ie. allowed dequeues drained >50% bin_size)
		if allowedWindowDrainUtil > 0.5 {
			cbq.workingBin++
		}
	}

}

func (cbq *CappedBinsQueue) Clear() {
	cbq.bins = make(map[int64]map[int]bool)
	cbq.curWindowDequeues = make(map[int64]int64)
	cbq.workingBin = 1
	cbq.queueBin = 1
	cbq.totalQueuedClients = 0
}

func (cbq *CappedBinsQueue) getUserBin(c client.Client) (ubin int64, ok bool) {
	if r, ok := c.GetThrottleCookieVal(cappedUserBinKey); ok {
		if ub, err := strconv.ParseInt(r, 10, 64); err == nil {
			ok = true
			ubin = ub
		}
	}
	return
}
