package badger

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type VersionBeacons struct {
	db *badger.DB
}

var _ storage.VersionBeacons = (*VersionBeacons)(nil)

func NewVersionBeacons(db *badger.DB) *VersionBeacons {
	res := &VersionBeacons{
		db: db,
	}

	return res
}

func (r *VersionBeacons) Highest(
	belowOrEqualTo uint64,
) (*flow.SealedVersionBeacon, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()

	var beacon flow.SealedVersionBeacon

	err := operation.LookupLastVersionBeaconByHeight(belowOrEqualTo, &beacon)(tx)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &beacon, nil
}
