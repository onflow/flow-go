// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// Finalizer is a simple wrapper around our temporary state to clean up after a
// block has been fully finalized to the persistent protocol state.
type Finalizer struct {
	db      *badger.DB
	headers storage.Headers
	state   protocol.MutableState
	cleanup CleanupFunc
	tracer  module.Tracer
}

// NewFinalizer creates a new finalizer for the temporary state.
func NewFinalizer(db *badger.DB,
	headers storage.Headers,
	state protocol.MutableState,
	tracer module.Tracer,
	options ...func(*Finalizer)) *Finalizer {
	f := &Finalizer{
		db:      db,
		state:   state,
		headers: headers,
		cleanup: CleanupNothing(),
		tracer:  tracer,
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

	span, ctx, _ := f.tracer.StartBlockSpan(context.Background(), blockID, trace.CONFinalizerFinalizeBlock)
	defer span.Finish()

	// STEP ONE: This is an idempotent operation. In case we are trying to
	// finalize a block that is already below finalized height, we want to do
	// one of two things: if it conflicts with the block already finalized at
	// that height, it's an invalid operation. Otherwise, it is a no-op.

	var finalized uint64
	err := f.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}

	pending, err := f.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve pending header: %w", err)
	}

	if pending.Height <= finalized {
		dup, err := f.headers.ByHeight(pending.Height)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized equivalent: %w", err)
		}
		if dup.ID() != blockID {
			return fmt.Errorf("cannot finalize pending block conflicting with finalized state (height: %d, pending: %x, finalized: %x)", pending.Height, blockID, dup.ID())
		}
		return nil
	}

	// STEP TWO: At least one block in the chain back to the finalized state is
	// a valid candidate for finalization. Figure out all blocks between the
	// to-be-finalized block and the last finalized block. If we can't trace
	// back to the last finalized block, this is also an invalid call.

	var finalID flow.Identifier
	err = f.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized header: %w", err)
	}
	pendingIDs := []flow.Identifier{blockID}
	ancestorID := pending.ParentID
	for ancestorID != finalID {
		ancestor, err := f.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve parent (%x): %w", ancestorID, err)
		}
		if ancestor.Height < finalized {
			return fmt.Errorf("cannot finalize pending block unconnected to last finalized block (height: %d, finalized: %d)", ancestor.Height, finalized)
		}
		pendingIDs = append(pendingIDs, ancestorID)
		ancestorID = ancestor.ParentID
	}

	// STEP THREE: We walk backwards through the collected ancestors, starting
	// with the first block after finalizing state, and finalize them one by
	// one in the protocol state.

	for i := len(pendingIDs) - 1; i >= 0; i-- {
		pendingID := pendingIDs[i]
		err = f.state.Finalize(ctx, pendingID)
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

// MakeValid marks a block as having passed HotStuff validation.
func (f *Finalizer) MakeValid(blockID flow.Identifier) error {
	err := f.state.MarkValid(blockID)
	if err != nil {
		return fmt.Errorf("could not mark block as valid (%x): %w", blockID, err)
	}
	return nil
}
