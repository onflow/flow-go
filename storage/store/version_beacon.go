package store

import (
	"errors"
	"math"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type VersionBeacons struct {
	db                   storage.DB
	highestVersionBeacon *flow.SealedVersionBeacon
	indexing             *sync.RWMutex
}

var _ storage.VersionBeacons = (*VersionBeacons)(nil)

func NewVersionBeacons(db storage.DB) *VersionBeacons {
	highestVersionBeacon, err := readStoredHighestVersionBeacon(db, math.MaxUint64)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		panic("failed to read highest version beacon: " + err.Error())
	}

	res := &VersionBeacons{
		db:                   db,
		highestVersionBeacon: highestVersionBeacon,
		indexing:             &sync.RWMutex{},
	}
	return res
}

// Highest finds the highest flow.SealedVersionBeacon but no higher than
// belowOrEqualTo
// Returns nil if no version beacon has been sealed below or equal to the
// input height.
func (r *VersionBeacons) Highest(
	belowOrEqualTo uint64,
) (*flow.SealedVersionBeacon, error) {
	cachedBeacon, ok := r.readCachedHighestVersionBeacon()
	if ok {
		if cachedBeacon.SealHeight <= belowOrEqualTo {
			return cachedBeacon, nil
		}
	}

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

func (r *VersionBeacons) BatchIndexVersionBeaconByHeight(
	rw storage.ReaderBatchWriter,
	beacon *flow.SealedVersionBeacon,
) error {
	// when indexing the version beacon, update the cache for the highest version beacon
	storage.OnCommitSucceed(rw, func() {
		r.indexing.Lock()
		defer r.indexing.Unlock()

		if r.highestVersionBeacon == nil || beacon.SealHeight > r.highestVersionBeacon.SealHeight {
			r.highestVersionBeacon = beacon
		}
	})

	return operation.IndexVersionBeaconByHeight(rw.Writer(), beacon)
}

func (r *VersionBeacons) readCachedHighestVersionBeacon() (*flow.SealedVersionBeacon, bool) {
	r.indexing.RLock()
	defer r.indexing.RUnlock()

	if r.highestVersionBeacon == nil {
		return nil, false
	}

	return r.highestVersionBeacon, true
}

func readStoredHighestVersionBeacon(db storage.DB, belowOrEqualTo uint64) (*flow.SealedVersionBeacon, error) {
	var beacon flow.SealedVersionBeacon
	err := operation.LookupLastVersionBeaconByHeight(db.Reader(), belowOrEqualTo, &beacon)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &beacon, nil
}
