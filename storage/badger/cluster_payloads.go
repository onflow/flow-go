package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// ClusterPayloads implements storage of block payloads for collection node
// cluster consensus.
type ClusterPayloads struct {
	db *badger.DB
}

func NewClusterPayloads(db *badger.DB) *ClusterPayloads {
	cp := &ClusterPayloads{db: db}
	return cp
}

func (cp *ClusterPayloads) Store(header *flow.Header, payload *cluster.Payload) error {
	return operation.RetryOnConflict(cp.db.Update, func(tx *badger.Txn) error {

		if header.PayloadHash != payload.Hash() {
			return fmt.Errorf("payload integrity check failed")
		}

		// insert the payload, allow duplicates because it is valid for two
		// identical payloads on competing forks to co-exist.
		err := procedure.InsertClusterPayload(header, payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert cluster payload: %w", err)
		}

		// index the payload by the block containing it
		err = procedure.IndexClusterPayload(header, payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index cluster payload: %w", err)
		}

		return nil
	})
}

func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	var payload cluster.Payload
	err := cp.db.View(func(tx *badger.Txn) error {
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not get header: %w", err)
		}

		err = procedure.RetrieveClusterPayload(blockID, &payload)(tx)
		if err != nil {
			return fmt.Errorf("could not get payload: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &payload, nil
}
