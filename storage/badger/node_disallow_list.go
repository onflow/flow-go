package store

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type NodeDisallowList struct {
	db storage.DB
}

var _ storage.NodeDisallowList = (*NodeDisallowList)(nil)

func NewNodeDisallowList(db storage.DB) *NodeDisallowList {
	return &NodeDisallowList{db: db}
}

// Store writes the given disallowList to the database.
// To avoid legacy entries in the database, we purge
// the entire database entry if disallowList is empty.
// No errors are expected during normal operations.
func (dl *NodeDisallowList) Store(disallowList map[flow.Identifier]struct{}) error {
	return dl.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		if len(disallowList) == 0 {
			return operation.PurgeNodeDisallowList(rw.Writer())
		}
		return operation.PersistNodeDisallowList(rw.Writer(), disallowList)
	})
}

// Retrieve reads the set of disallowed nodes from the database.
// No error is returned if no database entry exists.
// No errors are expected during normal operations.
func (dl *NodeDisallowList) Retrieve(disallowList *map[flow.Identifier]struct{}) error {
	err := operation.RetrieveNodeDisallowList(dl.db.Reader(), disallowList)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	return nil
}
