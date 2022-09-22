package impl

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/network_mock"
)

// LazyPollingClient has a random chance to go inactive and sleep for a random duration.
type LazyPollingClient struct {
	*BaseClient
	SkipPollingRoundProbabilityPct float64
	MaxSleepMs                     int
}

func (lpc *LazyPollingClient) StartPolling() {
	if lpc.StartedPolling || !lpc.isLocked {
		return
	}
	lpc.makeSinglePollRequest()
	if !lpc.StartedPolling {
		return
	}
	lpc.PollStopper = make(chan struct{})
	lpc.pollTimer = time.NewTimer(lpc.DefaultPollInterval)
	go func() {
		for {
			select {
			case <-lpc.Ctx.Done():
				return
			case <-lpc.PollStopper:
				return
			case <-lpc.pollTimer.C:
				if rand.Float64() < lpc.SkipPollingRoundProbabilityPct {
					labelTag := fmt.Sprintf("client_label:%s", lpc.Label())
					metrics.Incr("server.checkout", []string{"operation:temp_exit", labelTag})
					inactiveDur := time.Duration(rand.Intn(lpc.MaxSleepMs)) * time.Millisecond
					lpc.pollTimer.Reset(inactiveDur)
					continue
				}
				time.Sleep(time.Duration(rand.Intn(lpc.MaxNetworkJitterMs)) * time.Millisecond)
				lpc.Lock()
				if !lpc.IsQueued() {
					lpc.StopPolling()
					lpc.Unlock()
					return
				}
				lpc.makeSinglePollRequest()
				lpc.Unlock()
				lpc.pollTimer.Reset(lpc.DefaultPollInterval)
			}
		}
	}()
}

func (lpc *LazyPollingClient) HandleResponse(resp *network_mock.MockResponse) error {
	if !lpc.isLocked {
		return errors.New("lock required to handle server response")
	}
	lpc.delegateResponseTo(lpc, resp)
	return nil
}
