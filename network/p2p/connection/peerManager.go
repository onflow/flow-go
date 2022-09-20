package connection

import (
	"fmt"
	mrand "math/rand"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/p2p"

	"github.com/onflow/flow-go/engine"
)

// DefaultPeerUpdateInterval is default duration for which the peer manager waits in between attempts to update peer connections
var DefaultPeerUpdateInterval = 10 * time.Minute

// PeerManager adds and removes connections to peers periodically and on request
type PeerManager struct {
	unit               *engine.Unit
	logger             zerolog.Logger
	peersProvider      p2p.PeersProvider // callback to retrieve list of peers to connect to
	peerRequestQ       chan struct{}     // a channel to queue a peer update request
	connector          p2p.Connector     // connector to connect or disconnect from peers
	peerUpdateInterval time.Duration     // interval the peer manager runs on
}

// Option represents an option for the peer manager.
type Option func(*PeerManager)

func WithInterval(period time.Duration) Option {
	return func(pm *PeerManager) {
		pm.peerUpdateInterval = period
	}
}

// NewPeerManager creates a new peer manager which calls the peersProvider callback to get a list of peers to connect to
// and it uses the connector to actually connect or disconnect from peers.
func NewPeerManager(logger zerolog.Logger, updateInterval time.Duration, peersProvider p2p.PeersProvider, connector p2p.Connector) *PeerManager {
	pm := &PeerManager{
		unit:               engine.NewUnit(),
		logger:             logger,
		peersProvider:      peersProvider,
		connector:          connector,
		peerRequestQ:       make(chan struct{}, 1),
		peerUpdateInterval: updateInterval,
	}
	return pm
}

// PeerManagerFactory generates a PeerManagerFunc that produces the default PeerManager with the given peer manager
// options and that uses the LibP2PConnector with the given LibP2P connector options
func PeerManagerFactory(connectionPruning bool, updateInterval time.Duration) p2p.PeerManagerFactoryFunc {
	return func(host host.Host, peersProvider p2p.PeersProvider, logger zerolog.Logger) (p2p.PeerManager, error) {
		connector, err := NewLibp2pConnector(logger, host, connectionPruning)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2pConnector: %w", err)
		}
		return NewPeerManager(logger, updateInterval, peersProvider, connector), nil
	}
}

// Ready kicks off the ambient periodic connection updates.
func (pm *PeerManager) Ready() <-chan struct{} {
	pm.unit.Launch(pm.updateLoop)

	// makes sure that peer update request is invoked once before returning
	pm.RequestPeerUpdate()

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
	peers := pm.peersProvider()

	pm.logger.Trace().
		Str("peers", fmt.Sprintf("%v", peers)).
		Msg("connecting to peers")

	// ask the connector to connect to all peers in the list
	pm.connector.UpdatePeers(pm.unit.Ctx(), peers)
}

func (pm *PeerManager) ForceUpdatePeers() {
	pm.updatePeers()
}
