package redis_queue

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"

	"github.com/go-redis/redis/v7"
)

// TODO: How do we handle expiry? What about shrinking the window from the left?
type SortedSetQueue struct {
	RedisClient   *redis.Client
	MinWindowSize int64
	WindowSize    int64
}

func (ssq *SortedSetQueue) Add(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "add", nil)
	if _, err := ssq.RedisClient.ZAddNX(buildSortedSetKey(), &redis.Z{
		Score:  float64(c.QueueEntryTime().UnixNano()),
		Member: strconv.Itoa(c.ID()),
	}).Result(); err != nil {
		panic(err)
	}
}

func (ssq *SortedSetQueue) Remove(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "remove", nil)
	if _, err := ssq.RedisClient.ZRem(
		buildSortedSetKey(), strconv.Itoa(c.ID()),
	).Result(); err != nil {
		panic(err)
	}
}

// IsCandidateToProceed returns true if the client is allowed to race into the rate tracker.
// TODO: Use entryTime to set a marker so that we could skip calls to redis.
// We will probably need to do a ZRangeWithScores call to obtain the entry
// time for the last index based on window size. If the number of elements is
// lesser than the window size, we should set lastEntryTime to current time so
// that we don't skip new entries.
func (ssq *SortedSetQueue) IsCandidateToProceed(c client.Client) bool {
	clientRank, err := ssq.RedisClient.ZRank(
		buildSortedSetKey(), strconv.Itoa(c.ID()),
	).Result()
	if err != nil {
		panic(err)
	}

	return clientRank < ssq.WindowSize
}

// Not bothering with mutex since only 1 goroutine will invoke this.
func (ssq *SortedSetQueue) ReceiveTrackerFeedback(feedback tracker.Feedback) {
	if earlyExit, ok := feedback.CustomFeedback["should_ignore_poll_util"]; ok && earlyExit == true {
		return
	}

	util := feedback.PollingUtil
	fmt.Printf("SSQ Stats: (LastWindowSize=%d) (Polling Util=%.2f)\n", ssq.WindowSize, util)

	// if util == 0.0 {
	//
	// Should we reset to initial size? If the polling intervals are not
	// constant, that will cause utilization to oscillate between a large
	// number and 0.0. When that happens, we don't want to be oscillating
	// between MinWindowSize and a large number. There could be a
	// possibility that ssq.WindowSize maximizes checkout rate, but we're
	// just unlucky for the previous second. Maybe this might occur if more
	// clients go away.
	//
	// It seems like this situation occurs when polling interval is larger
	// than window size (e.g. 5 seconds for polling interval, and 1 second for window size).
	//
	// ssq.WindowSize = ssq.MinWindowSize
	if util < 1.0 {
		// We do not want to increase the window size if we have no more elements.
		diff := int64(math.Ceil((1.2 - util) * float64(ssq.WindowSize)))

		if ssq.Size() > ssq.WindowSize+diff {
			// Expand window size.
			ssq.WindowSize += diff
		}
	} else if util > 1.2 {
		// Shrink at most half of window size.
		diff := math.Ceil((util - 1.2) * float64(ssq.WindowSize))
		ssq.WindowSize -= int64(math.Min(diff, float64(0.5*float64(ssq.WindowSize))))
		if ssq.WindowSize <= 0 {
			ssq.WindowSize = ssq.MinWindowSize
		}
	} else {
		// We will leave window size around 1.0 to 1.2 (both inclusive).
	}

	windowSizeFlt := float64(ssq.WindowSize)
	metrics.Gauge("ssq.window_size", windowSizeFlt, nil)
	metrics.Gauge("ssq.considered_client_util", windowSizeFlt/float64(ssq.Size()), nil)
}

func (ssq *SortedSetQueue) Clear() {
	if _, err := ssq.RedisClient.ZRemRangeByRank(
		buildSortedSetKey(), int64(0), int64(-1),
	).Result(); err != nil {
		panic(err)
	}
}

func (ssq *SortedSetQueue) Size() int64 {
	val, err := ssq.RedisClient.ZCard(buildSortedSetKey()).Result()
	if err != nil {
		panic(err)
	}
	return val
}

func buildSortedSetKey() string {
	return "sorted_set_queue:shop_id_0"
}
