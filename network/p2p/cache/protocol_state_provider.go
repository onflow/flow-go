package cache

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

// ProtocolStateIDCache implements an `id.IdentityProvider` and `p2p.IDTranslator` for the set of
// authorized Flow network participants as according to the given `protocol.State`.
// the implementation assumes that the node information changes rarely, while queries are frequent.
// Hence, we follow an event-driven design, where the ProtocolStateIDCache subscribes to relevant
// protocol notifications (mainly Epoch notifications) and updates its internally cached list of
// authorized node identities.
// Note: this implementation is _eventually consistent_, where changes in the protocol state will
// quickly, but not atomically, propagate to the ProtocolStateIDCache. This strongly benefits
// performance and modularity, as we can cache identities locally here, while the marginal
// delay of updates is of no concern to the protocol.
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

var _ module.IdentityProvider = (*ProtocolStateIDCache)(nil)
var _ protocol.Consumer = (*ProtocolStateIDCache)(nil)
var _ p2p.IDTranslator = (*ProtocolStateIDCache)(nil)

func NewProtocolStateIDCache(
	logger zerolog.Logger,
	state protocol.State,
	eventDistributor *events.Distributor,
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
	eventDistributor.AddConsumer(provider)

	return provider, nil
}

// EpochTransition is a callback function for notifying the `ProtocolStateIDCache`
// of an Epoch transition that just occurred. Upon such notification, the internally-cached
// Identity table of authorized network participants is updated.
//
// TODO: per API contract, implementations of `EpochTransition` should be non-blocking
// and virtually latency free. However, we run data base queries and acquire locks here,
// which is undesired.
func (p *ProtocolStateIDCache) EpochTransition(newEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("newEpochCounter", newEpochCounter).Msg("epoch transition")
	p.update(header.ID())
}

// EpochSetupPhaseStarted is a callback function for notifying the `ProtocolStateIDCache`
// that the EpochSetup Phase has just stared. Upon such notification, the internally-cached
// Identity table of authorized network participants is updated.
//
// TODO: per API contract, implementations of `EpochSetupPhaseStarted` should be non-blocking
// and virtually latency free. However, we run data base queries and acquire locks here,
// which is undesired.
func (p *ProtocolStateIDCache) EpochSetupPhaseStarted(currentEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("currentEpochCounter", currentEpochCounter).Msg("epoch setup phase started")
	p.update(header.ID())
}

// EpochCommittedPhaseStarted is a callback function for notifying the `ProtocolStateIDCache`
// that the EpochCommitted Phase has just stared. Upon such notification, the internally-cached
// Identity table of authorized network participants is updated.
//
// TODO: per API contract, implementations of `EpochCommittedPhaseStarted` should be non-blocking
// and virtually latency free. However, we run data base queries and acquire locks here,
// which is undesired.
func (p *ProtocolStateIDCache) EpochCommittedPhaseStarted(currentEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("currentEpochCounter", currentEpochCounter).Msg("epoch committed phase started")
	p.update(header.ID())
}

// update updates the cached identities stored in this provider.
// This is called whenever an epoch event occurs, signaling a possible change in
// protocol state identities.
// Caution: this function is non-negligible latency (data base reads and acquiring locks). Therefore,
// it is _not suitable_ to be executed by the publisher thread for protocol notifications.
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

// Identities returns the full identities of _all_ nodes currently known to the
// protocol that pass the provided filter. Caution, this includes ejected nodes.
// Please check the `Ejected` flag in the identities (or provide a filter for
// removing ejected nodes).
func (p *ProtocolStateIDCache) Identities(filter flow.IdentityFilter[flow.Identity]) flow.IdentityList {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.identities.Filter(filter)
}

// ByNodeID returns the full identity for the node with the given Identifier,
// where Identifier is the way the protocol refers to the node. The function
// has the same semantics as a map lookup, where the boolean return value is
// true if and only if Identity has been found, i.e. `Identity` is not nil.
// Caution: function returns include ejected nodes. Please check the `Ejected`
// flag in the identity.
func (p *ProtocolStateIDCache) ByNodeID(flowID flow.Identifier) (*flow.Identity, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	id, ok := p.lookup[flowID]
	return id, ok
}

// ByPeerID returns the full identity for the node with the given peer ID,
// where ID is the way the libP2P refers to the node. The function
// has the same semantics as a map lookup, where the boolean return value is
// true if and only if Identity has been found, i.e. `Identity` is not nil.
// Caution: function returns include ejected nodes. Please check the `Ejected`
// flag in the identity.
func (p *ProtocolStateIDCache) ByPeerID(peerID peer.ID) (*flow.Identity, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if flowID, ok := p.flowIDs[peerID]; ok {
		id, ok := p.lookup[flowID]
		return id, ok
	}
	return nil, false
}

// GetPeerID returns the peer ID for the given Flow ID.
// During normal operations, the following error returns are expected
//   - ErrUnknownId if the given Identifier is unknown
func (p *ProtocolStateIDCache) GetPeerID(flowID flow.Identifier) (peer.ID, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pid, found := p.peerIDs[flowID]
	if !found {
		return "", fmt.Errorf("flow ID %v was not found in cached identity list: %w", flowID, p2p.ErrUnknownId)
	}
	return pid, nil
}

// GetFlowID returns the Flow ID for the given peer ID.
// During normal operations, the following error returns are expected
//   - ErrUnknownId if the given Identifier is unknown
func (p *ProtocolStateIDCache) GetFlowID(peerID peer.ID) (flow.Identifier, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	fid, found := p.flowIDs[peerID]
	if !found {
		return flow.ZeroID, fmt.Errorf("peer ID %v was not found in cached identity list: %w", peerID, p2p.ErrUnknownId)
	}
	return fid, nil
}
