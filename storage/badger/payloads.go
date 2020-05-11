package badger

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Payloads struct {
	index      *Index
	identities *Identities
	guarantees *Guarantees
	seals      *Seals
}

func NewPayloads(index *Index, identities *Identities, guarantees *Guarantees, seals *Seals) *Payloads {

	p := &Payloads{
		index:      index,
		identities: identities,
		guarantees: guarantees,
		seals:      seals,
	}

	return p
}

func (p *Payloads) Store(blockID flow.Identifier, payload *flow.Payload) error {

	// make sure all payload entities are stored
	for _, identity := range payload.Identities {
		err := p.identities.Store(identity)
		if err != nil {
			return fmt.Errorf("could not store identity: %w", err)
		}
	}

	// make sure all payload guarantees are stored
	for _, guarantee := range payload.Guarantees {
		err := p.guarantees.Store(guarantee)
		if err != nil {
			return fmt.Errorf("could not store guarantee: %w", err)
		}
	}

	// make sure all payload seals are stored
	for _, seal := range payload.Seals {
		err := p.seals.Store(seal)
		if err != nil {
			return fmt.Errorf("could not store seal: %w", err)
		}
	}

	// store the index
	err := p.index.Store(blockID, payload.Index())
	if err != nil {
		return fmt.Errorf("could not store index: %w", err)
	}

	return nil
}

func (p *Payloads) ByBlockID(blockID flow.Identifier) (*flow.Payload, error) {

	index, err := p.index.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve index: %w", err)
	}

	identities := make(flow.IdentityList, 0, len(index.NodeIDs))
	for _, nodeID := range index.NodeIDs {
		identity, err := p.identities.ByNodeID(nodeID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve identity (%x): %w", nodeID, err)
		}
		identities = append(identities, identity)
	}

	guarantees := make([]*flow.CollectionGuarantee, 0, len(index.CollectionIDs))
	for _, collID := range index.CollectionIDs {
		guarantee, err := p.guarantees.ByCollectionID(collID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve guarantee (%x): %w", collID, err)
		}
		guarantees = append(guarantees, guarantee)
	}

	seals := make([]*flow.Seal, 0, len(index.SealIDs))
	for _, sealID := range index.SealIDs {
		seal, err := p.seals.ByID(sealID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve seal (%x): %w", sealID, err)
		}
		seals = append(seals, seal)
	}

	payload := &flow.Payload{
		Identities: identities,
		Guarantees: guarantees,
		Seals:      seals,
	}

	return payload, nil
}
