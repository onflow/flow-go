package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertHeader(w storage.Writer, headerID flow.Identifier, header *flow.Header) error {
	return UpsertByKey(w, MakePrefix(codeHeader, headerID), header)
}

func RetrieveHeader(r storage.Reader, blockID flow.Identifier, header *flow.Header) error {
	return RetrieveByKey(r, MakePrefix(codeHeader, blockID), header)
}

// IndexFinalizedBlockByHeight indexes the height of a block. It must only be called on finalized blocks.
// This function guarantees that the index is only inserted once for each height.
// The caller must acquire the [storage.LockFinalizeBlock] lock.
// Returns [storage.ErrAlreadyExists] if an ID has already been finalized for this height.
func IndexFinalizedBlockByHeight(lctx lockctx.Proof, rw storage.ReaderBatchWriter, height uint64, blockID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockFinalizeBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockFinalizeBlock)
	}

	var existingID flow.Identifier
	err := RetrieveByKey(rw.GlobalReader(), MakePrefix(codeHeightToBlock, height), &existingID)
	if err == nil {
		return fmt.Errorf("block ID already exists for height %d: %w", height, storage.ErrAlreadyExists)
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to check existing block ID for height %d: %w", height, err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeHeightToBlock, height), blockID)
}

// IndexCertifiedBlockByView indexes a block by its view.
// HotStuff guarantees that there is at most one certified block per view. Caution: this does not hold for
// uncertified proposals, as a byzantine leader might produce multiple proposals for the same view.
// Hence, only certified blocks (i.e. blocks that have received a QC) can be indexed!
func IndexCertifiedBlockByView(lctx lockctx.Proof, rw storage.ReaderBatchWriter, view uint64, blockID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	var existingID flow.Identifier
	err := RetrieveByKey(rw.GlobalReader(), MakePrefix(codeCertifiedBlockByView, view), &existingID)
	if err == nil {
		return fmt.Errorf("block ID already exists for view %d: %w", view, storage.ErrAlreadyExists)
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to check existing block ID for view %d: %w", view, err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeCertifiedBlockByView, view), blockID)
}

// LookupBlockHeight retrieves finalized blocks by height.
func LookupBlockHeight(r storage.Reader, height uint64, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeHeightToBlock, height), blockID)
}

// LookupCertifiedBlockByView retrieves the certified block by view. (certified blocks are blocks that have received QC)
// Returns `storage.ErrNotFound` if no certified block for the specified view is known.
func LookupCertifiedBlockByView(r storage.Reader, view uint64, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeCertifiedBlockByView, view), blockID)
}

// BlockExists checks whether the block exists in the database.
// No errors are expected during normal operation.
func BlockExists(r storage.Reader, blockID flow.Identifier) (bool, error) {
	return KeyExists(r, MakePrefix(codeHeader, blockID))
}

// IndexCollectionBlock indexes a block by a collection within that block.
func IndexCollectionBlock(w storage.Writer, collID flow.Identifier, blockID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeCollectionBlock, collID), blockID)
}

// LookupCollectionBlock looks up a block by a collection within that block.
func LookupCollectionBlock(r storage.Reader, collID flow.Identifier, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeCollectionBlock, collID), blockID)
}

// FindHeaders iterates through all headers, calling `filter` on each, and adding
// them to the `found` slice if `filter` returned true
func FindHeaders(r storage.Reader, filter func(header *flow.Header) bool, found *[]flow.Header) error {
	return TraverseByPrefix(r, MakePrefix(codeHeader), func(key []byte, getValue func(destVal any) error) (bail bool, err error) {
		var h flow.Header
		err = getValue(&h)
		if err != nil {
			return true, err
		}
		if filter(&h) {
			*found = append(*found, h)
		}
		return false, nil
	}, storage.DefaultIteratorOptions())
}
