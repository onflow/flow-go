package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertExecutionReceiptMeta inserts an execution receipt meta by ID.
func InsertExecutionReceiptMeta(receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// BatchInsertExecutionReceiptMeta inserts an execution receipt meta by ID.
func BatchInsertExecutionReceiptMeta(receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) func(batch *badger.WriteBatch) error {
	return batchInsert(makePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// RetrieveExecutionReceipt retrieves a execution receipt meta by ID.
func RetrieveExecutionReceiptMeta(receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// IndexOwnExecutionReceipt inserts an execution receipt ID keyed by block ID
func IndexOwnExecutionReceipt(blockID flow.Identifier, receiptID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// BatchIndexOwnExecutionReceipt inserts an execution receipt ID keyed by block ID into a batch
func BatchIndexOwnExecutionReceipt(blockID flow.Identifier, receiptID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchInsert(makePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// LookupOwnExecutionReceipt finds execution receipt ID by block
func LookupOwnExecutionReceipt(blockID flow.Identifier, receiptID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// IndexExecutionReceipts inserts an execution receipt ID keyed by block ID and receipt ID.
// one block could have multiple receipts, even if they are from the same executor
func IndexExecutionReceipts(blockID, receiptID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeAllBlockReceipts, blockID, receiptID), receiptID)
}

// BatchIndexExecutionReceipts inserts an execution receipt ID keyed by block ID and receipt ID into a batch
func BatchIndexExecutionReceipts(blockID, receiptID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchInsert(makePrefix(codeAllBlockReceipts, blockID, receiptID), receiptID)
}

// LookupExecutionReceipts finds all execution receipts by block ID
func LookupExecutionReceipts(blockID flow.Identifier, receiptIDs *[]flow.Identifier) func(*badger.Txn) error {
	iterationFunc := receiptIterationFunc(receiptIDs)
	return traverse(makePrefix(codeAllBlockReceipts, blockID), iterationFunc)
}

// receiptIterationFunc returns an in iteration function which returns all receipt IDs found during traversal
func receiptIterationFunc(receiptIDs *[]flow.Identifier) func() (checkFunc, createFunc, handleFunc) {
	check := func(key []byte) bool {
		return true
	}

	var receiptID flow.Identifier
	create := func() interface{} {
		return &receiptID
	}
	handle := func() error {
		*receiptIDs = append(*receiptIDs, receiptID)
		return nil
	}
	return func() (checkFunc, createFunc, handleFunc) {
		return check, create, handle
	}
}

func RemoveExecutionReceipt(blockID flow.Identifier, receipt *flow.ExecutionReceipt) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		receiptID := receipt.ID()
		err := remove(makePrefix(codeExecutionReceiptMeta, receiptID))(txn)
		if err != nil {
			return err
		}

		return remove(makePrefix(codeExecutionResult, blockID))(txn)
	}
}
