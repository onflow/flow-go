// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertCollection(hash flow.Fingerprint, collection *flow.GuaranteedCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeCollection, hash), collection)
}

func RetrieveCollection(hash flow.Fingerprint, collection *flow.GuaranteedCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, hash), collection)
}

func RemoveCollection(hash flow.Fingerprint) func(*badger.Txn) error {
	return remove(makePrefix(codeCollection, hash))
}

func IndexCollectionByBlockHash(blockHash crypto.Hash, collection *flow.GuaranteedCollection) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := insert(makePrefix(codeBlockHashToCollections, blockHash, collection.Hash()), collection.Fingerprint())(tx)
		if err != nil {
			return fmt.Errorf("failed to index collection by block hash: %w", err)
		}
		return nil
	}
}

func RetrieveCollectionsByBlockHash(blockHash crypto.Hash, collections *[]*flow.GuaranteedCollection) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		iter := func() (checkFunc, createFunc, handleFunc) {
			check := func([]byte) bool {
				return true
			}

			var hash flow.Fingerprint
			create := func() interface{} {
				return &hash
			}

			handle := func() error {
				if hash == nil {
					return fmt.Errorf("collection hash was not decoded")
				}

				var collection flow.GuaranteedCollection
				err := RetrieveCollection(hash, &collection)(tx)
				if err != nil {
					return fmt.Errorf("failed to retrieve collection: %w", err)
				}

				*collections = append(*collections, &collection)

				return nil
			}

			return check, create, handle
		}

		return iterate(makePrefix(codeBlockHashToCollections, blockHash), nil, iter)(tx)
	}
}
