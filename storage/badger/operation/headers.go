// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertHeader(headerID flow.Identifier, header *flow.Header) func(*badger.Txn) error {
	return insert(makePrefix(codeHeader, headerID), header)
}

func CheckHeader(blockID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeHeader, blockID), exists)
}

func RetrieveHeader(blockID flow.Identifier, header *flow.Header) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHeader, blockID), header)
}

func IndexCollectionBlock(collID flow.Identifier, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeCollectionBlock, collID), blockID)
}

func LookupCollectionBlock(collID flow.Identifier, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollectionBlock, collID), blockID)
}

func IndexBlockHeight(height uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeHeightToBlock, height), blockID)
}

func LookupBlockHeight(height uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHeightToBlock, height), blockID)
}

func InsertExecutedBlock(blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutedBlock), blockID)
}

func UpdateExecutedBlock(blockID flow.Identifier) func(*badger.Txn) error {
	return update(makePrefix(codeExecutedBlock), blockID)
}

func RetrieveExecutedBlock(blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutedBlock), blockID)
}
