package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertExecutionReceiptMeta inserts an execution receipt meta by ID.
func InsertExecutionReceiptMeta(receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// RetrieveExecutionReceipt retrieves a execution receipt meta by ID.
func RetrieveExecutionReceiptMeta(receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// IndexExecutionReceipt inserts an execution receipt ID keyed by block ID
func IndexExecutionReceipt(blockID flow.Identifier, receiptID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockExecutionReceipt, blockID), receiptID)
}

// LookupExecutionReceipt finds execution receipt ID by block
func LookupExecutionReceipt(blockID flow.Identifier, receiptID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockExecutionReceipt, blockID), receiptID)
}

func RemoveExecutionReceipt(blockID flow.Identifier, receipt *flow.ExecutionReceipt) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		receiptID := receipt.ID()
		err := remove(makePrefix(codeExecutionReceiptMeta, receiptID))(txn)
		if err != nil {
			return err
		}

		return remove(makePrefix(codeBlockExecutionReceipt, blockID))(txn)
	}
}
