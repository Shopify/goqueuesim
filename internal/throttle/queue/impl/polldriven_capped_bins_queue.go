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
	polldrivenUserBinKey      = "polldrivenUserBinId"
	polldrivenUserQueuePosKey = "polldrivenUserQueuePos"
)

type pollsPerSecondTracker struct {
	lock sync.Locker

	curSecondCount     uint
	prevSecondCount    uint
	prevUnixTimeSecond int64
}

func (ppst *pollsPerSecondTracker) LastCount() uint {
	return ppst.prevSecondCount
}

func (ppst *pollsPerSecondTracker) CountPollingEvent() {
	ppst.lock.Lock()
	defer ppst.lock.Unlock()

	curUnixSecond := time.Now().Unix()
	if curUnixSecond == ppst.prevUnixTimeSecond {
		ppst.curSecondCount += 1
	} else {
		ppst.prevSecondCount = ppst.curSecondCount
		ppst.curSecondCount = 1
		ppst.prevUnixTimeSecond = time.Now().Unix()
	}
}

type PollDrivenCappedBinsQueue struct {
	startSignalWaitGroup *sync.WaitGroup
	lock                 sync.Locker
	pollsPerSecTracker   *pollsPerSecondTracker

	binSize                  int
	checkoutsPerSecond       int
	maxTargetPollingUtil     float64
	utilUpdateInterval       time.Duration
	workingBinUpdateInterval time.Duration
	latestPollingUtilWeight  float64

	queueBin          int
	queueBinPos       int
	workingBin        int
	workingBinUpdated time.Time

	pollingUtil     float64
	totalPollsCount int
	totalClients    int
}

func (polldriven *PollDrivenCappedBinsQueue) Add(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "add", nil)
	polldriven.lock.Lock()
	defer polldriven.lock.Unlock()

	if polldriven.queueBinPos < polldriven.binSize {
		polldriven.queueBinPos++
	} else {
		polldriven.queueBinPos = 0
		polldriven.queueBin++
	}

	polldriven.totalClients++
	_ = c.SetThrottleCookieVal(polldrivenUserBinKey, strconv.FormatInt(int64(polldriven.queueBin), 10))
	_ = c.SetThrottleCookieVal(polldrivenUserQueuePosKey, strconv.FormatInt(int64(polldriven.totalClients), 10))
	relativeQueuePosition := float64(polldriven.totalClients) / float64(polldriven.checkoutsPerSecond)
	remMillisecsToPoll := relativeQueuePosition * 1000
	remDurToPoll := time.Duration(remMillisecsToPoll) * time.Millisecond
	c.AdvisePollAfter(time.Now().Add(remDurToPoll))
}

func (polldriven *PollDrivenCappedBinsQueue) Remove(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "remove", nil)
	polldriven.totalClients--
}

func (polldriven *PollDrivenCappedBinsQueue) IsCandidateToProceed(c client.Client) bool {
	binIdx, _ := polldriven.getUserBinIdx(c)

	polldriven.totalPollsCount++
	if int(binIdx) > polldriven.workingBin {
		return false
	}

	polldriven.pollsPerSecTracker.CountPollingEvent()
	return true
}

func (polldriven *PollDrivenCappedBinsQueue) ReceiveTrackerFeedback(feedback tracker.Feedback) {
	fmt.Printf(
		"workingBin=%d queueBin=%d lastWindowUniquePollersUtil=%.2f lastSecondRawPollingUtil=%.2f"+
			" queuedClients=%d totalPolls=%d\n",
		polldriven.workingBin, polldriven.queueBin, feedback.PollingUtil,
		polldriven.pollingUtil, polldriven.Size(), polldriven.totalPollsCount,
	)
}

func (polldriven *PollDrivenCappedBinsQueue) Size() int64 {
	return int64(polldriven.totalClients)
}

func (polldriven *PollDrivenCappedBinsQueue) Clear() {
	polldriven.queueBin = 0
	polldriven.queueBinPos = 0
	polldriven.workingBin = 0
}

func (polldriven *PollDrivenCappedBinsQueue) RoutinelyUpdatePollingUtil() {

	for {
		select {
		case <-time.After(polldriven.utilUpdateInterval):
			polldriven.pollingUtil = polldriven.weightedPollingUtil()

			if time.Now().Sub(polldriven.workingBinUpdated) >= polldriven.workingBinUpdateInterval {
				if polldriven.shouldUpdateWorkingBin() {
					polldriven.workingBin += 1
					polldriven.workingBinUpdated = time.Now()
				} else {
					fmt.Printf(
						"\nLastSecondPollingUtil=%.2f => delay working bin update\n\n",
						polldriven.pollingUtil,
					)
				}
			}
		}
	}
}

func (polldriven *PollDrivenCappedBinsQueue) weightedPollingUtil() float64 {
	latestPollingUtil := float64(polldriven.pollsPerSecTracker.LastCount()) / float64(polldriven.checkoutsPerSecond)
	if polldriven.pollingUtil == 0 {
		return latestPollingUtil
	} else {
		prevPollingUtil := (1.0 - polldriven.latestPollingUtilWeight) * polldriven.pollingUtil
		return latestPollingUtil*(polldriven.latestPollingUtilWeight) + prevPollingUtil
	}
}

func (polldriven *PollDrivenCappedBinsQueue) shouldUpdateWorkingBin() bool {
	return polldriven.pollingUtil <= polldriven.maxTargetPollingUtil
}

func (polldriven *PollDrivenCappedBinsQueue) getUserBinIdx(c client.Client) (int64, bool) {
	if binIdxStr, ok := c.GetThrottleCookieVal(polldrivenUserBinKey); ok {
		if binIdx, err := strconv.ParseInt(binIdxStr, 10, 64); err == nil {
			return binIdx, true
		}
	}
	return 0, false
}

func (polldriven *PollDrivenCappedBinsQueue) getUserQueuePos(c client.Client) (int64, bool) {
	if queuePosStr, ok := c.GetThrottleCookieVal(polldrivenUserQueuePosKey); ok {
		if queuePos, err := strconv.ParseInt(queuePosStr, 10, 64); err == nil {
			return queuePos, true
		}
	}
	return 0, false
}
