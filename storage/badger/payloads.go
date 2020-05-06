// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Payloads implements a simple read-only payload storage around a badger DB.
type Payloads struct {
	db         *badger.DB
	identities *Identities
	guarantees *Guarantees
	seals      *Seals
	idenIndex  *Cache
	guarIndex  *Cache
	sealIndex  *Cache
}

func NewPayloads(db *badger.DB, identities *Identities, guarantees *Guarantees, seals *Seals) *Payloads {

	storeIdentityIndex := func(blockID flow.Identifier, nodeIDs interface{}) error {
		return db.Update(operation.IndexPayloadIdentities(blockID, nodeIDs.([]flow.Identifier)))
	}

	retrieveIdentityIndex := func(blockID flow.Identifier) (interface{}, error) {
		var nodeIDs []flow.Identifier
		err := db.View(operation.LookupPayloadIdentities(blockID, &nodeIDs))
		return nodeIDs, err
	}

	storeGuaranteeIndex := func(blockID flow.Identifier, guarIDs interface{}) error {
		return db.Update(operation.IndexPayloadGuarantees(blockID, guarIDs.([]flow.Identifier)))
	}

	retrieveGuaranteeIndex := func(blockID flow.Identifier) (interface{}, error) {
		var guarIDs []flow.Identifier
		err := db.View(operation.LookupPayloadGuarantees(blockID, &guarIDs))
		return guarIDs, err
	}

	storeSealIndex := func(blockID flow.Identifier, sealIDs interface{}) error {
		return db.Update(operation.IndexPayloadSeals(blockID, sealIDs.([]flow.Identifier)))
	}

	retrieveSealIndex := func(blockID flow.Identifier) (interface{}, error) {
		var sealIDs []flow.Identifier
		err := db.View(operation.LookupPayloadSeals(blockID, &sealIDs))
		return sealIDs, err
	}

	p := &Payloads{
		db:         db,
		identities: identities,
		guarantees: guarantees,
		seals:      seals,
		idenIndex:  newCache(withLimit(1000), withStore(storeIdentityIndex), withRetrieve(retrieveIdentityIndex)),
		guarIndex:  newCache(withLimit(1000), withStore(storeGuaranteeIndex), withRetrieve(retrieveGuaranteeIndex)),
		sealIndex:  newCache(withLimit(1000), withStore(storeSealIndex), withRetrieve(retrieveSealIndex)),
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

	// store/cache identity index
	err := p.idenIndex.Put(blockID, flow.GetIDs(payload.Identities))
	if err != nil {
		return fmt.Errorf("could not put identity index: %w", err)
	}

	// store/cache guarantee index
	err = p.guarIndex.Put(blockID, flow.GetIDs(payload.Guarantees))
	if err != nil {
		return fmt.Errorf("could not put guarantee index: %w", err)
	}

	// store/cache seal index
	err = p.sealIndex.Put(blockID, flow.GetIDs(payload.Seals))
	if err != nil {
		return fmt.Errorf("could not put seal index: %w", err)
	}

	return nil
}

func (p *Payloads) ByBlockID(blockID flow.Identifier) (*flow.Payload, error) {

	identities, err := p.IdentitiesFor(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	guarantees, err := p.GuaranteesFor(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get guarantees: %w", err)
	}

	seals, err := p.SealsFor(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get seals: %w", err)
	}

	payload := &flow.Payload{
		Identities: identities,
		Guarantees: guarantees,
		Seals:      seals,
	}

	return payload, nil
}

func (p *Payloads) IdentitiesFor(blockID flow.Identifier) (flow.IdentityList, error) {

	// get the identity index
	idenIndex, err := p.idenIndex.Get(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get identity index: %w", err)
	}

	// load all the identities
	nodeIDs := idenIndex.([]flow.Identifier)
	identities := make(flow.IdentityList, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		identity, err := p.identities.ByNodeID(nodeID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve identity (%x): %w", nodeID, err)
		}
		identities = append(identities, identity)
	}

	return identities, nil
}

func (p *Payloads) GuaranteesFor(blockID flow.Identifier) ([]*flow.CollectionGuarantee, error) {

	// get the guarantee index
	guarIndex, err := p.guarIndex.Get(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get guarantee index: %w", err)
	}

	// load all the guarantees
	collIDs := guarIndex.([]flow.Identifier)
	guarantees := make([]*flow.CollectionGuarantee, 0, len(collIDs))
	for _, collID := range collIDs {
		guarantee, err := p.guarantees.ByCollectionID(collID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve guarantee (%x): %w", collID, err)
		}
		guarantees = append(guarantees, guarantee)
	}
	return guarantees, nil
}

func (p *Payloads) SealsFor(blockID flow.Identifier) ([]*flow.Seal, error) {

	// get the seal index
	sealIndex, err := p.sealIndex.Get(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get seal index: %w", err)
	}

	// load all the seals
	sealIDs := sealIndex.([]flow.Identifier)
	seals := make([]*flow.Seal, 0, len(sealIDs))
	for _, sealID := range sealIDs {
		seal, err := p.seals.ByID(sealID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve seal (%x): %w", sealID, err)
		}
		seals = append(seals, seal)
	}

	return seals, nil
}
