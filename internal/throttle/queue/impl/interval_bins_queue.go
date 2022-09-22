package impl

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"

	"github.com/Shopify/goqueuesim/internal/client"
)

const (
	intervalUserBinKey = "intervalUserBinId"
	intervalUserPosKey = "intervalUserQueuePos"
)

type IntervalBinsQueue struct {
	ctx                  context.Context
	startSignalWaitGroup *sync.WaitGroup

	windowDur             time.Duration
	maxCheckoutsPerWindow int64
	maxUnfairMilliseconds time.Duration

	binCounts                map[int64]int64
	latestBinIdx             int64
	maxConsideredBinIdx      int64
	consideredBinLastUpdated time.Time
	nextScheduledBinUpdate   time.Time

	totalQueuedClients int64

	binMutex sync.Mutex
}

func (ibq *IntervalBinsQueue) Add(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "add", nil)
	ibq.binMutex.Lock()
	defer ibq.binMutex.Unlock()

	binCounts := ibq.binCounts

	if _, ok := binCounts[ibq.latestBinIdx]; !ok {
		binCounts[ibq.latestBinIdx] = 0
	}

	binCounts[ibq.latestBinIdx]++
	ibq.totalQueuedClients++
	c.SetThrottleCookieVal(intervalUserBinKey, strconv.FormatInt(ibq.latestBinIdx, 10))
	c.SetThrottleCookieVal(intervalUserPosKey, strconv.FormatInt(ibq.totalQueuedClients, 10))
	relativeQueuePosition := float64(ibq.totalQueuedClients) / float64(ibq.maxCheckoutsPerWindow)
	remMillisecsToPoll := relativeQueuePosition * float64(ibq.windowDur.Milliseconds())
	remDurToPoll := time.Duration(remMillisecsToPoll) * time.Millisecond
	c.AdvisePollAfter(time.Now().Add(remDurToPoll))
}

func (ibq *IntervalBinsQueue) Remove(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "remove", nil)
	ibq.binMutex.Lock()
	defer ibq.binMutex.Unlock()
	if binIdx, ok := ibq.getUserBinIdx(c); ok {
		ibq.totalQueuedClients--
		ibq.binCounts[binIdx]--
	}
}

func (ibq *IntervalBinsQueue) Size() int64 {
	return ibq.totalQueuedClients
}

func (ibq *IntervalBinsQueue) IsCandidateToProceed(c client.Client) bool {
	isCandidate := false
	if binIdx, ok := ibq.getUserBinIdx(c); ok {
		isCandidate = binIdx <= ibq.maxConsideredBinIdx
		if (binIdx + 1) == ibq.maxConsideredBinIdx {
			c.AdvisePollAfter(ibq.nextScheduledBinUpdate)
		}
	}
	return isCandidate
}

func (ibq *IntervalBinsQueue) ReceiveTrackerFeedback(feedback tracker.Feedback) {
	// Method can effectively no-op since this queue does not rely on tracker feedback.
	fmt.Printf(
		"latestBinIdx=%d workingBin=%d checkoutUtil=%.2f queuedClients=%d\n",
		ibq.latestBinIdx, ibq.maxConsideredBinIdx, feedback.CheckoutUtil, ibq.Size(),
	)
	metrics.Gauge("ibq.working_bin", float64(ibq.maxConsideredBinIdx), nil)
}

func (ibq *IntervalBinsQueue) RoutinelyUpdateLatestBin() {
	ibq.startSignalWaitGroup.Wait()
	sleepTimer := time.NewTimer(ibq.maxUnfairMilliseconds)
	for {
		select { // block until interval elapses or termination received
		case <-ibq.ctx.Done():
			sleepTimer.Stop()
			return
		case <-sleepTimer.C:
			sleepTimer.Reset(ibq.maxUnfairMilliseconds)
			ibq.latestBinIdx++
		}
	}
}

func (ibq *IntervalBinsQueue) RoutinelyMaximizeFairThroughput() {
	ibq.startSignalWaitGroup.Wait()
	time.Sleep(ibq.maxUnfairMilliseconds)
	for {
		ibq.binMutex.Lock()
		if ibq.maxConsideredBinIdx >= ibq.latestBinIdx {
			// Never increment beyond latest queueing bin.
			ibq.binMutex.Unlock()
			time.Sleep(ibq.maxUnfairMilliseconds)
			continue
		}
		// Compute allowedInactiveMillisecs = (clients in bin / max checkouts per window) * window duration
		binSzMaxCpwRatio := float64(ibq.binCounts[ibq.maxConsideredBinIdx]) / float64(ibq.maxCheckoutsPerWindow)
		binMillisecsShare := binSzMaxCpwRatio * float64(ibq.windowDur.Milliseconds())
		allowedInactiveMillis := math.Max(float64(ibq.maxUnfairMilliseconds.Milliseconds()), binMillisecsShare)
		allowedInactiveDur := time.Duration(allowedInactiveMillis) * time.Millisecond
		// Update working bin if grace period exhausted, else sleep for remaining grace period.
		ibq.nextScheduledBinUpdate = ibq.consideredBinLastUpdated.Add(allowedInactiveDur)
		remAllowedInactivity := ibq.nextScheduledBinUpdate.Sub(time.Now())
		if remAllowedInactivity <= 0 {
			ibq.maxConsideredBinIdx++
			ibq.consideredBinLastUpdated = time.Now()
			ibq.binMutex.Unlock()
		} else {
			ibq.binMutex.Unlock()
			time.Sleep(remAllowedInactivity)
		}
	}
}

func (ibq *IntervalBinsQueue) Clear() {
	ibq.binCounts = make(map[int64]int64)
	ibq.latestBinIdx = 0
	ibq.maxConsideredBinIdx = 0
	ibq.totalQueuedClients = 0
}

func (ibq *IntervalBinsQueue) getUserBinIdx(c client.Client) (int64, bool) {
	if binIdxStr, ok := c.GetThrottleCookieVal(intervalUserBinKey); ok {
		if binIdx, err := strconv.ParseInt(binIdxStr, 10, 64); err == nil {
			return binIdx, true
		}
	}
	return 0, false
}

func (ibq *IntervalBinsQueue) getUserQueuePos(c client.Client) (int64, bool) {
	if queuePosStr, ok := c.GetThrottleCookieVal(intervalUserPosKey); ok {
		if queuePos, err := strconv.ParseInt(queuePosStr, 10, 64); err == nil {
			return queuePos, true
		}
	}
	return 0, false
}
