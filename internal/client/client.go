package client

import (
	"time"

	"github.com/Shopify/goqueuesim/internal/network_mock"
)

type Client interface {
	ID() int
	Label() string
	Lock()
	Unlock()

	QueueEntryTime() time.Time
	QueueExitTime() time.Time
	QueueDuration() time.Duration

	SetThrottleCookieVal(key, value string) error
	GetThrottleCookieVal(key string) (string, bool)
	RemoveThrottleCookieVal(key string) error
	ClearThrottleCookie() error

	IsQueued() bool
	InCheckout() bool
	ReachedCheckout() bool
	HasExited() bool
	MarkQueued() error
	MarkInCheckout() error
	MarkExited() error

	AdvisePollAfter(t time.Time) error
	AdvisedPollAfter() time.Time

	ThrottleState() ThrottleState
	SessionData() network_mock.SessionData

	SendInitialCheckoutRequest()
	StartPolling()
	StopPolling() error
	HandleResponse(resp *network_mock.MockResponse) error
}
