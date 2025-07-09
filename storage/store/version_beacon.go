package store

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type VersionBeacons struct {
	db storage.DB
}

var _ storage.VersionBeacons = (*VersionBeacons)(nil)

func NewVersionBeacons(db storage.DB) *VersionBeacons {
	res := &VersionBeacons{
		db: db,
	}

	return res
}

func (r *VersionBeacons) Highest(
	belowOrEqualTo uint64,
) (*flow.SealedVersionBeacon, error) {
	var beacon flow.SealedVersionBeacon

	err := operation.LookupLastVersionBeaconByHeight(r.db.Reader(), belowOrEqualTo, &beacon)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &beacon, nil
}
