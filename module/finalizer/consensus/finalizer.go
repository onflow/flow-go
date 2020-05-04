// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Finalizer is a simple wrapper around our temporary state to clean up after a
// block has been fully finalized to the persistent protocol state.
type Finalizer struct {
	db       *badger.DB
	headers  storage.Headers
	payloads storage.Payloads
	proto    protocol.State
	cleanup  CleanupFunc
}

// NewFinalizer creates a new finalizer for the temporary state.
func NewFinalizer(db *badger.DB, headers storage.Headers, payloads storage.Payloads, proto protocol.State, options ...func(*Finalizer)) *Finalizer {
	f := &Finalizer{
		db:       db,
		proto:    proto,
		headers:  headers,
		payloads: payloads,
		cleanup:  CleanupNothing(),
	}
	for _, option := range options {
		option(f)
	}
	return f
}

// MakeFinal will finalize the block with the given ID and clean up the memory
// pools after it.
//
// This assumes that guarantees and seals are already in persistent state when
// included in a block proposal. Between entering the non-finalized chain state
// and being finalized, entities should be present in both the volatile memory
// pools and persistent storage.
func (f *Finalizer) MakeFinal(blockID flow.Identifier) error {

	// STEP ONE: Check if the block itself would conflict with finalized state.

	// retrieve the finalized height
	var finalized uint64
	err := f.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}

	// retrieve the pending block
	pending, err := f.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve pending header: %w", err)
	}

	// Check to see if we are trying to finalize at a heigth that is already
	// finalized; if so, compary the ID of the finalized block.
	// 1) If it matches, this is a no-op.
	// 2) If it doesn't match, this is an invalid operation.
	if pending.Height <= finalized {
		dup, err := f.headers.ByHeight(pending.Height)
		if err != nil {
			return fmt.Errorf("could not get non-pending alternative: %w", err)
		}
		if dup.ID() != blockID {
			return fmt.Errorf("cannot finalize conflicting state (height: %d, pending: %x, finalized: %x)", pending.Height, blockID, dup.ID())
		}
		return nil
	}

	// STEP TWO: Check if the block actually connects to the last finalized
	// block through its ancestors - and keep a list while we are at it.

	// get the finalized block ID
	var finalID flow.Identifier
	err = f.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized header: %w", err)
	}

	// in order to validate the validity of all changes, we need to iterate
	// through the blocks that need to be finalized from oldest to youngest;
	// we thus start at the youngest and remember the intermediary steps
	// while tracing back until we reach the finalized state
	pendingIDs := []flow.Identifier{blockID}
	ancestorID := pending.ParentID
	for ancestorID != finalID {
		ancestor, err := f.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve parent (%x): %w", ancestorID, err)
		}
		if ancestor.Height < finalized {
			return fmt.Errorf("ancestor conflicts with finalized state (height: %d, finalized: %d)", ancestor.Height, finalized)
		}
		pendingIDs = append(pendingIDs, ancestorID)
		ancestorID = ancestor.ParentID
	}

	// STEP 4: Now we can step backwards in order to go from oldest to youngest; for
	// each header, we apply the changes to the protocol state and then clean
	// up behind it.
	for i := len(pendingIDs) - 1; i >= 0; i-- {
		pendingID := pendingIDs[i]
		err = f.proto.Mutate().Finalize(pendingID)
		if err != nil {
			return fmt.Errorf("could not finalize block (%x): %w", pendingID, err)
		}
		err := f.cleanup(pendingID)
		if err != nil {
			return fmt.Errorf("could not execute cleanup (%x): %w", pendingID, err)
		}
	}

	return nil
}

// MakePending indexes a block by its parent. The index is useful for looking up the child block
// of a finalized block.
func (f *Finalizer) MakePending(blockID flow.Identifier, parentID flow.Identifier) error {
	return f.db.Update(procedure.IndexBlockChild(parentID, blockID))
}
