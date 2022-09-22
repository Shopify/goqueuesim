package impl

import (
	"errors"
	"math/rand"
	"time"

	"github.com/Shopify/goqueuesim/internal/network_mock"
)

type JitGreedyPoller struct {
	*BaseClient
	windowCheaterOnly bool
	greedyPollsCount  int
	greedyPollsDelay  time.Duration
	earlyPollLeadTime time.Duration
	windowDur         time.Duration
}

func (jitgp *JitGreedyPoller) StartPolling() {
	if jitgp.StartedPolling || !jitgp.isLocked {
		return
	}
	jitgp.makeSinglePollRequest()
	if !jitgp.StartedPolling {
		return
	}
	jitgp.PollStopper = make(chan struct{})
	jitWindowDur := (jitgp.windowDur - jitgp.earlyPollLeadTime) % jitgp.windowDur
	jitgp.pollTimer = time.NewTimer(jitWindowDur)
	go func() {
		for {
			select {
			case <-jitgp.Ctx.Done():
				return
			case <-jitgp.PollStopper:
				return
			case <-jitgp.pollTimer.C:
				time.Sleep(time.Duration(rand.Intn(jitgp.MaxNetworkJitterMs)) * time.Millisecond)
				jitgp.Lock()
				if !jitgp.IsQueued() {
					jitgp.StopPolling()
					jitgp.Unlock()
					return
				}
				if jitgp.windowCheaterOnly || jitgp.AdvisedPollAfter().IsZero() {
					jitgp.makeSinglePollRequest()
					jitgp.Unlock()
					jitgp.doGreedyPolls()
					jitgp.pollTimer.Reset(jitWindowDur)
					continue
				}
				earlyPollTime := jitgp.AdvisedPollAfter().Add(-jitgp.earlyPollLeadTime)
				remTimeToPoll := earlyPollTime.Sub(time.Now())
				if remTimeToPoll > 0 {
					time.Sleep(remTimeToPoll)
				}
				jitgp.makeSinglePollRequest()
				jitgp.Unlock()
				jitgp.doGreedyPolls()
				jitgp.pollTimer.Reset(jitWindowDur)
			}
		}
	}()
}

func (jitgp *JitGreedyPoller) doGreedyPolls() {
	for reps := 1; reps < jitgp.greedyPollsCount; reps++ {
		jitter := time.Duration(rand.Intn(jitgp.MaxNetworkJitterMs)) * time.Millisecond
		time.Sleep(jitgp.greedyPollsDelay + jitter)
		jitgp.Lock()
		if !jitgp.IsQueued() {
			jitgp.StopPolling()
			jitgp.Unlock()
			return
		}
		jitgp.makeSinglePollRequest()
		jitgp.Unlock()
	}
}

func (jitgp *JitGreedyPoller) HandleResponse(resp *network_mock.MockResponse) error {
	if !jitgp.isLocked {
		return errors.New("lock required to handle server response")
	}
	jitgp.delegateResponseTo(jitgp, resp)
	return nil
}
