package node_builder

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
)

var _ component.Component = (*upstreamConnector)(nil)

// upstreamConnector tries to connect the unstaked AN with at least one of the configured bootstrap access nodes
type upstreamConnector struct {
	bootstrapIdentities flow.IdentityList
	logger              zerolog.Logger
	unstakedNode        *p2p.Node
	*component.ComponentManager
}

func newUpstreamConnector(bootstrapIdentities flow.IdentityList, unstakedNode *p2p.Node, logger zerolog.Logger) *upstreamConnector {

	connector := &upstreamConnector{
		bootstrapIdentities: bootstrapIdentities,
		unstakedNode:        unstakedNode,
		logger:              logger,
	}

	connector.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			go connector.onStart(ctx)

			ready()

			<-ctx.Done()
		}).Build()

	return connector
}

func (connector *upstreamConnector) onStart(parent irrecoverable.SignalerContext) {

	bootstrapPeerCnt := len(connector.bootstrapIdentities)
	resultChan := make(chan result, bootstrapPeerCnt)
	defer close(resultChan)

	// a shorter context for the connection worker
	workerCtx, workerCancel := context.WithTimeout(parent, 30*time.Second)
	defer workerCancel()

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
			connector.logger.Warn().Msg("timed out or cancelled before connections to bootstrap nodes could be established")
			return
		}
	}

	// if there was at least one successful connect, then return no error
	if len(successfulConnects) > 0 {
		connector.logger.Info().Strs("bootstrap_peers", successfulConnects).Msg("successfully connected to bootstrap node")
		return
	}

	err := errors.ErrorOrNil()
	// no point continuing further since the unstaked AN cannot connect to any of the bootstrap peers

	parent.Throw(irrecoverable.NewRecoverableError(fmt.Errorf("Failed to connect to a bootstrap node. "+
		"Please ensure the network address and public key of the bootstrap access node are correct "+
		"and that the node is running and reachable. error: %w", err)))

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
