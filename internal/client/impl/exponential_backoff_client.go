package impl

import (
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/Shopify/goqueuesim/internal/network_mock"
)

type ExponentialBackoffClient struct {
	*BaseClient
	MaximumBackoff time.Duration
}

func (ebc *ExponentialBackoffClient) StartPolling() {
	if ebc.StartedPolling || !ebc.isLocked {
		return
	}
	pollsSoFar := 0
	ebc.makeSinglePollRequest()
	pollsSoFar++
	if !ebc.StartedPolling {
		return
	}
	ebc.PollStopper = make(chan struct{})
	ebc.pollTimer = time.NewTimer(ebc.DefaultPollInterval)
	go func() {
		for {
			select {
			case <-ebc.Ctx.Done():
				return
			case <-ebc.PollStopper:
				return
			case <-ebc.pollTimer.C:
				time.Sleep(time.Duration(rand.Intn(ebc.MaxNetworkJitterMs)) * time.Millisecond)
				ebc.Lock()
				if !ebc.IsQueued() {
					ebc.StopPolling()
					ebc.Unlock()
					return
				}
				ebc.makeSinglePollRequest()
				ebc.Unlock()
				// Sleep for min(((2^pollsSoFar)*default_interval), maximum_backoff) + random_jitter.
				backoffSecs := math.Pow(2, float64(pollsSoFar)) * ebc.DefaultPollInterval.Seconds()
				cappedBackoffSecs := math.Min(backoffSecs, ebc.MaximumBackoff.Seconds())
				backoffDur := time.Duration(cappedBackoffSecs) * time.Second
				ebc.pollTimer.Reset(backoffDur)
			}
		}
	}()
}

func (ebc *ExponentialBackoffClient) HandleResponse(resp *network_mock.MockResponse) error {
	if !ebc.isLocked {
		return errors.New("lock required to handle server response")
	}
	ebc.delegateResponseTo(ebc, resp)
	return nil
}
