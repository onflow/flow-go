package peermgr

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

var PeerUpdateInterval = 1 * time.Minute

type Connector interface {
	ConnectPeers(ctx context.Context, ids flow.IdentityList) error
	DisconnectPeers(ctx context.Context, ids flow.IdentityList) error
}

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

// RequestPeerUpdate initiates an update to the connections of this node
func (pm *PeerManager) RequestPeerUpdate() {
	select {
	case pm.peerRequestQ <- struct{}{}:
	default:
	}
}

func (pm *PeerManager) updatePeers() {

	pm.logger.Debug().Msg("updating peers")

	idProvider := pm.idsProvider
	if idProvider == nil {
		pm.logger.Error().Msg("failed to update peers: id provider callback is not set")
	}

	ids, err := idProvider()
	if err != nil {
		pm.logger.Error().Err(err).Msg("failed to update peers")
	}

	excludedPeers := pm.currentIDs.Filter(filter.Not(filter.In(ids)))
	pm.currentIDs = ids

	// connect to all the new ids
	pm.logger.Debug().
		Str("peers", fmt.Sprintf("%v", ids.NodeIDs())).
		Msg("connecting to peers")
	err = pm.connector.ConnectPeers(pm.ctx, ids)
	if err != nil {
		pm.logger.Error().Err(err).Msg("failed to connect to peers")
	}

	// disconnect from the peers no longer in the new id list
	if len(excludedPeers) > 0 {
		pm.logger.Info().
			Str("excluded_peers", fmt.Sprintf("%v", excludedPeers.NodeIDs())).
			Msg("disconnecting from peers")

		err = pm.connector.DisconnectPeers(pm.ctx, excludedPeers)
		if err != nil {
			pm.logger.Error().Err(err).Msg("failed to disconnect from peers")
		}
	}
}

// updateLoop request periodic connection update
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
