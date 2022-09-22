package simulator

import (
	"context"
	"fmt"
	"sync"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/metrics"
	"github.com/Shopify/goqueuesim/internal/network_mock"
	"github.com/Shopify/goqueuesim/internal/throttle"
)

type SimulationDriver struct {
	Ctx                    context.Context
	CtxCancelFunc          context.CancelFunc
	CheckoutThrottleDriver *throttle.CheckoutThrottleDriver

	Clients             []client.Client
	NumServerWorkers    int
	InventoryStockTotal int

	RequestTargetChannel chan *network_mock.MockRequest
	ResponseChannelsMap  map[int]chan *network_mock.MockResponse

	// Listener channel awaiting message to notify that simulation has ended.
	SimulationCompletedListenerChannel chan struct{}

	StartSignalWaitGroup     *sync.WaitGroup
	ClientsFinishedWaitGroup *sync.WaitGroup

	// Mutex-guarded dict for { clientId -> client }
	ClientRepo ClientRepo

	MaxUnfairnessToleranceSeconds float64
}

func (d *SimulationDriver) StartSimulation() {
	go d.monitorSimulationCompletion()

	for w := 0; w < d.NumServerWorkers; w++ {
		go d.runServerWorker()
	}
	d.startClients()
	d.StartSignalWaitGroup.Done()

	// Block on postprocessing until simulation completes.
	<-d.SimulationCompletedListenerChannel
	d.aggregateFairnessResults()
}

// Blocking routine to monitor simulation completion (either context cancelled or clients completed).
func (d *SimulationDriver) monitorSimulationCompletion() {
	go func() { <-d.Ctx.Done(); close(d.SimulationCompletedListenerChannel); return }()
	d.ClientsFinishedWaitGroup.Wait()
	close(d.SimulationCompletedListenerChannel)
}

func (d *SimulationDriver) runServerWorker() {
	for { // loop infinitely
		select { // block waiting to handle client request
		case req := <-d.RequestTargetChannel:
			c := d.ClientRepo.FetchClientById(req.ClientData.Id)
			c.Lock()
			switch req.Endpoint {
			case network_mock.CheckoutEndpoint:
				metrics.Incr("server.requests", []string{"endpoint:checkout"})
			case network_mock.PollingEndpoint:
				metrics.Incr("server.requests", []string{"endpoint:poll"})
			}
			d.CheckoutThrottleDriver.TryThrottleStateTransition(c)
			d.ResponseChannelsMap[c.ID()] <- network_mock.MakeServerResponse(c.SessionData())
			c.Unlock()
		}
	}
}

func (d *SimulationDriver) startClients() {
	for i := 0; i < len(d.Clients); i++ {
		c := d.Clients[i]
		d.ClientRepo.WriteClient(c)
		// Provide initial server handshake -> then kickstart client worker.
		d.ResponseChannelsMap[c.ID()] <- network_mock.MakeServerResponse(c.SessionData())
		go d.runClientWorker(c)
	}
}

func (d *SimulationDriver) runClientWorker(c client.Client) {
	serverRespChannel := d.ResponseChannelsMap[c.ID()]
	d.StartSignalWaitGroup.Wait()
	for { // loop infinitely
		select { // block waiting to handle server response
		case resp := <-serverRespChannel:
			c.Lock()
			_ = c.HandleResponse(resp)
			c.Unlock()
		}
	}
}

func (d *SimulationDriver) aggregateFairnessResults() {
	clientsSubsetReachedCheckout := make([]client.Client, 0)
	for _, c := range d.Clients {
		if c.ReachedCheckout() {
			clientsSubsetReachedCheckout = append(clientsSubsetReachedCheckout, c)
		}
	}

	// We will calculate fairness tolerance for each client. For every client X, find client Y such that:
	// -> entryTime(X) < entryTime(Y)
	// -> exitTime(X) > exitTime(Y)
	// -> entryTime(Y) - entryTime(X) is maximized
	// If no such client exists, then max unfairness is 0 (very fair).
	numUnfairEvents := 0
	uniqueCheatedClients := make(map[int]bool)
	uniqueUnfairClients := make(map[int]bool)
	numCheatedClientsByLabel := make(map[string]int)
	numUnfairClientsByLabel := make(map[string]int)
	summedUnfairnessSecs := 0.0
	globalMaxUnfairnessSecs := 0.0
	var globalMaxCheatedClient, globalMaxUnfairClient client.Client
	for _, cx := range clientsSubsetReachedCheckout {
		localMaxUnfairnessSecs := float64(0)
		maxCy := cx
		for _, cy := range clientsSubsetReachedCheckout {
			lateArrivingClient := cy.QueueEntryTime().After(cx.QueueEntryTime())
			if lateArrivingClient && cy.QueueExitTime().Before(cx.QueueExitTime()) {
				numUnfairEvents++
				if !uniqueCheatedClients[cx.ID()] {
					numCheatedClientsByLabel[cx.Label()]++
				}
				if !uniqueUnfairClients[cy.ID()] {
					numUnfairClientsByLabel[cy.Label()]++
				}
				uniqueCheatedClients[cx.ID()] = true
				uniqueUnfairClients[cy.ID()] = true
				unfairDuration := cy.QueueEntryTime().Sub(cx.QueueEntryTime()).Seconds()
				summedUnfairnessSecs += unfairDuration
				if unfairDuration > localMaxUnfairnessSecs {
					maxCy = cy
					localMaxUnfairnessSecs = unfairDuration
				}
			}
		}

		// TODO: should we gauge something with max unfairness threshold?

		if localMaxUnfairnessSecs > globalMaxUnfairnessSecs {
			globalMaxUnfairnessSecs = localMaxUnfairnessSecs
			globalMaxCheatedClient = cx
			globalMaxUnfairClient = maxCy
		}
		cheatedClientLabel := fmt.Sprintf("cheated_client_label:%s", cx.Label())
		unfairClientLabel := fmt.Sprintf("unfair_client_label:%s", maxCy.Label())
		metrics.Distribution(
			"cheated_client.local_max_unfairness_seconds",
			localMaxUnfairnessSecs,
			[]string{cheatedClientLabel, unfairClientLabel},
		)
	}
	for label, count := range numCheatedClientsByLabel {
		metrics.Gauge(
			"total_cheated_clients",
			float64(count),
			[]string{fmt.Sprintf("cheated_client_label:%s", label)},
		)
	}
	for label, count := range numUnfairClientsByLabel {
		metrics.Gauge(
			"total_unfair_clients",
			float64(count),
			[]string{fmt.Sprintf("unfair_client_label:%s", label)},
		)
	}
	for i := 0; i < 50; i++ { // Ensure this message is notified.
		metrics.Gauge("global_max_unfair_seconds", globalMaxUnfairnessSecs, nil)
	}
	avgUnfairnessSecs := summedUnfairnessSecs / float64(numUnfairEvents)
	fmt.Printf("\nnum_unfair_events=%d\nnum_cheated_clients=%d", numUnfairEvents, len(uniqueCheatedClients))
	fmt.Printf("\nnum_unfair_clients=%d\n", len(uniqueUnfairClients))
	fmt.Printf("\nmax_unfair_secs=%.2f\navg_unfair_secs=%.2f\n", globalMaxUnfairnessSecs, avgUnfairnessSecs)
	labelX, labelY := globalMaxCheatedClient.Label(), globalMaxUnfairClient.Label()
	fmt.Printf("\nmax_unfair_client_type=%s\nmax_cheated_client_type=%s", labelX, labelY)
}
