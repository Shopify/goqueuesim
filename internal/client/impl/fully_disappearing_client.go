package impl

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/network_mock"
)

// FullyDisappearingClient will stop polling entirely after some input # iterations.
type FullyDisappearingClient struct {
	*BaseClient
	PollsBeforeDisappearing int
}

func (fdc *FullyDisappearingClient) StartPolling() {
	if fdc.StartedPolling || !fdc.IsQueued() || !fdc.isLocked {
		return
	}
	pollsSoFar := 0
	if fdc.PollsBeforeDisappearing >= 1 {
		fdc.makeSinglePollRequest()
		if !fdc.StartedPolling {
			return
		}
		pollsSoFar += 1
	}
	fdc.PollStopper = make(chan struct{})
	fdc.pollTimer = time.NewTimer(fdc.DefaultPollInterval)
	go func() {
		for {
			select {
			case <-fdc.Ctx.Done():
				return
			case <-fdc.PollStopper:
				return
			case <-fdc.pollTimer.C:
				if pollsSoFar >= fdc.PollsBeforeDisappearing {
					fdc.Lock()
					_ = fdc.MarkExited()
					fdc.StopPolling()
					labelTag := fmt.Sprintf("client_label:%s", fdc.Label())
					metrics.Incr("server.checkout", []string{"operation:vanished", labelTag})
					fdc.Unlock()
					return
				}
				time.Sleep(time.Duration(rand.Intn(fdc.MaxNetworkJitterMs)) * time.Millisecond)
				fdc.Lock()
				if !fdc.IsQueued() {
					fdc.StopPolling()
					fdc.Unlock()
					return
				}
				fdc.makeSinglePollRequest()
				fdc.Unlock()
				fdc.pollTimer.Reset(fdc.DefaultPollInterval)
				pollsSoFar += 1
			}
		}
	}()
}

func (fdc *FullyDisappearingClient) HandleResponse(resp *network_mock.MockResponse) error {
	if !fdc.isLocked {
		return errors.New("lock required to handle server response")
	}
	fdc.delegateResponseTo(fdc, resp)
	return nil
}
