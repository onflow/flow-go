package identity

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

type IdentifierProvider interface {
	Identifiers() flow.IdentifierList
}

// TODO: rename to ProtocolStateIDProvider
type ProtocolStateIdentifierProvider struct {
	events.Noop
	identities flow.IdentityList // TODO: actually we *can* just use identifiers after all
	state      protocol.State
	mu         sync.RWMutex
	filter     flow.IdentityFilter
	peerIDs    map[flow.Identifier]peer.ID // TODO: need to initialize these in constructor!!!
	flowIDs    map[peer.ID]flow.Identifier
}

func WithFilter(filter flow.IdentityFilter) ProtocolStateIdentifierProviderOption {
	return func(provider *ProtocolStateIdentifierProvider) {
		provider.filter = filter
	}
}

type ProtocolStateIdentifierProviderOption func(*ProtocolStateIdentifierProvider)

// TODO: this one also implements IDTranslator!!!
func NewProtocolStateIdentifierProvider(
	state protocol.State,
	eventDistributer *events.Distributor,
	opts ...ProtocolStateIdentifierProviderOption,
) (*ProtocolStateIdentifierProvider, error) {
	provider := &ProtocolStateIdentifierProvider{
		state:  state,
		filter: filter.Any,
	}

	for _, opt := range opts {
		opt(provider)
	}

	head, err := state.Final().Head()
	if err != nil {
		return nil, err // TODO: format the error
	}

	provider.update(head.ID())
	eventDistributer.AddConsumer(provider)

	return provider, nil
}

func (p *ProtocolStateIdentifierProvider) EpochTransition(_ uint64, header *flow.Header) {
	p.update(header.ID())
}

func (p *ProtocolStateIdentifierProvider) EpochSetupPhaseStarted(_ uint64, header *flow.Header) {
	p.update(header.ID())
}

func (p *ProtocolStateIdentifierProvider) EpochCommittedPhaseStarted(_ uint64, header *flow.Header) {
	p.update(header.ID())
}

func (p *ProtocolStateIdentifierProvider) update(blockID flow.Identifier) {
	identities, err := p.state.AtBlockID(blockID).Identities(p.filter)
	if err != nil {
		// TODO: log fatal. Reasoning here is, we don't want to continue with an expired list.
	}

	nIds := identities.Count()

	peerIDs := make(map[flow.Identifier]peer.ID, nIds)
	flowIDs := make(map[peer.ID]flow.Identifier, nIds)

	for _, identity := range identities {
		pid, err := p2p.IdentityToPeerID(identity)
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

func (p *ProtocolStateIdentifierProvider) Identifiers() flow.IdentifierList {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.identities.NodeIDs()
}

func (p *ProtocolStateIdentifierProvider) GetPeerID(flowID flow.Identifier) (pid peer.ID, found bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pid, found = p.peerIDs[flowID]
	return
}

func (p *ProtocolStateIdentifierProvider) GetFlowID(peerID peer.ID) (fid flow.Identifier, found bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	fid, found = p.flowIDs[peerID]
	return
}

type PeerstoreIdentifierProvider struct {
	store        peerstore.Peerstore
	idTranslator p2p.IDTranslator
}

func NewPeerstoreIdentifierProvider(store peerstore.Peerstore, idTranslator p2p.IDTranslator) *PeerstoreIdentifierProvider {
	return &PeerstoreIdentifierProvider{store: store, idTranslator: idTranslator}
}

func (p *PeerstoreIdentifierProvider) Identifiers() flow.IdentifierList {
	var result flow.IdentifierList

	pids := p.store.PeersWithAddrs() // TODO: should we just call Peers here?
	for _, pid := range pids {
		flowID, success := p.idTranslator.GetFlowID(pid)
		if success {
			result = append(result, flowID)
		}
	}

	return result
}
