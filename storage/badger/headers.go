// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Headers implements a simple read-only header storage around a badger DB.
type Headers struct {
	db *badger.DB
}

func NewHeaders(db *badger.DB) *Headers {
	h := &Headers{
		db: db,
	}
	return h
}

func (h *Headers) Store(header *flow.Header) error {
	return h.db.Update(operation.InsertHeader(header))
}

func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	var header flow.Header
	err := h.db.View(operation.RetrieveHeader(blockID, &header))
	return &header, err
}

func (h *Headers) ByNumber(number uint64) (*flow.Header, error) {

	var header flow.Header
	err := h.db.View(func(tx *badger.Txn) error {

		// get the hash by height
		var blockID flow.Identifier
		err := operation.RetrieveNumber(number, &blockID)(tx)
		if err != nil {
			return errors.Wrap(err, "could not retrieve blockID")
		}

		// get the header by hash
		err = operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return errors.Wrap(err, "could not retrieve header")
		}

		return nil
	})

	return &header, err
}
