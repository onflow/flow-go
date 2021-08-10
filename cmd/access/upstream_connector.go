package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/network/p2p"
)

// upstreamConnector tries to connect the unstaked AN with atleast one of the configured bootstrap access nodes
type upstreamConnector struct {
	lm                  *lifecycle.LifecycleManager
	bootstrapIdentities flow.IdentityList
	logger              zerolog.Logger
	unstakedNode        *p2p.Node
	ctx                 context.Context
	cancel              context.CancelFunc
}

func newUpstreamConnector(ctx context.Context, bootstrapIdentities flow.IdentityList, unstakedNode *p2p.Node, logger zerolog.Logger) *upstreamConnector {

	ctx, cancel := context.WithCancel(ctx)
	return &upstreamConnector{
		lm:                  lifecycle.NewLifecycleManager(),
		bootstrapIdentities: bootstrapIdentities,
		unstakedNode:        unstakedNode,
		logger:              logger,
		ctx:                 ctx,
		cancel:              cancel,
	}
}
func (connector *upstreamConnector) Ready() <-chan struct{} {
	connector.lm.OnStart(func() {

		successChan := make(chan string, len(connector.bootstrapIdentities))
		defer close(successChan)
		errorChan := make(chan error, len(connector.bootstrapIdentities))
		defer close(errorChan)

		// a shorter context for the connection worker
		ctx, cancel := context.WithTimeout(connector.ctx, time.Second*2)
		defer cancel()

		var wg sync.WaitGroup
		// spawn a connect worker for each bootstrap peer
		for _, b := range connector.bootstrapIdentities {
			wg.Add(1)
			go connector.connect(ctx, &wg, *b, successChan, errorChan)
		}

		allWorkersDone := make(chan struct{})
		go func() {
			defer close(allWorkersDone)
			wg.Done()
		}()

		var successfulConnects []string
		var errors *multierror.Error

		// wait for all connect workers to finish or the context to be done
	ConnectLoop:
		for {
			select {

			// gather all successes
			case id := <-successChan:
				successfulConnects = append(successfulConnects, id)

				// gather all errors
			case err := <-errorChan:
				connector.logger.Err(err)
				errors = multierror.Append(errors, err)

				// if all connect workers are done, break loop
			case <-allWorkersDone:
				break ConnectLoop

				// premature exits if needed
			case <-connector.ctx.Done():
				connector.logger.Warn().Msg("context done before connection to bootstrap peer could be established")
				return
			}
		}

		// if there was atleast one successful connect, then return no error
		if len(successfulConnects) > 0 {
			connector.logger.Info().Strs("bootstrap_peers", successfulConnects).Msg("successfully connected to upstream access node")
			return
		}

		err := errors.ErrorOrNil()
		// log fatal as there is no point continuing further  the unstaked AN cannot connect to any of the bootstrap peers
		connector.logger.Fatal().Err(err).
			Msg("Failed to connect to the staked access node. " +
				"Please ensure the node ID, network address and public key of the staked access node are correct " +
				"and that the staked access node is running and reachable.")
	})
	return connector.lm.Started()
}

// connect is run in the worker routine to connect to an boostrap peer
// successchan is used to report the flow.Identity that succeeded
// errChan is used to report the connection error
func (connector *upstreamConnector) connect(ctx context.Context, wg *sync.WaitGroup, bootstrapPeer flow.Identity, successChan chan<- string, errChan chan<- error) {
	defer wg.Done()
	select {
	// check for a cancelled/expired context
	case <-ctx.Done():
		return
	default:
	}
	err := connector.unstakedNode.AddPeer(ctx, bootstrapPeer)
	if err != nil {
		errChan <- fmt.Errorf("failed to connect to %s: %w", bootstrapPeer.String(), err)
		return
	}
	successChan <- bootstrapPeer.String()
}

func (connector *upstreamConnector) Done() <-chan struct{} {
	connector.lm.OnStop(func() {
		connector.cancel()
	})
	return connector.lm.Stopped()
}
