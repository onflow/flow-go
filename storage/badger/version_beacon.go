package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type VersionBeacons struct {
	db *badger.DB
}

var _ storage.VersionBeacons = (*VersionBeacons)(nil)

func NewVersionBeacons(collector module.CacheMetrics, db *badger.DB) *VersionBeacons {

	res := &VersionBeacons{
		db: db,
	}

	return res
}

// Highest finds the highest flow.VersionBeacon but no higher than maxHeight.
// Returns storage.ErrNotFound if version beacon exists at or below the given height.
func (r *VersionBeacons) Highest(maxHeight uint64) (*flow.VersionBeacon, uint64, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()

	var beacon flow.VersionBeacon
	var height uint64

	err := operation.LookupLastVersionBeaconByHeight(maxHeight, &beacon, &height)(tx)
	if err != nil {
		return nil, 0, err
	}
	return &beacon, height, nil
}
