package main

import (
	"context"
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

		bootstrapPeerCnt := len(connector.bootstrapIdentities)
		resultChan := make(chan result, bootstrapPeerCnt)
		defer close(resultChan)

		// a shorter context for the connection worker
		ctx, cancel := context.WithTimeout(connector.ctx, 5*time.Second)
		defer cancel()

		// spawn a connect worker for each bootstrap node
		for _, b := range connector.bootstrapIdentities {
			go connector.connect(ctx, *b, resultChan)
		}

		var successfulConnects []string
		var errors *multierror.Error

		// wait for all connect workers to finish or the context to be done
		for i := 0; i < bootstrapPeerCnt; i++ {
			select {
			// gather all successes
			case result := <-resultChan:
				if result.err != nil {
					connector.logger.Error().Str("bootstap_node", result.id.String()).Err(result.err).Msg("failed to connect")
					// gather all the errors
					errors = multierror.Append(errors, result.err)
				} else {
					// gather all the successes
					successfulConnects = append(successfulConnects, result.id.String())
				}

				// premature exits if needed
			case <-connector.ctx.Done():
				connector.logger.Warn().Msg("context done before connection to bootstrap node could be established")
				return
			}
		}

		// if there was at least one successful connect, then return no error
		if len(successfulConnects) > 0 {
			connector.logger.Info().Strs("bootstrap_peers", successfulConnects).Msg("successfully connected to bootstrap node")
			return
		}

		err := errors.ErrorOrNil()
		// log fatal as there is no point continuing further  the unstaked AN cannot connect to any of the bootstrap peers
		connector.logger.Fatal().Err(err).
			Msg("Failed to connect to a bootstrap node. " +
				"Please ensure the network address and public key of the bootstrap access node are correct " +
				"and that the node is running and reachable.")
	})
	return connector.lm.Started()
}

type result struct {
	id  flow.Identity
	err error
}

// connect is run in the worker routine to connect to an boostrap peer
// resultChan is used to report the flow.Identity that succeeded or failed along with the error
func (connector *upstreamConnector) connect(ctx context.Context, bootstrapPeer flow.Identity, resultChan chan<- result) {

	select {
	// check for a cancelled/expired context
	case <-ctx.Done():
		return
	default:
	}
	// try and connect to the bootstrap server
	err := connector.unstakedNode.AddPeer(ctx, bootstrapPeer)
	resultChan <- result{
		id:  bootstrapPeer,
		err: err,
	}

}

func (connector *upstreamConnector) Done() <-chan struct{} {
	connector.lm.OnStop(func() {
		connector.cancel()
	})
	return connector.lm.Stopped()
}
