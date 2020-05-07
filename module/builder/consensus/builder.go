// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	db       *badger.DB
	seals    storage.Seals
	headers  storage.Headers
	payloads storage.Payloads
	blocks   storage.Blocks
	guarPool mempool.Guarantees
	sealPool mempool.Seals
	cfg      Config
}

// NewBuilder creates a new block builder.
func NewBuilder(db *badger.DB, headers storage.Headers, seals storage.Seals, payloads storage.Payloads, blocks storage.Blocks, guarPool mempool.Guarantees, sealPool mempool.Seals, options ...func(*Config)) *Builder {

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
		seals:    seals,
		payloads: payloads,
		blocks:   blocks,
		guarPool: guarPool,
		sealPool: sealPool,
		cfg:      cfg,
	}
	return b
}

// BuildOn creates a new block header build on the provided parent, using the given view and applying the
// custom setter function to allow the caller to make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {

	// STEP ONE: Load some things we need to do our work.

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

	// STEP TWO: Create a lookup of all previously used guarantees on the part
	// of the chain we care about. We do this separately for unfinalized and
	// finalized sections of the chain to decide whether removing guarantees
	// from the memory pool is necessary.

	ancestorID := parentID
	pendingLookup := make(map[flow.Identifier]struct{})
	for ancestorID != finalID {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		if ancestor.Height <= finalized {
			return nil, fmt.Errorf("should always build on last finalized block")
		}
		payload, err := b.payloads.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, guarantee := range payload.Guarantees {
			pendingLookup[guarantee.ID()] = struct{}{}
		}
		ancestorID = ancestor.ParentID
	}

	// for now, we check at most 1000 blocks of history
	// TODO: look back based on referenc block ID and expiry

	limit := finalized - 1000
	if limit > finalized { // overflow check
		limit = 0
	}

	finalLookup := make(map[flow.Identifier]struct{})
	ancestorID = finalID
	for {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		payload, err := b.payloads.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, guarantee := range payload.Guarantees {
			finalLookup[guarantee.ID()] = struct{}{}
		}
		if ancestor.Height <= limit {
			break
		}
		ancestorID = ancestor.ParentID
	}

	// STEP THREE: Build a valid guarantee payload.

	var guarantees []*flow.CollectionGuarantee
	for _, guarantee := range b.guarPool.All() {

		// get the reference block for expiration
		refHeader, err := b.headers.ByBlockID(guarantee.ReferenceBlockID)
		if err != nil {
			return nil, fmt.Errorf("could not get reference block: %w", err)
		}

		// for now, we simply ignore unfinalized reference blocks
		// TODO: decide on whether we should remove those guarantees and when
		if finalized < refHeader.Height {
			continue
		}

		// check if the reference block is already too old
		collID := guarantee.ID()
		if uint(finalized-refHeader.Height) > flow.DefaultTransactionExpiry {
			_ = b.guarPool.Rem(collID)
			continue
		}

		// check if the guarantee was already finalized in a block
		_, duplicated := finalLookup[collID]
		if duplicated {
			_ = b.guarPool.Rem(collID)
			continue
		}

		// check if the guarantee is pending on this branch; don't remove from
		// memory pool yet, but skip for payload
		_, duplicated = pendingLookup[collID]
		if duplicated {
			continue
		}

		guarantees = append(guarantees, guarantee)
	}

	// STEP FOUR: Get the block seal from the parent and see how far we can
	// extend the chain of sealed blocks with the seals in the memory pool.

	// we map each seal to the parent of its sealed block; that way, we can
	// retrieve a valid seal that will *follow* each block's own seal
	byParent := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range b.sealPool.All() {
		sealed, err := b.headers.ByBlockID(seal.BlockID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not retrieve sealed header (%x): %w", seal.BlockID, err)
		}
		byParent[sealed.ParentID] = seal
	}

	// starting at the paren't seal, we try to find a seal to extend the current
	// last sealed block; if we do, we keep going until we don't
	// we also execute a sanity check on whether the execution state of the next
	// seal propely connects to the previous seal
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

	// STEP FIVE: We now have guarantees and seals we can validly include
	// in the payload built on top of the given parent. Now we need to build
	// and store the block header, as well as index the payload contents.

	// build the payload so we can get the hash
	payload := &flow.Payload{
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
	header := &flow.Header{
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
	err = setter(header)
	if err != nil {
		return nil, fmt.Errorf("could not apply setter: %w", err)
	}

	// insert the proposal into the database
	proposal := &flow.Block{
		Header:  header,
		Payload: payload,
	}
	err = b.blocks.Store(proposal)
	if err != nil {
		return nil, fmt.Errorf("could ot store proposal: %w", err)
	}

	// update protocol state index for the seal and initialize children index
	blockID := proposal.ID()
	err = operation.RetryOnConflict(b.db.Update, func(tx *badger.Txn) error {
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

	return header, err
}
