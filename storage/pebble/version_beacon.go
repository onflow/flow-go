package pebble

import (
	"errors"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type VersionBeacons struct {
	db *pebble.DB
}

var _ storage.VersionBeacons = (*VersionBeacons)(nil)

func NewVersionBeacons(db *pebble.DB) *VersionBeacons {
	res := &VersionBeacons{
		db: db,
	}

	return res
}

func (r *VersionBeacons) Highest(
	belowOrEqualTo uint64,
) (*flow.SealedVersionBeacon, error) {
	var beacon flow.SealedVersionBeacon

	err := operation.LookupLastVersionBeaconByHeight(belowOrEqualTo, &beacon)(r.db)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &beacon, nil
}
