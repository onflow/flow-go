package node_builder

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
	cancel              context.CancelFunc
}

func newUpstreamConnector(bootstrapIdentities flow.IdentityList, unstakedNode *p2p.Node, logger zerolog.Logger) *upstreamConnector {
	return &upstreamConnector{
		lm:                  lifecycle.NewLifecycleManager(),
		bootstrapIdentities: bootstrapIdentities,
		unstakedNode:        unstakedNode,
		logger:              logger,
	}
}
func (connector *upstreamConnector) Ready() <-chan struct{} {
	connector.lm.OnStart(func() {
		select {
		case <-connector.unstakedNode.Start():
			connector.logger.Debug().Msg("libp2p node starts successfully")
		case <-time.After(30 * time.Second):
			connector.logger.Fatal().Msg("could not start libp2p node within timeout")
		}

		// eventually, context will be passed in to Start method: https://github.com/dapperlabs/flow-go/issues/5730
		ctx, cancel := context.WithCancel(context.TODO())
		connector.cancel = cancel

		bootstrapPeerCnt := len(connector.bootstrapIdentities)
		resultChan := make(chan result, bootstrapPeerCnt)
		defer close(resultChan)

		// a shorter context for the connection worker
		workerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// spawn a connect worker for each bootstrap node
		for _, b := range connector.bootstrapIdentities {
			go connector.connect(workerCtx, *b, resultChan)
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
			case <-workerCtx.Done():
				connector.logger.Warn().Msg("timed out before connections to bootstrap nodes could be established")
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

// result is the result returned by the connect worker. The ID is the ID of the bootstrap peer that was attempted,
// err is the error that occurred or nil
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

	peerAddrInfo, err := p2p.PeerAddressInfo(bootstrapPeer)

	if err != nil {
		resultChan <- result{
			id:  flow.Identity{},
			err: err,
		}
	}

	// try and connect to the bootstrap server
	err = connector.unstakedNode.AddPeer(ctx, peerAddrInfo)
	resultChan <- result{
		id:  bootstrapPeer,
		err: err,
	}

}

func (connector *upstreamConnector) Done() <-chan struct{} {
	connector.lm.OnStop(func() {
		// this function will only be executed if connector.lm.OnStart was previously called,
		// in which case connector.cancel != nil
		connector.cancel()
	})
	return connector.lm.Stopped()
}
