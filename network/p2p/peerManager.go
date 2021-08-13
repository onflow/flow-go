package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
)

// Connector connects to peer and disconnects from peer using the underlying networking library
type Connector interface {

	// UpdatePeers connects to the given flow.Identities and returns a map of identifiers which failed. It also
	// disconnects from any other peers with which it may have previously established connection.
	// UpdatePeers implementation should be idempotent such that multiple calls to connect to the same peer should not
	// return an error or create multiple connections
	UpdatePeers(ctx context.Context, ids flow.IdentityList) error
}

// DefaultPeerUpdateInterval is default duration for which the peer manager waits in between attempts to update peer connections
var DefaultPeerUpdateInterval = 10 * time.Minute

// PeerManager adds and removes connections to peers periodically and on request
type PeerManager struct {
	unit               *engine.Unit
	logger             zerolog.Logger
	idsProvider        func() (flow.IdentityList, error) // callback to retrieve list of peers to connect to
	peerRequestQ       chan struct{}                     // a channel to queue a peer update request
	connector          Connector                         // connector to connect or disconnect from peers
	peerUpdateInterval time.Duration                     // interval the peer manager runs on
}

// Option represents an option for the peer manager.
type Option func(*PeerManager)

func WithInterval(period time.Duration) Option {
	return func(pm *PeerManager) {
		pm.peerUpdateInterval = period
	}
}

// NewPeerManager creates a new peer manager which calls the idsProvider callback to get a list of peers to connect to
// and it uses the connector to actually connect or disconnect from peers.
func NewPeerManager(logger zerolog.Logger, idsProvider func() (flow.IdentityList, error),
	connector Connector, options ...Option) *PeerManager {
	pm := &PeerManager{
		unit:         engine.NewUnit(),
		logger:       logger,
		idsProvider:  idsProvider,
		connector:    connector,
		peerRequestQ: make(chan struct{}, 1),
	}
	// apply options
	for _, o := range options {
		o(pm)
	}
	return pm
}

// Ready kicks off the ambient periodic connection updates.
func (pm *PeerManager) Ready() <-chan struct{} {
	// makes sure that peer update request is invoked
	// once before returning
	pm.RequestPeerUpdate()

	// also starts running it periodically
	pm.unit.LaunchPeriodically(pm.RequestPeerUpdate, pm.peerUpdateInterval, time.Duration(0))

	pm.unit.Launch(pm.updateLoop)

	return pm.unit.Ready()
}

func (pm *PeerManager) Done() <-chan struct{} {
	return pm.unit.Done()
}

// updateLoop triggers an update peer request when it has been requested
func (pm *PeerManager) updateLoop() {
	for {
		select {
		case <-pm.peerRequestQ:
			pm.updatePeers()
		case <-pm.unit.Quit():
			return
		}
	}
}

// RequestPeerUpdate requests an update to the peer connections of this node.
// If a peer update has already been requested (either as a periodic request or an on-demand request) and is outstanding,
// then this call is a no-op.
func (pm *PeerManager) RequestPeerUpdate() {
	select {
	case pm.peerRequestQ <- struct{}{}:
	default:
	}
}

// updatePeers updates the peers by connecting to all the nodes provided by the idsProvider callback and disconnecting from
// previous nodes that are no longer in the new list of nodes.
func (pm *PeerManager) updatePeers() {

	// get all the ids to connect to
	ids, err := pm.idsProvider()
	if err != nil {
		pm.logger.Error().Err(err).Msg("failed to update peers")
		return
	}

	pm.logger.Trace().
		Str("peers", fmt.Sprintf("%v", ids.NodeIDs())).
		Msg("connecting to peers")

	// ask the connector to connect to all peers in the list
	err = pm.connector.UpdatePeers(pm.unit.Ctx(), ids)
	if err != nil {
		// one of more identities in the identity table could not be connected to
		pm.logger.Error().Err(err).Msg("failed to connect to one or more peers")
	}
}
