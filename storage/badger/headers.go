// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
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

func (h *Headers) ByHash(hash crypto.Hash) (*flow.Header, error) {
	var header flow.Header
	err := h.db.View(operation.RetrieveHeader(hash, &header))
	return &header, err
}

func (h *Headers) ByNumber(number uint64) (*flow.Header, error) {

	var header flow.Header
	err := h.db.View(func(tx *badger.Txn) error {

		// get the hash by height
		var hash crypto.Hash
		err := operation.RetrieveHash(number, &hash)(tx)
		if err != nil {
			return errors.Wrap(err, "could not retrieve hash")
		}

		// get the header by hash
		err = operation.RetrieveHeader(hash, &header)(tx)
		if err != nil {
			return errors.Wrap(err, "could not retrieve header")
		}

		return nil
	})

	return &header, err
}
