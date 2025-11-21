package pebble

import (
	"errors"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RegisterSnapshotReader provides the default implementation of the
// RegisterSnapshotReader interface.
//
// It wraps a storage.RegisterIndexReader and adds logic to construct
// register snapshots at a given block height. The reader enforces explicit
// range checks before creating a snapshot, ensuring that queries outside of
// the indexed range return a clear storage.ErrHeightNotIndexed error.
//
// This design improves user experience by preventing confusing error
// propagation from deeper layers (such as FVM or Cadence).
type RegisterSnapshotReader struct {
	storage.RegisterIndexReader
}

var _ storage.RegisterSnapshotReader = (*RegisterSnapshotReader)(nil)

// NewRegisterSnapshotReader returns a new RegisterSnapshotReader that wraps
// the provided RegisterIndexReader.
func NewRegisterSnapshotReader(registers storage.RegisterIndexReader) *RegisterSnapshotReader {
	return &RegisterSnapshotReader{
		RegisterIndexReader: registers,
	}
}

// StorageSnapshot returns a snapshot of register values at the given block height.
//
// Expected error returns during normal operation:
//   - [storage.ErrHeightNotIndexed]: If the requested height is outside the range of indexed blocks.
func (r *RegisterSnapshotReader) StorageSnapshot(height uint64) (snapshot.StorageSnapshot, error) {
	if height < r.RegisterIndexReader.FirstHeight() {
		return nil, storage.ErrHeightNotIndexed
	}

	if height > r.RegisterIndexReader.LatestHeight() {
		return nil, storage.ErrHeightNotIndexed
	}

	return snapshot.NewReadFuncStorageSnapshot(func(registerID flow.RegisterID) (flow.RegisterValue, error) {
		value, err := r.RegisterIndexReader.Get(registerID, height)
		if err != nil {
			// FVM expects the storage snapshot to return nil for non-existent registers
			if errors.Is(err, storage.ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}

		return value, nil
	}), nil
}
