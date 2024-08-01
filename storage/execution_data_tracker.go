package storage

import (
	"encoding/binary"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/module/blobs"
)

// key prefixes
const (
	PrefixGlobalState  byte = iota + 1 // global state variables
	PrefixLatestHeight                 // tracks, for each blob, the latest height at which there exists a block whose execution data contains the blob
	PrefixBlobRecord                   // tracks the set of blobs at each height
)

const (
	GlobalStateFulfilledHeight byte = iota + 1 // latest fulfilled block height
	GlobalStatePrunedHeight                    // latest pruned block height
)

const BlobRecordKeyLength = 1 + 8 + blobs.CidLength
const LatestHeightKeyLength = 1 + blobs.CidLength

const CidsPerBatch = 16         // number of cids to track per batch
const DeleteItemsPerBatch = 256 // number of items to delete per batch

// ParseBlobRecordKey parses a blob record key and returns the block height and CID.
func ParseBlobRecordKey(key []byte) (uint64, cid.Cid, error) {
	blockHeight := binary.BigEndian.Uint64(key[1:])
	c, err := cid.Cast(key[1+8:])
	return blockHeight, c, err
}

// TrackBlobsFn is passed to the UpdateFn provided to ExecutionDataTracker.Update,
// and can be called to track a list of cids at a given block height.
// It returns an error if the update failed.
type TrackBlobsFn func(blockHeight uint64, cids ...cid.Cid) error

// UpdateFn is implemented by the user and passed to ExecutionDataTracker.Update,
// which ensures that it will never be run concurrently with any call
// to ExecutionDataTracker.Prune.
// Any returned error will be returned from the surrounding call to ExecutionDataTracker.Update.
// The function must never make any calls to the ExecutionDataTracker interface itself,
// and should instead only modify the storage via the provided TrackBlobsFn.
type UpdateFn func(TrackBlobsFn) error

// PruneCallback is a function which can be provided by the user which
// is called for each CID when the last height at which that CID appears
// is pruned.
// Any returned error will be returned from the surrounding call to ExecutionDataTracker.Prune.
// The prune callback can be used to delete the corresponding
// blob data from the blob store.
type PruneCallback func(cid.Cid) error

type ExecutionDataTracker interface {
	// Update is used to track new blob CIDs.
	// It can be used to track blobs for both sealed and unsealed
	// heights, and the same blob may be added multiple times for
	// different heights.
	// The same blob may also be added multiple times for the same
	// height, but it will only be tracked once per height.
	Update(UpdateFn) error

	// GetFulfilledHeight returns the current fulfilled height.
	//
	// No errors are expected during normal operation.
	GetFulfilledHeight() (uint64, error)

	// SetFulfilledHeight updates the fulfilled height value,
	// which is the lowest from the highest block heights `h` such that all
	// heights <= `h` are sealed and the sealed execution data
	// has been downloaded or indexed.
	// It is up to the caller to ensure that this is never
	// called with a value lower than the pruned height.
	//
	// No errors are expected during normal operation
	SetFulfilledHeight(height uint64) error

	// GetPrunedHeight returns the current pruned height.
	//
	// No errors are expected during normal operation.
	GetPrunedHeight() (uint64, error)

	// PruneUpToHeight removes all data from storage corresponding
	// to block heights up to and including the given height,
	// and updates the latest pruned height value.
	// It locks the ExecutionDataTracker and ensures that no other writes
	// can occur during the pruning.
	// It is up to the caller to ensure that this is never
	// called with a value higher than the fulfilled height.
	PruneUpToHeight(height uint64) error
}

// DeleteInfo contains information for a deletion operation.
type DeleteInfo struct {
	Cid                      cid.Cid
	Height                   uint64
	DeleteLatestHeightRecord bool
}
