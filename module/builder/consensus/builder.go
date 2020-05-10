// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	metrics  module.MempoolMetrics
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
func NewBuilder(metrics module.MempoolMetrics, db *badger.DB, headers storage.Headers, seals storage.Seals, payloads storage.Payloads, blocks storage.Blocks, guarPool mempool.Guarantees, sealPool mempool.Seals, options ...func(*Config)) *Builder {

	// initialize default config
	cfg := Config{
		minInterval: 500 * time.Millisecond,
		maxInterval: 10 * time.Second,
		expiry:      flow.DefaultTransactionExpiry,
	}

	// apply option parameters
	for _, option := range options {
		option(&cfg)
	}

	b := &Builder{
		metrics:  metrics,
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

	// STEP ONE: Create a lookup of all previously used guarantees on the part
	// of the chain that we are building on. We do this separately for pending
	// and finalized ancestors, so we can differentiate what to do about it.

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

	// we look back only as far as the expiry limit for the current height we
	// are building for; any guarantee with a reference block before that can
	// not be included anymore anyway
	parent, err := b.headers.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent: %w", err)
	}
	height := parent.Height + 1
	limit := height - uint64(b.cfg.expiry)
	if limit > height { // overflow check
		limit = 0
	}

	ancestorID = finalID
	finalLookup := make(map[flow.Identifier]struct{})
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

	// STEP TWO: Go through the guarantees in our memory pool.
	// 1) If it was already included on the finalized part of the chain, remove
	// it from the memory pool and skip.
	// 2) If the reference block has an expired height, also remove it from the
	// memory pool and skip.
	// 3) If it was already included on the pending part of the chain, skip, but
	// keep in memory pool for now.
	// 4) Otherwise, this guarantee can be included in the payload.

	var guarantees []*flow.CollectionGuarantee
	for _, guarantee := range b.guarPool.All() {
		collID := guarantee.ID()
		_, duplicated := finalLookup[collID]
		if duplicated {
			_ = b.guarPool.Rem(collID)
			continue
		}
		ref, err := b.headers.ByBlockID(guarantee.ReferenceBlockID)
		if errors.Is(err, storage.ErrNotFound) {
			_ = b.guarPool.Rem(collID)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get reference block: %w", err)
		}
		if ref.Height < limit {
			_ = b.guarPool.Rem(collID)
			continue
		}
		_, duplicated = pendingLookup[collID]
		if duplicated {
			continue
		}
		guarantees = append(guarantees, guarantee)
	}

	b.metrics.MempoolEntries(metrics.ResourceGuarantee, b.guarPool.Size())

	// STEP FOUR: We try to get all ancestors from last sealed block all the way
	// to the parent. Then we try to get seals for each of them until we don't
	// find one. This creates a valid chain of seals from the last sealed block
	// to at most the parent.

	// create a mapping of block to seal for all seals in our pool
	byBlock := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range b.sealPool.All() {
		byBlock[seal.BlockID] = seal
	}
	if int(b.sealPool.Size()) > len(byBlock) {
		return nil, fmt.Errorf("multiple seals for the same block")
	}

	// create a list of ancestors from parent to just before last sealed
	last, err := b.seals.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal: %w", err)
	}
	ancestorID = parentID
	var sealableIDs []flow.Identifier
	for ancestorID != last.BlockID {
		sealableIDs = append(sealableIDs, ancestorID)
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get sealable ancestor (%x): %w", ancestorID, err)
		}
		ancestorID = ancestor.ParentID
	}

	// iterate backwards from just after last sealed to parent and build a chain
	// of seals for those blocks that is as big as possible
	var seals []*flow.Seal
	lastID := lastSeal.ID()
	sealedID := lastSeal.BlockID
	prevState := lastSeal.FinalState
	for len(byParent) > 0 {

		// if we don't find the next seal, we are done building the chain
		next, found := byParent[sealedID]
		if !found {
			break
		}

		// if the states mismatch between two subsequent seals, it's an error
		if !bytes.Equal(next.InitialState, prevState) {
			return nil, fmt.Errorf("seal execution states do not connect")
		}

		// add seal to payload seals and delete from parent lookup
		seals = append(seals, next)
		delete(byParent, lastSeal.BlockID)
		if len(byParent) == 0 {
			break
		}

		// if we have reached the seal for the parent, we are also done
		if next.BlockID == parentID {
			break
		}

		// try looking for next seal
		lastID = next.ID()
		sealedID = next.BlockID
		prevState = next.FinalState
	}

	// STEP FOUR: We now have guarantees and seals we can validly include
	// in the payload built on top of the given parent. Now we need to build
	// and store the block header, as well as index the payload contents.

	// build the payload so we can get the hash
	payload := &flow.Payload{
		Identities: nil,
		Guarantees: guarantees,
		Seals:      seals,
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
		Height:      height,
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
		err = operation.IndexBlockSeal(blockID, lastID)(tx)
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
