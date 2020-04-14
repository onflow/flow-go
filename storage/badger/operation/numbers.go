// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertNumber(number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeFinalizedBlockNumber, number), blockID)
}

func RetrieveNumber(number uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeFinalizedBlockNumber, number), blockID)
}

type HighestExecutedBlock struct {
	Number  uint64
	BlockID flow.Identifier
}

func InsertHighestExecutedBlockNumber(number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeHighestExecutedBlockNumber), HighestExecutedBlock{number, blockID})
}

func UpdateHighestExecutedBlockNumber(number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return update(makePrefix(codeHighestExecutedBlockNumber), HighestExecutedBlock{number, blockID})
}

func RetrieveHighestExecutedBlockNumber(number *uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		var highestExecutedBlock HighestExecutedBlock
		err := retrieve(makePrefix(codeHighestExecutedBlockNumber), &highestExecutedBlock)(txn)
		if err != nil {
			return err
		}
		*number = highestExecutedBlock.Number
		*blockID = highestExecutedBlock.BlockID
		return nil
	}
}
