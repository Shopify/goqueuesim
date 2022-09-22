package impl

import (
	"fmt"
	"time"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"
)

type NoopQueue struct {
	totalQueuedClients int64
	totalPollsCount    int64
}

func (nopq *NoopQueue) Add(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "add", nil)
	nopq.totalQueuedClients++
}

func (nopq *NoopQueue) Remove(c client.Client) {
	defer metrics.BenchmarkMethod(time.Now(), "remove", nil)
	nopq.totalQueuedClients--
}

func (nopq *NoopQueue) Size() int64 {
	return nopq.totalQueuedClients
}

func (nopq *NoopQueue) IsCandidateToProceed(c client.Client) bool {
	nopq.totalPollsCount++
	return true
}

func (nopq *NoopQueue) ReceiveTrackerFeedback(feedback tracker.Feedback) {
	fmt.Printf(
		"checkoutUtil=%.2f pollingUtil=%.2f queuedClients=%d totalPolls=%d\n",
		feedback.CheckoutUtil, feedback.PollingUtil, nopq.Size(), nopq.totalPollsCount,
	)
}

func (nopq *NoopQueue) Clear() {
	nopq.totalQueuedClients = 0
	nopq.totalPollsCount = 0
}
