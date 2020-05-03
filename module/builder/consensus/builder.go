// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"bytes"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	db       *badger.DB
	headers  storage.Headers
	payloads storage.Payloads
	seals    storage.Seals
	guarPool mempool.Guarantees
	sealPool mempool.Seals
	cfg      Config
}

// NewBuilder creates a new block builder.
func NewBuilder(db *badger.DB, headers storage.Headers, payloads storage.Payloads, seals storage.Seals, guarPool mempool.Guarantees, sealPool mempool.Seals, options ...func(*Config)) *Builder {

	// initialize default config
	cfg := Config{
		minInterval:  500 * time.Millisecond,
		maxInterval:  10 * time.Second,
		expiryBlocks: 64,
	}

	// apply option parameters
	for _, option := range options {
		option(&cfg)
	}

	b := &Builder{
		db:       db,
		headers:  headers,
		payloads: payloads,
		seals:    seals,
		guarPool: guarPool,
		sealPool: sealPool,
		cfg:      cfg,
	}
	return b
}

// BuildOn creates a new block header build on the provided parent, using the given view and applying the
// custom setter function to allow the caller to make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {

	// STEP ONE: Create a lookup of all collection guarantees included in one of
	// the past 1000 blocks proceeding the proposal. We can then include all
	// collection guarantees from the memory pool that are not in this lookup.

	var finalized uint64
	err := b.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = b.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return nil, fmt.Errorf("could not lookup finalized block: %w", err)
	}

	limit := finalized - 1000
	if limit > finalized { // overflow check
		limit = 0
	}
	ancestorID := parentID
	gLookup := make(map[flow.Identifier]struct{})
	for ancestorID != finalID {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		if ancestor.Height <= limit {
			return nil, fmt.Errorf("should always build on last finalized block")
		}
		payload, err := b.payloads.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, guarantee := range payload.Guarantees {
			gLookup[guarantee.ID()] = struct{}{}
		}
		ancestorID = ancestor.ParentID
	}

	var guarantees []*flow.CollectionGuarantee
	for _, guarantee := range b.guarPool.All() {
		_, duplicated := gLookup[guarantee.ID()]
		if duplicated {
			continue
		}
		guarantees = append(guarantees, guarantee)
	}

	// STEP TWO: Find the last sealed block on our branch of the blockchain. We
	// can then use the associated seal to try and build a chain of seals from
	// the memory pool.

	byParent := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range b.sealPool.All() {
		sealed, err := b.headers.ByBlockID(seal.BlockID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve sealed header: %w", err)
		}
		byParent[sealed.ParentID] = seal
	}

	lastSeal, err := b.seals.ByBlockID(parentID)
	var seals []*flow.Seal
	for len(byParent) > 0 {
		seal, found := byParent[lastSeal.BlockID]
		if !found {
			break
		}
		if !bytes.Equal(seal.InitialState, lastSeal.FinalState) {
			return nil, fmt.Errorf("seal execution states do not connect")
		}
		delete(byParent, lastSeal.BlockID)
		seals = append(seals, seal)
		lastSeal = seal
	}

	// STEP THREE: we have the guarantees and seals we can validly include
	// in the payload built on top of the given block. Now we need to build
	// and store the block header, as well as index the payload contents.

	// build the payload so we can get the hash
	payload := flow.Payload{
		Identities: nil,
		Guarantees: guarantees,
		Seals:      seals,
	}

	// retrieve the parent to set the height
	parent, err := b.headers.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent: %w", err)
	}

	// calculate the timestamp and cutoffs
	timestamp := time.Now().UTC()
	from := parent.Timestamp.Add(b.cfg.minInterval)
	to := parent.Timestamp.Add(b.cfg.maxInterval)

	// adjust timestamp if outside of cutoffs
	if timestamp.Before(from) {
		timestamp = from
	}
	if timestamp.After(to) {
		timestamp = to
	}

	// construct default block on top of the provided parent
	proposal := flow.Header{
		ChainID:     parent.ChainID,
		ParentID:    parentID,
		Height:      parent.Height + 1,
		Timestamp:   timestamp,
		PayloadHash: payload.Hash(),

		// the following fields should be set by the custom function as needed
		// NOTE: we could abstract all of this away into an interface{} field,
		// but that would be over the top as we will probably always use hotstuff
		View:           0,
		ParentVoterIDs: nil,
		ParentVoterSig: nil,
		ProposerID:     flow.ZeroID,
		ProposerSig:    nil,
	}

	// apply the custom fields setter of the consensus algorithm
	err = setter(&proposal)
	if err != nil {
		return nil, fmt.Errorf("could not apply setter: %w", err)
	}

	// insert the proposal into the database
	blockID := proposal.ID()
	err = operation.RetryOnConflict(b.db.Update, func(tx *badger.Txn) error {
		err = operation.InsertHeader(blockID, &proposal)(tx)
		if err != nil {
			return fmt.Errorf("could not insert proposal header: %w", err)
		}
		err = procedure.InsertPayload(blockID, &payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert proposal payload: %w", err)
		}
		err = operation.IndexBlockSeal(blockID, lastSeal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index proposal seal: %w", err)
		}
		err = operation.InsertBlockChildren(blockID, nil)(tx)
		if err != nil {
			return fmt.Errorf("could not insert empty block children: %w", err)
		}

		return nil
	})

	return &proposal, err
}
