package p2p

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

// ProtocolStateIDCache implements an IdentityProvider and IDTranslator for the set of authorized
// Flow network participants as according to the given `protocol.State`.
type ProtocolStateIDCache struct {
	events.Noop
	identities flow.IdentityList
	state      protocol.State
	mu         sync.RWMutex
	peerIDs    map[flow.Identifier]peer.ID
	flowIDs    map[peer.ID]flow.Identifier
	lookup     map[flow.Identifier]*flow.Identity
	logger     zerolog.Logger
}

func NewProtocolStateIDCache(
	logger zerolog.Logger,
	state protocol.State,
	eventDistributer *events.Distributor,
) (*ProtocolStateIDCache, error) {
	provider := &ProtocolStateIDCache{
		state:  state,
		logger: logger.With().Str("component", "protocol-state-id-cache").Logger(),
	}

	head, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest state header: %w", err)
	}

	provider.update(head.ID())
	eventDistributer.AddConsumer(provider)

	return provider, nil
}

func (p *ProtocolStateIDCache) EpochTransition(newEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("newEpochCounter", newEpochCounter).Msg("epoch transition")
	p.update(header.ID())
}

func (p *ProtocolStateIDCache) EpochSetupPhaseStarted(currentEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("currentEpochCounter", currentEpochCounter).Msg("epoch setup phase started")
	p.update(header.ID())
}

func (p *ProtocolStateIDCache) EpochCommittedPhaseStarted(currentEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("currentEpochCounter", currentEpochCounter).Msg("epoch committed phase started")
	p.update(header.ID())
}

// update updates the cached identities stored in this provider.
// This is called whenever an epoch event occurs, signaling a possible change in
// protocol state identities.
func (p *ProtocolStateIDCache) update(blockID flow.Identifier) {
	p.logger.Info().Str("blockID", blockID.String()).Msg("updating cached identities")

	identities, err := p.state.AtBlockID(blockID).Identities(filter.Any)
	if err != nil {
		// We don't want to continue with an expired identity list.
		p.logger.Fatal().Err(err).Msg("failed to fetch new identities")
	}

	nIds := identities.Count()

	peerIDs := make(map[flow.Identifier]peer.ID, nIds)
	flowIDs := make(map[peer.ID]flow.Identifier, nIds)

	for _, identity := range identities {
		p.logger.Debug().Interface("identity", identity).Msg("extracting peer ID from network key")

		pid, err := keyutils.PeerIDFromFlowPublicKey(identity.NetworkPubKey)
		if err != nil {
			p.logger.Err(err).Interface("identity", identity).Msg("failed to extract peer ID from network key")
			continue
		}

		flowIDs[pid] = identity.NodeID
		peerIDs[identity.NodeID] = pid
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.identities = identities
	p.flowIDs = flowIDs
	p.peerIDs = peerIDs
	p.lookup = identities.Lookup()
}

func (p *ProtocolStateIDCache) Identities(filter flow.IdentityFilter) flow.IdentityList {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.identities.Filter(filter)
}

func (p *ProtocolStateIDCache) ByNodeID(flowID flow.Identifier) (*flow.Identity, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	id, ok := p.lookup[flowID]
	return id, ok
}

func (p *ProtocolStateIDCache) ByPeerID(peerID peer.ID) (*flow.Identity, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if flowID, ok := p.flowIDs[peerID]; ok {
		id, ok := p.lookup[flowID]
		return id, ok
	}
	return nil, false
}

func (p *ProtocolStateIDCache) GetPeerID(flowID flow.Identifier) (pid peer.ID, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pid, found := p.peerIDs[flowID]
	if !found {
		err = fmt.Errorf("flow ID %v was not found in cached identity list", flowID)
	}

	return
}

func (p *ProtocolStateIDCache) GetFlowID(peerID peer.ID) (fid flow.Identifier, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	fid, found := p.flowIDs[peerID]
	if !found {
		err = fmt.Errorf("peer ID %v was not found in cached identity list", peerID)
	}

	return
}
