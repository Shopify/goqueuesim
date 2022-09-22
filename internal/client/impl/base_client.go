package impl

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/network_mock"
	"github.com/rs/zerolog/log"
)

type BaseClient struct {
	Ctx   context.Context
	Id    int
	label string

	ClientsFinishedWaitGroup *sync.WaitGroup
	RequestTargetChannel     chan<- *network_mock.MockRequest

	DefaultPollInterval time.Duration
	MaxInitialDelayMs   int
	MaxNetworkJitterMs  int
	ObeysPollAfter      bool

	StartedPolling bool
	PollStopper    chan struct{} // Channel closed to stop polling.

	HitCheckoutStep bool

	advisedPollAfter time.Time
	state            client.ThrottleState

	queueEntryTime time.Time
	queueExitTime  time.Time

	mutex     sync.RWMutex
	isLocked  bool
	pollTimer *time.Timer

	throttleCookie map[string]string // Analogous to "key1=value1;...;keyN=valueN" cookie.
}

func (bc *BaseClient) ID() int {
	return bc.Id

}
func (bc *BaseClient) Label() string {
	return bc.label
}

func (bc *BaseClient) Lock() {
	bc.mutex.Lock()
	bc.isLocked = true
}

func (bc *BaseClient) Unlock() {
	bc.isLocked = false
	bc.mutex.Unlock()
}

func (bc *BaseClient) QueueEntryTime() time.Time {
	return bc.queueEntryTime
}

func (bc *BaseClient) QueueExitTime() time.Time {
	return bc.queueExitTime
}

func (bc *BaseClient) QueueDuration() time.Duration {
	return bc.queueExitTime.Sub(bc.queueEntryTime)
}

func (bc *BaseClient) SetThrottleCookieVal(key, value string) error {
	if !bc.isLocked {
		return errors.New("lock required to mutate throttle cookie")
	}
	if bc.throttleCookie == nil {
		bc.throttleCookie = make(map[string]string)
	}
	bc.throttleCookie[key] = value
	return nil
}

func (bc *BaseClient) GetThrottleCookieVal(key string) (value string, ok bool) {
	if !bc.isLocked {
		return "", false
	}
	if bc.throttleCookie == nil {
		value, ok = "", false
	} else {
		value, ok = bc.throttleCookie[key]
	}
	return value, ok
}

func (bc *BaseClient) RemoveThrottleCookieVal(key string) error {
	if !bc.isLocked {
		return errors.New("lock required to mutate throttle cookie")
	}
	if bc.throttleCookie != nil {
		delete(bc.throttleCookie, key)
	}
	return nil
}

func (bc *BaseClient) ClearThrottleCookie() error {
	if !bc.isLocked {
		return errors.New("lock required to mutate throttle cookie")
	}
	bc.throttleCookie = make(map[string]string)
	return nil
}

func (bc *BaseClient) IsQueued() bool {
	return bc.state == client.Queued
}

func (bc *BaseClient) MarkQueued() error {
	if !bc.isLocked {
		return errors.New("lock required to mutate throttle state")
	}
	bc.queueEntryTime = time.Now()
	bc.state = client.Queued
	bc.SetThrottleCookieVal(client.ThrottleStateKey, client.Queued.String())
	return nil
}

func (bc *BaseClient) InCheckout() bool {
	return bc.state == client.InCheckout
}

func (bc *BaseClient) ReachedCheckout() bool {
	return bc.HitCheckoutStep
}

func (bc *BaseClient) MarkInCheckout() error {
	if !bc.isLocked {
		return errors.New("lock required to mutate throttle state")
	}
	bc.state = client.InCheckout
	bc.SetThrottleCookieVal(client.ThrottleStateKey, client.InCheckout.String())
	bc.queueExitTime = time.Now()
	bc.HitCheckoutStep = true
	return nil
}

func (bc *BaseClient) HasExited() bool {
	return bc.state == client.Exited
}

func (bc *BaseClient) AdvisePollAfter(t time.Time) error {
	if !bc.isLocked {
		return errors.New("lock required to mutate advised poll after")
	}
	bc.advisedPollAfter = t
	return nil
}

func (bc *BaseClient) AdvisedPollAfter() time.Time {
	return bc.advisedPollAfter
}

func (bc *BaseClient) MarkExited() error {
	if !bc.isLocked {
		return errors.New("lock required to mutate throttle state")
	}
	bc.state = client.Exited
	_ = bc.SetThrottleCookieVal(client.ThrottleStateKey, client.Exited.String())
	bc.ClientsFinishedWaitGroup.Done()
	return nil
}

func (bc *BaseClient) ThrottleState() client.ThrottleState {
	return bc.state
}

func (bc *BaseClient) SessionData() network_mock.SessionData {
	return network_mock.SessionData{
		Id:               bc.ID(),
		QueueEntryTime:   bc.QueueEntryTime(),
		AdvisedPollAfter: bc.AdvisedPollAfter(),
		ThrottleState:    bc.ThrottleState().String(),
		ThrottleCookie:   bc.throttleCookie,
	}
}

func (bc *BaseClient) dieOnRequestTimeout(request *network_mock.MockRequest) {
	select {
	case bc.RequestTargetChannel <- request:
	default:
		log.Debug().Msg(fmt.Sprintf("Failed to send request...killing client %d\n", bc.ID()))
		bc.MarkExited()
		bc.StopPolling()
		labelTag := fmt.Sprintf("client_label:%s", bc.Label())
		metrics.Incr("server.timeout", []string{"operation:timeout", labelTag})
	}
}

func (bc *BaseClient) SendInitialCheckoutRequest() {
	time.Sleep(time.Duration(rand.Intn(bc.MaxInitialDelayMs)) * time.Millisecond)
	checkoutRequest := network_mock.MakeCheckoutRequest(bc.SessionData())
	bc.dieOnRequestTimeout(checkoutRequest)
}

func (bc *BaseClient) makeSinglePollRequest() error {
	if !bc.isLocked {
		return errors.New("lock required to mutate polling state")
	}
	if !bc.IsQueued() {
		bc.StopPolling()
		return nil
	}
	log.Info().Int("client_id", bc.Id).Msg("sending poll request...")
	bc.StartedPolling = true
	pollingRequest := network_mock.MakePollRequest(bc.SessionData())
	bc.dieOnRequestTimeout(pollingRequest)
	return nil
}

func (bc *BaseClient) StartPolling() {
	if bc.StartedPolling || !bc.isLocked {
		return
	}
	bc.makeSinglePollRequest()
	if !bc.StartedPolling {
		return
	}
	bc.PollStopper = make(chan struct{})
	bc.pollTimer = time.NewTimer(bc.DefaultPollInterval)
	go func() {
		for {
			select {
			case <-bc.Ctx.Done():
				return
			case <-bc.PollStopper:
				return
			case <-bc.pollTimer.C:
				time.Sleep(time.Duration(rand.Intn(bc.MaxNetworkJitterMs)) * time.Millisecond)
				bc.Lock()
				if !bc.IsQueued() {
					bc.StopPolling()
					bc.Unlock()
					return
				}
				willObeyPollAfter := bc.ObeysPollAfter && !bc.AdvisedPollAfter().IsZero()
				if willObeyPollAfter && time.Now().Before(bc.AdvisedPollAfter()) {
					bc.makeSinglePollRequest()
					bc.Unlock()
					bc.pollTimer.Reset(time.Until(bc.AdvisedPollAfter()))
					continue
				}
				bc.makeSinglePollRequest()
				bc.Unlock()
				bc.pollTimer.Reset(bc.DefaultPollInterval)
			}
		}
	}()
}

func (bc *BaseClient) StopPolling() error {
	if !bc.isLocked {
		return errors.New("lock required to mutate polling state")
	}
	if !bc.StartedPolling {
		return nil
	}
	bc.StartedPolling = false
	bc.pollTimer.Stop()
	close(bc.PollStopper)
	return nil
}

func (bc *BaseClient) delegateResponseTo(c client.Client, resp *network_mock.MockResponse) {
	clientState := c.ThrottleState()
	if resp.ClientData.ThrottleState != clientState.String() {
		panic("Asymmetric data: server failed to mark target response state on client")
	}
	if !resp.ClientData.AdvisedPollAfter.IsZero() {
		c.AdvisePollAfter(resp.ClientData.AdvisedPollAfter)
	}
	switch clientState {
	case client.Initial:
		log.Info().Int("client_id", c.ID()).Msg("sending initial checkout request...")
		c.SendInitialCheckoutRequest()
	case client.Queued:
		log.Info().Int("client_id", c.ID()).Msg("started polling...")
		c.StartPolling()
	case client.InCheckout:
		log.Info().Int("client_id", c.ID()).Msg("stopped polling")
		_ = c.MarkExited()
		c.StopPolling()
		labelTag := fmt.Sprintf("client_label:%s", c.Label())
		metrics.Incr("server.checkout", []string{"operation:success", labelTag})
		metrics.Distribution(
			"client.queue_time_ms",
			float64(c.QueueDuration().Milliseconds()),
			[]string{"operation:checkout_successful", labelTag},
		)
	case client.Exited:
		return
	default:
		panic("Encountered unrecognized state set on client")
	}
}

func (bc *BaseClient) HandleResponse(resp *network_mock.MockResponse) error {
	if !bc.isLocked {
		return errors.New("lock required to handle server response")
	}
	bc.delegateResponseTo(bc, resp)
	return nil
}
