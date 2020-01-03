// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertCollection(collection *flow.Collection) func(*badger.Txn) error {
	return insert(makePrefix(codeCollection, collection.Fingerprint()), collection)
}

func RetrieveCollection(hash flow.Fingerprint, collection *flow.Collection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, hash), collection)
}

func RemoveCollection(hash flow.Fingerprint) func(*badger.Txn) error {
	return remove(makePrefix(codeCollection, hash))
}

func InsertGuaranteedCollection(gc *flow.GuaranteedCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeGuaranteedCollection, gc.Fingerprint()), gc)
}

func PersistGuaranteedCollection(gc *flow.GuaranteedCollection) func(*badger.Txn) error {
	return persist(makePrefix(codeGuaranteedCollection, gc.Fingerprint()), gc)
}

func RetrieveGuaranteedCollection(hash flow.Fingerprint, gc *flow.GuaranteedCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuaranteedCollection, hash), gc)
}

func IndexGuaranteedCollectionByBlockHash(blockHash crypto.Hash, gc *flow.GuaranteedCollection) func(*badger.Txn) error {
	return persist(makePrefix(codeBlockHashToCollections, blockHash, gc.Fingerprint()), gc.Fingerprint())
}

func RetrieveGuaranteedCollectionsByBlockHash(blockHash crypto.Hash, collections *[]*flow.GuaranteedCollection) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		iter := func() (checkFunc, createFunc, handleFunc) {
			check := func(key []byte) bool {
				return true
			}

			var hash flow.Fingerprint
			create := func() interface{} {
				hash = flow.Fingerprint{}
				return &hash
			}

			handle := func() error {
				if hash == nil {
					return fmt.Errorf("collection hash was not decoded")
				}

				var collection flow.GuaranteedCollection
				err := RetrieveGuaranteedCollection(hash, &collection)(tx)
				if err != nil {
					return fmt.Errorf("failed to retrieve collection: %w", err)
				}

				*collections = append(*collections, &collection)

				return nil
			}

			return check, create, handle
		}

		return iterate(makePrefix(codeBlockHashToCollections, blockHash), nil, nil, iter)(tx)
	}
}
