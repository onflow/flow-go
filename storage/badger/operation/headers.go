// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertHeader(headerID flow.Identifier, header *flow.Header) func(*badger.Txn) error {
	return insert(makePrefix(codeHeader, headerID), header)
}

func RetrieveHeader(blockID flow.Identifier, header *flow.Header) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHeader, blockID), header)
}

func IndexBlockHeight(height uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeHeightToBlock, height), blockID)
}

func LookupBlockHeight(height uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHeightToBlock, height), blockID)
}

// InsertBlockValidity marks a block as valid or invalid, defined by the consensus algorithm.
func InsertBlockValidity(blockID flow.Identifier, valid bool) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockValidity, blockID), valid)
}

// RetrieveBlockValidity returns a block's validity wrt the consensus algorithm.
func RetrieveBlockValidity(blockID flow.Identifier, valid *bool) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockValidity, blockID), valid)
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

// IndexCollectionBlock indexes a block by a collection within that block.
func IndexCollectionBlock(collID flow.Identifier, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeCollectionBlock, collID), blockID)
}

// LookupCollectionBlock looks up a block by a collection within that block.
func LookupCollectionBlock(collID flow.Identifier, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollectionBlock, collID), blockID)
}

// FindHeaders iterates through all headers, calling `filter` on each, and adding
// them to the `found` slice if `filter` returned true
func FindHeaders(filter func(header *flow.Header) bool, found *[]flow.Header) func(*badger.Txn) error {
	return traverse(makePrefix(codeHeader), func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			return true
		}
		var val flow.Header
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			if filter(&val) {
				*found = append(*found, val)
			}
			return nil
		}
		return check, create, handle
	})
}
