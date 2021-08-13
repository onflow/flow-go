package p2p

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

type ProtocolStateIDCache struct {
	events.Noop
	identities flow.IdentityList
	state      protocol.State
	mu         sync.RWMutex
	peerIDs    map[flow.Identifier]peer.ID // TODO: need to initialize these in constructor!!!
	flowIDs    map[peer.ID]flow.Identifier
}

func NewProtocolStateIDCache(
	state protocol.State,
	eventDistributer *events.Distributor,
) (*ProtocolStateIDCache, error) {
	provider := &ProtocolStateIDCache{
		state: state,
	}

	head, err := state.Final().Head()
	if err != nil {
		return nil, err // TODO: format the error
	}

	provider.update(head.ID())
	eventDistributer.AddConsumer(provider)

	return provider, nil
}

func (p *ProtocolStateIDCache) EpochTransition(_ uint64, header *flow.Header) {
	// TODO: maybe we actually want to log the epoch information from the arguments here (epoch phase, etc)
	p.update(header.ID())
}

func (p *ProtocolStateIDCache) EpochSetupPhaseStarted(_ uint64, header *flow.Header) {
	p.update(header.ID())
}

func (p *ProtocolStateIDCache) EpochCommittedPhaseStarted(_ uint64, header *flow.Header) {
	p.update(header.ID())
}

func (p *ProtocolStateIDCache) update(blockID flow.Identifier) {
	// TODO: log status here
	identities, err := p.state.AtBlockID(blockID).Identities(filter.Any)
	if err != nil {
		// TODO: log fatal. Reasoning here is, we don't want to continue with an expired list.
	}

	nIds := identities.Count()

	peerIDs := make(map[flow.Identifier]peer.ID, nIds)
	flowIDs := make(map[peer.ID]flow.Identifier, nIds)

	for _, identity := range identities {
		pid, err := IdentityToPeerID(identity)
		if err != nil {
			// maybe don't log fatal here. It's probably okay if we miss some ppl in our mapping
		}

		flowIDs[pid] = identity.NodeID
		peerIDs[identity.NodeID] = pid
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.identities = identities
	p.flowIDs = flowIDs
	p.peerIDs = peerIDs
}

func (p *ProtocolStateIDCache) Identities(filter flow.IdentityFilter) flow.IdentityList {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.identities.Filter(filter)
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

func IdentityToPeerID(id *flow.Identity) (pid peer.ID, err error) {
	pk, err := PublicKey(id.NetworkPubKey)
	if err != nil {
		// TODO: format the error
		return
	}

	pid, err = peer.IDFromPublicKey(pk)
	if err != nil {
		// TODO: format the error
		return
	}

	return
}
