package p2p

import (
	"context"
	"fmt"
	mrand "math/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
)

// Connector connects to peer and disconnects from peer using the underlying networking library
type Connector interface {

	// UpdatePeers connects to the given peer.IDs and returns a map of peers which failed. It also
	// disconnects from any other peers with which it may have previously established connection.
	// UpdatePeers implementation should be idempotent such that multiple calls to connect to the same peer should not
	// return an error or create multiple connections
	UpdatePeers(ctx context.Context, peerIDs peer.IDSlice)
}

// DefaultPeerUpdateInterval is default duration for which the peer manager waits in between attempts to update peer connections
var DefaultPeerUpdateInterval = 10 * time.Minute

// PeerManager adds and removes connections to peers periodically and on request
type PeerManager struct {
	unit               *engine.Unit
	logger             zerolog.Logger
	peersProvider      func() (peer.IDSlice, error) // callback to retrieve list of peers to connect to
	peerRequestQ       chan struct{}                // a channel to queue a peer update request
	connector          Connector                    // connector to connect or disconnect from peers
	peerUpdateInterval time.Duration                // interval the peer manager runs on
}

// Option represents an option for the peer manager.
type Option func(*PeerManager)

func WithInterval(period time.Duration) Option {
	return func(pm *PeerManager) {
		pm.peerUpdateInterval = period
	}
}

type PeersProvider func() (peer.IDSlice, error)

// NewPeerManager creates a new peer manager which calls the peersProvider callback to get a list of peers to connect to
// and it uses the connector to actually connect or disconnect from peers.
func NewPeerManager(logger zerolog.Logger, peersProvider PeersProvider,
	connector Connector, options ...Option) *PeerManager {
	pm := &PeerManager{
		unit:               engine.NewUnit(),
		logger:             logger,
		peersProvider:      peersProvider,
		connector:          connector,
		peerRequestQ:       make(chan struct{}, 1),
		peerUpdateInterval: DefaultPeerUpdateInterval,
	}
	// apply options
	for _, o := range options {
		o(pm)
	}
	return pm
}

// PeerManagerFactoryFunc is a factory function type for generating a PeerManager instance using the given host,
// peersProvider and logger
type PeerManagerFactoryFunc func(host host.Host, peersProvider PeersProvider, logger zerolog.Logger) (*PeerManager, error)

// PeerManagerFactory generates a PeerManagerFunc that produces the default PeerManager with the given peer manager
// options and that uses the LibP2PConnector with the given LibP2P connector options
func PeerManagerFactory(peerManagerOptions []Option, connectorOptions ...ConnectorOption) PeerManagerFactoryFunc {
	return func(host host.Host, peersProvider PeersProvider, logger zerolog.Logger) (*PeerManager, error) {
		connector, err := NewLibp2pConnector(host, logger, connectorOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2pConnector: %w", err)
		}
		return NewPeerManager(logger, peersProvider, connector, peerManagerOptions...), nil
	}
}

// Ready kicks off the ambient periodic connection updates.
func (pm *PeerManager) Ready() <-chan struct{} {
	pm.unit.Launch(pm.updateLoop)

	// makes sure that peer update request is invoked once before returning
	pm.peerRequestQ <- struct{}{}

	// also starts running it periodically
	//
	// add a random delay to initial launch to avoid synchronizing this
	// potentially expensive operation across the network
	delay := time.Duration(mrand.Int63n(pm.peerUpdateInterval.Nanoseconds()))
	pm.unit.LaunchPeriodically(pm.RequestPeerUpdate, pm.peerUpdateInterval, delay)

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

// updatePeers updates the peers by connecting to all the nodes provided by the peersProvider callback and disconnecting from
// previous nodes that are no longer in the new list of nodes.
func (pm *PeerManager) updatePeers() {

	// get all the peer ids to connect to
	peers, err := pm.peersProvider()
	if err != nil {
		pm.logger.Error().Err(err).Msg("failed to update peers")
		return
	}

	pm.logger.Trace().
		Str("peers", fmt.Sprintf("%v", peers)).
		Msg("connecting to peers")

	// ask the connector to connect to all peers in the list
	pm.connector.UpdatePeers(pm.unit.Ctx(), peers)
}
