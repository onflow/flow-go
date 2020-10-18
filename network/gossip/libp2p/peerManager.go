package libp2p

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// Connector connects to peer and disconnects from peer using the underlying networking library
type Connector interface {

	// ConnectPeers connect to the given flow.Identities and returns a map of identities which failed.
	// ConnectPeer implementation should be idempotent such that multiple calls to connect to the same peer should not
	// return an error or create multiple connections
	ConnectPeers(ctx context.Context, ids flow.IdentityList) map[flow.Identity]error

	// DisconnectPeers disconnect from the given flow.Identities and returns a map of identities which failed.
	// DisconnectPeers implementation should be idempotent such that multiple calls to connect to the same peer should
	// not return an error
	DisconnectPeers(ctx context.Context, ids flow.IdentityList) map[flow.Identity]error
}

// PeerUpdateInterval is how long the peer manager waits in between attempts to update peer connections
var PeerUpdateInterval = 1 * time.Minute

// PeerManager adds and removes connections to peers periodically and on request
type PeerManager struct {
	ctx         context.Context
	logger      zerolog.Logger
	idsProvider func() (flow.IdentityList, error) // callback to retrieve list of peers to connect to

	peerRequestQ chan struct{}     // a channel to queue a peer update request
	connector    Connector         // connector to connect or disconnect from peers
	currentIDs   flow.IdentityList // a list of currentIDs this node should be connected to
}

// NewPeerManager creates a new peer manager which calls the idsProvider callback to get a list of peers to connect to
// and it uses the connector to actually connect or disconnect from peers.
func NewPeerManager(ctx context.Context, logger zerolog.Logger, idsProvider func() (flow.IdentityList, error), connector Connector) *PeerManager {
	return &PeerManager{
		ctx:          ctx,
		logger:       logger,
		idsProvider:  idsProvider,
		connector:    connector,
		peerRequestQ: make(chan struct{}, 1),
	}
}

// Start kicks off the ambient periodic connection updates
func (pm *PeerManager) Start() error {
	go pm.updateLoop()
	go pm.periodicUpdate()
	return nil
}

// updateLoop triggers an update peer request when it has been requested
func (pm *PeerManager) updateLoop() {
	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-pm.peerRequestQ:
			pm.updatePeers()
		}
	}
}

// updateLoop request periodic connection update
func (pm *PeerManager) periodicUpdate() {

	// request initial discovery
	pm.RequestPeerUpdate()

	ticker := time.NewTicker(PeerUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.RequestPeerUpdate()
		case <-pm.ctx.Done():
			return
		}
	}
}

// RequestPeerUpdate initiates an update to peer connections of this node
func (pm *PeerManager) RequestPeerUpdate() {
	select {
	case pm.peerRequestQ <- struct{}{}:
	default:
	}
}

// updatePeers updates the peers by connecting to all the nodes provided by the idsProvider callback and disconnecting from
// previous nodes that are no longer in the new list of nodes.
func (pm *PeerManager) updatePeers() {

	idProvider := pm.idsProvider
	if idProvider == nil {
		pm.logger.Error().Msg("failed to update peers: id provider callback is not set")
	}

	ids, err := idProvider()
	if err != nil {
		pm.logger.Error().Err(err).Msg("failed to update peers")
	}

	// connect to all the current IDs
	pm.connect(ids)

	// find the ids in the old id list which are no longer in the new id list
	excludedPeers := pm.currentIDs.Filter(filter.Not(filter.In(ids)))

	// disconnect from the peers no longer in the new id list
	if len(excludedPeers) > 0 {
		pm.disconnect(excludedPeers)
	}

	// remember the new id list for next time
	pm.currentIDs = ids
}

func (pm *PeerManager) connect(ids flow.IdentityList) {
	pm.logger.Debug().
		Str("peers", fmt.Sprintf("%v", ids.NodeIDs())).
		Msg("connecting to peers")

	// ask the connector to connect to all peers in the list
	failedIDs := pm.connector.ConnectPeers(pm.ctx, ids)

	err := failedIDMapToSingleError(failedIDs)
	if err != nil {
		pm.logger.Error().Int("failed_peer_count", len(failedIDs)).Err(err).Msg("failed to connect to peers")
	}
}

func (pm *PeerManager) disconnect(ids flow.IdentityList) {
	pm.logger.Info().
		Str("excluded_peers", fmt.Sprintf("%v", ids.NodeIDs())).
		Msg("disconnecting from peers")

	// ask the connector to disconnect from all peers in the list
	failedIDs := pm.connector.DisconnectPeers(pm.ctx, ids)

	err := failedIDMapToSingleError(failedIDs)
	if err != nil {
		pm.logger.Error().Int("failed_peer_count", len(failedIDs)).Err(err).Msg("failed to disconnect from peers")
	}
}

// failedIDMapToSingleError converts the failedIDs map to a single error
func failedIDMapToSingleError(failedIDs map[flow.Identity]error) error {
	multierr := new(multierror.Error)
	for id, err := range failedIDs {
		multierr = multierror.Append(multierr, fmt.Errorf("failed to connect to %s: %w", id.String(), err))
	}
	return multierr.ErrorOrNil()
}
