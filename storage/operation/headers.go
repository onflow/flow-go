package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertHeader(w storage.Writer, headerID flow.Identifier, header *flow.Header) error {
	return UpsertByKey(w, MakePrefix(codeHeader, headerID), header)
}

func RetrieveHeader(r storage.Reader, blockID flow.Identifier, header *flow.Header) error {
	return RetrieveByKey(r, MakePrefix(codeHeader, blockID), header)
}

// UnsafeIndexBlockHeight indexes the height of a block. It should only be called on
// finalized blocks.
func UnsafeIndexBlockHeight(w storage.Writer, height uint64, blockID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeHeightToBlock, height), blockID)
}

// LookupBlockHeight retrieves finalized blocks by height.
func LookupBlockHeight(r storage.Reader, height uint64, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeHeightToBlock, height), blockID)
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
	return TraverseByPrefix(r, MakePrefix(codeHeader), func() (CheckFunc, CreateFunc, HandleFunc) {
		check := func(key []byte) (bool, error) {
			return true, nil
		}
		var val flow.Header
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			if filter(&val) {
				*found = append(*found, val)
			}
			return nil
		}
		return check, create, handle
	}, storage.DefaultIteratorOptions())
}
