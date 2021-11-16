package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage/badger/operation"
)

type DKGState struct {
	db *badger.DB
}

func NewDKGState(db *badger.DB) (*DKGState, error) {
	return &DKGState{db: db}, nil
}

func (ds *DKGState) SetDKGStarted(epochCounter uint64) error {
	return ds.db.Update(operation.InsertDKGStartedForEpoch(epochCounter))
}

func (ds *DKGState) GetDKGStarted(epochCounter uint64) (bool, error) {
	var started bool
	err := ds.db.View(operation.RetrieveDKGStartedForEpoch(epochCounter, &started))
	return started, err
}
