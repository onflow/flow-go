package connection

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/rand"
)

// DefaultPeerUpdateInterval is default duration for which the peer manager waits in between attempts to update peer connections.
// We set it to 1 second to be aligned with the heartbeat intervals of libp2p, alsp, and gossipsub.
var DefaultPeerUpdateInterval = time.Second

var _ p2p.PeerManager = (*PeerManager)(nil)
var _ component.Component = (*PeerManager)(nil)
var _ p2p.RateLimiterConsumer = (*PeerManager)(nil)

// PeerManager adds and removes connections to peers periodically and on request
type PeerManager struct {
	component.Component

	logger             zerolog.Logger
	peersProvider      p2p.PeersProvider // callback to retrieve list of peers to connect to
	peerRequestQ       chan struct{}     // a channel to queue a peer update request
	connector          p2p.PeerUpdater   // connector to connect or disconnect from peers
	peerUpdateInterval time.Duration     // interval the peer manager runs on

	peersProviderMu sync.RWMutex
}

// NewPeerManager creates a new peer manager which calls the peersProvider callback to get a list of peers to connect to
// and it uses the connector to actually connect or disconnect from peers.
func NewPeerManager(logger zerolog.Logger, updateInterval time.Duration, connector p2p.PeerUpdater) *PeerManager {
	pm := &PeerManager{
		logger:             logger.With().Str("component", "peer-manager").Logger(),
		connector:          connector,
		peerRequestQ:       make(chan struct{}, 1),
		peerUpdateInterval: updateInterval,
	}

	pm.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			// makes sure that peer update request is invoked once before returning
			pm.RequestPeerUpdate()

			ready()
			pm.updateLoop(ctx)
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			pm.periodicLoop(ctx)
		}).
		Build()

	return pm
}

// updateLoop triggers an update peer request when it has been requested
func (pm *PeerManager) updateLoop(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-pm.peerRequestQ:
			pm.updatePeers(ctx)
		}
	}
}

// periodicLoop periodically triggers an update peer request
func (pm *PeerManager) periodicLoop(ctx irrecoverable.SignalerContext) {
	// add a random delay to initial launch to avoid synchronizing this
	// potentially expensive operation across the network
	r, err := rand.Uint64n(uint64(pm.peerUpdateInterval.Nanoseconds()))
	if err != nil {
		ctx.Throw(fmt.Errorf("unable to generate random interval: %w", err))
	}
	delay := time.Duration(r)

	ticker := time.NewTicker(pm.peerUpdateInterval)
	defer ticker.Stop()

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.RequestPeerUpdate()
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
func (pm *PeerManager) updatePeers(ctx context.Context) {
	pm.peersProviderMu.RLock()
	defer pm.peersProviderMu.RUnlock()

	if pm.peersProvider == nil {
		pm.logger.Error().Msg("peers provider not set")
		return
	}

	// get all the peer ids to connect to
	peers := pm.peersProvider()

	pm.logger.Trace().
		Str("peers", fmt.Sprintf("%v", peers)).
		Msg("connecting to peers")

	// ask the connector to connect to all peers in the list
	pm.connector.UpdatePeers(ctx, peers)
}

// ForceUpdatePeers initiates an update to the peer connections of this node immediately
func (pm *PeerManager) ForceUpdatePeers(ctx context.Context) {
	pm.updatePeers(ctx)
}

// SetPeersProvider sets the peers provider
// SetPeersProvider may be called at most once
func (pm *PeerManager) SetPeersProvider(peersProvider p2p.PeersProvider) {
	pm.peersProviderMu.Lock()
	defer pm.peersProviderMu.Unlock()

	if pm.peersProvider != nil {
		pm.logger.Fatal().Msg("peers provider already set")
	}

	pm.peersProvider = peersProvider
}

// OnRateLimitedPeer rate limiter distributor consumer func that will be called when a peer is rate limited, the rate limited peer
// is disconnected immediately after being rate limited.
func (pm *PeerManager) OnRateLimitedPeer(pid peer.ID, role, msgType, topic, reason string) {
	pm.logger.Warn().
		Str("peer_id", p2plogging.PeerId(pid)).
		Str("role", role).
		Str("message_type", msgType).
		Str("topic", topic).
		Str("reason", reason).
		Bool(logging.KeySuspicious, true).
		Msg("pruning connection to rate-limited peer")
	pm.RequestPeerUpdate()
}
