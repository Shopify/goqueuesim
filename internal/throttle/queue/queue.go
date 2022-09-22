package queue

import (
	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"
)

type Queue interface {
	Add(client client.Client)
	Remove(client client.Client)
	IsCandidateToProceed(client client.Client) bool
	ReceiveTrackerFeedback(trackerFeedback tracker.Feedback)
	Size() int64
	Clear()
}
