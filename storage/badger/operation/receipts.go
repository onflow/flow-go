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
// TODO: rename to BatchUpdate
func BatchInsertExecutionReceiptMeta(receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// RetrieveExecutionReceiptMeta retrieves a execution receipt meta by ID.
func RetrieveExecutionReceiptMeta(receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// IndexOwnExecutionReceipt inserts an execution receipt ID keyed by block ID
func IndexOwnExecutionReceipt(blockID flow.Identifier, receiptID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// BatchIndexOwnExecutionReceipt inserts an execution receipt ID keyed by block ID into a batch
// TODO: rename to BatchUpdate
func BatchIndexOwnExecutionReceipt(blockID flow.Identifier, receiptID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// LookupOwnExecutionReceipt finds execution receipt ID by block
func LookupOwnExecutionReceipt(blockID flow.Identifier, receiptID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// RemoveOwnExecutionReceipt removes own execution receipt index by blockID
func RemoveOwnExecutionReceipt(blockID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeOwnBlockReceipt, blockID))
}

// BatchRemoveOwnExecutionReceipt removes blockID-to-my-receiptID index entries keyed by a blockID in a provided batch.
// No errors are expected during normal operation, but it may return generic error
// if badger fails to process request
func BatchRemoveOwnExecutionReceipt(blockID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchRemove(makePrefix(codeOwnBlockReceipt, blockID))
}

// IndexExecutionReceipts inserts an execution receipt ID keyed by block ID and receipt ID.
// one block could have multiple receipts, even if they are from the same executor
func IndexExecutionReceipts(blockID, receiptID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeAllBlockReceipts, blockID, receiptID), receiptID)
}

// BatchIndexExecutionReceipts inserts an execution receipt ID keyed by block ID and receipt ID into a batch
func BatchIndexExecutionReceipts(blockID, receiptID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeAllBlockReceipts, blockID, receiptID), receiptID)
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
