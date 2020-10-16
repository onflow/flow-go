package peermgr

import (
	"context"
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

type PeerManager struct {
	ctx         context.Context
	logger      zerolog.Logger
	idsProvider func() (flow.IdentityList, error)

	peerRequestQ chan struct{}
	connector Connector
	currentIDs flow.IdentityList
}

func NewPeerManager(ctx context.Context, logger zerolog.Logger, idsProvider func() (flow.IdentityList, error), connector Connector) *PeerManager {
	return &PeerManager{
		ctx:          ctx,
		logger:       logger,
		idsProvider:  idsProvider,
		connector: connector,
		peerRequestQ: make(chan struct{}, 1),
	}
}

func (pm *PeerManager) Start() error {
	go pm.updateLoop()
	go pm.periodicUpdate()
	return nil
}

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
	err = pm.connector.ConnectPeers(pm.ctx, ids)
	if err != nil {
		pm.logger.Error().Err(err).Msg("failed to connect to peers")
	}

	// disconnect from the peers no longer in the new id list
	err = pm.connector.DisconnectPeers(pm.ctx, excludedPeers)
	if err != nil {
		pm.logger.Error().Err(err).Msg("failed to disconnect from peers")
	}

}

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
