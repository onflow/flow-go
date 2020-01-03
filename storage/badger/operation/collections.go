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

func PersistCollection(collection *flow.Collection) func(*badger.Txn) error {
	return persist(makePrefix(codeCollection, collection.Fingerprint()), collection)
}

func RetrieveCollection(hash flow.Fingerprint, collection *flow.Collection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, hash), collection)
}

func RemoveCollection(hash flow.Fingerprint) func(*badger.Txn) error {
	return remove(makePrefix(codeCollection, hash))
}

func InsertCollectionGuarantee(gc *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeCollectionGuarantee, gc.Fingerprint()), gc)
}

func PersistCollectionGuarantee(gc *flow.CollectionGuarantee) func(*badger.Txn) error {
	return persist(makePrefix(codeCollectionGuarantee, gc.Fingerprint()), gc)
}

func RetrieveCollectionGuarantee(hash flow.Fingerprint, gc *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollectionGuarantee, hash), gc)
}

func IndexCollectionGuaranteeByBlockHash(blockHash crypto.Hash, gc *flow.CollectionGuarantee) func(*badger.Txn) error {
	return persist(makePrefix(codeBlockHashToCollections, blockHash, gc.Fingerprint()), gc.Fingerprint())
}

func RetrieveCollectionGuaranteesByBlockHash(blockHash crypto.Hash, collections *[]*flow.CollectionGuarantee) func(*badger.Txn) error {
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

				var collection flow.CollectionGuarantee
				err := RetrieveCollectionGuarantee(hash, &collection)(tx)
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
